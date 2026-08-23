# vrclog-go データ整合性強化 指示書兼実装仕様書

## 0. この文書の扱い

この文書は `github.com/vrclog/vrclog-go` の現行刷新後アーキテクチャを完成させるための最上位実装仕様である。

現在の基本設計は維持する。

```text
output_log
    ↓
ReadFile / Follow
    ↓
Record
    ↓
Engine
    ↓
Adapter
    ↓
canonical Event
    ↓
Observation / Diagnostic
```

旧Parser、ParserChain、YAML RegexParser、runtime plugin、WASM、任意event mapは復活させない。
`AdapterEvent`はこの不変条件に対する明示的かつ唯一の例外であり、外部Adapter向けにsizeとshapeを制限したextension envelopeとしてのみ許可する。

互換性維持は不要である。公開APIを変更してよい。ただし、APIを増やす場合は今回の不変条件を実現する最小限に絞る。

## 1. リポジトリ境界

### このリポジトリの責務

- VRChat `output_log` の検出
- 有限ファイルの読み取り
- ライブ追従、rotation、cursor再開
- 物理ログRecordの生成
- canonical Event型
- Adapter API
- Engineによる全Adapter実行
- Observation / Diagnostic生成
- VRChatクライアント自身のbuilt-in Adapter
- Companionがcatch-up判定に使うログ位置snapshot

### このリポジトリの責務ではないもの

- YamaPlayer、iwaSync3等のcommunity固有ログ
- SQLite
- Current World / Players / Media Attempt
- semantic correlation
- HTTP API、SSE、UI
- Discord通知
- URLへのHTTPアクセス
- title、thumbnail、duration等の外部取得
- runtime pattern pack

依存方向は次のとおりである。

```text
vrclog-go ← vrclog-adapters ← vrclog-companion
```

## 2. 今回解決する問題

現行実装には、少なくとも次の整合性問題がある。

1. `Follow` が一時的なEOF時の未改行fragmentを完成済みRecordとしてemitし、offsetを進める。
2. `Follow` が一部の`stat`、directory listing、file open、SourceID取得エラーを無視または別のsentinelへ潰す。
3. Observation IDがAdapterのemission indexに依存し、emission順序変更で同一IDが別eventへ割り当たり得る。
4. 同一Record・同一Adapterから同じRule IDを複数emitすることを拒否していない。
5. canonical Eventのnested payload validationが不足している。
6. built-in Adapterのprefix/regexが完全にanchorされておらず、埋め込まれたログ断片を誤認し得る。
7. Companionが「source開始前から存在したbytes」と「source開始後に追記されたbytes」を区別するための位置snapshot APIがない。

## 3. 絶対に維持する不変条件

### 3.1 Record境界

- `ReadFile`では、EOFに達した未改行の最終fragmentを1 Recordとしてflushしてよい。
- `Follow`では、一時的なEOFに達しただけでは未改行fragmentをemitしてはならない。
- `Follow`は未完成fragmentの先へcursor、offset、line numberを進めてはならない。
- 同じ完成済み物理行を`ReadFile`と`Follow`で読んだ場合、Record ID、offset、next offset、raw hashは一致する。
- context cancellation時に未完成fragmentをflushしない。
- rotationで旧ファイルが確実にsettleした後だけ、その旧ファイルの未改行最終fragmentを1回だけflushしてよい。

### 3.2 Observation同一性

Observation IDは次の意味論で固定する。

```text
observation_id = SHA-256(
    record_id + NUL +
    adapter_id + NUL +
    rule_id
)
```

emission index、event kind、payload、Adapter version、timestampをIDへ含めない。

- emission順序を変えても同じRuleのObservation IDは変わらない。
- 1 Adapterが1 Recordに対して同じRule IDを複数回返すことは禁止する。
- Rule IDは「このRecordからこの意味のEventを抽出する規則」を安定して識別する。
- 1つの規則から意味の異なる複数Eventを出す必要がある場合、意味ごとに別Rule IDを使う。

### 3.3 エラー

- 一時的でないfilesystem errorを「新しい行がない」に変換しない。
- エラーを握り潰して別ファイルへ進まない。
- source truncation、source missing、permission、directory listing失敗を区別可能にする。
- Coreはretry policyを持たない。`Follow`はerrorを返して終了し、retryはCompanion側が担当する。

## 4. 実装仕様

## 4.1 有限読み取りとライブ読み取りのEOF policyを分離する

`lineReader.next()`をそのまま両用途で使い、EOF時に常にflushする構造をやめる。

