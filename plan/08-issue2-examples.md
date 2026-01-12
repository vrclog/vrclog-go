# Issue #2: 実装コード例

## Parser Interface

### 基本インターフェース

```go
// pkg/vrclog/parser.go

// ParseResult は ParseLine の結果を表す
type ParseResult struct {
    Events  []Event
    Matched bool  // パターンにマッチしたか（Events==nilでもtrueの場合あり）
}

// Parser はログ行をパースするインターフェース
type Parser interface {
    ParseLine(ctx context.Context, line string) (ParseResult, error)
}

// ParserFunc は関数をParserとして使うアダプタ
type ParserFunc func(context.Context, string) (ParseResult, error)

func (f ParserFunc) ParseLine(ctx context.Context, line string) (ParseResult, error) {
    return f(ctx, line)
}
```

### DefaultParser

```go
// pkg/vrclog/parser_default.go

// DefaultParser は既存のパーサーをラップ
type DefaultParser struct{}

func (DefaultParser) ParseLine(ctx context.Context, line string) (ParseResult, error) {
    ev, err := parser.Parse(line)
    if err != nil {
        return ParseResult{}, err
    }
    if ev == nil {
        return ParseResult{Matched: false}, nil
    }
    return ParseResult{Events: []Event{*ev}, Matched: true}, nil
}
```

### ParserChain

```go
// pkg/vrclog/parser.go

// ChainMode はチェーン動作モード
type ChainMode int

const (
    ChainAll             ChainMode = iota // 全パーサー実行、結果を結合（デフォルト）
    ChainFirst                            // 最初にマッチで終了
    ChainContinueOnError                  // エラー発生パーサーをスキップして継続
)

// ParserChain は複数パーサーをチェーン実行
type ParserChain struct {
    Mode    ChainMode
    Parsers []Parser
}

func (c *ParserChain) ParseLine(ctx context.Context, line string) (ParseResult, error) {
    var allEvents []Event
    var errs []error
    anyMatched := false

    for _, p := range c.Parsers {
        result, err := p.ParseLine(ctx, line)
        if err != nil {
            if c.Mode == ChainContinueOnError {
                errs = append(errs, err)
                continue
            }
            return ParseResult{}, err
        }
        if result.Matched {
            anyMatched = true
            allEvents = append(allEvents, result.Events...)
            if c.Mode == ChainFirst {
                return ParseResult{Events: allEvents, Matched: true}, nil
            }
        }
    }

    // ChainContinueOnErrorでエラーがあれば最後にまとめて返す
    if len(errs) > 0 {
        return ParseResult{Events: allEvents, Matched: anyMatched}, errors.Join(errs...)
    }

    return ParseResult{Events: allEvents, Matched: anyMatched}, nil
}
```

---

## Host側（wazero）- セキュリティ強化版

### WasmParser構造体

```go
// internal/wasm/parser.go

package wasm

import (
    "context"
    "encoding/json"
    "fmt"
    "log/slog"
    "os"
    "strings"
    "sync"
    "time"

    "github.com/tetratelabs/wazero"
    "github.com/tetratelabs/wazero/api"
    "github.com/vrclog/vrclog-go/pkg/vrclog"
    "golang.org/x/time/rate"
)

// WasmParser はWasmプラグインをParserとして扱う
type WasmParser struct {
    rt      wazero.Runtime
    mod     api.Module
    alloc   api.Function
    free    api.Function
    parseFn api.Function
    mem     api.Memory

    // セキュリティ設定
    timeout    time.Duration
    maxLineLen int

    // Host Function用
    regexCache *RegexCache
    logLimiter *rate.Limiter
}

// Config はWasmParserの設定
type Config struct {
    Timeout         time.Duration // parse_line全体のタイムアウト（デフォルト: 50ms）
    RegexTimeout    time.Duration // 正規表現1回のタイムアウト（デフォルト: 5ms）
    MaxLineLength   int           // 最大入力行長（デフォルト: 8192）
    RegexCacheSize  int           // 正規表現キャッシュサイズ（デフォルト: 100）
    LogRateLimit    rate.Limit    // ログレート制限（デフォルト: 10/秒）
}

// DefaultConfig はデフォルト設定を返す
func DefaultConfig() *Config {
    return &Config{
        Timeout:         50 * time.Millisecond,
        RegexTimeout:    5 * time.Millisecond,
        MaxLineLength:   8192,
        RegexCacheSize:  100,
        LogRateLimit:    10, // 10回/秒
    }
}
```

