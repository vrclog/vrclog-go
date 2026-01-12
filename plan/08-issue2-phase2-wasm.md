# Phase 2: Wasmプラグイン基盤

## 概要

wazeroを使用したWasmプラグインシステムを実装する。
TinyGo/Rust等で開発したプラグインをサンドボックス環境で安全に実行できる。

## 背景

### なぜWasmプラグインが必要か

1. **複雑なロジック**: YAMLパターンでは表現できない状態遷移（VRPokerの「フォールド→勝者決定」等）
2. **サンドボックス**: 任意のコードを安全に実行
3. **言語非依存**: TinyGo、Rust、AssemblyScript等で開発可能
4. **配布容易**: .wasmファイル1つで完結

### wazeroを選択した理由

| ライブラリ | 理由 |
|-----------|------|
| wazero | 純Go、依存ゼロ、Go 1.21+、Arcjet本番実績 |
| Extism | 依存が増える、独自ABI |
| knqyf263/go-plugin | Go 1.24+必須 |

---

## 実装ファイル

### 新規ファイル

| ファイル | 説明 |
|---------|------|
| `internal/wasm/loader.go` | Wasmモジュールのロード、ABI検証 |
| `internal/wasm/parser.go` | WasmParser（Parser実装） |
| `internal/wasm/host.go` | Host Functions（regex_match等） |
| `internal/wasm/cache.go` | 正規表現キャッシュ（LRU） |
| `internal/wasm/errors.go` | エラー型定義 |
| `internal/wasm/*_test.go` | テスト |
| `examples/plugins/vrpoker/` | VRPokerサンプルプラグイン |

### 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `go.mod` | `github.com/tetratelabs/wazero v1.8.0`、`golang.org/x/time` 依存追加 |
| `cmd/vrclog/tail.go` | `--plugin` フラグ追加 |
| `cmd/vrclog/parse.go` | `--plugin` フラグ追加 |

---

## ABI仕様

詳細は [08-issue2-abi-spec.md](./08-issue2-abi-spec.md) を参照。

### 必須Export関数

```c
uint32_t abi_version();           // 現在: 1
uint32_t alloc(uint32_t size);    // メモリ確保（出力バッファ専用）
void free(uint32_t ptr, uint32_t len);  // メモリ解放
// パース実行
// input_ptr, input_len: Host側がWasmメモリに直接書き込んだ入力の位置とサイズ
// 戻り値: 上位32bitがlength、下位32bitがpointer
uint64_t parse_line(uint32_t input_ptr, uint32_t input_len);
```

### Host Functions

```c
// 正規表現（envモジュール）
uint32_t regex_match(str_ptr, str_len, re_ptr, re_len);
uint32_t regex_find_submatch(str_ptr, str_len, re_ptr, re_len, out_ptr, out_len);
void log(level, ptr, len);
uint64_t now_ms();
```

---

## 実装詳細

### loader.go