実装方式は自由だが、概念上は次の2 policyを持つこと。

```go
// 概念例。名前は実装に合わせてよい。
type eofPolicy uint8

const (
    eofFlushFinalFragment eofPolicy = iota // ReadFile / settled old file
    eofKeepPartialFragment                 // active Follow source
)
```

### `ReadFile`

- 改行終端行を通常どおりemitする。
- EOF時にfragmentがあれば最終Recordとしてemitする。
- 最終Recordのhashには存在するraw bytesだけを含める。

### `Follow`のactive file

- 改行が見つかった行だけemitする。
- EOF時に未改行fragmentが存在しても、現在のcommitted offset/lineを変えない。
- 次pollではそのfragmentの開始offsetから再読してよい。persistent bufferを持ってもよい。
- fragmentが後続bytesと改行によって完成した時点で、1 Recordだけemitする。
- fragmentの途中状態をDiagnosticにしない。

### rotation

rotation settle後の扱いを明示的に分ける。

1. 旧current fileをsettled sourceとして最後に1回読む。
2. 旧fileの未改行fragmentがあれば、この時点でのみflushする。
3. 複数の新fileが存在する場合、中間fileはfinite fileとして読む。
4. 最後の最新fileはactive fileとして読み、未改行fragmentをflushしない。

初回起動時のlatest fileもactive fileとして扱う。

cursorから再開したfileがlatest fileならactive policy、より新しいfileが既に存在する歴史fileならsettled policyを使う。

### oversized line

- 既存の最大行長制限を維持する。
- 未完成中はRecordも`line_too_long` Diagnosticもemitしない。
- 改行またはsettled final EOFによって物理行が完成した時点で、1件のRecordIssueとしてemitする。
- memory使用量は上限内に抑える。

## 4.2 Follow state machineのエラー処理を修正する

以下のようなerror discardをすべて削除する。

```go
grown, _ := hasFileGrown(...)
```

`findNewerFiles`、`readEntireFile`等もerrorを返すAPIへ変更する。

概念例：

```go
func (s *followState) findNewerFiles() ([]logfile.LogFileInfo, error)
func readEntireFile(...) (offset int64, line uint64, error)
```

### cursor開始時

`ErrCursorSourceMissing`へ変換してよいのは以下だけである。

- cursor pathが実際に存在しない
- cursorに記録したSourceIDと現在のSourceIDが一致しない

以下は元のerrorをwrapして返す。

- `filepath.Abs`失敗
- permission denied
- regular fileでない
- SourceID計算中のI/O error
- file open失敗（not-exist以外）
- seek失敗
- directory listing失敗

### truncation

新しいsentinelを追加する。

```go
var ErrSourceTruncated = errors.New("log source truncated")
```

active sourceのsizeがcommitted offsetより小さくなった場合：

- `ErrSourceTruncated`をwrapしたerrorを返す。
- cursorを巻き戻さない。
- 同じpathの先頭から黙って再開しない。

### current sourceの消失

active sourceがfollow中に消えた場合、data lossを隠して新fileへ黙って進まない。

- filesystem errorを返して`Follow`を終了する。
- Companionが保存済みcursorから再構築する。
- 「新fileが存在するから成功」とは扱わない。

### newer file

- directory listing失敗を「新fileなし」にしない。
- 新file open/SourceID取得失敗を成功としてskipしない。
- source切替は、それまでのRecordが正常にyieldされた後だけ行う。

## 4.3 Observation IDをemission順序から独立させる

`generateObservationID`から`emissionIndex`を削除する。

```go
func generateObservationID(
    recordID RecordID,
    adapterID AdapterID,
    ruleID RuleID,
) ObservationID
```

`Engine.Process`は、Adapterごとに全EmissionのRule IDを事前検査する。

### duplicate Rule IDの扱い

新しいDiagnostic codeを追加する。

```go
DiagnosticDuplicateRuleID DiagnosticCode = "duplicate_rule_id"
```

同一Adapterの1 Decode結果内で同じRule IDが複数回現れた場合：

- その重複Rule IDに属する全Emissionをskipする。
- そのRule IDについて1 Diagnosticを生成する。
- 同じDecode結果内の一意なRule IDのEmissionは処理してよい。
- 「最初だけ採用」「最後だけ採用」は禁止する。
- 出力順序変更で結果が変わらないこと。

空Rule、nil Event、invalid Eventの既存Diagnostic方針は維持する。

### テスト

