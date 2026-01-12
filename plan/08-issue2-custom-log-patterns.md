# Issue #2: カスタムログパターンの拡張機能

## 概要

VRChatのUdonワールド（VRPoker等）が出力する独自ログを解析できるようにする機能。

## 要件

1. **拡張可能性** - ユーザーがカスタムパーサーを書ける
2. **気軽に配布** - .wasmファイル単体で配布可能
3. **セルフホスト** - 誰でもGitHub等でホスティングして配布可能
4. **セキュリティ** - サンドボックス実行

## 決定事項

### issueコメントで合意された方針

- **案A**: Parser interface（Go開発者向け）
- **案B**: YAMLパターンファイル + RegexParser（簡易用途向け）
- 両方をサポートする設計
- XDG Base Directory Specificationに従う
- Named capture groups使用（Grafanaスタイル）

### 技術的決定事項

- **スコープ**: Phase 1a-1c（基盤）+ Phase 2（Wasm）+ Phase 3（エコシステム）
- **リモートURL**: Phase 2以降で対応
- **ChainMode**: デフォルトは `ChainAll`（全パーサー実行、結果を結合）
- **wazeroバージョン**: v1.8.0（2024年末時点の安定版）

---

## 責務分離の設計思想

### 基本原則

**VRChat標準ログ**と**ワールド固有ログ**で責任を明確に分離する。

```
┌─────────────────────────────────────────────────────────────┐
│ vrclog-go (このリポジトリ)                                    │
│                                                               │
│ 責任範囲:                                                     │
│ ・VRChat標準ログのパース（player_join, player_left, world_join）│
│ ・VRChatアップデートへの追従                                   │
│ ・Wasmプラグインローダー機能の提供                             │
│ ・コア機能の安定性維持                                         │
├─────────────────────────────────────────────────────────────┤
│ 外部リポジトリ（ワールド別）                                    │
│                                                               │
│ 例:                                                           │
│ ・github.com/xxx/vrpoker-vrclog-plugin                        │
│ ・github.com/xxx/blackjack-vrclog-plugin                      │
│                                                               │
│ 責任範囲:                                                     │
│ ・各ワールド固有のログパターン定義                              │
│ ・ワールドアップデートへの追従                                  │
│ ・.wasmファイルの配布・バージョン管理                           │
└─────────────────────────────────────────────────────────────┘
```

### メリット

1. **責任の明確化**: VRChat標準ログはvrclog-go、ワールド固有ログは各コミュニティ
2. **迅速な対応**: ワールド開発者が自分のログ変更に即座に対応可能
3. **エコシステム拡大**: 誰でもプラグインを作って配布可能
4. **vrclog-goの安定性**: コアは標準ログのみに集中、肥大化を防ぐ

### ログフォーマットの課題への対応

VRChatのログフォーマットは公式に文書化されておらず、頻繁に変更される。
詳細は [08-issue2-log-format-spec.md](./08-issue2-log-format-spec.md) を参照。

| ログ種別 | 管理者 | 更新タイミング |
|---------|--------|---------------|
| VRChat標準ログ（OnPlayerJoined等） | vrclog-goリポジトリ | VRChatアップデート時 |
| ワールド固有ログ（VRPoker等） | 各プラグインリポジトリ | ワールドアップデート時 |

---

## 推奨アーキテクチャ: 2層構成

```
┌─────────────────────────────────────────────────────────────┐
│ Layer 2: Wasm Plugins (wazero)                              │
│ - 外部メンテナによるプラグイン                                │
│ - .wasmファイルで配布                                        │
│ - サンドボックスで安全に実行                                  │
│ - TinyGo/Rust/AssemblyScriptで開発                          │
├─────────────────────────────────────────────────────────────┤
│ Layer 1: Core (Go)                                          │
│ - Parser interface（ライブラリユーザー向け）                  │
│ - RegexParser（YAMLパターンファイル用）                       │
│ - Wasmローダー（Parser実装の一つとして）                      │
│ - 複雑なフィルタは jq にパイプで委譲                          │
└─────────────────────────────────────────────────────────────┘
```

---

## パッケージ構成