```go
// internal/wasm/loader.go

package wasm

import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "path/filepath"
    "time"

    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/api"
    "golang.org/x/time/rate"
)

const (
    MaxWasmFileSize = 10 * 1024 * 1024 // 10MB
)

// Config はWasmParserの設定
type Config struct {
    Timeout         time.Duration // parse_line全体（デフォルト: 50ms）
    RegexTimeout    time.Duration // 正規表現1回（デフォルト: 5ms）
    MaxLineLength   int           // 最大入力行長（デフォルト: 8192）
    RegexCacheSize  int           // 正規表現キャッシュ（デフォルト: 100）
    LogRateLimit    rate.Limit    // ログレート制限（デフォルト: 10/秒）
}

func DefaultConfig() *Config {
    return &Config{
        Timeout:         50 * time.Millisecond,
        RegexTimeout:    5 * time.Millisecond,
        MaxLineLength:   8192,
        RegexCacheSize:  100,
        LogRateLimit:    10,
    }
}

// Load はWasmファイルからParserを生成
func Load(ctx context.Context, wasmPath string) (*WasmParser, error) {
    return LoadWithConfig(ctx, wasmPath, DefaultConfig())
}

// LoadWithConfig は設定付きでロード
func LoadWithConfig(ctx context.Context, wasmPath string, cfg *Config) (*WasmParser, error) {
    // ファイルサイズチェック
    info, err := os.Stat(wasmPath)
    if err != nil {
        return nil, fmt.Errorf("stat wasm: %w", err)
    }
    if info.Size() > MaxWasmFileSize {
        return nil, fmt.Errorf("%w: %d bytes (max %d)", ErrFileTooLarge, info.Size(), MaxWasmFileSize)
    }

    wasmBytes, err := os.ReadFile(wasmPath)
    if err != nil {
        return nil, fmt.Errorf("read wasm: %w", err)
    }
    return LoadBytesWithConfig(ctx, wasmBytes, cfg)
}

// LoadBytesWithConfig はバイト列からロード
func LoadBytesWithConfig(ctx context.Context, wasmBytes []byte, cfg *Config) (*WasmParser, error) {
    // wazeroランタイム作成（ディスクキャッシュ付き）
    cacheDir := getCacheDir()
    cache, err := wazero.NewCompilationCacheWithDir(cacheDir)
    if err != nil {
        slog.Warn("failed to create wazero cache", "error", err)
    }

    rtConfig := wazero.NewRuntimeConfig()
    if cache != nil {
        rtConfig = rtConfig.WithCompilationCache(cache)
    }
    rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)

    // WasmParser作成
    parser := &WasmParser{
        rt:          rt,
        timeout:     cfg.Timeout,
        maxLineLen:  cfg.MaxLineLength,
        regexCache:  NewRegexCache(cfg.RegexCacheSize, cfg.RegexTimeout),
        logLimiter:  rate.NewLimiter(cfg.LogRateLimit, int(cfg.LogRateLimit)),
    }

    // Host Functions登録
    _, err = rt.NewHostModuleBuilder("env").
        NewFunctionBuilder().WithFunc(parser.regexMatch).Export("regex_match").
        NewFunctionBuilder().WithFunc(parser.regexFindSubmatch).Export("regex_find_submatch").
        NewFunctionBuilder().WithFunc(parser.hostLog).Export("log").
        NewFunctionBuilder().WithFunc(nowMs).Export("now_ms").
        Instantiate(ctx)
    if err != nil {
        rt.Close(ctx)
        return nil, fmt.Errorf("host module: %w", err)
    }

    // AOTコンパイル
    compiled, err := rt.CompileModule(ctx, wasmBytes)
    if err != nil {
        rt.Close(ctx)
        return nil, fmt.Errorf("compile: %w", err)
    }

    // メモリ制限: 4MiB (64 pages)
    modConfig := wazero.NewModuleConfig().WithMemoryLimitPages(64)
    mod, err := rt.InstantiateModule(ctx, compiled, modConfig)
    if err != nil {
        rt.Close(ctx)
        return nil, fmt.Errorf("instantiate: %w", err)
    }

    // ABI検証
    if err := parser.validateExports(ctx, mod); err != nil {
        mod.Close(ctx)
        rt.Close(ctx)
        return nil, err
    }

    parser.mod = mod
    parser.alloc = mod.ExportedFunction("alloc")
    parser.free = mod.ExportedFunction("free")
    parser.parseFn = mod.ExportedFunction("parse_line")
    parser.mem = mod.Memory()

    return parser, nil
}

func getCacheDir() string {
    // os.UserCacheDir()を使用（Windowsでは%LocalAppData%を返す）
    if cacheDir, err := os.UserCacheDir(); err == nil {
        return filepath.Join(cacheDir, "vrclog", "wasm")
    }
    // フォールバック: XDG_CACHE_HOME または ~/.cache
    if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
        return filepath.Join(dir, "vrclog", "wasm")
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".cache", "vrclog", "wasm")
}
```

### parser.go