### ローダー

```go
// internal/wasm/loader.go

// Load はWasmファイルからParserを生成
func Load(ctx context.Context, wasmPath string) (*WasmParser, error) {
    return LoadWithConfig(ctx, wasmPath, DefaultConfig())
}

// LoadWithConfig は設定付きでWasmファイルからParserを生成
func LoadWithConfig(ctx context.Context, wasmPath string, cfg *Config) (*WasmParser, error) {
    // ファイルサイズチェック（10MB上限）
    info, err := os.Stat(wasmPath)
    if err != nil {
        return nil, fmt.Errorf("stat wasm: %w", err)
    }
    if info.Size() > 10*1024*1024 {
        return nil, fmt.Errorf("wasm file too large: %d bytes (max 10MB)", info.Size())
    }

    wasmBytes, err := os.ReadFile(wasmPath)
    if err != nil {
        return nil, fmt.Errorf("read wasm: %w", err)
    }
    return LoadBytesWithConfig(ctx, wasmBytes, cfg)
}

// LoadBytesWithConfig はWasmバイト列からParserを生成
func LoadBytesWithConfig(ctx context.Context, wasmBytes []byte, cfg *Config) (*WasmParser, error) {
    // wazeroランタイムキャッシュ設定
    cacheDir := getCacheDir()
    cache, err := wazero.NewCompilationCacheWithDir(cacheDir)
    if err != nil {
        // キャッシュ作成失敗は警告のみ
        slog.Warn("failed to create wazero cache", "error", err)
        cache = nil
    }

    rtConfig := wazero.NewRuntimeConfig()
    if cache != nil {
        rtConfig = rtConfig.WithCompilationCache(cache)
    }
    rt := wazero.NewRuntimeWithConfig(ctx, rtConfig)

    // プラグイン固有のセキュリティ設定
    parser := &WasmParser{
        rt:          rt,
        timeout:     cfg.Timeout,
        maxLineLen:  cfg.MaxLineLength,
        regexCache:  NewRegexCache(cfg.RegexCacheSize, cfg.RegexTimeout),
        logLimiter:  rate.NewLimiter(cfg.LogRateLimit, int(cfg.LogRateLimit)),
    }

    // Host Function登録
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

    mod, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
    if err != nil {
        rt.Close(ctx)
        return nil, fmt.Errorf("instantiate: %w", err)
    }

    // 必須関数取得・検証
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

func (p *WasmParser) validateExports(ctx context.Context, mod api.Module) error {
    // 必須関数チェック
    required := []string{"abi_version", "alloc", "free", "parse_line"}
    for _, name := range required {
        if mod.ExportedFunction(name) == nil {
            return fmt.Errorf("%w: %s", ErrMissingExport, name)
        }
    }

    // ABIバージョンチェック
    abiVersion := mod.ExportedFunction("abi_version")
    results, err := abiVersion.Call(ctx)
    if err != nil {
        return fmt.Errorf("call abi_version: %w", err)
    }
    version := uint32(results[0])
    if version != 1 {
        return &ABIVersionError{Got: version, Expected: 1}
    }

    return nil
}
```

### Host Functions（セキュリティ強化版）

