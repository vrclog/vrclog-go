# Phase 1a: Parser Interface基盤

## 概要

既存動作を維持しつつParser interfaceを導入する。
このインターフェースはPhase 1b/1c/Phase 2の全ての基盤となる。

## 背景

### なぜParser interfaceが必要か

1. **拡張性**: カスタムパーサー（YAML、Wasm）を統一的に扱える
2. **テスタビリティ**: モックパーサーでテストが容易に
3. **既存互換性**: DefaultParserで既存動作を維持

### 設計上の決定事項

| 決定事項 | 理由 |
|---------|------|
| `ParseResult`に`Matched`フィールド | `nil`/空スライスの曖昧さを排除 |
| `Parser`はinterfaceのみ | 実装はDefaultParser、RegexParser、WasmParser等に分離 |
| `ParserFunc`アダプタ | 関数をParserとして使える利便性 |
| `ChainMode`は3種類 | 全実行、最初マッチ、エラースキップの3パターンをカバー |
| **Context引数を追加** | タイムアウト・キャンセルの伝播、将来の拡張性確保 |

---

## 実装ファイル

### 新規ファイル

| ファイル | 説明 |
|---------|------|
| `pkg/vrclog/parser.go` | Parser interface、ParseResult、ParserChain、ParserFunc、ChainMode |
| `pkg/vrclog/parser_default.go` | DefaultParser（既存internal/parserをラップ） |
| `pkg/vrclog/parser_test.go` | Parser関連テスト |

### 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `pkg/vrclog/options.go` | `WithParser()` option追加、watchConfig/parseConfigにparserフィールド追加 |
| `pkg/vrclog/watcher.go` | `cfg.parser.ParseLine(line)` に変更 |
| `pkg/vrclog/parse.go` | `cfg.parser.ParseLine(line)` に変更 |

---

## 実装詳細

### parser.go

```go
// pkg/vrclog/parser.go

package vrclog

import (
    "context"
    "errors"

    "github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

// ParseResult はパース結果を表す
type ParseResult struct {
    // Events はパースで抽出されたイベント
    Events []event.Event

    // Matched はパターンにマッチしたかどうか
    // Events==nil/空でもtrue の場合がある（例: マッチしたが出力なしのフィルタ）
    Matched bool
}

// Parser はログ行をパースするインターフェース
type Parser interface {
    ParseLine(ctx context.Context, line string) (ParseResult, error)
}

// ParserFunc は関数をParserとして使うアダプタ
type ParserFunc func(context.Context, string) (ParseResult, error)

// ParseLine はParserインターフェースを実装
func (f ParserFunc) ParseLine(ctx context.Context, line string) (ParseResult, error) {
    return f(ctx, line)
}

// ChainMode はチェーン動作モード
type ChainMode int

const (
    // ChainAll は全パーサーを実行し、結果を結合する（デフォルト）
    ChainAll ChainMode = iota

    // ChainFirst は最初にマッチしたパーサーで終了
    ChainFirst

    // ChainContinueOnError はエラー発生パーサーをスキップして継続
    // エラーは最後にまとめて返す
    ChainContinueOnError
)

// ParserChain は複数パーサーをチェーン実行
type ParserChain struct {
    Mode    ChainMode
    Parsers []Parser
}

// ParseLine はParserインターフェースを実装
func (c *ParserChain) ParseLine(ctx context.Context, line string) (ParseResult, error) {
    var allEvents []event.Event
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

### parser_default.go

```go
// pkg/vrclog/parser_default.go

package vrclog

import (
    "context"

    "github.com/vrclog/vrclog-go/internal/parser"
    "github.com/vrclog/vrclog-go/pkg/vrclog/event"
)

// DefaultParser は既存のパーサーをラップ
// VRChat標準ログ（player_join, player_left, world_join）をパースする
type DefaultParser struct{}

