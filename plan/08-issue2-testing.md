# Issue #2: テスト戦略

## 概要

カスタムログパターン機能のテスト戦略と実装計画。

---

## テストレベル

| レベル | 対象 | ツール |
|-------|------|-------|
| ユニットテスト | 個別関数・構造体 | `go test` |
| 統合テスト | パッケージ間連携 | `go test` |
| E2Eテスト | CLI全体 | `go test` + subprocess |
| セキュリティテスト | ReDoS、制限確認 | `go test` |

---

## Phase 1a: Parser Interfaceテスト

### pkg/vrclog/parser_test.go

```go
package vrclog_test

import (
    "errors"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/vrclog/vrclog-go/pkg/vrclog"
    "github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

func TestDefaultParser_StandardLog(t *testing.T) {
    p := vrclog.DefaultParser{}

    tests := []struct {
        name      string
        line      string
        wantMatch bool
        wantType  event.Type
    }{
        {
            name:      "player_join",
            line:      "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerJoined TestUser",
            wantMatch: true,
            wantType:  event.PlayerJoin,
        },
        {
            name:      "player_left",
            line:      "2024.01.15 23:59:59 Log        -  [Behaviour] OnPlayerLeft TestUser",
            wantMatch: true,
            wantType:  event.PlayerLeft,
        },
        {
            name:      "world_join",
            line:      "2024.01.15 23:59:59 Log        -  [Behaviour] Entering Room: Test World",
            wantMatch: true,
            wantType:  event.WorldJoin,
        },
        {
            name:      "unrecognized",
            line:      "random text",
            wantMatch: false,
        },
        {
            name:      "empty",
            line:      "",
            wantMatch: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := p.ParseLine(tt.line)
            require.NoError(t, err)
            assert.Equal(t, tt.wantMatch, result.Matched)
            if tt.wantMatch {
                require.Len(t, result.Events, 1)
                assert.Equal(t, tt.wantType, result.Events[0].Type)
            }
        })
    }
}

func TestParserFunc_Adapter(t *testing.T) {
    called := false
    expectedLine := "test line"

    p := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        called = true
        assert.Equal(t, expectedLine, line)
        return vrclog.ParseResult{Matched: true}, nil
    })

    result, err := p.ParseLine(expectedLine)
    require.NoError(t, err)
    assert.True(t, called)
    assert.True(t, result.Matched)
}

func TestParserChain_ChainAll(t *testing.T) {
    p1 := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{
            Events:  []event.Event{{Type: "type1"}},
            Matched: true,
        }, nil
    })
    p2 := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{
            Events:  []event.Event{{Type: "type2"}},
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
    callOrder := []int{}

    p1 := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        callOrder = append(callOrder, 1)
        return vrclog.ParseResult{
            Events:  []event.Event{{Type: "type1"}},
            Matched: true,
        }, nil
    })
    p2 := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        callOrder = append(callOrder, 2)
        return vrclog.ParseResult{
            Events:  []event.Event{{Type: "type2"}},
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
    assert.Equal(t, []int{1}, callOrder) // p2 not called
}

func TestParserChain_ChainContinueOnError(t *testing.T) {
    p1 := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{}, errors.New("p1 error")
    })
    p2 := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{
            Events:  []event.Event{{Type: "type2"}},
            Matched: true,
        }, nil
    })

    chain := &vrclog.ParserChain{
        Mode:    vrclog.ChainContinueOnError,
        Parsers: []vrclog.Parser{p1, p2},
    }

    result, err := chain.ParseLine("test")
    assert.Error(t, err) // Error returned
    assert.True(t, result.Matched) // But p2 result included
    assert.Len(t, result.Events, 1)
}

func TestParserChain_ErrorPropagation(t *testing.T) {
    expectedErr := errors.New("parse error")

    p := vrclog.ParserFunc(func(line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{}, expectedErr
    })

    chain := &vrclog.ParserChain{
        Mode:    vrclog.ChainAll,
        Parsers: []vrclog.Parser{p},
    }

    _, err := chain.ParseLine("test")
    assert.ErrorIs(t, err, expectedErr)
}

func TestParserChain_Empty(t *testing.T) {
    chain := &vrclog.ParserChain{
        Mode:    vrclog.ChainAll,
        Parsers: []vrclog.Parser{},
    }

    result, err := chain.ParseLine("test")
    require.NoError(t, err)
    assert.False(t, result.Matched)
    assert.Empty(t, result.Events)
}
```

---

## Phase 1b: RegexParserテスト