```go
// internal/wasm/parser.go

package wasm

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/api"
    "golang.org/x/time/rate"

    "github.com/vrclog/vrclog-go/pkg/vrclog"
    "github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

const (
    INPUT_REGION = 0x10000 // 入力固定領域（64KB〜72KB）
)

// WasmParser はWasmプラグインをラップしたパーサー
type WasmParser struct {
    rt         wazero.Runtime
    mod        api.Module
    mem        api.Memory
    alloc      api.Function
    free       api.Function
    parseFn    api.Function
    timeout    time.Duration
    maxLineLen int
    regexCache *RegexCache
    logLimiter *rate.Limiter
}

// ParseLine はParserインターフェースを実装
// ctxからタイムアウトを取得し、parse_line実行時に適用する
func (p *WasmParser) ParseLine(ctx context.Context, line string) (vrclog.ParseResult, error) {
    // Contextからタイムアウトを設定（引数のctxにタイムアウトがなければデフォルト値を使用）
    _, hasDeadline := ctx.Deadline()
    if !hasDeadline {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, p.timeout)
        defer cancel()
    }

    // 入力行長チェック
    if len(line) > p.maxLineLen {
        return vrclog.ParseResult{}, fmt.Errorf("%w: %d bytes (max %d)",
            ErrInputTooLarge, len(line), p.maxLineLen)
    }

    // リクエストJSON作成
    req := struct {
        Line      string `json:"line"`
        Timestamp string `json:"timestamp,omitempty"`
    }{
        Line: line,
    }
    reqBytes, err := json.Marshal(req)
    if err != nil {
        return vrclog.ParseResult{}, fmt.Errorf("marshal request: %w", err)
    }

    // 入力を固定領域（INPUT_REGION = 0x10000）に直接書き込み
    if ok := p.mem.Write(INPUT_REGION, reqBytes); !ok {
        return vrclog.ParseResult{}, fmt.Errorf("write input failed")
    }

    // パース実行（入力は固定領域なのでallocを使わない）
    outRes, err := p.parseFn.Call(ctx, INPUT_REGION, uint64(len(reqBytes)))
    if err != nil {
        return vrclog.ParseResult{}, fmt.Errorf("plugin error: %w", err)
    }

    // 結果を読み取り
    result := outRes[0]
    outPtr := uint32(result & 0xFFFFFFFF)
    outLen := uint32(result >> 32)

    if outPtr == 0 || outLen == 0 {
        return vrclog.ParseResult{Matched: false}, nil
    }

    outBytes, ok := p.mem.Read(outPtr, outLen)
    if !ok {
        return vrclog.ParseResult{}, fmt.Errorf("read output failed")
    }

    // レスポンスをパース
    var resp struct {
        OK     bool          `json:"ok"`
        Events []event.Event `json:"events"`
        Error  string        `json:"error,omitempty"`
    }
    if err := json.Unmarshal(outBytes, &resp); err != nil {
        return vrclog.ParseResult{}, fmt.Errorf("unmarshal response: %w", err)
    }

    if !resp.OK {
        return vrclog.ParseResult{}, fmt.Errorf("plugin returned error: %s", resp.Error)
    }

    matched := len(resp.Events) > 0
    return vrclog.ParseResult{Events: resp.Events, Matched: matched}, nil
}

// Close はリソースを解放
func (p *WasmParser) Close(ctx context.Context) error {
    if p.mod != nil {
        if err := p.mod.Close(ctx); err != nil {
            return err
        }
    }
    if p.rt != nil {
        return p.rt.Close(ctx)
    }
    return nil
}

// validateExports はABI必須関数の存在を確認
func (p *WasmParser) validateExports(ctx context.Context, mod api.Module) error {
    abiVer := mod.ExportedFunction("abi_version")
    if abiVer == nil {
        return fmt.Errorf("%w: abi_version not found", ErrInvalidABI)
    }

    res, err := abiVer.Call(ctx)
    if err != nil {
        return fmt.Errorf("abi_version call failed: %w", err)
    }
    if res[0] != 1 {
        return fmt.Errorf("%w: got version %d, expected 1", ErrInvalidABI, res[0])
    }

    required := []string{"alloc", "free", "parse_line"}
    for _, name := range required {
        if mod.ExportedFunction(name) == nil {
            return fmt.Errorf("%w: %s not found", ErrInvalidABI, name)
        }
    }

    return nil
}
```