- 同じEmission集合を順序だけ変えても、RuleごとのObservation IDが同じ。
- Event payload変更でもIDは同じであり、下流Storeがcontent conflictを検出できる。
- duplicate Rule IDはどちらもObservationにならない。
- duplicate以外のRuleは維持される。
- Adapter登録順はObservation列の表示順には影響してよいが、各IDには影響しない。

## 4.4 canonical Event validationを完成させる

ValidationはAdapterと下流の信頼境界である。nested valueも必ず検査する。

### RemoteResource

`validateRemoteResource`は最低限以下を保証する。

- URLは空でない。
- UTF-8 byte長が16 KiB以下。
- control characterと空白を含まない。
- `url.Parse`可能なabsolute URL。
- schemeは現在のサポート範囲では`http`または`https`。
- hostが空でない。
- userinfoを持たない。
- Kind、Roleが定義済みenum。

VRChat built-in Adapterとcommunity Adapterが使える共通validation/helperへURL検査を集約し、同じロジックを複製しない。

### MediaTarget

`Target != nil`の場合：

- `Component`は必須。
- Componentは128 bytes以下。
- Keyは任意だが256 bytes以下。
- control characterを拒否する。
- Backendは`unknown`、`avpro`、`unity`のいずれか。空文字は禁止し、未特定なら明示的に`unknown`を設定する。

共通の`validateMediaTarget`を作る。

### Resource events

- `ResourceURLObserved.validate()`はResourceとTargetの両方を検査する。
- `ResourceResolved.validate()`はInput、Output、Targetを検査する。

### MediaErrorObserved

- Stageは定義済みenum。
- CodeまたはMessageの少なくとも一方が必要。
- Codeは128 bytes以下、control characterなし。
- Messageは4096 bytes以下、改行/control characterなし。tabを許可するかは統一して明文化する。
- ResourceがあればRemoteResource validation。
- TargetがあればMediaTarget validation。

### built-in Adapterのerror text

VRChat本体のerror messageをcanonical Eventへ入れる前に：

- 前後空白を除去する。
- 改行/control characterを安全な空白へ正規化する。
- 4096 bytesへUTF-8安全にtruncateする。
- HTTP(S) URLをmessageへそのまま残さない。URLを別Resourceとして確実に抽出できない場合は`<url>`へredactする。

外部アクセスは行わない。

## 4.5 built-in VRChat Adapterを完全anchorする

既知prefixがmessage途中に埋め込まれているだけの行をmatchしてはならない。

- precheckは`strings.Contains`ではなく`strings.HasPrefix`または等価な完全context判定を使う。
- regexは`^...$`でanchorする。
- AVPro Openingもmessage先頭からのみ認識する。
- exclusion substringによる後付け除外へ依存しすぎない。

最低限、以下のnegative testを追加する。

```text
[SomeUdon] copied: [Video Playback] Attempting to resolve URL 'https://...'
foo [AVProVideo] Opening https://...
Debug: [Behaviour] OnPlayerJoined Alice
```

これらからEventを生成してはならない。

正規のVRChatログfixtureは従来どおり認識する。

すべてのbuilt-in MediaTargetでBackendを明示する。

```text
VRChat resolver: unknown
AVPro: avpro
Unity Video: unity
```

## 4.6 LogSnapshot APIを追加する

Companionが「source開始前から存在したbytes」と「開始後の追記」をtimestamp推測なしで区別できるように、immutableなsnapshot APIを公開する。

推奨公開契約：

```go
type LogSnapshot struct {
    // fieldsは非公開でよい
}

// CaptureLogSnapshot captures the byte head of every currently existing
// VRChat output_log file in the selected directory.
// Empty directory argument uses DefaultLogDirectory.
// No log files is a valid empty snapshot, not an error.
func CaptureLogSnapshot(directory string) (LogSnapshot, error)

// Contains reports whether all bytes of record already existed when the
// snapshot was captured.
func (s LogSnapshot) Contains(record Record) bool
```

正確な名前はより良いものへ変更してよいが、意味論は固定する。

### snapshot内容

各fileについて最低限次を内部保持する。

```text
SourceID → capture時点のfile size
```

`Contains(record)`は次の場合だけtrue。

```text
snapshotにrecord.SourceIDが存在する
AND
record.NextOffset <= captured size
```

### 必要な性質

- snapshotはcapture後immutableで、並行read可能。
- 新しく作られたfileのRecordはfalse。
- 同じfileでもcapture後に追記された範囲のRecordはfalse。
- capture時に未完成だった行が後で改行され、その`NextOffset`がcapture sizeを超えた場合false。
- fileが同じpathで置換されSourceIDが変わった場合false。
- directory listing、stat、SourceID取得失敗はerrorとして返す。
- no log filesはempty snapshot + nil error。
- `Follow`と同じdirectory normalization、file selection、SourceID計算を再利用する。