```go
// internal/wasm/host.go

package wasm

import (
    "context"
    "encoding/json"
    "log/slog"
    "regexp"
    "strings"
    "sync"
    "time"

    "github.com/tetratelabs/wazero/api"
)

// regex_match: ReDoS対策付き
func (p *WasmParser) regexMatch(ctx context.Context, mod api.Module, strPtr, strLen, rePtr, reLen uint32) uint32 {
    mem := mod.Memory()

    // パターン長チェック
    if reLen > 512 {
        slog.Debug("regex pattern too long", "len", reLen)
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

    // キャッシュから正規表現取得（コンパイルエラーも処理）
    re, err := p.regexCache.Get(pattern)
    if err != nil {
        slog.Debug("regex compile error", "pattern", pattern, "error", err)
        return 0
    }

    // タイムアウト付きマッチ実行
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
        slog.Debug("regex match timeout", "pattern", pattern)
        return 0
    }
}

// regex_find_submatch: Wasm側バッファ渡し方式
func (p *WasmParser) regexFindSubmatch(ctx context.Context, mod api.Module,
    strPtr, strLen, rePtr, reLen, outBufPtr, outBufLen uint32) uint32 {

    mem := mod.Memory()

    // パターン長チェック
    if reLen > 512 {
        return 0
    }

    // 出力バッファサイズチェック（4KB上限）
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

    // タイムアウト付きマッチ実行
    ctx, cancel := context.WithTimeout(ctx, p.regexCache.timeout)
    defer cancel()

    type result struct {
        matches []string
    }
    resultCh := make(chan *result, 1)
    go func() {
        matches := re.FindStringSubmatch(str)
        resultCh <- &result{matches: matches}
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

    // JSON配列として結果を生成
    jsonBytes, err := json.Marshal(matches)
    if err != nil {
        return 0
    }

    // サイズチェック
    if uint32(len(jsonBytes)) > outBufLen {
        slog.Debug("regex result too large", "size", len(jsonBytes), "max", outBufLen)
        return 0
    }

    // Wasmメモリに書き込み
    ok = mem.Write(outBufPtr, jsonBytes)
    if !ok {
        return 0
    }

    return uint32(len(jsonBytes))
}

// log: レート制限 + サイズ制限
func (p *WasmParser) hostLog(ctx context.Context, mod api.Module, level, ptr, msgLen uint32) {
    // レート制限チェック
    if !p.logLimiter.Allow() {
        return
    }

    mem := mod.Memory()

    // サイズ制限（256バイト）
    if msgLen > 256 {
        msgLen = 256
    }

    msgBytes, ok := mem.Read(ptr, msgLen)
    if !ok {
        return
    }

    // UTF-8サニタイズ
    msg := strings.ToValidUTF8(string(msgBytes), "\uFFFD")

    levelStr := []string{"DEBUG", "INFO", "WARN", "ERROR"}
    if int(level) < len(levelStr) {
        slog.Log(ctx, levelToSlogLevel(level), "[PLUGIN] "+msg)
    }
}

func levelToSlogLevel(level uint32) slog.Level {
    switch level {
    case 0:
        return slog.LevelDebug
    case 1:
        return slog.LevelInfo
    case 2:
        return slog.LevelWarn
    case 3:
        return slog.LevelError
    default:
        return slog.LevelInfo
    }
}

func nowMs(ctx context.Context) uint64 {
    return uint64(time.Now().UnixMilli())
}
```

### 正規表現キャッシュ

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
    order   []string // LRU順序
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
    // Read-lock for cache hit
    c.mu.RLock()
    if entry, ok := c.cache[pattern]; ok {
        c.mu.RUnlock()
        return entry.re, entry.err
    }
    c.mu.RUnlock()

    // Write-lock for cache miss
    c.mu.Lock()
    defer c.mu.Unlock()

    // Double-check
    if entry, ok := c.cache[pattern]; ok {
        return entry.re, entry.err
    }

    // コンパイル（エラーもキャッシュ）
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

### ParseLine実装