### host.go（セキュリティ強化版）

```go
// internal/wasm/host.go

package wasm

import (
    "context"
    "encoding/json"
    "log/slog"
    "strings"
    "time"

    "github.com/tetratelabs/wazero/api"
)

// regex_match: ReDoS対策付き
func (p *WasmParser) regexMatch(ctx context.Context, mod api.Module, strPtr, strLen, rePtr, reLen uint32) uint32 {
    mem := mod.Memory()

    // パターン長チェック（512バイト上限）
    if reLen > 512 {
        return 0
    }

    strBytes, ok := mem.Read(strPtr, strLen)
    if !ok {
        return 0
    }
    reBytes, ok := mem.Read(rePtr, reLen)
    if !ok {
        return 0
    }

    // UTF-8検証
    str := strings.ToValidUTF8(string(strBytes), "\uFFFD")
    pattern := strings.ToValidUTF8(string(reBytes), "\uFFFD")

    re, err := p.regexCache.Get(pattern)
    if err != nil {
        return 0
    }

    // タイムアウト付きマッチ（5ms）
    ctx, cancel := context.WithTimeout(ctx, p.regexCache.timeout)
    defer cancel()

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
        slog.Debug("regex match timeout", "pattern", pattern[:min(50, len(pattern))])
        return 0
    }
}

// regex_find_submatch: Wasm側バッファ渡し方式
func (p *WasmParser) regexFindSubmatch(ctx context.Context, mod api.Module,
    strPtr, strLen, rePtr, reLen, outBufPtr, outBufLen uint32) uint32 {

    mem := mod.Memory()

    if reLen > 512 {
        return 0
    }
    if outBufLen > 4096 {
        outBufLen = 4096
    }

    strBytes, ok := mem.Read(strPtr, strLen)
    if !ok {
        return 0
    }
    reBytes, ok := mem.Read(rePtr, reLen)
    if !ok {
        return 0
    }

    str := strings.ToValidUTF8(string(strBytes), "\uFFFD")
    pattern := strings.ToValidUTF8(string(reBytes), "\uFFFD")

    re, err := p.regexCache.Get(pattern)
    if err != nil {
        return 0
    }

    ctx, cancel := context.WithTimeout(ctx, p.regexCache.timeout)
    defer cancel()

    type result struct{ matches []string }
    resultCh := make(chan *result, 1)
    go func() {
        resultCh <- &result{matches: re.FindStringSubmatch(str)}
    }()

    var matches []string
    select {
    case r := <-resultCh:
        matches = r.matches
    case <-ctx.Done():
        return 0
    }

    if matches == nil {
        return 0
    }

    jsonBytes, err := json.Marshal(matches)
    if err != nil || uint32(len(jsonBytes)) > outBufLen {
        return 0
    }

    if ok := mem.Write(outBufPtr, jsonBytes); !ok {
        return 0
    }

    return uint32(len(jsonBytes))
}

// log: レート制限 + サイズ制限
func (p *WasmParser) hostLog(ctx context.Context, mod api.Module, level, ptr, msgLen uint32) {
    if !p.logLimiter.Allow() {
        return
    }

    mem := mod.Memory()
    if msgLen > 256 {
        msgLen = 256
    }

    msgBytes, ok := mem.Read(ptr, msgLen)
    if !ok {
        return
    }

    msg := strings.ToValidUTF8(string(msgBytes), "\uFFFD")
    slog.Log(ctx, levelToSlogLevel(level), "[PLUGIN] "+msg)
}

func nowMs(ctx context.Context) uint64 {
    return uint64(time.Now().UnixMilli())
}
```

### cache.go

