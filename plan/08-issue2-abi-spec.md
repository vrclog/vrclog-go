# Issue #2: ABI仕様

## 概要

vrclog-go Wasmプラグインのバイナリインターフェース仕様。
プラグインはwazeroランタイム上で実行され、サンドボックス環境で安全に動作する。

---

## アーキテクチャ概要

```
┌─────────────────────────────────────────────────────────────────┐
│ Host (vrclog-go)                                                │
│                                                                 │
│  ┌─────────────┐    ┌─────────────────────────────────────────┐ │
│  │ ParserChain │───>│ WasmParser                              │ │
│  └─────────────┘    │                                         │ │
│                     │  ┌─────────────────────────────────────┐│ │
│                     │  │ wazero Runtime                      ││ │
│                     │  │                                     ││ │
│                     │  │  ┌───────────────────────────────┐  ││ │
│                     │  │  │ Plugin Module (.wasm)         │  ││ │
│                     │  │  │                               │  ││ │
│                     │  │  │ Exports:                      │  ││ │
│                     │  │  │ - abi_version()               │  ││ │
│                     │  │  │ - alloc(), free()             │  ││ │
│                     │  │  │ - parse_line()                │  ││ │
│                     │  │  │                               │  ││ │
│                     │  │  │ Imports (Host Functions):     │  ││ │
│                     │  │  │ - regex_match()               │  ││ │
│                     │  │  │ - regex_find_submatch()       │  ││ │
│                     │  │  │ - log()                       │  ││ │
│                     │  │  │ - now_ms()                    │  ││ │
│                     │  │  └───────────────────────────────┘  ││ │
│                     │  └─────────────────────────────────────┘│ │
│                     └─────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

---

## データ交換形式

- **JSON を標準**（デバッグ容易、互換性重視）
- 将来的にMessagePackオプション追加可能
- `abi_capabilities()` で対応形式を宣言

### 背景

MessagePackはJSONより効率的だが、デバッグが困難になる。
Phase 2ではJSONのみをサポートし、パフォーマンスが問題になったらMessagePackを追加する。

---

## Event型の変更

```go
// pkg/vrclog/event/event.go
type Event struct {
    Type       Type              `json:"type"`
    Timestamp  time.Time         `json:"timestamp"`
    PlayerName string            `json:"player_name,omitempty"`
    PlayerID   string            `json:"player_id,omitempty"`
    WorldID    string            `json:"world_id,omitempty"`
    WorldName  string            `json:"world_name,omitempty"`
    InstanceID string            `json:"instance_id,omitempty"`
    RawLine    string            `json:"raw_line,omitempty"`
    Data       map[string]string `json:"data,omitempty"`  // NEW: プラグイン用
}
```

### 背景

`Data`フィールドはプラグインが任意のキー・バリューを格納するために使用。
`map[string]string`を採用した理由：
- JSONシリアライズが容易
- Wasmとのデータ交換がシンプル
- 型安全性よりも柔軟性を優先（プラグインは外部開発者が作成）

---

## Wasm Export関数

### 必須

```c
// ABIバージョン（現在: 1）
// 破壊的変更時にバージョンを上げる
uint32_t abi_version();

// 対応機能（JSON配列: ["json", "msgpack"]等）
// 現在はJSON必須
uint64_t abi_capabilities();  // returns (len<<32 | ptr)

// メモリ確保
// 8バイトアライメント推奨
uint32_t alloc(uint32_t size);

// メモリ解放
// Bump allocatorの場合はno-opでもよい
void free(uint32_t ptr, uint32_t len);

// パース実行
// 引数: input_ptr, input_len はHost側がWasmメモリに直接書き込んだ位置とサイズ
// 戻り値: 上位32bitがlength、下位32bitがpointer
// **注意**: parse_line開始時にBump allocatorをリセット可能（heapOff=0）
//         出力バッファはalloc()で確保する
uint64_t parse_line(uint32_t input_ptr, uint32_t input_len);
```

### オプション

```c
// プラグイン初期化（設定受け取り）
// Phase 3で使用予定
uint64_t init(uint32_t config_ptr, uint32_t config_len);