```go
// internal/wasm/parser.go

// ParseLine はParserインターフェースを実装
func (p *WasmParser) ParseLine(ctx context.Context, line string) (vrclog.ParseResult, error) {
    // 入力長チェック
    if len(line) > p.maxLineLen {
        return vrclog.ParseResult{}, fmt.Errorf("line too long: %d bytes (max %d)", len(line), p.maxLineLen)
    }

    // Contextからタイムアウトを設定（引数のctxにタイムアウトがなければデフォルト値を使用）
    _, hasDeadline := ctx.Deadline()
    if !hasDeadline {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, p.timeout)
        defer cancel()
    }

    // panic recovery
    defer func() {
        if r := recover(); r != nil {
            slog.Error("plugin panic", "recover", r)
        }
    }()

    // Request JSON作成
    req := map[string]interface{}{
        "line": line,
    }
    reqBytes, err := json.Marshal(req)
    if err != nil {
        return vrclog.ParseResult{}, err
    }

    // 入力を固定領域（INPUT_REGION = 0x10000）に直接書き込み
    const INPUT_REGION = 0x10000
    if ok := p.mem.Write(INPUT_REGION, reqBytes); !ok {
        return vrclog.ParseResult{}, fmt.Errorf("write input failed")
    }

    // パース実行（入力は固定領域なのでallocを使わない）
    outRes, err := p.parseFn.Call(ctx, INPUT_REGION, uint64(len(reqBytes)))
    if err != nil {
        if ctx.Err() != nil {
            return vrclog.ParseResult{}, ErrPluginTimeout
        }
        return vrclog.ParseResult{}, fmt.Errorf("parse_line: %w", err)
    }

    // 結果読み取り
    packed := outRes[0]
    outPtr := uint32(packed)
    outLen := uint32(packed >> 32)
    if outPtr == 0 || outLen == 0 {
        return vrclog.ParseResult{Matched: false}, nil
    }
    defer p.free.Call(ctx, uint64(outPtr), uint64(outLen))

    outBytes, ok := p.mem.Read(outPtr, outLen)
    if !ok {
        return vrclog.ParseResult{}, fmt.Errorf("read output failed")
    }

    // Response解析
    var resp struct {
        OK     bool           `json:"ok"`
        Events []vrclog.Event `json:"events"`
        Code   string         `json:"code"`
        Msg    string         `json:"message"`
    }
    if err := json.Unmarshal(outBytes, &resp); err != nil {
        return vrclog.ParseResult{}, fmt.Errorf("unmarshal response: %w", err)
    }

    if !resp.OK {
        return vrclog.ParseResult{}, &PluginResponseError{Code: resp.Code, Message: resp.Msg}
    }

    return vrclog.ParseResult{
        Events:  resp.Events,
        Matched: len(resp.Events) > 0,
    }, nil
}

// Close はリソースを解放
func (p *WasmParser) Close(ctx context.Context) error {
    if p.mod != nil {
        p.mod.Close(ctx)
    }
    if p.rt != nil {
        p.rt.Close(ctx)
    }
    return nil
}
```

---

## TinyGoプラグイン例（VRPoker - Bump Allocatorリセット対応版）

```go
//go:build tinygo.wasm

package main

import (
    "unsafe"
)

//go:wasmimport env regex_match
func regex_match(strPtr, strLen, rePtr, reLen uint32) uint32

//go:wasmimport env regex_find_submatch
func regex_find_submatch(strPtr, strLen, rePtr, reLen, outBufPtr, outBufLen uint32) uint32

//go:wasmimport env log
func host_log(level, ptr, msgLen uint32)

// Bump allocator
var heap [1 << 20]byte // 1MiB
var heapOff uintptr

//export alloc
func alloc(size uint32) uint32 {
    off := (heapOff + 7) &^ 7 // 8-byte alignment
    if off+uintptr(size) > uintptr(len(heap)) {
        return 0 // OOM
    }
    ptr := unsafe.Pointer(&heap[off])
    heapOff = off + uintptr(size)
    return uint32(uintptr(ptr))
}

//export free
func free(ptr, size uint32) {
    // bump allocator: no-op
}

//export abi_version
func abi_version() uint32 { return 1 }

//export abi_capabilities
func abi_capabilities() uint64 {
    return writeBytes([]byte(`["json"]`))
}

// ログパターン定義
type pattern struct {
    regex     string
    eventType string
}

var patterns = []pattern{
    // ホールカード
    {`\[Seat\]: Draw Local Hole Cards: (\w+), (\w+)`, "poker_hole_cards"},
    // フロップ
    {`\[Dealer\]: Flop: (\w+), (\w+), (\w+)`, "poker_flop"},
    // ターン
    {`\[Dealer\]: Turn: (\w+)`, "poker_turn"},
    // リバー
    {`\[Dealer\]: River: (\w+)`, "poker_river"},
    // 勝者
    {`\[PotManager\]: .* player (\d+) won (\d+)`, "poker_winner"},
}

//export parse_line
func parse_line(inputPtr, inputLen uint32) uint64 {
    // ★★★ 重要: 呼び出し開始時にヒープリセット ★★★
    // 入力はHost側が固定領域（例: 0x10000〜）に書き込み済みなので破壊されない
    heapOff = 0

    // リクエストJSONからlineを抽出
    input := ptrToBytes(inputPtr, inputLen)
    line := extractLine(input)
    if line == nil {
        return writeError("PARSE_ERROR", "failed to extract line from input")
    }

    linePtr := bytesToPtr(line)
    lineLen := uint32(len(line))

    // regex_find_submatch用の出力バッファを確保（4KB）
    outBufPtr := alloc(4096)
    if outBufPtr == 0 {
        return writeError("INTERNAL_ERROR", "failed to allocate output buffer")
    }

    // 各パターンでマッチを試行
    for _, p := range patterns {
        rePtr := bytesToPtr([]byte(p.regex))
        reLen := uint32(len(p.regex))

        written := regex_find_submatch(linePtr, lineLen, rePtr, reLen, outBufPtr, 4096)
        if written > 0 {
            // マッチした
            matches := ptrToBytes(outBufPtr, written)
            return buildSuccessResponse(p.eventType, matches)
        }
    }

    // マッチなし
    return writeBytes([]byte(`{"ok":true,"events":[]}`))
}

func extractLine(input []byte) []byte {
    // {"line": "..."} から line の値を抽出
    prefix := []byte(`"line":"`)
    start := -1
    for i := 0; i <= len(input)-len(prefix); i++ {
        match := true
        for j := 0; j < len(prefix); j++ {
            if input[i+j] != prefix[j] {
                match = false
                break
            }
        }
        if match {
            start = i + len(prefix)
            break
        }
    }
    if start == -1 {
        return nil
    }

    // 終端の " を探す（エスケープ考慮なし - 簡易実装）
    end := start
    for end < len(input) && input[end] != '"' {
        end++
    }
    if end >= len(input) {
        return nil
    }

    return input[start:end]
}

