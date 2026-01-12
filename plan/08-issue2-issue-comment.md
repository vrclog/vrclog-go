# Issue #2: issueコメント案

以下はGitHub issueに投稿するコメントの案です。

---

## 検討結果

@tatsu020 詳細な提案ありがとうございます！案A/Bを検討し、**両方のメリットを活かす設計**を考えました。

### 方針: 案Aベース + Wasmで案Bの「誰でも使える」を実現

| ユーザー層 | 方法 |
|-----------|------|
| Go開発者 | `Parser` interfaceを直接実装（案A） |
| 一般ユーザー | `.wasm`ファイルを`--plugin`で指定 |

案BのYAML設定も魅力的ですが、Wasmにした理由：
- **正規表現だけでは限界**: VRPokerの「フォールド→勝者決定」のような状態遷移は正規表現では表現困難
- **配布の容易さ**: `.wasm`ファイル1つで完結、依存なし
- **サンドボックス実行**: 任意のコードを安全に実行可能

---

## Parser Interface（ライブラリAPI）

```go
// パース結果
type ParseResult struct {
    Events  []Event
    Matched bool  // nil/空スライスとの区別
}

// Parserインターフェース
type Parser interface {
    ParseLine(line string) (ParseResult, error)
}

// 使用例
watcher, _ := vrclog.NewWatcherWithOptions(
    vrclog.WithParser(&MyCustomParser{}),
)
```

既存の標準パーサー（world_join等）も `Parser` interface経由で利用。
複数パーサーを`ParserChain`でチェーン実行可能（全パーサー実行 or 最初のマッチで終了）。

---

## Wasm ABI仕様

### プラグインがExportする関数

```c
// ABIバージョン（現在: 1）
uint32_t abi_version();

// メモリ管理
uint32_t alloc(uint32_t size);
void free(uint32_t ptr, uint32_t len);

// パース実行（戻り値: len<<32 | ptr）
uint64_t parse_line(uint32_t ptr, uint32_t len);
```

### Host Functions（vrclog-goが提供）

TinyGoの正規表現制限対策として、**正規表現はHost側で実行**しプラグインにはHost Function経由で提供：

```c
// 正規表現マッチ（0=不一致, 1=一致）
uint32_t regex_match(str_ptr, str_len, re_ptr, re_len);

// キャプチャグループ取得（JSON配列で返却）
uint64_t regex_find_submatch(str_ptr, str_len, re_ptr, re_len);

// デバッグログ（level: 0=debug, 1=info, 2=warn, 3=error）
void log(uint32_t level, uint32_t ptr, uint32_t len);
```

### parse_line JSONプロトコル

**Request:**
```json
{"line": "2025.12.31 01:46:48 Debug - [Seat]: Draw Local Hole Cards: Jc, 6d"}
```

**Response（成功）:**
```json
{
  "ok": true,
  "events": [{
    "type": "poker_hole_cards",
    "timestamp": "2025-12-31T01:46:48+09:00",
    "data": {"card1": "Jc", "card2": "6d"}
  }]
}
```

**Response（マッチなし）:** `{"ok": true, "events": []}`

---

## CLI使用例

```bash
# ローカルプラグイン
vrclog tail --plugin ./vrpoker.wasm

# 複数プラグイン（標準パーサー + カスタム）
vrclog tail --plugin ./vrpoker.wasm --plugin ./custom.wasm

# リモートURL（Phase 3で対応予定）
vrclog tail --plugin https://github.com/.../vrpoker.wasm --checksum sha256:abc123...
```

---

## プラグイン開発（TinyGo例）

```go
//go:build tinygo.wasm

package main

import "unsafe"

//go:wasmimport env regex_match
func regex_match(strPtr, strLen, rePtr, reLen uint32) uint32

//export abi_version
func abi_version() uint32 { return 1 }

//export parse_line
func parse_line(ptr, size uint32) uint64 {
    // Host Functionで正規表現マッチ
    re := []byte(`\[Seat\]: Draw Local Hole Cards: (\w+), (\w+)`)
    if regex_match(ptr, size, rePtr, uint32(len(re))) == 1 {
        // マッチ時の処理
    }
    // ...
}
```

ビルド: `tinygo build -o vrpoker.wasm -target=wasi -no-debug main.go`

---

## 実装ロードマップ

| Phase | 内容 | 成果物 |
|-------|------|--------|
| 1 | 基盤構築 | Parser interface, wazero統合, `--plugin`フラグ, VRPokerサンプル |
| 2 | 実用機能 | 複数プラグイン, タイムアウト(50ms), 正規表現キャッシュ |
| 3 | エコシステム | リモートURL, checksum検証, 信頼リスト, テンプレートリポジトリ |

### リソース制限（Phase 2）

| リソース | デフォルト |
|---------|-----------|
| 実行タイムアウト | 50ms |
| メモリ | 4MiB |
| 正規表現キャッシュ | 100パターン |

---

## 技術選定

**wazero（純Go Wasmランタイム）を採用：**
- 依存ゼロ（CGO不要）
- Go 1.21+対応
- 本番実績あり（Arcjet: p50=10ms, p99=30ms）
- サンドボックスによるセキュリティ

**比較検討した代替案：**
- Extism: 依存が増える、独自ABI
- knqyf263/go-plugin: Go 1.24+必須
- 式言語(Expr): jqと競合、学習コスト増

---

## フィードバック募集

1. **Wasm + Parser interface** の方針について
2. VRPoker以外に優先すべきユースケース（USharpVideo等）
3. Event.Dataのキー命名規則（snake_case / camelCase）
4. ABI仕様へのコメント

---

## @tatsu020 への協力依頼

実装を手伝いたいとのこと、ありがとうございます！

最初に以下をお願いできると助かります：

- **VRPokerのログパターン追加**: issue本文のサンプル以外にパースしたいパターンがあれば
- **優先度**: どのイベント（ホールカード、ベット、勝敗等）を最初にサポートすべきか

Phase 1実装後：
- プラグインの動作テスト
- ドキュメントレビュー
- 別プラグイン（USharpVideo等）の開発

参加可能な範囲を教えてください！

---

## 詳細ドキュメント

- [メイン計画](./08-issue2-custom-log-patterns.md)
- [技術調査](./08-issue2-research.md)
- [ABI仕様](./08-issue2-abi-spec.md)
- [実装例](./08-issue2-examples.md)
