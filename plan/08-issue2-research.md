# Issue #2: 技術調査結果

## 調査したプロダクト・技術

### ログ/データパイプライン系

| プロダクト | プラグイン方式 | 特徴 | 参考URL |
|-----------|--------------|------|---------|
| Fluent Bit | C + Wasm | Input/Filter/Output分離、軽量 | [docs](https://docs.fluentbit.io/manual/development/wasm-filter-plugins) |
| Telegraf | Go静的リンク | Registry+Factory、型安全 | |
| Vector | VRL（独自DSL） | stateless、progressive type safety | [VRL design](https://github.com/vectordotdev/vrl/blob/main/DESIGN.md) |
| Loki Promtail | Pipeline Stages | YAMLベース、regex/json/template | |

### Wasmフレームワーク

| ライブラリ | ランタイム | 特徴 | 参考URL |
|-----------|----------|------|---------|
| **wazero** | 純Go | Go 1.21+、ゼロ依存、Arcjet本番使用 | [wazero.io](https://wazero.io/) |
| Extism | wazero | PDK充実、16+言語対応 | [extism.org](https://extism.org/) |
| waPC | wazero | bidirectional RPC、MessagePack | [wapc.io](https://wapc.io/) |
| knqyf263/go-plugin | wazero | Proto定義、**Go 1.24+必須** | [GitHub](https://github.com/knqyf263/go-plugin) |

### 式言語・ルールエンジン

| ライブラリ | 用途 | 採用実績 | 参考URL |
|-----------|-----|---------|---------|
| **Expr** | 式評価 | Google, Uber, ByteDance | [expr-lang.org](https://expr-lang.org/) |
| CEL | 式評価 | Google Cloud Platform | [cel.dev](https://cel.dev/) |
| Starlark | 設定DSL | Bazel | [starlark-go](https://github.com/google/starlark-go) |
| OPA/Rego | ポリシー | Kubernetes admission | [openpolicyagent.org](https://www.openpolicyagent.org/) |

### Go Plugin系

| プロダクト | 方式 | 特徴 | 参考URL |
|-----------|-----|------|---------|
| HashiCorp go-plugin | gRPC/RPC | プロセス分離、Terraform/Vault採用 | [GitHub](https://github.com/hashicorp/go-plugin) |
| Grafana Plugin SDK | go-plugin + gRPC | frontend/backend分離 | [docs](https://grafana.com/developers/plugin-tools/) |
| Caddy modules | 静的リンク | xcaddyでビルド | [docs](https://caddyserver.com/docs/extending-caddy) |
| Traefik/Yaegi | Go interpreter | 10%未満のオーバーヘッド | [GitHub](https://github.com/traefik/yaegi) |

### ログフィルタツール

| ツール | 特徴 | 参考URL |
|-------|------|---------|
| stern | K8sログビューア、正規表現フィルタ | [GitHub](https://github.com/stern/stern) |
| jq | JSON処理、強力なクエリ言語 | [jqlang.org](https://jqlang.org/) |
| LogQL | Loki用クエリ言語、PromQL風 | [Grafana docs](https://grafana.com/docs/loki/latest/query/) |

---

## 調査から得た知見

### Arcjet（wazero本番使用）
- 起動時にWasm事前コンパイル（AOT）で性能向上
- p50=10ms、p99=30msを達成
- Host Functionで機能拡張可能
- 参考: [Lessons from running WebAssembly in production](https://blog.arcjet.com/lessons-from-running-webassembly-in-production-with-go-wazero/)

### TinyGoの制限
- `encoding/json`がパニック（手動パース推奨）
- `regexp`が大きくなる、go-re2は最近TinyGo非対応
- **対策: 正規表現はHost側で実行し、Wasmにはマッチ結果のみ渡す**

### VRL設計思想
- 「Luaのような完全言語と静的変換の中間」
- stateless、イベント単位処理
- 意図的にループ・クラス・カスタム関数を省略

### Wasm セキュリティ
- Capability-based security: デフォルトでホストリソースアクセス不可
- メモリ安全: 境界チェックでバッファオーバーフロー防止
- whitelistアプローチ: 明示的に許可されたものだけアクセス可能
- 参考: [WebAssembly Security](https://webassembly.org/docs/security/)

### Unix哲学
- "do one thing, and do it well"
- "small tools that do one thing well, combined into powerful pipelines"
- "text streams as a universal interface"

---

## Parser Interface設計の調査

### 戻り値の型
- `[]Event`（値スライス）を推奨
- イベントが大きくなく、共有・ミューテーションが不要なら値で十分

### マッチ判定
- `ParseResult.Matched` フィールドで区別
- `nil`/空スライスの曖昧さを排除

### Context引数
- パースがCPU内で完結するなら必須にしない
- 将来の重い実装用に拡張可能な設計に

### ステートフルパーサー
- 複数行イベント対応には `StatefulParser` interface
- `Reset()` と `Flush()` メソッドを追加

---

## リモートURL/セキュリティ調査

### 信頼モデル
- **B (URLプレフィックス) + C (完全URL) を基本**
- A (ドメインベース) は範囲が広すぎてリスクあり

### 初回確認UX
- TTY: 3択（今回だけ / このプレフィックス / 拒否）
- 非TTY: trust + checksum必須、なければ拒否

### キャッシュ安全性
- パーミッション: ディレクトリ 0700、ファイル 0600
- TOCTOU対策: temp → 検証 → rename
- 再検証: 既存キャッシュでも必ずhash検証

---

## 配布/エコシステム調査

### テンプレートリポジトリ
- 「forkしてすぐ使える」が最重要
- GitHub Actionsでtag push → Release自動化
- manifest.jsonをタグからバージョン埋め込み

### SDK設計
- ABI層（安定性最優先）とラッパー層（使いやすさ）を分離
- `go get github.com/vrclog/sdk-tinygo` で取得可能に

### プラグインレジストリ
- 最初は分散型（GitHub上のmanifest集約）で十分
- 中央レジストリは運用コスト大

---

## VRChatログパーサー調査（2025-01-03追加）

### 既存のVRChatログパーサー

| ツール | 言語 | 特徴 | URL |
|--------|------|------|-----|
| VRCX | C# | 最も広く使われる、ログ形式変更への迅速対応 | [GitHub](https://github.com/vrcx-team/VRCX) |
| XSOverlay-VRChat-Parser | C# | XSOverlay通知連携、Portal/World Changed対応 | [GitHub](https://github.com/nnaaa-vr/XSOverlay-VRChat-Parser) |
| VRChat-Log-Monitor | Python | リアルタイム監視、Discord連携、設定ベースイベント検出 | [GitHub](https://github.com/Kavex/VRChat-Log-Monitor) |
| vrchat-log-rs | Rust | crates.io公開、型安全なパーサー | [GitHub](https://github.com/sksat/vrchat-log-rs) |
| VRChatActivityTools | C# | SQLite DB保存、履歴管理 | [GitHub](https://github.com/nukora/VRChatActivityTools) |
| nyanpa.su Parser | Web | カスタム正規表現対応、タイムライン表示 | [nyanpa.su](https://nyanpa.su/vrchatlog/) |

### VRChatログ形式の課題

1. **公式ドキュメントなし**: ログ形式は公式に文書化されていない
2. **頻繁な変更**: VRChatアップデートでフォーマットが予告なく変更される
3. **パーサーの対応遅延**: VRCXなどが対応を迫られる（Issue #142参照）
4. **User ID問題**: 従来はOnPlayerJoinedに表示名のみ、User IDは未記載
   - Feature Request済み、2025年から対応開始

### ログ形式変更への対応事例

**VRCX Issue #142 (2024年):**
- VRChatアップデートでLocation/OnPlayerJoined/OnPlayerLeftのフォーマット変更
- パーサーが動作しなくなり、修正コミットで対応
- 教訓: ログフォーマットは不安定、正規表現は柔軟に設計すべき

### vrclog-goへの示唆

1. **標準ログのパターンは複数用意**: 旧形式・新形式両対応
2. **ワールド固有ログはプラグインへ**: コア部分を安定させる
3. **正規表現の外部化検討**: 設定ファイルでパターン更新可能に?

詳細は [08-issue2-log-format-spec.md](./08-issue2-log-format-spec.md) を参照。

---

## 批判的レビューで得られた知見

### API設計

| 問題 | 対策 |
|------|------|
| ParseResultのセマンティクス曖昧 | `Matched`は「パターンにマッチしたか」を表す。`len(Events)==0 && Matched==true`は「マッチしたがイベント出力なし」を意味 |
| WithParser optionの欠落 | `watchConfig`/`parseConfig`に`parser Parser`フィールド追加、`WithParser(Parser) WatchOption`を実装 |
| パーサー実行順序未定義 | `ParserChain`に`Mode`フィールド: `ChainAll`（全実行）、`ChainFirst`（最初のマッチで終了）、`ChainContinueOnError`（エラースキップ） |
| interfaceの配置パッケージ | `pkg/vrclog/parser.go`に定義（import cycle回避） |
| WatchOption vs ParseOption | 明確に分離: `WithParser(Parser) WatchOption`、`WithParseParser(Parser) ParseOption` |

### セキュリティ

| リスク | 対策 |
|-------|------|
| ReDoS攻撃 | goroutine + channel で5msタイムアウト。将来的に`regexp2`ライブラリ（RE2互換）への移行検討 |
| log関数サンドボックス脱出 | レート制限(10回/秒、`golang.org/x/time/rate`使用)＋サイズ制限(256バイト/回) |
| TOCTOU攻撃 | `filepath.EvalSymlinks`→temp→検証→`os.Rename`の原子的操作 |
| 巨大ファイル | Wasm: 10MB、YAML: 1MB、ダウンロード: 50MB |
| 不正UTF-8 | `strings.ToValidUTF8`で置換 |

### 実装上の問題

| 問題 | 対策 |
|------|------|
| regex_find_submatch実装複雑 | Wasm側バッファ渡し方式に変更: `regex_find_submatch(str_ptr, str_len, re_ptr, re_len, out_ptr, out_max_len) -> written_len` |
| Bump Allocatorリセット | `parse_line`開始時に`heapOff=0`でリセット。ABI仕様とSDKに明記 |
| JSONシリアライズ性能 | `sync.Pool`でバッファ再利用。将来的にMessagePack追加 |
| 正規表現キャッシュスレッドセーフ | `sync.RWMutex` + double-check locking |
| プラグインクラッシュリカバリ | `defer recover()`で保護、エラーログ後に当該行スキップ |

### UX/ドキュメント

| 問題 | 対策 |
|------|------|
| YAMLフィールド名分かりにくい | `name`→`id`、`expression`→`regex`に変更 |
| エラーメッセージ | 行番号・列番号を含む構造化エラー、Hintと参照リンク追加 |

---

## regexp2ライブラリの検討

### 概要

`regexp2`はRE2互換のGoライブラリで、タイムアウト機能を内蔵している。

### メリット

- RE2互換でReDoS耐性が高い
- `regexp2.MatchTimeout`でタイムアウト設定可能
- goroutine不要でシンプル

### デメリット

- 標準`regexp`とAPIが異なる
- `SubexpNames()`がない（named capture groupsの取得方法が異なる）

### 結論

Phase 2ではgoroutine + channelでタイムアウトを実装。
パフォーマンス問題があれば`regexp2`への移行を検討。

---

## wazeroディスクキャッシュ

### 使用方法

```go
cache, err := wazero.NewCompilationCacheWithDir(cacheDir)
if err != nil {
    // キャッシュなしで続行
}
config := wazero.NewRuntimeConfig().WithCompilationCache(cache)
rt := wazero.NewRuntimeWithConfig(ctx, config)
```

### キャッシュディレクトリ

XDG Base Directoryに従う:
- `$XDG_CACHE_HOME/vrclog/wasm/` (通常 `~/.cache/vrclog/wasm/`)

### 効果

- 初回: AOTコンパイルで数百ms
- 2回目以降: キャッシュヒットで数ms

---

## 参考リンク

- [wazero](https://wazero.io/)
- [Extism](https://extism.org/)
- [Expr](https://expr-lang.org/)
- [Fluent Bit Wasm Plugins](https://docs.fluentbit.io/manual/development/wasm-filter-plugins)
- [Arcjet Wasm Production Lessons](https://blog.arcjet.com/lessons-from-running-webassembly-in-production-with-go-wazero/)
- [HashiCorp go-plugin](https://github.com/hashicorp/go-plugin)
- [Vector VRL](https://vector.dev/docs/reference/vrl/)
- [stern](https://github.com/stern/stern)
- [jq Manual](https://jqlang.org/manual/)
- [WebAssembly Security](https://webassembly.org/docs/security/)
- [TinyGo WebAssembly](https://tinygo.org/docs/guides/webassembly/)
- [regexp2](https://github.com/dlclark/regexp2) - RE2互換Goライブラリ
- [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/latest/)

### VRChatログ関連

- [VRChat Debugging Docs](https://creators.vrchat.com/worlds/udon/debugging-udon-projects/)
- [VRChat Log - Unofficial Wiki](http://vrchat.wikidot.com/worlds:guides:log)
- [VRCX](https://github.com/vrcx-team/VRCX)
- [XSOverlay-VRChat-Parser](https://github.com/nnaaa-vr/XSOverlay-VRChat-Parser)
- [VRChat-Log-Monitor](https://github.com/Kavex/VRChat-Log-Monitor)
- [UserIDs in output logs Feature Request](https://feedback.vrchat.com/feature-requests/p/provide-userids-in-output-logs)