func buildSuccessResponse(eventType string, matchesJSON []byte) uint64 {
    // matchesJSONをパースしてdataを構築
    // 簡易実装: matchesJSONは["full", "group1", "group2", ...]形式

    // JSONを手動で構築（TinyGoではencoding/jsonが使えないため）
    response := []byte(`{"ok":true,"events":[{"type":"` + eventType + `","data":{`)

    // matchesからgroup1, group2等を抽出してdataに追加
    // (完全な実装ではmatchesJSONをパースする必要あり)
    response = append(response, []byte(`}}]}`)...)

    return writeBytes(response)
}

func ptrToBytes(ptr, size uint32) []byte {
    return unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), int(size))
}

func bytesToPtr(data []byte) uint32 {
    ptr := alloc(uint32(len(data)))
    if ptr == 0 {
        return 0
    }
    copy(ptrToBytes(ptr, uint32(len(data))), data)
    return ptr
}

func writeBytes(data []byte) uint64 {
    ptr := bytesToPtr(data)
    return (uint64(len(data)) << 32) | uint64(ptr)
}

func writeError(code, msg string) uint64 {
    out := []byte(`{"ok":false,"code":"` + code + `","message":"` + msg + `"}`)
    return writeBytes(out)
}

func main() {}
```

### ビルドコマンド

```bash
tinygo build -o vrpoker.wasm -target=wasi -no-debug -scheduler=none plugin/main.go
```

**ビルドオプションの説明**:
- `-target=wasi`: WASI互換バイナリを生成
- `-no-debug`: デバッグ情報を除去してサイズ削減
- `-scheduler=none`: goroutineスケジューラを無効化（サイズ削減）

---

## RegexParser（YAMLパターン用）

```go
// pkg/vrclog/pattern/regex_parser.go

package pattern