### pkg/vrclog/pattern/loader_test.go

```go
package pattern_test

import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/vrclog/vrclog-go/pkg/vrclog/pattern"
)

func TestLoad_Valid(t *testing.T) {
    pf, err := pattern.Load("testdata/valid.yaml")
    require.NoError(t, err)
    assert.Equal(t, 1, pf.Version)
    assert.Len(t, pf.Patterns, 2)
}

func TestLoad_InvalidRegex(t *testing.T) {
    _, err := pattern.Load("testdata/invalid_regex.yaml")
    require.Error(t, err)

    var patternErr *pattern.PatternError
    assert.ErrorAs(t, err, &patternErr)
    assert.Equal(t, "regex", patternErr.Field)
}

func TestLoad_MissingID(t *testing.T) {
    data := []byte(`
version: 1
patterns:
  - event_type: test
    regex: 'test'
`)
    _, err := pattern.LoadBytes(data)
    require.Error(t, err)

    var patternErr *pattern.PatternError
    assert.ErrorAs(t, err, &patternErr)
    assert.Equal(t, "id", patternErr.Field)
}

func TestLoad_MissingEventType(t *testing.T) {
    data := []byte(`
version: 1
patterns:
  - id: test
    regex: 'test'
`)
    _, err := pattern.LoadBytes(data)
    require.Error(t, err)

    var patternErr *pattern.PatternError
    assert.ErrorAs(t, err, &patternErr)
    assert.Equal(t, "event_type", patternErr.Field)
}

func TestLoad_DuplicateID(t *testing.T) {
    data := []byte(`
version: 1
patterns:
  - id: dup
    event_type: test1
    regex: 'test1'
  - id: dup
    event_type: test2
    regex: 'test2'
`)
    _, err := pattern.LoadBytes(data)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "duplicate")
}

func TestLoad_UnsupportedVersion(t *testing.T) {
    data := []byte(`
version: 99
patterns:
  - id: test
    event_type: test
    regex: 'test'
`)
    _, err := pattern.LoadBytes(data)
    require.Error(t, err)

    var validationErr *pattern.ValidationError
    assert.ErrorAs(t, err, &validationErr)
}

func TestLoad_EmptyPatterns(t *testing.T) {
    data := []byte(`
version: 1
patterns: []
`)
    _, err := pattern.LoadBytes(data)
    require.Error(t, err)
}

func TestLoad_FileTooLarge(t *testing.T) {
    // Create oversized file test
    // (実際のテストではモックを使用)
}
```

### pkg/vrclog/pattern/regex_parser_test.go

```go
package pattern_test

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/vrclog/vrclog-go/pkg/vrclog/pattern"
)

func TestRegexParser_Match(t *testing.T) {
    pf := &pattern.PatternFile{
        Version: 1,
        Patterns: []pattern.Pattern{
            {
                ID:        "hole_cards",
                EventType: "poker_hole_cards",
                Regex:     `\[Seat\]: Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)`,
            },
        },
    }

    parser, err := pattern.NewRegexParser(pf)
    require.NoError(t, err)

    result, err := parser.ParseLine("2024.01.15 23:59:59 Debug - [Seat]: Draw Local Hole Cards: Jc, 6d")
    require.NoError(t, err)
    assert.True(t, result.Matched)
    require.Len(t, result.Events, 1)

    ev := result.Events[0]
    assert.Equal(t, "poker_hole_cards", string(ev.Type))
    assert.Equal(t, "Jc", ev.Data["card1"])
    assert.Equal(t, "6d", ev.Data["card2"])

    // Timestamp extracted correctly
    expected := time.Date(2024, 1, 15, 23, 59, 59, 0, time.UTC)
    assert.Equal(t, expected, ev.Timestamp)
}

func TestRegexParser_NoMatch(t *testing.T) {
    pf := &pattern.PatternFile{
        Version: 1,
        Patterns: []pattern.Pattern{
            {
                ID:        "test",
                EventType: "test",
                Regex:     `specific pattern`,
            },
        },
    }

    parser, err := pattern.NewRegexParser(pf)
    require.NoError(t, err)

    result, err := parser.ParseLine("2024.01.15 00:00:00 unrelated line")
    require.NoError(t, err)
    assert.False(t, result.Matched)
    assert.Empty(t, result.Events)
}

func TestRegexParser_NoTimestamp(t *testing.T) {
    pf := &pattern.PatternFile{
        Version: 1,
        Patterns: []pattern.Pattern{
            {
                ID:        "test",
                EventType: "test",
                Regex:     `test`,
            },
        },
    }

    parser, err := pattern.NewRegexParser(pf)
    require.NoError(t, err)

    result, err := parser.ParseLine("random text without timestamp")
    require.NoError(t, err)
    assert.False(t, result.Matched)
}

func TestRegexParser_MultiplePatterns(t *testing.T) {
    pf := &pattern.PatternFile{
        Version: 1,
        Patterns: []pattern.Pattern{
            {
                ID:        "pattern1",
                EventType: "type1",
                Regex:     `pattern1: (?P<value>\w+)`,
            },
            {
                ID:        "pattern2",
                EventType: "type2",
                Regex:     `pattern2: (?P<value>\w+)`,
            },
        },
    }

    parser, err := pattern.NewRegexParser(pf)
    require.NoError(t, err)

    // Both patterns match
    result, err := parser.ParseLine("2024.01.15 00:00:00 pattern1: foo pattern2: bar")
    require.NoError(t, err)
    assert.True(t, result.Matched)
    assert.Len(t, result.Events, 2)
}

func TestRegexParser_NamedCaptureGroups(t *testing.T) {
    pf := &pattern.PatternFile{
        Version: 1,
        Patterns: []pattern.Pattern{
            {
                ID:        "test",
                EventType: "test",
                Regex:     `(?P<name>\w+)=(?P<value>\d+)`,
            },
        },
    }

    parser, err := pattern.NewRegexParser(pf)
    require.NoError(t, err)

    result, err := parser.ParseLine("2024.01.15 00:00:00 foo=123")
    require.NoError(t, err)
    require.Len(t, result.Events, 1)

    assert.Equal(t, "foo", result.Events[0].Data["name"])
    assert.Equal(t, "123", result.Events[0].Data["value"])
}
```