// プラグイン情報取得
// プラグイン名、バージョン、作者等をJSONで返す
uint64_t get_info();  // returns (len<<32 | ptr)
```

### 戻り値のパック形式

64bit整数で、上位32bitがlength、下位32bitがpointer:

```
返却値 = (length << 32) | pointer
```

この形式を採用した理由：
- 単一の戻り値で2つの情報を返せる
- Wasm仕様ではmulti-value returnsが複雑になる場合がある
- wazeroで扱いやすい

---

## Host Function（Core → Wasm）

Host Functionsは `env` モジュールにエクスポートされる。

### 正規表現

#### regex_match

```c
// 正規表現マッチ（0=不一致, 1=一致）
// 5msタイムアウト付き（ReDoS対策）
uint32_t regex_match(uint32_t str_ptr, uint32_t str_len,
                     uint32_t re_ptr, uint32_t re_len);
```

**実装上の注意**:
- goroutine + channel でタイムアウトを実装
- 正規表現パターンはLRUキャッシュで保持（100パターン）
- パターン長が512バイトを超える場合は拒否

#### regex_find_submatch（改訂版）

```c
// 正規表現キャプチャグループ取得
// **Wasm側がバッファを事前確保して渡す方式**
//
// 引数:
//   str_ptr, str_len: 検索対象文字列
//   re_ptr, re_len: 正規表現パターン
//   out_buf_ptr: 出力バッファ（Wasm側がallocで確保）
//   out_buf_len: 出力バッファサイズ
//
// 戻り値:
//   実際に書き込んだバイト数（0=マッチなしまたはエラー）
//
// 出力形式:
//   JSON配列 ["full_match", "group1", "group2", ...]
uint32_t regex_find_submatch(uint32_t str_ptr, uint32_t str_len,
                             uint32_t re_ptr, uint32_t re_len,
                             uint32_t out_buf_ptr, uint32_t out_buf_len);
```

**背景**:
当初はHost側がメモリを確保してポインタを返す方式を検討したが、
Wasm側からHost確保メモリへのアクセスが複雑になるため、
Wasm側が事前にバッファを確保する方式に変更した。

**制限**:
- 出力バッファサイズ上限: 4KB
- タイムアウト: 5ms

### ユーティリティ

#### log

```c
// デバッグログ出力
// level: 0=debug, 1=info, 2=warn, 3=error
//
// セキュリティ制限:
// - レート制限: 10回/秒（golang.org/x/time/rate使用）
// - サイズ制限: 256バイト/回（超過分は切り捨て）
// - UTF-8検証: 不正なバイトは置換
void log(uint32_t level, uint32_t ptr, uint32_t len);
```

**背景**:
log関数はサンドボックス脱出のリスクがあるため、厳格な制限を設ける。
レート制限はプラグインインスタンスごとに適用。

#### now_ms

```c
// 現在時刻取得（Unix milliseconds）
// タイムスタンプ生成に使用
uint64_t now_ms();
```

### TinyGoでのHost Function呼び出し

```go
//go:wasmimport env regex_match
func regex_match(strPtr, strLen, rePtr, reLen uint32) uint32

//go:wasmimport env regex_find_submatch
func regex_find_submatch(strPtr, strLen, rePtr, reLen, outBufPtr, outBufLen uint32) uint32

//go:wasmimport env log
func host_log(level, ptr, len uint32)