```go
// internal/wasm/cache.go

package wasm

import (
    "regexp"
    "sync"
    "time"
)

// RegexCache はLRU正規表現キャッシュ
type RegexCache struct {
    mu      sync.RWMutex
    cache   map[string]*cacheEntry
    order   []string
    maxSize int
    timeout time.Duration
}

type cacheEntry struct {
    re  *regexp.Regexp
    err error
}

func NewRegexCache(maxSize int, timeout time.Duration) *RegexCache {
    return &RegexCache{
        cache:   make(map[string]*cacheEntry),
        order:   make([]string, 0, maxSize),
        maxSize: maxSize,
        timeout: timeout,
    }
}

func (c *RegexCache) Get(pattern string) (*regexp.Regexp, error) {
    // Read path (fast)
    c.mu.RLock()
    if entry, ok := c.cache[pattern]; ok {
        c.mu.RUnlock()
        return entry.re, entry.err
    }
    c.mu.RUnlock()

    // Write path
    c.mu.Lock()
    defer c.mu.Unlock()

    // Double-check
    if entry, ok := c.cache[pattern]; ok {
        return entry.re, entry.err
    }

    // Compile (errors are cached too)
    re, err := regexp.Compile(pattern)
    entry := &cacheEntry{re: re, err: err}

    // LRU eviction
    if len(c.cache) >= c.maxSize {
        oldest := c.order[0]
        delete(c.cache, oldest)
        c.order = c.order[1:]
    }

    c.cache[pattern] = entry
    c.order = append(c.order, pattern)

    return re, err
}
```

### errors.go

```go
// internal/wasm/errors.go

package wasm

import "errors"

var (
    // ErrInvalidABI はABI仕様違反
    ErrInvalidABI = errors.New("invalid wasm ABI")

    // ErrPluginTimeout はparse_lineタイムアウト
    ErrPluginTimeout = errors.New("plugin timeout")

    // ErrInputTooLarge は入力行が長すぎる
    ErrInputTooLarge = errors.New("input too large")

    // ErrFileTooLarge はWasmファイルが大きすぎる
    ErrFileTooLarge = errors.New("wasm file too large")
)
```

---

## CLI統合

```go
// cmd/vrclog/tail.go に追加

var (
    tailPluginFlags []string
    // ...
)

func init() {
    tailCmd.Flags().StringArrayVar(&tailPluginFlags, "plugin", nil,
        "Wasm plugin file (can be specified multiple times)")
    // ...
}

func buildParser(ctx context.Context, patternFiles, pluginFiles []string) (vrclog.Parser, error) {
    if len(patternFiles) == 0 && len(pluginFiles) == 0 {
        return nil, nil
    }

    parsers := []vrclog.Parser{vrclog.DefaultParser{}}

    // YAMLパターン
    for _, path := range patternFiles {
        rp, err := pattern.NewRegexParserFromFile(path)
        if err != nil {
            return nil, fmt.Errorf("load pattern file %s: %w", path, err)
        }
        parsers = append(parsers, rp)
    }

    // Wasmプラグイン
    for _, path := range pluginFiles {
        wp, err := wasm.Load(ctx, path)
        if err != nil {
            return nil, fmt.Errorf("load plugin %s: %w", path, err)
        }
        parsers = append(parsers, wp)
    }

    return &vrclog.ParserChain{
        Mode:    vrclog.ChainAll,
        Parsers: parsers,
    }, nil
}
```

---

## VRPokerサンプルプラグイン

### examples/plugins/vrpoker/main.go

```go
//go:build tinygo.wasm

package main

import "unsafe"

//go:wasmimport env regex_find_submatch
func regex_find_submatch(strPtr, strLen, rePtr, reLen, outPtr, outLen uint32) uint32

// Bump allocator
var heap [1 << 20]byte
var heapOff uintptr

//export alloc
func alloc(size uint32) uint32 {
    off := (heapOff + 7) &^ 7
    if off+uintptr(size) > uintptr(len(heap)) {
        return 0
    }
    ptr := unsafe.Pointer(&heap[off])
    heapOff = off + uintptr(size)
    return uint32(uintptr(ptr))
}

//export free
func free(ptr, size uint32) {}

//export abi_version
func abi_version() uint32 { return 1 }

//export parse_line
func parse_line(inputPtr, inputLen uint32) uint64 {
    // ★★★ 重要: 呼び出し開始時にヒープリセット ★★★
    // 入力はHost側が固定領域（INPUT_REGION = 0x10000）に書き込み済みなので破壊されない
    heapOff = 0

    // リクエストJSONからlineを抽出
    input := ptrToBytes(inputPtr, inputLen)
    line := extractLine(input)
    if line == nil {
        return writeBytes([]byte(`{"ok":true,"events":[]}`))
    }

    // Check hole cards pattern
    re := []byte(`\[Seat\]: Draw Local Hole Cards: (\w+), (\w+)`)
    outBuf := alloc(4096)
    written := regex_find_submatch(bytesToPtr(line), uint32(len(line)),
        bytesToPtr(re), uint32(len(re)), outBuf, 4096)

    if written > 0 {
        // Build response with matches
        return writeBytes([]byte(`{"ok":true,"events":[{"type":"poker_hole_cards","data":{}}]}`))
    }

    return writeBytes([]byte(`{"ok":true,"events":[]}`))
}

func main() {}
```