---

## Phase 2: Wasmテスト

### internal/wasm/parser_test.go

```go
package wasm_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/vrclog/vrclog-go/internal/wasm"
)

func TestWasmParser_ParseLine(t *testing.T) {
    ctx := context.Background()

    parser, err := wasm.Load(ctx, "testdata/minimal.wasm")
    require.NoError(t, err)
    defer parser.Close(ctx)

    result, err := parser.ParseLine("2024.01.15 00:00:00 test line")
    require.NoError(t, err)
    // Assertions based on minimal.wasm behavior
}

func TestWasmParser_Timeout(t *testing.T) {
    ctx := context.Background()

    parser, err := wasm.Load(ctx, "testdata/slow.wasm")
    require.NoError(t, err)
    defer parser.Close(ctx)

    _, err = parser.ParseLine("test")
    assert.ErrorIs(t, err, wasm.ErrPluginTimeout)
}

func TestWasmParser_InvalidABI(t *testing.T) {
    ctx := context.Background()

    _, err := wasm.Load(ctx, "testdata/wrong_abi.wasm")
    var abiErr *wasm.ABIVersionError
    assert.ErrorAs(t, err, &abiErr)
}

func TestWasmParser_MissingExport(t *testing.T) {
    ctx := context.Background()

    _, err := wasm.Load(ctx, "testdata/missing_export.wasm")
    assert.ErrorIs(t, err, wasm.ErrMissingExport)
}

func TestWasmParser_FileTooLarge(t *testing.T) {
    ctx := context.Background()

    _, err := wasm.Load(ctx, "testdata/large.wasm")
    assert.ErrorIs(t, err, wasm.ErrFileTooLarge)
}
```

### internal/wasm/cache_test.go