//go:wasmimport env now_ms
func now_ms() uint64
```

---

## なぜ正規表現をHost側で実行するか

TinyGoでの正規表現処理には以下の問題がある：

1. **TinyGoの`regexp`パッケージはWasmサイズが大きくなる**
   - 標準のregexpパッケージは100KB以上のサイズ増加

2. **go-re2は最近TinyGo非対応**
   - RE2はReDoS耐性があるが、TinyGoサポートが不安定

3. **Host側でキャッシュすることで性能向上**
   - コンパイル済み正規表現をLRUキャッシュで保持
   - 同じパターンの再コンパイルを回避

4. **ReDoS対策をHost側で一元管理**
   - goroutine + channelでタイムアウト実装
   - プラグイン側で対策を実装する必要がない

---

## parse_line JSONスキーマ

### Request (Host → Plugin)

```json
{
  "line": "[2024.01.01 12:34:56] User joined ...",
  "context": {
    "source": "client_log"
  }
}
```

### Response (成功)

```json
{
  "ok": true,
  "events": [
    {
      "type": "poker_hole_cards",
      "timestamp": "2024-01-01T12:34:56Z",
      "data": {
        "card1": "Jc",
        "card2": "6d"
      }
    }
  ]
}
```

### Response (マッチなし)

```json
{
  "ok": true,
  "events": []
}
```

### Response (エラー)

```json
{
  "ok": false,
  "code": "INVALID_FORMAT",
  "message": "unexpected token at position 15"
}
```

---

## エラーコード

| コード | 説明 |
|-------|------|
| `PARSE_ERROR` | 入力が解析できない |
| `INVALID_INPUT` | schema不一致 |
| `UNSUPPORTED` | 未対応の形式 |
| `INTERNAL_ERROR` | プラグイン内部エラー |
| `TIMEOUT` | 処理タイムアウト |

---

## メモリ管理

### parse_line呼び出しフロー（改訂版）

```
Host                                    Plugin (Wasm)
  │                                         │
  │──── write(INPUT_REGION, input_json) ──>│  ※固定領域(例:0x10000〜)に直接書き込み
  │                                         │
  │──── parse_line(INPUT_REGION, len) ────>│
  │                                         │ ← heapOff = 0（リセット）
  │                                         │ ← 入力をコピー（必要なら）
  │                                         │ ← パース処理
  │                                         │ ← regex_match呼び出し (Host Function)
  │                                         │ ← alloc()で出力バッファ確保
  │<─── (out_len<<32 | out_ptr) ────────────│
  │                                         │
  │──── read(out_ptr, out_len) ────────────>│
  │<─── output_json ────────────────────────│
  │                                         │
  │──── free(out_ptr, out_len) ────────────>│  ※入力領域はfree不要
  │                                         │
```

**変更点**:
- **入力はHost側がWasmメモリの固定領域に直接書き込み**（例: 0x10000〜0x12000の8KB領域）
- allocは出力バッファ確保にのみ使用
- parse_line開始時にheapOff=0でリセット可能（入力は固定領域にあるため破壊されない）

### Bump Allocatorとリセットタイミング（改訂版）

**重要**: `parse_line`の呼び出し開始時に`heapOff = 0`でヒープをリセットする。
**入力は固定領域**にあるため、リセットしても破壊されない。

```go
// TinyGoプラグイン側の実装
const INPUT_REGION = 0x10000  // 入力固定領域（64KB〜72KB）

var heap [1 << 20]byte  // 1MiB Bump allocator用（通常は0番地から）
var heapOff uintptr

//export parse_line
func parse_line(inputPtr, inputLen uint32) uint64 {
    // ★★★ 呼び出し開始時にヒープリセット ★★★
    // 入力はINPUT_REGION（固定領域）にあるため破壊されない
    heapOff = 0

    // 入力JSON読み取り（固定領域から）
    inputBytes := ptrToBytes(inputPtr, inputLen)

    // ... パース処理 ...

    // 出力バッファをalloc()で確保
    outputJSON := []byte(`{"ok":true,"events":[]}`)
    outPtr := alloc(uint32(len(outputJSON)))
    writeBytes(outPtr, outputJSON)

    return (uint64(len(outputJSON)) << 32) | uint64(outPtr)
}

//export alloc
func alloc(size uint32) uint32 {
    off := (heapOff + 7) &^ 7  // 8-byte alignment
    if off+uintptr(size) > uintptr(len(heap)) {
        return 0  // OOM
    }
    ptr := unsafe.Pointer(&heap[off])
    heapOff = off + uintptr(size)
    return uint32(uintptr(ptr))
}