```
pkg/vrclog/
├── parser.go           # Parser interface, ParseResult, ParserChain (NEW)
├── parser_default.go   # DefaultParser (NEW)
├── parse.go            # ParseLine (既存、互換性維持)
├── watcher.go          # WithParser option追加
├── options.go          # WithParser/WithParsers追加
├── pattern/            # RegexParser用 (NEW)
│   ├── pattern.go      # PatternFile, Pattern型定義
│   ├── regex_parser.go # RegexParser (Parser実装)
│   └── loader.go       # YAMLローダー
internal/wasm/          # Wasmプラグイン用 (NEW)
├── loader.go           # Wasmモジュールのロード、ABI検証
├── parser.go           # WasmParser (Parser実装)
├── host.go             # Host functions (regex_match等)
├── cache.go            # 正規表現キャッシュ
└── fetch.go            # リモート取得 (Phase 3)
cmd/vrclog/
├── tail.go             # --patterns, --plugin オプション追加
├── parse.go            # --patterns, --plugin オプション追加
└── plugin.go           # plugin trust/cache サブコマンド (Phase 3)
```

---

## なぜこの構成か

### wazero直結を選択

1. **依存ゼロ** - 配布が楽
2. **Go 1.21+** - knqyf263/go-pluginの1.24+制約なし
3. **API設計自由度** - ログパース向けインターフェース設計可能
4. **本番実績** - Arcjet等で運用実績あり（p50=10ms、p99=30ms）

### 式言語（Expr等）を採用しない理由

1. **jqと競合** - jqが既に強力で、表現力・拡張性は上位互換
2. **学習コスト** - ユーザーはjqとExprの二重学習が必要
3. **Unix哲学** - vrclog-goは「パース」に集中、フィルタはjqに任せる

### Extism/waPCを見送った理由

- Extism: 依存が増える、独自ABI依存
- waPC: 仕様が古め、ログパースにはRPC過剰

---

## CLIとライブラリの両立

### ライブラリユーザー向け

- `Parser` インターフェースを直接Goで実装可能
- `WithParser()` / `WithParsers()` で差し替え

### CLIユーザー向け

- `--patterns` でYAMLパターンファイルを指定
- `--plugin` でWasmファイルを指定（Phase 2）
- ローカルファイル or リモートURL

---

## 実装ロードマップ

### Phase 1a: Parser Interface基盤

**目標**: 既存動作を維持しつつParser interfaceを導入

**背景**:
- Parser interfaceを最初に導入することで、Phase 1b/1c/Phase 2の全てがこのインターフェースを使える
- 既存の`ParseLine`関数はラッパーとして維持し、後方互換性を確保

**詳細**: [08-issue2-phase1a-parser-interface.md](./08-issue2-phase1a-parser-interface.md)

#### 新規ファイル

| ファイル | 用途 |
|---------|------|
| `pkg/vrclog/parser.go` | Parser interface, ParseResult, ParserChain, ParserFunc, ChainMode |
| `pkg/vrclog/parser_default.go` | DefaultParser（既存internal/parserをラップ） |
| `pkg/vrclog/parser_test.go` | Parser interfaceテスト |

#### 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `pkg/vrclog/options.go` | `WithParser()` option追加 |
| `pkg/vrclog/watcher.go` | `cfg.parser.ParseLine(line)` に変更 |
| `pkg/vrclog/parse.go` | `cfg.parser.ParseLine(line)` に変更 |

#### タスクチェックリスト

- [ ] Parser interface定義
- [ ] ParseResult型定義
- [ ] ParserChain実装（ChainAll/ChainFirst/ChainContinueOnError）
- [ ] DefaultParser実装
- [ ] WithParser() option追加
- [ ] watcher.go更新
- [ ] parse.go更新
- [ ] 既存テストがパスすることを確認

---

### Phase 1b: Event.Data + RegexParser

**目標**: YAMLパターンファイルによるカスタムイベント

**背景**:
- プログラミング不要でカスタムパターンを定義できるようにする
- Grafana Promtailのpipeline stagesスタイルを参考（named capture groups）
- YAMLスキーマのフィールド名は批判的レビューで改善（`name`→`id`、`expression`→`regex`）