// ParseLine はParserインターフェースを実装
// ctxは将来の拡張用（現在は使用しない）
func (DefaultParser) ParseLine(ctx context.Context, line string) (ParseResult, error) {
    ev, err := parser.Parse(line)
    if err != nil {
        return ParseResult{}, err
    }
    if ev == nil {
        return ParseResult{Matched: false}, nil
    }
    return ParseResult{Events: []event.Event{*ev}, Matched: true}, nil
}

// Ensure DefaultParser implements Parser
var _ Parser = DefaultParser{}
```

### options.go 変更

```go
// pkg/vrclog/options.go に追加

// watchConfig にparserフィールドを追加
type watchConfig struct {
    // ... 既存フィールド
    parser Parser  // NEW
}

// デフォルト値設定
func defaultWatchConfig() *watchConfig {
    return &watchConfig{
        // ... 既存設定
        parser: DefaultParser{},  // NEW
    }
}

// WithParser はカスタムパーサーを設定する
func WithParser(p Parser) WatchOption {
    return func(c *watchConfig) error {
        if p == nil {
            return errors.New("parser cannot be nil")
        }
        c.parser = p
        return nil
    }
}

// WithParsers は複数パーサーをChainAllで設定する
func WithParsers(parsers ...Parser) WatchOption {
    return func(c *watchConfig) error {
        if len(parsers) == 0 {
            return errors.New("at least one parser required")
        }
        c.parser = &ParserChain{
            Mode:    ChainAll,
            Parsers: parsers,
        }
        return nil
    }
}

// parseConfig にも同様の変更
type parseConfig struct {
    // ... 既存フィールド
    parser Parser  // NEW
}

// WithParseParser はParseWithOptions用のパーサー設定
func WithParseParser(p Parser) ParseOption {
    return func(c *parseConfig) error {
        if p == nil {
            return errors.New("parser cannot be nil")
        }
        c.parser = p
        return nil
    }
}
```

### watcher.go 変更

```go
// pkg/vrclog/watcher.go の変更箇所

// processLine メソッド（または同等の処理）
func (w *Watcher) processLine(ctx context.Context, line string) {
    result, err := w.cfg.parser.ParseLine(ctx, line)
    if err != nil {
        w.errCh <- fmt.Errorf("parse error: %w", err)
        return
    }
    if !result.Matched {
        return
    }
    for _, ev := range result.Events {
        w.eventCh <- ev
    }
}
```

---

## テスト計画

### parser_test.go

```go
// pkg/vrclog/parser_test.go

package vrclog_test

import (
    "context"
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
            name:      "unrecognized",
            line:      "random text",
            wantMatch: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := p.ParseLine(context.Background(), tt.line)
            require.NoError(t, err)
            assert.Equal(t, tt.wantMatch, result.Matched)
            if tt.wantMatch {
                require.Len(t, result.Events, 1)
                assert.Equal(t, tt.wantType, result.Events[0].Type)
            }
        })
    }
}

func TestParserFunc(t *testing.T) {
    called := false
    p := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
        called = true
        return vrclog.ParseResult{Matched: true}, nil
    })

    result, err := p.ParseLine(context.Background(), "test")
    require.NoError(t, err)
    assert.True(t, called)
    assert.True(t, result.Matched)
}

func TestParserChain_ChainAll(t *testing.T) {
    p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{
            Events:  []event.Event{{Type: "type1"}},
            Matched: true,
        }, nil
    })
    p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{
            Events:  []event.Event{{Type: "type2"}},
            Matched: true,
        }, nil
    })

    chain := &vrclog.ParserChain{
        Mode:    vrclog.ChainAll,
        Parsers: []vrclog.Parser{p1, p2},
    }

    result, err := chain.ParseLine(context.Background(), "test")
    require.NoError(t, err)
    assert.True(t, result.Matched)
    assert.Len(t, result.Events, 2)
    assert.Equal(t, event.Type("type1"), result.Events[0].Type)
    assert.Equal(t, event.Type("type2"), result.Events[1].Type)
}