//export free
func free(ptr, size uint32) {
    // bump allocator: no-op
}
```

**背景**:
- **入力と出力で異なるメモリ領域を使用**
  - 入力: HOST側が書き込む固定領域（例: 0x10000〜）
  - 出力: Bump allocatorで確保（heapから）
- parse_line開始時のheapOff=0リセットで出力用メモリをクリア
- 入力領域は固定なのでリセットの影響を受けない
- メモリリークを防ぎつつシンプルな実装を維持

### アロケータ設計の選択肢

| 方式 | 特徴 | 推奨シナリオ |
|-----|------|------------|
| Bump Allocator | 高速、リセットで一括解放 | 1リクエスト完結型（推奨） |
| Arena | 複数アリーナ、個別解放可 | 複雑な状態管理が必要な場合 |
| malloc/free | 汎用、オーバーヘッドあり | 長期間動作が必要な場合 |

---

## 制限

| 項目 | 値 | 理由 |
|------|-----|------|
| 入力行 | 最大 8192 バイト | VRChatログ1行の現実的な上限 |
| 出力イベント | 最大 16 個 | 1行から大量のイベントは異常 |
| タイムアウト | 50ms（デフォルト） | 遅いプラグインがtailをブロックしないため |
| メモリ | 4MiB（64 pages） | TinyGoのデフォルトヒープサイズ |
| 正規表現タイムアウト | 5ms | ReDoS攻撃対策 |
| 正規表現パターン長 | 最大 512 バイト | 複雑すぎるパターンを防止 |
| 正規表現キャッシュ | 100 パターン | メモリ使用量とのバランス |
| log関数レート | 10回/秒 | ログスパム防止 |
| log関数サイズ | 256 バイト/回 | ログ肥大化防止 |
| Wasmファイルサイズ | 10 MB | 不正に大きいファイルを拒否 |

---

## バージョニング

### ABIバージョン

- `abi_version()`で互換性チェック
- 破壊的変更時は新しいバージョン番号
- 後方互換の追加は同一バージョン内で可能

### 互換性ルール

| 変更種別 | ABIバージョン | 例 |
|---------|--------------|---|
| Host Function追加 | 維持 | 新しいユーティリティ関数 |
| Host Functionシグネチャ変更 | 上げる | 引数追加・削除 |
| Export関数追加（オプション） | 維持 | get_info追加 |
| Export関数シグネチャ変更 | 上げる | parse_lineの戻り値変更 |

### Host側の互換性サポート

- 現在のABIバージョン: 1
- 古いABIバージョンもサポート（当面は不要）

---

## wazero最適化

### AOT Compilation

```go
// 起動時に事前コンパイル
compiledMod, err := runtime.CompileModule(ctx, wasmBytes)
if err != nil {
    return err
}

// インスタンス化時は高速
mod, err := runtime.InstantiateModule(ctx, compiledMod, config)
```

**背景**:
Arcjetの本番事例では、AOTコンパイルにより p50=10ms、p99=30ms を達成。
初回コンパイルは遅いが、キャッシュにより2回目以降は高速。

### Module Cache

```go
// ディスクキャッシュで起動高速化
cache, err := wazero.NewCompilationCacheWithDir(cacheDir)
if err != nil {
    return err
}
config := wazero.NewRuntimeConfig().WithCompilationCache(cache)
runtime := wazero.NewRuntimeWithConfig(ctx, config)
```

**キャッシュディレクトリ**:
```
~/.cache/vrclog/wasm/
├── <sha256>.cache      # コンパイル済みモジュール
└── manifest.json       # メタデータ
```

### Instance Reuse

- 可能ならインスタンスを再利用
- メモリリセットが必要な場合は新規インスタンス
- Bump allocatorのリセットで対応可能な場合は再利用

---

## get_info JSONスキーマ

```json
{
  "name": "vrpoker-plugin",
  "version": "1.0.0",
  "description": "VRPoker log parser plugin",
  "author": "tatsu020",
  "license": "MIT",
  "homepage": "https://github.com/vrclog/vrpoker-plugin",
  "abi_version": 1,
  "capabilities": ["json"],
  "required_host_functions": ["regex_match", "log"]
}
```

---

## 別リポジトリ構成（vrclog-plugins）

```
vrclog-plugins/
├── sdk/
│   ├── go/                   # TinyGo SDK
│   │   ├── vrclog.go         # ヘルパー関数
│   │   └── examples/
│   ├── rust/                 # Rust SDK（将来）
│   └── docs/
├── plugins/
│   ├── official/             # 公式プラグイン
│   │   └── vrpoker/
│   └── community/            # コミュニティプラグイン
├── registry/
│   └── manifest.json         # プラグイン一覧
├── tools/
│   └── build.sh              # ビルドスクリプト
└── docs/
    ├── ABI.md
    └── CONTRIBUTING.md
