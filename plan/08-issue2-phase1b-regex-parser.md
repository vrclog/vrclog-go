# Phase 1b: Event.Data + RegexParser

## 概要

YAMLパターンファイルによるカスタムイベント定義機能を実装する。
プログラミング不要でカスタムパターンを定義できるようにする。

## 背景

### なぜYAMLパターンファイルか

1. **プログラミング不要**: YAMLを書くだけでカスタムイベントを定義
2. **可読性**: 正規表現とイベントタイプが明確に対応
3. **バージョン管理**: テキストファイルなのでgitで管理しやすい
4. **Grafana Promtailとの親和性**: 同じスタイルの設定ファイル

### 設計上の決定事項

| 決定事項 | 理由 |
|---------|------|
| Named capture groups使用 | Grafana Promtailスタイル、意味が明確 |
| `id`フィールド | パターン識別子としてログ・デバッグに使用 |
| `event_type`フィールド | 出力イベントのType値を明示 |
| `regex`フィールド | 正規表現であることが明確 |
| `version`フィールド | スキーマのバージョニング対応 |

---

## YAMLスキーマ

### スキーマ定義

```yaml
# version: スキーマバージョン（必須）
version: 1

# patterns: パターン定義の配列（必須、1個以上）
patterns:
  - id: poker_hole_cards           # パターン識別子（必須）
    event_type: poker_hole_cards   # Event.Type値（必須）
    regex: '\[Seat\]: Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)'
    # Named capture groups (?P<name>...) → Event.Dataに格納

  - id: poker_winner
    event_type: poker_winner
    regex: '\[PotManager\]: .* player (?P<seat_id>\d+) won (?P<amount>\d+)'
```

### 出力例

**入力行:**
```
2025.12.31 01:46:48 Debug - [Seat]: Draw Local Hole Cards: Jc, 6d
```

**出力Event:**
```json
{
  "type": "poker_hole_cards",
  "timestamp": "2025-12-31T01:46:48+09:00",
  "data": {
    "card1": "Jc",
    "card2": "6d"
  }
}
```

---

## 実装ファイル

### 新規ファイル

| ファイル | 説明 |
|---------|------|
| `pkg/vrclog/pattern/pattern.go` | PatternFile、Pattern型定義 |
| `pkg/vrclog/pattern/loader.go` | YAMLローダー、バリデーション |
| `pkg/vrclog/pattern/regex_parser.go` | RegexParser（Parser実装） |
| `pkg/vrclog/pattern/errors.go` | エラー型定義 |
| `pkg/vrclog/pattern/loader_test.go` | ローダーテスト |
| `pkg/vrclog/pattern/regex_parser_test.go` | RegexParserテスト |
| `pkg/vrclog/pattern/testdata/` | テストデータ |

### 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `go.mod` | `gopkg.in/yaml.v3` 依存追加 |
| `pkg/vrclog/event/event.go` | `Data map[string]string` フィールド追加 |

---

## 実装詳細

### Event型の拡張

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
    Data       map[string]string `json:"data,omitempty"`  // NEW
}
```

**背景**:
- `Data`フィールドはカスタムパーサー用の汎用フィールド
- `map[string]string`を採用した理由:
  - JSONシリアライズが容易
  - Named capture groupsから直接マッピング可能
  - 型安全性より柔軟性を優先（外部開発者が定義）

### pattern.go

```go
// pkg/vrclog/pattern/pattern.go

package pattern

// PatternFile はYAMLパターンファイルの構造
type PatternFile struct {
    Version  int       `yaml:"version"`
    Patterns []Pattern `yaml:"patterns"`
}

// Pattern はパターン定義
type Pattern struct {
    ID        string `yaml:"id"`         // パターン識別子（ログ・デバッグ用）
    EventType string `yaml:"event_type"` // Event.Type値
    Regex     string `yaml:"regex"`      // 正規表現パターン
}
```

### loader.go

```go
// pkg/vrclog/pattern/loader.go

package pattern

import (
    "fmt"
    "os"
    "regexp"

    "gopkg.in/yaml.v3"
)