**詳細**: [08-issue2-phase1b-regex-parser.md](./08-issue2-phase1b-regex-parser.md)

#### 新規ファイル

| ファイル | 用途 |
|---------|------|
| `pkg/vrclog/pattern/pattern.go` | PatternFile, Pattern型定義 |
| `pkg/vrclog/pattern/regex_parser.go` | RegexParser（Parser実装） |
| `pkg/vrclog/pattern/loader.go` | YAMLローダー |
| `pkg/vrclog/pattern/*_test.go` | テスト |

#### 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `go.mod` | `gopkg.in/yaml.v3` 依存追加 |
| `pkg/vrclog/event/event.go` | `Data map[string]string` フィールド追加 |

#### タスクチェックリスト

- [ ] Event.Dataフィールド追加
- [ ] YAMLスキーマ定義（version 1）
- [ ] PatternFileローダー実装
- [ ] RegexParser実装
- [ ] named capture groups対応
- [ ] タイムスタンプ自動抽出
- [ ] テスト作成

---

### Phase 1c: CLI統合

**目標**: `--patterns`フラグでYAMLパターンを指定

**背景**:
- CLIユーザーがプログラミングなしでカスタムパターンを使えるようにする
- 複数パターンファイルのチェーン実行をサポート

**詳細**: [08-issue2-phase1c-cli.md](./08-issue2-phase1c-cli.md)

#### 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `cmd/vrclog/tail.go` | `--patterns` フラグ追加 |
| `cmd/vrclog/parse.go` | `--patterns` フラグ追加 |
| `cmd/vrclog/format.go` | Event.Data対応（JSON出力時に含める） |

#### タスクチェックリスト

- [ ] --patternsフラグ追加（tail/parse）
- [ ] PatternFile→RegexParser→ParserChain変換
- [ ] Event.DataのJSON出力対応
- [ ] エラーメッセージの改善
- [ ] CLIテスト作成

---

### Phase 2: Wasmプラグイン基盤

**目標**: TinyGo/Rust等でプラグイン開発可能に

**背景**:
- YAMLパターンでは表現できない複雑なロジック（状態遷移等）に対応
- wazeroのサンドボックスで安全に実行
- TinyGoの制限（encoding/json、regexp）はHost Functionsで回避

**詳細**: [08-issue2-phase2-wasm.md](./08-issue2-phase2-wasm.md)

#### 新規ファイル

| ファイル | 用途 |
|---------|------|
| `internal/wasm/loader.go` | Wasmモジュールのロード、ABI検証 |
| `internal/wasm/parser.go` | WasmParser（Parser実装） |
| `internal/wasm/host.go` | Host functions（regex_match, regex_find_submatch, log） |
| `internal/wasm/cache.go` | 正規表現キャッシュ（LRU） |
| `internal/wasm/errors.go` | エラー型定義 |
| `internal/wasm/*_test.go` | テスト |
| `examples/plugins/vrpoker/` | VRPokerサンプルプラグイン |

#### 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `go.mod` | `github.com/tetratelabs/wazero v1.8.0` 依存追加 |
| `cmd/vrclog/tail.go` | `--plugin` フラグ追加 |
| `cmd/vrclog/parse.go` | `--plugin` フラグ追加 |

#### リソース制限

| リソース | デフォルト | 設定可能 |
|---------|-----------|---------|
| 実行タイムアウト | 50ms | `--plugin-timeout` |
| メモリ | 4MiB (64 pages) | `--plugin-memory` |
| 正規表現キャッシュ | 100パターン | 設定ファイル |
| 入力行最大長 | 8192バイト | - |
| 出力イベント最大数 | 16個 | - |

#### タスクチェックリスト

- [ ] wazero依存追加
- [ ] ABI実装（abi_version, alloc, free, parse_line）
- [ ] Host functions実装（regex_match, regex_find_submatch, log, now_ms）
- [ ] 正規表現キャッシュ（LRU、sync.Map）
- [ ] ReDoS対策（5msタイムアウト）
- [ ] log関数レート制限（10回/秒）
- [ ] WasmParser実装
- [ ] --pluginフラグ追加
- [ ] VRPokerサンプル作成
- [ ] テスト作成

---

### Phase 3: エコシステム