このAPIはcatch-upというアプリ概念を持たない。「capture時点でbytesが存在したか」だけを表す。

## 5. API削減・禁止事項

今回、次を追加してはならない。

- Parser、ParserChain、RegexParser
- runtime Adapter loading
- YAML pattern
- WASM
- plugin registry download
- SQLiteやProjector依存
- community project prefix
- catch-up/live enum
- URL metadata fetch
- retry supervisor

既存のシンプルなAdapter/Engine契約を維持する。

## 6. 推奨実装フェーズ

### Phase 1: 再現テスト

先に以下の失敗テストを追加する。

- active Followへ半行だけwrite → Record 0件
- 後半+改行write → Record 1件
- cancellation → fragment flushなし
- rotation後だけ旧file fragmentを1件flush
- `stat`、list、open errorが返る
- truncationが`ErrSourceTruncated`
- Observation IDがorder-independent
- duplicate Rule ID
- embedded prefix negative test
- nested validation

### Phase 2: Observation identity

ID公式とduplicate Rule IDを実装する。

### Phase 3: Follow framing

finite/live EOF policyとrotation state machineを実装する。

### Phase 4: filesystem errors

error propagationとtruncationを実装する。

### Phase 5: LogSnapshot

APIとテストを実装する。

### Phase 6: validation / built-in Adapter

nested validation、text sanitize、full anchoringを実装する。

### Phase 7: 文書・整理

- READMEのID公式を更新
- API docs更新
- stale comment削除
- `go mod tidy`
- CHANGELOGがある場合はbreaking changeを記載

## 7. 必須テストケース

### Follow

1. finite fileの未改行最終行はemitされる。
2. live active fileの未改行fragmentはemitされない。
3. fragment完成後に1回だけemitされる。
4. fragment途中で複数pollしても重複しない。
5. cursor resumeでも同じ結果。
6. rotation settle後の旧file未改行fragmentは1回だけemit。
7. newest active fileのfragmentはflushしない。
8. oversized incomplete lineは完成までemitしない。
9. current file truncationはfatal error。
10. directory permission/list errorは伝播。
11. newer file open errorは伝播。

### Observation

1. emission reorderでID不変。
2. payload変更でID不変。
3. record/adapter/ruleのいずれかが変わるとID変更。
4. duplicate Rule IDを拒否。
5. deterministic JSON encode/decode。

### Validation

1. URL userinfo拒否。
2. relative URL拒否。
3. whitespace/control拒否。
4. oversized URL拒否。
5. Target component空拒否。
6. Target backend空/未知値拒否。
7. nested Resource/Target不正をEvent validationで拒否。
8. error text上限。

### LogSnapshot

1. capture前のRecordはtrue。
2. capture後追記Recordはfalse。
3. 新fileはfalse。
4. capture時partial、後で完成したRecordはfalse。
5. source置換はfalse。
6. empty directoryはempty snapshot。

## 8. 受け入れコマンド

少なくとも以下をすべて成功させる。

```bash
gofmt -w .
go vet ./...
go test -count=1 ./...
go test -race -count=1 ./...
go test -run 'TestFollow|TestObservation|TestLogSnapshot' -count=20 ./...
go mod tidy
git diff --exit-code -- go.mod go.sum
```

最後の`git diff`は、`go mod tidy`後に依存差分を意図的に確認・stageした後で実行すること。CI定義も現行のWindows/Linux/race coverageを維持する。

## 9. 完了条件

- active Followが未完成行を一切emitしない。
- filesystem failureを正常EOF扱いしない。
- Observation IDがemission順序から独立している。
- duplicate Rule IDを決定的に拒否する。
- canonical Eventがnested payloadまでvalidationする。
- built-in Adapterが埋め込みprefixを誤認しない。
- Companionが使用できるLogSnapshot APIが存在する。
- production codeにcommunity固有処理がない。
- 全テスト・race testが成功する。

## 10. 下流への完了報告

実装完了時、`vrclog-adapters` と `vrclog-companion` の担当へ以下を明記する。

```text
- 新しいObservation ID公式
- duplicate Rule IDの挙動
- Event validationで新たに必須となった値
- LogSnapshot APIの実名と使用例
- ErrSourceTruncated等の新しいerror
- 参照すべきcommit SHA/version
```

明示的な指示なしにcommit、push、tag、releaseは行わない。