const (
    MaxPatternFileSize   = 1024 * 1024 // 1MB
    CurrentSchemaVersion = 1
)

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

    if err := pf.Validate(); err != nil {
        return nil, err
    }

    return &pf, nil
}

// Validate はPatternFileを検証
func (pf *PatternFile) Validate() error {
    if pf.Version != CurrentSchemaVersion {
        return &ValidationError{
            Field:   "version",
            Message: fmt.Sprintf("unsupported version: %d (expected %d)", pf.Version, CurrentSchemaVersion),
        }
    }

    if len(pf.Patterns) == 0 {
        return &ValidationError{
            Field:   "patterns",
            Message: "no patterns defined",
        }
    }

    ids := make(map[string]bool)
    for i, p := range pf.Patterns {
        if p.ID == "" {
            return &PatternError{Index: i, Field: "id", Message: "required"}
        }
        if ids[p.ID] {
            return &PatternError{Index: i, Field: "id", Message: "duplicate: " + p.ID}
        }
        ids[p.ID] = true

        if p.EventType == "" {
            return &PatternError{Index: i, ID: p.ID, Field: "event_type", Message: "required"}
        }
        if p.Regex == "" {
            return &PatternError{Index: i, ID: p.ID, Field: "regex", Message: "required"}
        }

        // 正規表現の構文チェック
        if _, err := regexp.Compile(p.Regex); err != nil {
            return &PatternError{
                Index:   i,
                ID:      p.ID,
                Field:   "regex",
                Message: err.Error(),
            }
        }
    }

    return nil
}
```

### errors.go

```go
// pkg/vrclog/pattern/errors.go

package pattern

import "fmt"

// ValidationError はスキーマレベルのエラー
type ValidationError struct {
    Field   string
    Message string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error: %s: %s", e.Field, e.Message)
}

// PatternError は個別パターンのエラー
type PatternError struct {
    Index   int
    ID      string
    Field   string
    Message string
}

func (e *PatternError) Error() string {
    if e.ID != "" {
        return fmt.Sprintf("pattern %q: %s: %s", e.ID, e.Field, e.Message)
    }
    return fmt.Sprintf("pattern[%d]: %s: %s", e.Index, e.Field, e.Message)
}
```

### regex_parser.go

```go
// pkg/vrclog/pattern/regex_parser.go

package pattern