func TestParserChain_ChainFirst(t *testing.T) {
    callOrder := []int{}
    p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
        callOrder = append(callOrder, 1)
        return vrclog.ParseResult{
            Events:  []event.Event{{Type: "type1"}},
            Matched: true,
        }, nil
    })
    p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
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

    result, err := chain.ParseLine(context.Background(), "test")
    require.NoError(t, err)
    assert.True(t, result.Matched)
    assert.Len(t, result.Events, 1)
    assert.Equal(t, []int{1}, callOrder) // p2は呼ばれない
}

func TestParserChain_ChainContinueOnError(t *testing.T) {
    p1 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{}, errors.New("p1 error")
    })
    p2 := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{
            Events:  []event.Event{{Type: "type2"}},
            Matched: true,
        }, nil
    })

    chain := &vrclog.ParserChain{
        Mode:    vrclog.ChainContinueOnError,
        Parsers: []vrclog.Parser{p1, p2},
    }

    result, err := chain.ParseLine(context.Background(), "test")
    assert.Error(t, err) // エラーは返る
    assert.True(t, result.Matched) // p2の結果は含まれる
    assert.Len(t, result.Events, 1)
}

func TestParserChain_NoMatch(t *testing.T) {
    p := vrclog.ParserFunc(func(ctx context.Context, line string) (vrclog.ParseResult, error) {
        return vrclog.ParseResult{Matched: false}, nil
    })

    chain := &vrclog.ParserChain{
        Mode:    vrclog.ChainAll,
        Parsers: []vrclog.Parser{p},
    }

    result, err := chain.ParseLine(context.Background(), "test")
    require.NoError(t, err)
    assert.False(t, result.Matched)
    assert.Empty(t, result.Events)
}
```

### 既存テストの確認

```bash
# 既存テストがパスすることを確認
go test ./pkg/vrclog/...
go test ./internal/parser/...
```

---

## 実装手順

1. **parser.go作成**
   - ParseResult型定義
   - Parser interface定義
   - ParserFunc定義
   - ChainMode定義
   - ParserChain実装

2. **parser_default.go作成**
   - DefaultParser実装
   - internal/parserへの委譲

3. **parser_test.go作成**
   - 上記テスト実装

4. **options.go変更**
   - watchConfig.parserフィールド追加
   - parseConfig.parserフィールド追加
   - WithParser()実装
   - WithParsers()実装
   - WithParseParser()実装

5. **watcher.go変更**
   - processLine内でcfg.parser.ParseLine()を使用

6. **parse.go変更**
   - 同様の変更

7. **テスト実行**
   - 新規テストがパス
   - 既存テストがパス

---

## 後方互換性

### ParseLine関数

既存の`pkg/vrclog.ParseLine()`関数は維持する:

```go
// pkg/vrclog/parse.go

// ParseLine は既存互換のための関数
// 内部でDefaultParserを使用
func ParseLine(line string) (*event.Event, error) {
    result, err := DefaultParser{}.ParseLine(context.Background(), line)
    if err != nil {
        return nil, err
    }
    if !result.Matched || len(result.Events) == 0 {
        return nil, nil
    }
    return &result.Events[0], nil
}
```

---

## チェックリスト

- [ ] `pkg/vrclog/parser.go` 作成
- [ ] `pkg/vrclog/parser_default.go` 作成
- [ ] `pkg/vrclog/parser_test.go` 作成
- [ ] `pkg/vrclog/options.go` にWithParser追加
- [ ] `pkg/vrclog/watcher.go` 更新
- [ ] `pkg/vrclog/parse.go` 更新
- [ ] 新規テストパス確認
- [ ] 既存テストパス確認
- [ ] `go vet ./...` パス確認
- [ ] `golangci-lint run` パス確認

---

## 関連ドキュメント

- [メイン計画](./08-issue2-custom-log-patterns.md)
- [実装例](./08-issue2-examples.md)
- [Phase 1b: RegexParser](./08-issue2-phase1b-regex-parser.md)
