# Phase 1c: CLI統合

## 概要

`--patterns`フラグでYAMLパターンファイルをCLIから指定できるようにする。

## 背景

### なぜCLIフラグが必要か

1. **簡単に試せる**: コード変更なしでカスタムパターンを使える
2. **複数パターン**: 複数ファイルを指定してチェーン実行
3. **標準パーサーとの併用**: デフォルトパーサーと組み合わせ可能

---

## 使用例

```bash
# YAMLパターン指定
vrclog tail --patterns ./vrpoker-patterns.yaml
vrclog parse --patterns ./vrpoker-patterns.yaml log.txt

# 複数パターン（ChainAll）
vrclog tail --patterns ./vrpoker.yaml --patterns ./custom.yaml

# 出力例（JSON形式）
{"type":"world_join","timestamp":"...","world_name":"VR Poker"}
{"type":"poker_hole_cards","timestamp":"...","data":{"card1":"Jc","card2":"6d"}}
```

---

## 実装ファイル

### 変更ファイル

| ファイル | 変更内容 |
|---------|---------|
| `cmd/vrclog/tail.go` | `--patterns` フラグ追加、ParserChain構築 |
| `cmd/vrclog/parse.go` | `--patterns` フラグ追加、ParserChain構築 |
| `cmd/vrclog/format.go` | Event.DataのJSON出力対応 |

---

## 実装詳細

### tail.go

```go
// cmd/vrclog/tail.go

var (
    tailPatternsFlags []string  // NEW
    // ... 既存フラグ
)

func init() {
    // NEW: --patterns フラグ
    tailCmd.Flags().StringArrayVar(&tailPatternsFlags, "patterns", nil,
        "YAML pattern file (can be specified multiple times)")

    // ... 既存フラグ
}

func runTail(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()

    // パーサー構築
    parser, err := buildParser(ctx, tailPatternsFlags)
    if err != nil {
        return fmt.Errorf("build parser: %w", err)
    }

    // Watcher作成
    opts := []vrclog.WatchOption{}
    if parser != nil {
        opts = append(opts, vrclog.WithParser(parser))
    }
    // ... 既存オプション追加

    watcher, err := vrclog.NewWatcherWithOptions(opts...)
    if err != nil {
        return err
    }

    // ... 既存のWatch処理
}

// buildParser はフラグからParserを構築
func buildParser(ctx context.Context, patternFiles []string) (vrclog.Parser, error) {
    if len(patternFiles) == 0 {
        return nil, nil // デフォルトパーサーを使用
    }

    parsers := []vrclog.Parser{vrclog.DefaultParser{}}

    for _, path := range patternFiles {
        rp, err := pattern.NewRegexParserFromFile(path)
        if err != nil {
            return nil, fmt.Errorf("load pattern file %s: %w", path, err)
        }
        parsers = append(parsers, rp)
    }

    return &vrclog.ParserChain{
        Mode:    vrclog.ChainAll,
        Parsers: parsers,
    }, nil
}
```

### parse.go

```go
// cmd/vrclog/parse.go

var (
    parsePatternsFlags []string  // NEW
    // ... 既存フラグ
)

func init() {
    // NEW: --patterns フラグ
    parseCmd.Flags().StringArrayVar(&parsePatternsFlags, "patterns", nil,
        "YAML pattern file (can be specified multiple times)")

    // ... 既存フラグ
}

func runParse(cmd *cobra.Command, args []string) error {
    ctx := cmd.Context()

    // パーサー構築
    parser, err := buildParser(ctx, parsePatternsFlags)
    if err != nil {
        return fmt.Errorf("build parser: %w", err)
    }

    // ParseWithOptions
    opts := []vrclog.ParseOption{}
    if parser != nil {
        opts = append(opts, vrclog.WithParseParser(parser))
    }
    // ... 既存オプション追加

    events, err := vrclog.ParseWithOptions(reader, opts...)
    if err != nil {
        return err
    }

    // ... 既存の出力処理
}
```

### format.go

```go
// cmd/vrclog/format.go

// formatEvent はEventをフォーマットする
func formatEvent(ev *event.Event, format string) (string, error) {
    switch format {
    case "json":
        // Event.DataがあればJSON出力に含める
        b, err := json.Marshal(ev)
        if err != nil {
            return "", err
        }
        return string(b), nil

    case "text":
        return formatEventText(ev), nil

    default:
        return "", fmt.Errorf("unknown format: %s", format)
    }
}

// formatEventText はテキスト形式でフォーマット
func formatEventText(ev *event.Event) string {
    var sb strings.Builder
    sb.WriteString(ev.Timestamp.Format(time.RFC3339))
    sb.WriteString(" ")
    sb.WriteString(string(ev.Type))

    // 既存フィールド
    if ev.PlayerName != "" {
        sb.WriteString(" player=")
        sb.WriteString(ev.PlayerName)
    }
    if ev.WorldName != "" {
        sb.WriteString(" world=")
        sb.WriteString(ev.WorldName)
    }

    // Data フィールド（NEW）
    if len(ev.Data) > 0 {
        for k, v := range ev.Data {
            sb.WriteString(" ")
            sb.WriteString(k)
            sb.WriteString("=")
            sb.WriteString(v)
        }
    }

    return sb.String()
}
```

---

## エラーメッセージ

### パターンファイルエラー

```
Error: build parser: load pattern file ./bad.yaml: pattern "test": regex: error parsing regexp: ...

Hint: Check the regex syntax in your pattern file.
See: https://pkg.go.dev/regexp/syntax
```

### ファイル読み込みエラー

```
Error: build parser: load pattern file ./missing.yaml: stat: no such file or directory
```

---

## テスト計画

### 統合テスト

```go
// cmd/vrclog/tail_test.go

func TestTailWithPatterns(t *testing.T) {
    // テスト用パターンファイル作成
    patternFile := filepath.Join(t.TempDir(), "patterns.yaml")
    err := os.WriteFile(patternFile, []byte(`
version: 1
patterns:
  - id: test
    event_type: test_event
    regex: 'test: (?P<value>\w+)'
`), 0644)
    require.NoError(t, err)

    // テスト用ログファイル作成
    logFile := filepath.Join(t.TempDir(), "output_log_2024-01-01_00-00-00.txt")
    err = os.WriteFile(logFile, []byte(
        "2024.01.01 00:00:00 Log - test: hello\n",
    ), 0644)
    require.NoError(t, err)

    // CLI実行
    cmd := rootCmd
    cmd.SetArgs([]string{"parse", "--patterns", patternFile, logFile})
    var out bytes.Buffer
    cmd.SetOut(&out)

    err = cmd.Execute()
    require.NoError(t, err)

    // 出力確認
    assert.Contains(t, out.String(), "test_event")
    assert.Contains(t, out.String(), "hello")
}
```

---

## チェックリスト

- [ ] `cmd/vrclog/tail.go` に `--patterns` フラグ追加
- [ ] `cmd/vrclog/parse.go` に `--patterns` フラグ追加
- [ ] `buildParser` 関数実装
- [ ] `cmd/vrclog/format.go` でEvent.Data対応
- [ ] 統合テスト作成
- [ ] ヘルプメッセージ確認
- [ ] `go build ./cmd/vrclog` 成功確認

---

## 関連ドキュメント

- [メイン計画](./08-issue2-custom-log-patterns.md)
- [Phase 1b: RegexParser](./08-issue2-phase1b-regex-parser.md)
- [Phase 2: Wasm](./08-issue2-phase2-wasm.md)