### examples/plugins/vrpoker/Makefile

```makefile
.PHONY: build clean

build:
	tinygo build -o vrpoker.wasm -target=wasi -no-debug -scheduler=none main.go

clean:
	rm -f vrpoker.wasm
```

---

## テスト計画

### テストデータ

```
internal/wasm/testdata/
├── minimal.wasm       # 最小限の実装
├── echo.wasm          # 入力をそのまま返す
├── slow.wasm          # タイムアウトテスト用
└── panic.wasm         # クラッシュリカバリ確認
```

### ユニットテスト

```go
// internal/wasm/parser_test.go

func TestWasmParser_ParseLine(t *testing.T) {
    ctx := context.Background()

    parser, err := Load(ctx, "testdata/minimal.wasm")
    require.NoError(t, err)
    defer parser.Close(ctx)

    result, err := parser.ParseLine(ctx, "2024.01.15 00:00:00 test line")
    require.NoError(t, err)
    // アサーション
}

func TestWasmParser_Timeout(t *testing.T) {
    ctx := context.Background()

    parser, err := Load(ctx, "testdata/slow.wasm")
    require.NoError(t, err)
    defer parser.Close(ctx)

    _, err = parser.ParseLine(ctx, "test")
    assert.ErrorIs(t, err, ErrPluginTimeout)
}
```

### CIでのTinyGoビルド

```yaml
# .github/workflows/test.yml
- name: Install TinyGo
  uses: nicois/tinygo-action@v1
  with:
    version: "0.32.0"

- name: Build test Wasm
  run: |
    cd internal/wasm/testdata
    for f in *.go; do
      tinygo build -o "${f%.go}.wasm" -target=wasi -no-debug "$f"
    done
```

---

## リソース制限

| リソース | デフォルト | 変更可能 |
|---------|-----------|---------|
| parse_lineタイムアウト | 50ms | `--plugin-timeout` |
| 正規表現タイムアウト | 5ms | - |
| メモリ | 4MiB | `--plugin-memory` |
| 正規表現キャッシュ | 100パターン | - |
| 入力行最大長 | 8192バイト | - |
| 出力イベント最大数 | 16個 | - |
| 正規表現パターン長 | 512バイト | - |
| logサイズ | 256バイト/回 | - |
| logレート | 10回/秒 | - |
| Wasmファイルサイズ | 10MB | - |

---

## チェックリスト

- [ ] `go get github.com/tetratelabs/wazero@v1.8.0`
- [ ] `go get golang.org/x/time`
- [ ] `internal/wasm/` ディレクトリ作成
- [ ] `loader.go` 実装
- [ ] `parser.go` 実装
- [ ] `host.go` 実装
- [ ] `cache.go` 実装
- [ ] `errors.go` 実装
- [ ] テストデータ作成
- [ ] ユニットテスト作成
- [ ] `--plugin` フラグ追加
- [ ] VRPokerサンプル作成
- [ ] CIにTinyGoビルド追加

---

## 関連ドキュメント

- [メイン計画](./08-issue2-custom-log-patterns.md)
- [ABI仕様](./08-issue2-abi-spec.md)
- [実装例](./08-issue2-examples.md)
- [セキュリティ](./08-issue2-security.md)