```

---

## manifest.json スキーマ

```json
{
  "name": "vrpoker-plugin",
  "version": "1.0.0",
  "description": "VRPoker log parser plugin",
  "author": "tatsu020",
  "license": "MIT",
  "homepage": "https://github.com/vrclog/vrpoker-plugin",
  "abi_version": 1,
  "sdk": {
    "language": "tinygo",
    "sdk_version": "0.1.0"
  },
  "required_host_functions": ["regex_match", "log"],
  "vrclog_compatible": ">=0.8.0",
  "checksum": "sha256:e3b0c442..."
}
```

---

## セキュリティ

### Wasmサンドボックス

Wasmはデフォルトで以下の特性を持つ:
- **メモリ安全**: 境界チェックによるバッファオーバーフロー防止
- **Capability-based**: 明示的に許可されたリソースのみアクセス可能
- **ホストリソース隔離**: ファイルシステム、ネットワーク等へのアクセス不可

### Host Function設計原則

1. **最小権限**: 必要最小限のHost Functionのみ提供
2. **副作用の制限**: `regex_match`, `log`, `now_ms` はすべて読み取り専用または限定的
3. **リソース制限**: タイムアウト、メモリ上限で暴走防止
4. **入力検証**: 全てのポインタとサイズを検証

### ReDoS対策の実装

```go
func regexMatch(ctx context.Context, str, pattern string) uint32 {
    // 5msタイムアウト
    ctx, cancel := context.WithTimeout(ctx, 5*time.Millisecond)
    defer cancel()

    re, err := regexCache.Get(pattern)
    if err != nil {
        return 0
    }

    resultCh := make(chan bool, 1)
    go func() {
        resultCh <- re.MatchString(str)
    }()

    select {
    case result := <-resultCh:
        if result {
            return 1
        }
        return 0
    case <-ctx.Done():
        return 0  // タイムアウト
    }
}
```

**将来的な改善案**:
- `regexp2`ライブラリへの移行（RE2互換、タイムアウト内蔵）
- wazeroのfuel機能を使った命令数制限

### リモートURL取得時のセキュリティ

| 対策 | 説明 |
|------|------|
| HTTPS必須 | HTTPは拒否 |
| 信頼リスト | 事前に許可したURLのみ実行 |
| checksum検証 | sha256でバイナリ完全性確認 |
| TOCTOU防止 | temp→検証→rename の原子的操作 |
| 非TTY時checksum必須 | CI/CD環境での安全性確保 |

### キャッシュディレクトリ

```
~/.cache/vrclog/plugins/
├── <sha256>.wasm       # 0600権限
└── manifest.json       # メタデータ

~/.config/vrclog/
└── trust.json          # 0600権限、信頼リスト
```

ディレクトリ権限: 0700

---

## 関連ドキュメント

- [メイン計画](./08-issue2-custom-log-patterns.md)
- [技術調査](./08-issue2-research.md)
- [VRChatログフォーマット仕様](./08-issue2-log-format-spec.md)
- [実装例](./08-issue2-examples.md)
- [セキュリティ](./08-issue2-security.md)
- [Phase 2: Wasm](./08-issue2-phase2-wasm.md)