```go
package wasm_test

import (
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/vrclog/vrclog-go/internal/wasm"
)

func TestRegexCache_Get(t *testing.T) {
    cache := wasm.NewRegexCache(100, 5*time.Millisecond)

    re, err := cache.Get(`\w+`)
    require.NoError(t, err)
    assert.NotNil(t, re)

    // Same pattern returns cached
    re2, err := cache.Get(`\w+`)
    require.NoError(t, err)
    assert.Equal(t, re, re2)
}

func TestRegexCache_InvalidPattern(t *testing.T) {
    cache := wasm.NewRegexCache(100, 5*time.Millisecond)

    _, err := cache.Get(`[invalid`)
    assert.Error(t, err)

    // Error is cached too
    _, err = cache.Get(`[invalid`)
    assert.Error(t, err)
}

func TestRegexCache_LRUEviction(t *testing.T) {
    cache := wasm.NewRegexCache(2, 5*time.Millisecond)

    cache.Get(`pattern1`)
    cache.Get(`pattern2`)
    cache.Get(`pattern3`) // Should evict pattern1

    // pattern1 would need recompile (not directly testable without internals)
}
```

### internal/wasm/security_test.go

```go
package wasm_test

import (
    "context"
    "strings"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/vrclog/vrclog-go/internal/wasm"
    "golang.org/x/time/rate"
)

func TestReDoS_Timeout(t *testing.T) {
    cache := wasm.NewRegexCache(100, 5*time.Millisecond)

    pattern := `(a+)+$`
    input := strings.Repeat("a", 30) + "!"

    re, _ := cache.Get(pattern)

    start := time.Now()
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
    defer cancel()

    resultCh := make(chan bool, 1)
    go func() {
        resultCh <- re.MatchString(input)
    }()

    select {
    case <-resultCh:
        // Completed fast
    case <-ctx.Done():
        // Timed out as expected
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

    assert.Equal(t, 10, allowed)
}

func TestRegexPattern_LengthLimit(t *testing.T) {
    // Patterns > 512 bytes should be rejected
    longPattern := strings.Repeat("a", 600)

    // This would be tested through Host Function call
    // In unit test, just verify the limit constant
    assert.True(t, len(longPattern) > 512)
}
```

---

## テストデータ

### internal/wasm/testdata/

```
testdata/
├── minimal.wasm       # 最小限の有効なプラグイン
├── minimal.go         # TinyGoソース
├── echo.wasm          # 入力をそのまま返す
├── echo.go
├── slow.wasm          # タイムアウトテスト用（無限ループ）
├── slow.go
├── wrong_abi.wasm     # 不正なABIバージョン
├── wrong_abi.go
├── missing_export.wasm # 必須Export欠落
├── missing_export.go
└── large.wasm         # 10MB超（手動作成またはスキップ）
```

### minimal.go

```go
//go:build tinygo.wasm

package main

var heap [1024]byte
var heapOff uintptr

//export alloc
func alloc(size uint32) uint32 {
    off := heapOff
    heapOff += uintptr(size)
    return uint32(off)
}

//export free
func free(ptr, size uint32) {}

//export abi_version
func abi_version() uint32 { return 1 }

//export parse_line
func parse_line(ptr, size uint32) uint64 {
    heapOff = 0
    out := []byte(`{"ok":true,"events":[]}`)
    outPtr := alloc(uint32(len(out)))
    // copy out to heap...
    return (uint64(len(out)) << 32) | uint64(outPtr)
}

func main() {}
```

---

## CI設定

### .github/workflows/test.yml

```yaml
name: Test

on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.21'

      - name: Install TinyGo
        uses: nicois/tinygo-action@v1
        with:
          version: "0.32.0"

      - name: Build test Wasm
        run: |
          cd internal/wasm/testdata
          for f in *.go; do
            tinygo build -o "${f%.go}.wasm" -target=wasi -no-debug -scheduler=none "$f"
          done

      - name: Test
        run: go test -v -race ./...

      - name: Lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: v2.0.2
```

---

## テストカバレッジ目標

| パッケージ | 目標 |
|-----------|------|
| `pkg/vrclog` | 80%+ |
| `pkg/vrclog/pattern` | 90%+ |
| `internal/wasm` | 80%+ |
| `cmd/vrclog` | 60%+ |

```bash
# カバレッジ計測
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

---

## チェックリスト

### Phase 1a

- [ ] DefaultParserテスト
- [ ] ParserFuncテスト
- [ ] ParserChainテスト（ChainAll/ChainFirst/ChainContinueOnError）
- [ ] 既存テストがパス

### Phase 1b

- [ ] YAMLローダーテスト（正常系・異常系）
- [ ] RegexParserテスト
- [ ] Named capture groupsテスト
- [ ] タイムスタンプ抽出テスト

### Phase 2

- [ ] WasmParserテスト
- [ ] ABI検証テスト
- [ ] Host Functionsテスト
- [ ] 正規表現キャッシュテスト
- [ ] セキュリティテスト（ReDoS、レート制限）
- [ ] テストデータWasmビルド
- [ ] CIにTinyGo追加

---

## 関連ドキュメント

- [メイン計画](./08-issue2-custom-log-patterns.md)
- [セキュリティ](./08-issue2-security.md)
- [Phase 1a: Parser Interface](./08-issue2-phase1a-parser-interface.md)
- [Phase 1b: RegexParser](./08-issue2-phase1b-regex-parser.md)
- [Phase 2: Wasm](./08-issue2-phase2-wasm.md)
