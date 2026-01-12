# 計画: errors.Is/errors.As の使用例を追加

## 目的

`example_test.go` に `errors.Is` と `errors.As` の使用例を追加し、エラーハンドリングのベストプラクティスをドキュメント化する。

## 背景

- `pkg/vrclog/errors.go` で定義されているエラー型には「`errors.As` を使用してください」とコメントがあるが、具体例がない
- 新規ユーザーがエラーハンドリングの正しい方法を理解しやすくする

## 変更範囲

- `pkg/vrclog/example_test.go` (Example関数を追加)

## 実装手順

### 1. errors.Is の Example を追加

```go
package vrclog_test

import (
	"errors"
	"fmt"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

// Example_errorsIs demonstrates how to check for sentinel errors using errors.Is.
func Example_errorsIs() {
	// Simulate a wrapped error (e.g., from NewWatcherWithOptions)
	err := fmt.Errorf("failed to initialize watcher: %w", vrclog.ErrLogDirNotFound)

	// Use errors.Is to check for specific sentinel errors
	if errors.Is(err, vrclog.ErrLogDirNotFound) {
		fmt.Println("VRChat log directory not found")
	}
	// Output: VRChat log directory not found
}
```

### 2. errors.As の Example を追加（ParseError）

```go
// Example_errorsAs_ParseError demonstrates how to extract ParseError details.
func Example_errorsAs_ParseError() {
	// Simulate a parse error
	originalErr := fmt.Errorf("invalid timestamp")
	err := fmt.Errorf("processing failed: %w", &vrclog.ParseError{
		Line: "malformed log line here",
		Err:  originalErr,
	})

	// Use errors.As to extract the ParseError
	var parseErr *vrclog.ParseError
	if errors.As(err, &parseErr) {
		fmt.Printf("Failed to parse line: %q\n", parseErr.Line)
		fmt.Printf("Cause: %v\n", parseErr.Err)
	}
	// Output:
	// Failed to parse line: "malformed log line here"
	// Cause: invalid timestamp
}
```

### 3. errors.As の Example を追加（WatchError）

```go
// Example_errorsAs_WatchError demonstrates how to extract WatchError details.
func Example_errorsAs_WatchError() {
	// Simulate a watch error
	err := fmt.Errorf("watcher failed: %w", &vrclog.WatchError{
		Op:   vrclog.WatchOpTail,
		Path: "/path/to/log.txt",
		Err:  fmt.Errorf("file not accessible"),
	})

	// Use errors.As to extract the WatchError
	var watchErr *vrclog.WatchError
	if errors.As(err, &watchErr) {
		fmt.Printf("Operation: %s\n", watchErr.Op)
		fmt.Printf("Path: %s\n", watchErr.Path)
		fmt.Printf("Error: %v\n", watchErr.Err)
	}
	// Output:
	// Operation: tail
	// Path: /path/to/log.txt
	// Error: file not accessible
}
```

### 4. テストの実行

```bash
go test ./pkg/vrclog -run Example
```

## 受け入れ基準

- [ ] `Example_errorsIs` が正しく動作し、Output が一致する
- [ ] `Example_errorsAs_ParseError` が正しく動作し、Output が一致する
- [ ] `Example_errorsAs_WatchError` が正しく動作し、Output が一致する
- [ ] `go doc` でExample関数が表示される
- [ ] `go test` ですべてのExampleがパスする

## テスト

```bash
# Exampleのテスト
go test ./pkg/vrclog -run Example -v

# godocで確認
go doc github.com/vrclog/vrclog-go/pkg/vrclog
```

## 注意点/リスク

- Example関数の `// Output:` コメントは正確に一致する必要がある
- 実行環境に依存しない形で書く（パスやタイムスタンプを避ける）
- 外部ファイルやネットワークに依存しない

## 参考

- 既存のexample_test.go: `pkg/vrclog/example_test.go`
- errors.go の定義: `pkg/vrclog/errors.go`
- Go testing examples: https://go.dev/blog/examples