import (
    "regexp"
    "time"

    "github.com/vrclog/vrclog-go/pkg/vrclog"
    "github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

// RegexParser はYAMLパターンを使ったParser実装
type RegexParser struct {
    patterns []*compiledPattern
}

type compiledPattern struct {
    id         string
    eventType  event.Type
    regex      *regexp.Regexp
    groupNames []string
}

// NewRegexParser はPatternFileからRegexParserを作成
func NewRegexParser(pf *PatternFile) (*RegexParser, error) {
    patterns := make([]*compiledPattern, 0, len(pf.Patterns))

    for _, p := range pf.Patterns {
        re, err := regexp.Compile(p.Regex)
        if err != nil {
            return nil, &PatternError{
                ID:      p.ID,
                Field:   "regex",
                Message: err.Error(),
            }
        }

        patterns = append(patterns, &compiledPattern{
            id:         p.ID,
            eventType:  event.Type(p.EventType),
            regex:      re,
            groupNames: re.SubexpNames(),
        })
    }

    return &RegexParser{patterns: patterns}, nil
}

// ParseLine はParserインターフェースを実装
func (p *RegexParser) ParseLine(ctx context.Context, line string) (vrclog.ParseResult, error) {
    // タイムスタンプ抽出
    ts, restOfLine, err := extractTimestamp(line)
    if err != nil {
        // タイムスタンプがない行はスキップ
        return vrclog.ParseResult{Matched: false}, nil
    }

    var events []event.Event

    for _, cp := range p.patterns {
        matches := cp.regex.FindStringSubmatch(restOfLine)
        if matches == nil {
            continue
        }

        // Named capture groupsからDataを構築
        data := make(map[string]string)
        for i, name := range cp.groupNames {
            if name != "" && i < len(matches) {
                data[name] = matches[i]
            }
        }

        events = append(events, event.Event{
            Type:      cp.eventType,
            Timestamp: ts,
            Data:      data,
        })
    }

    return vrclog.ParseResult{
        Events:  events,
        Matched: len(events) > 0,
    }, nil
}

// extractTimestamp はVRChatログ形式のタイムスタンプを抽出
func extractTimestamp(line string) (time.Time, string, error) {
    // VRChatログ形式: 2024.01.15 23:59:59
    if len(line) < 19 {
        return time.Time{}, "", fmt.Errorf("line too short")
    }

    tsStr := line[:19]
    ts, err := time.Parse("2006.01.02 15:04:05", tsStr)
    if err != nil {
        return time.Time{}, "", err
    }

    // タイムスタンプ以降を返す（スペースをスキップ）
    rest := line[19:]
    for len(rest) > 0 && rest[0] == ' ' {
        rest = rest[1:]
    }

    return ts, rest, nil
}
```

---

## YAMLローダー

```go
// pkg/vrclog/pattern/loader.go

package pattern

import (
    "fmt"
    "os"

    "gopkg.in/yaml.v3"
)

const (
    MaxPatternFileSize = 1024 * 1024 // 1MB
    CurrentSchemaVersion = 1
)

// PatternFile はYAMLパターンファイルの構造
type PatternFile struct {
    Version  int       `yaml:"version"`
    Patterns []Pattern `yaml:"patterns"`
}

// Pattern はパターン定義
type Pattern struct {
    ID        string `yaml:"id"`
    EventType string `yaml:"event_type"`
    Regex     string `yaml:"regex"`
}

// PatternError はパターンファイルのエラー
type PatternError struct {
    ID      string
    Field   string
    Message string
}

func (e *PatternError) Error() string {
    return fmt.Sprintf("pattern %q: %s: %s", e.ID, e.Field, e.Message)
}

// Load はYAMLファイルからPatternFileを読み込む
func Load(path string) (*PatternFile, error) {
    // ファイルサイズチェック
    info, err := os.Stat(path)
    if err != nil {
        return nil, fmt.Errorf("stat: %w", err)
    }
    if info.Size() > MaxPatternFileSize {
        return nil, fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), MaxPatternFileSize)
    }

    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read: %w", err)
    }

    return LoadBytes(data)
}

// LoadBytes はYAMLバイト列からPatternFileを読み込む
func LoadBytes(data []byte) (*PatternFile, error) {
    var pf PatternFile
    if err := yaml.Unmarshal(data, &pf); err != nil {
        return nil, fmt.Errorf("unmarshal: %w", err)
    }

    // バリデーション
    if err := pf.Validate(); err != nil {
        return nil, err
    }

    return &pf, nil
}