**目標**: リモートURL対応、信頼リスト、テンプレートリポジトリ

**背景**:
- GitHub Releasesから直接プラグインを取得できるようにする
- 信頼できるソースからのみ実行を許可するセキュリティモデル
- プラグイン開発者向けのテンプレートとSDKを提供

#### 新規ファイル

| ファイル | 用途 |
|---------|------|
| `internal/wasm/fetch.go` | リモートURL取得、TOCTOU-safe |
| `internal/wasm/trust.go` | 信頼リスト管理 |
| `cmd/vrclog/plugin.go` | `plugin` サブコマンド |

#### ディレクトリ構造

```
~/.cache/vrclog/plugins/
├── <sha256>.wasm       # キャッシュされたプラグイン
└── manifest.json       # メタデータ

~/.config/vrclog/
└── trust.json          # 信頼リスト
```

#### 信頼モデル

| パターン | 例 | マッチ |
|---------|---|--------|
| 完全URL | `https://github.com/.../plugin.wasm` | 完全一致のみ |
| URLプレフィックス | `https://github.com/vrclog/` | プレフィックス一致 |

非TTY環境: `--checksum` 必須

#### タスクチェックリスト

- [ ] リモートURL取得
- [ ] HTTPS必須
- [ ] 信頼リスト管理
- [ ] checksum検証（sha256）
- [ ] TOCTOU対策（temp→検証→rename）
- [ ] キャッシュ機構
- [ ] テンプレートリポジトリ作成
- [ ] TinyGo SDK

---

### 別リポジトリ

#### vrclog-plugins

```
github.com/vrclog/vrclog-plugins/
├── sdk/tinygo/         # TinyGo SDK
│   ├── vrclog/         # ヘルパーパッケージ
│   └── examples/       # サンプルコード
├── plugins/official/   # 公式プラグイン
│   └── vrpoker/
└── docs/               # ドキュメント
```

#### vrclog-plugin-template

```
github.com/vrclog/vrclog-plugin-template/
├── main.go             # テンプレート
├── Makefile
├── manifest.json
└── .github/workflows/
    └── release.yml     # タグpush → Release自動化
```

---

## セキュリティ考慮事項

詳細は [08-issue2-security.md](./08-issue2-security.md) を参照。

| リスク | 対策 |
|-------|------|
| ReDoS攻撃 | goroutine + channel で5msタイムアウト |
| Wasmサンドボックス脱出 | log関数のレート制限(10回/秒)＋サイズ制限(256バイト/回) |
| TOCTOU攻撃 | `filepath.EvalSymlinks`→temp→検証→`os.Rename`の原子的操作 |
| パストラバーサル | パス検証関数で許可ディレクトリ外アクセスを拒否 |
| 巨大ファイル | Wasm: 10MB上限、YAML: 1MB上限、ダウンロード: 50MB上限 |
| 不正UTF-8 | `strings.ToValidUTF8`で置換 |

---

## テスト戦略

詳細は [08-issue2-testing.md](./08-issue2-testing.md) を参照。

- Phase 1a: ParserChain, DefaultParser, ParserFuncのユニットテスト
- Phase 1b: YAMLローダー、RegexParserのユニットテスト
- Phase 1c: CLIインテグレーションテスト
- Phase 2: Host functionsセキュリティテスト、TinyGoビルドテスト
- Phase 3: リモート取得、信頼リストテスト

---

## 関連ファイル

- [技術調査](./08-issue2-research.md)
- [VRChatログフォーマット仕様](./08-issue2-log-format-spec.md)
- [ABI仕様](./08-issue2-abi-spec.md)
- [実装例](./08-issue2-examples.md)
- [issueコメント案](./08-issue2-issue-comment.md)
- [セキュリティ](./08-issue2-security.md)
- [テスト戦略](./08-issue2-testing.md)
- [Phase 1a: Parser Interface](./08-issue2-phase1a-parser-interface.md)
- [Phase 1b: RegexParser](./08-issue2-phase1b-regex-parser.md)
- [Phase 1c: CLI](./08-issue2-phase1c-cli.md)
- [Phase 2: Wasm](./08-issue2-phase2-wasm.md)