import (
    "context"
    "fmt"
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

    for i, p := range pf.Patterns {
        re, err := regexp.Compile(p.Regex)
        if err != nil {
            return nil, &PatternError{
                Index:   i,
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

// NewRegexParserFromFile はYAMLファイルからRegexParserを作成
func NewRegexParserFromFile(path string) (*RegexParser, error) {
    pf, err := Load(path)
    if err != nil {
        return nil, err
    }
    return NewRegexParser(pf)
}

// ParseLine はParserインターフェースを実装
// ctxは将来の正規表現タイムアウト対応用（Phase 2で実装予定）
func (p *RegexParser) ParseLine(ctx context.Context, line string) (vrclog.ParseResult, error) {
    // タイムスタンプ抽出
    ts, restOfLine, ok := extractTimestamp(line)
    if !ok {
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

// VRChatログ形式のタイムスタンプレイアウト
const timestampLayout = "2006.01.02 15:04:05"

// extractTimestamp はVRChatログ形式のタイムスタンプを抽出
func extractTimestamp(line string) (time.Time, string, bool) {
    // VRChatログ形式: 2024.01.15 23:59:59 ...
    if len(line) < 19 {
        return time.Time{}, "", false
    }

    tsStr := line[:19]
    ts, err := time.Parse(timestampLayout, tsStr)
    if err != nil {
        return time.Time{}, "", false
    }

    // タイムスタンプ以降を返す（スペースをスキップ）
    rest := line[19:]
    for len(rest) > 0 && rest[0] == ' ' {
        rest = rest[1:]
    }

    return ts, rest, true
}

// Ensure RegexParser implements Parser
var _ vrclog.Parser = (*RegexParser)(nil)
```

---

## テスト計画

### testdata/valid.yaml

```yaml
version: 1
patterns:
  - id: poker_hole_cards
    event_type: poker_hole_cards
    regex: '\[Seat\]: Draw Local Hole Cards: (?P<card1>\w+), (?P<card2>\w+)'
  - id: poker_winner
    event_type: poker_winner
    regex: '\[PotManager\]: .* player (?P<seat_id>\d+) won (?P<amount>\d+)'
```

### testdata/invalid_regex.yaml

```yaml
version: 1
patterns:
  - id: broken
    event_type: broken
    regex: '[invalid regex'
```

### loader_test.go

```go
// pkg/vrclog/pattern/loader_test.go

package pattern_test

import (
    "context"
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
    assert.Equal(t, "poker_hole_cards", pf.Patterns[0].ID)
}

func TestLoad_InvalidRegex(t *testing.T) {
    _, err := pattern.Load("testdata/invalid_regex.yaml")
    require.Error(t, err)

    var patternErr *pattern.PatternError
    assert.ErrorAs(t, err, &patternErr)
    assert.Equal(t, "regex", patternErr.Field)
}

func TestLoad_MissingFields(t *testing.T) {
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
    assert.Equal(t, "version", validationErr.Field)
}
```

### regex_parser_test.go

```go
// pkg/vrclog/pattern/regex_parser_test.go

package pattern_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/vrclog/vrclog-go/pkg/vrclog/pattern"
)

func TestRegexParser_ParseLine(t *testing.T) {
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

    t.Run("match", func(t *testing.T) {
        result, err := parser.ParseLine(context.Background(), "2024.01.15 23:59:59 Debug - [Seat]: Draw Local Hole Cards: Jc, 6d")
        require.NoError(t, err)
        assert.True(t, result.Matched)
        require.Len(t, result.Events, 1)

        ev := result.Events[0]
        assert.Equal(t, "poker_hole_cards", string(ev.Type))
        assert.Equal(t, "Jc", ev.Data["card1"])
        assert.Equal(t, "6d", ev.Data["card2"])
    })

    t.Run("no match", func(t *testing.T) {
        result, err := parser.ParseLine(context.Background(), "2024.01.15 23:59:59 Log - unrelated line")
        require.NoError(t, err)
        assert.False(t, result.Matched)
    })

    t.Run("no timestamp", func(t *testing.T) {
        result, err := parser.ParseLine(context.Background(), "random text")
        require.NoError(t, err)
        assert.False(t, result.Matched)
    })
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

    // 両方マッチするケース
    result, err := parser.ParseLine("2024.01.15 00:00:00 pattern1: foo pattern2: bar")
    require.NoError(t, err)
    assert.True(t, result.Matched)
    assert.Len(t, result.Events, 2)
}
```

---

## 実装手順

1. **go.mod更新**
   ```bash
   go get gopkg.in/yaml.v3
   ```

2. **Event.Data追加**
   - `pkg/vrclog/event/event.go` に `Data` フィールド追加

3. **patternパッケージ作成**
   - `pattern.go`: 型定義
   - `errors.go`: エラー型
   - `loader.go`: YAMLローダー
   - `regex_parser.go`: Parser実装

4. **テストデータ作成**
   - `testdata/valid.yaml`
   - `testdata/invalid_regex.yaml`

5. **テスト作成・実行**
   - `loader_test.go`
   - `regex_parser_test.go`

---

## チェックリスト

- [ ] `go get gopkg.in/yaml.v3`
- [ ] `pkg/vrclog/event/event.go` にDataフィールド追加
- [ ] `pkg/vrclog/pattern/` ディレクトリ作成
- [ ] `pattern.go` 作成
- [ ] `errors.go` 作成
- [ ] `loader.go` 作成
- [ ] `regex_parser.go` 作成
- [ ] `testdata/` 作成
- [ ] `loader_test.go` 作成
- [ ] `regex_parser_test.go` 作成
- [ ] 全テストパス確認
- [ ] `go vet ./...` パス確認

---

## 関連ドキュメント

- [メイン計画](./08-issue2-custom-log-patterns.md)
- [Phase 1a: Parser Interface](./08-issue2-phase1a-parser-interface.md)
- [Phase 1c: CLI](./08-issue2-phase1c-cli.md)