// Validate はPatternFileを検証
func (pf *PatternFile) Validate() error {
    if pf.Version != CurrentSchemaVersion {
        return fmt.Errorf("unsupported version: %d (expected %d)", pf.Version, CurrentSchemaVersion)
    }

    if len(pf.Patterns) == 0 {
        return fmt.Errorf("no patterns defined")
    }

    ids := make(map[string]bool)
    for i, p := range pf.Patterns {
        if p.ID == "" {
            return &PatternError{ID: fmt.Sprintf("[%d]", i), Field: "id", Message: "required"}
        }
        if ids[p.ID] {
            return &PatternError{ID: p.ID, Field: "id", Message: "duplicate"}
        }
        ids[p.ID] = true

        if p.EventType == "" {
            return &PatternError{ID: p.ID, Field: "event_type", Message: "required"}
        }
        if p.Regex == "" {
            return &PatternError{ID: p.ID, Field: "regex", Message: "required"}
        }
    }

    return nil
}
```

---

## Options実装例

```go
// pkg/vrclog/options.go に追加

// WithParser はカスタムパーサーを設定する
func WithParser(p Parser) WatchOption {
    return func(c *watchConfig) error {
        c.parser = p
        return nil
    }
}

// WithParsers は複数パーサーをChainAllで設定する
func WithParsers(parsers ...Parser) WatchOption {
    return func(c *watchConfig) error {
        c.parser = &ParserChain{
            Mode:    ChainAll,
            Parsers: parsers,
        }
        return nil
    }
}

// WithParseParser はParseWithOptions用のパーサー設定
func WithParseParser(p Parser) ParseOption {
    return func(c *parseConfig) error {
        c.parser = p
        return nil
    }
}
```

---

## エラー定義

```go
// internal/wasm/errors.go

package wasm

import (
    "errors"
    "fmt"
)

// エラー定義
var (
    ErrPluginNotFound  = errors.New("plugin file not found")
    ErrInvalidABI      = errors.New("invalid ABI version")
    ErrMissingExport   = errors.New("missing required export")
    ErrPluginTimeout   = errors.New("plugin execution timeout")
    ErrAllocFailed     = errors.New("plugin memory allocation failed")
    ErrInvalidResponse = errors.New("invalid plugin response")
    ErrFileTooLarge    = errors.New("file too large")
)

// ABIVersionError はABIバージョン不一致エラー
type ABIVersionError struct {
    Got      uint32
    Expected uint32
}

func (e *ABIVersionError) Error() string {
    return fmt.Sprintf("ABI version mismatch: got %d, expected %d", e.Got, e.Expected)
}

func (e *ABIVersionError) Unwrap() error {
    return ErrInvalidABI
}

// PluginResponseError はプラグインが返したエラー
type PluginResponseError struct {
    Code    string
    Message string
}

func (e *PluginResponseError) Error() string {
    return fmt.Sprintf("plugin error [%s]: %s", e.Code, e.Message)
}
```

---

## CLIオプション

```bash
# YAMLパターンファイル指定
vrclog tail --patterns ./vrpoker-patterns.yaml
vrclog parse --patterns ./vrpoker-patterns.yaml log.txt

# 複数パターンファイル（ChainAll）
vrclog tail --patterns ./vrpoker.yaml --patterns ./custom.yaml

# Wasmプラグイン指定（Phase 2）
vrclog tail --plugin ./vrpoker.wasm
vrclog parse --plugin ./vrpoker.wasm log.txt

# 複数プラグイン（標準パーサー + カスタム）
vrclog tail --plugin ./vrpoker.wasm --plugin ./custom.wasm

# 混合（YAML + Wasm）
vrclog tail --patterns ./simple.yaml --plugin ./complex.wasm

# リモートURL（Phase 3）
vrclog tail --plugin https://github.com/.../vrpoker.wasm --checksum sha256:abc123...
```

---

## テスト例

### Parser Interfaceテスト

```go
// pkg/vrclog/parser_test.go

package vrclog_test

import (
    "errors"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/vrclog/vrclog-go/pkg/vrclog"
)

func TestDefaultParser(t *testing.T) {
    p := vrclog.DefaultParser{}

    t.Run("標準ログをパース", func(t *testing.T) {
        result, err := p.ParseLine("2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser")
        require.NoError(t, err)
        assert.True(t, result.Matched)
        assert.Len(t, result.Events, 1)
        assert.Equal(t, vrclog.EventPlayerJoin, result.Events[0].Type)
    })

    t.Run("認識できない行", func(t *testing.T) {
        result, err := p.ParseLine("random text")
        require.NoError(t, err)
        assert.False(t, result.Matched)
        assert.Empty(t, result.Events)
    })
}

func TestParserChain_ChainAll(t *testing.T) {
    // 複数パーサーがマッチした場合、全結果を結合
    p1 := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{
            Events:  []vrclog.Event{{Type: "type1"}},
            Matched: true,
        }, nil
    })
    p2 := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{
            Events:  []vrclog.Event{{Type: "type2"}},
            Matched: true,
        }, nil
    })

    chain := &vrclog.ParserChain{
        Mode:    vrclog.ChainAll,
        Parsers: []vrclog.Parser{p1, p2},
    }

    result, err := chain.ParseLine("test")
    require.NoError(t, err)
    assert.True(t, result.Matched)
    assert.Len(t, result.Events, 2)
}

func TestParserChain_ChainFirst(t *testing.T) {
    called := make([]bool, 2)
    p1 := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        called[0] = true
        return vrclog.ParseResult{
            Events:  []vrclog.Event{{Type: "type1"}},
            Matched: true,
        }, nil
    })
    p2 := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        called[1] = true
        return vrclog.ParseResult{
            Events:  []vrclog.Event{{Type: "type2"}},
            Matched: true,
        }, nil
    })

    chain := &vrclog.ParserChain{
        Mode:    vrclog.ChainFirst,
        Parsers: []vrclog.Parser{p1, p2},
    }

    result, err := chain.ParseLine("test")
    require.NoError(t, err)
    assert.True(t, result.Matched)
    assert.Len(t, result.Events, 1)
    assert.True(t, called[0])
    assert.False(t, called[1]) // p2は呼ばれない
}

func TestParserChain_ChainContinueOnError(t *testing.T) {
    p1 := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{}, errors.New("p1 error")
    })
    p2 := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{
            Events:  []vrclog.Event{{Type: "type2"}},
            Matched: true,
        }, nil
    })

    chain := &vrclog.ParserChain{
        Mode:    vrclog.ChainContinueOnError,
        Parsers: []vrclog.Parser{p1, p2},
    }

    result, err := chain.ParseLine("test")
    // エラーは返るが、p2の結果は含まれる
    assert.Error(t, err)
    assert.True(t, result.Matched)
    assert.Len(t, result.Events, 1)
}
```

### セキュリティテスト

```go
// internal/wasm/security_test.go

func TestReDoS_Timeout(t *testing.T) {
    // ReDoS脆弱なパターン
    cache := NewRegexCache(100, 5*time.Millisecond)

    // 正常なパターンはキャッシュできる
    _, err := cache.Get(`\w+`)
    require.NoError(t, err)

    // 複雑なパターン + 長い入力でタイムアウト確認
    pattern := `(a+)+$`
    input := strings.Repeat("a", 30) + "!"

    re, err := cache.Get(pattern)
    require.NoError(t, err)

    start := time.Now()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
    defer cancel()

    resultCh := make(chan bool, 1)
    go func() {
        resultCh <- re.MatchString(input)
    }()

    select {
    case <-resultCh:
        t.Log("match completed within timeout")
    case <-ctx.Done():
        t.Log("match timed out as expected")
    }

    elapsed := time.Since(start)
    assert.Less(t, elapsed, 20*time.Millisecond)
}

func TestLog_RateLimit(t *testing.T) {
    limiter := rate.NewLimiter(10, 10)

    allowed := 0
    for i := 0; i < 20; i++ {
        if limiter.Allow() {
            allowed++
        }
    }

    // 最初の10回は許可される
    assert.Equal(t, 10, allowed)
}
```

---

## 関連ドキュメント

- [メイン計画](./08-issue2-custom-log-patterns.md)
- [技術調査](./08-issue2-research.md)
- [VRChatログフォーマット仕様](./08-issue2-log-format-spec.md)
- [ABI仕様](./08-issue2-abi-spec.md)
- [セキュリティ](./08-issue2-security.md)
- [テスト戦略](./08-issue2-testing.md)
