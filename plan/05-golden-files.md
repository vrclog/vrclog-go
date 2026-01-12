# 計画: CLI出力の Golden Files テスト導入

## 目的

CLI の pretty フォーマット出力に対して Golden Files テストを導入し、出力の回帰を検出しやすくする。

## 背景

- パーサーのテストは現在の table-driven tests で十分（構造体比較で意図が明確）
- CLI の出力（特に pretty フォーマット）は複数行にまたがり、Golden Files が効果的
- 出力の変更を視覚的に差分で確認できる

## 変更範囲

- `cmd/vrclog/format_test.go` (テスト追加)
- `cmd/vrclog/testdata/golden/` (新規ディレクトリ)
- `cmd/vrclog/testdata/golden/*.golden` (期待出力ファイル)

## 実装手順

### 1. testdata/golden ディレクトリを作成

```
cmd/vrclog/testdata/golden/
├── pretty_player_join.golden
├── pretty_player_left.golden
├── pretty_world_join.golden
└── jsonl_player_join.golden
```

### 2. format_test.go に Golden Files テストを追加

```go
package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vrclog/vrclog-go/pkg/vrclog"
)

var updateGolden = flag.Bool("update-golden", false, "update golden files")

func TestOutputEvent_Golden(t *testing.T) {
	// 固定時刻を使用（再現性のため）
	fixedTime := time.Date(2024, 1, 15, 23, 59, 59, 0, time.UTC)

	tests := []struct {
		name   string
		format string
		event  vrclog.Event
	}{
		{
			name:   "pretty_player_join",
			format: "pretty",
			event: vrclog.Event{
				Type:       vrclog.EventPlayerJoin,
				Timestamp:  fixedTime,
				PlayerName: "TestUser",
			},
		},
		{
			name:   "pretty_player_left",
			format: "pretty",
			event: vrclog.Event{
				Type:       vrclog.EventPlayerLeft,
				Timestamp:  fixedTime,
				PlayerName: "TestUser",
			},
		},
		{
			name:   "pretty_world_join",
			format: "pretty",
			event: vrclog.Event{
				Type:      vrclog.EventWorldJoin,
				Timestamp: fixedTime,
				WorldName: "Test World",
			},
		},
		{
			name:   "jsonl_player_join",
			format: "jsonl",
			event: vrclog.Event{
				Type:       vrclog.EventPlayerJoin,
				Timestamp:  fixedTime,
				PlayerName: "TestUser",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := OutputEvent(tt.format, tt.event, &buf); err != nil {
				t.Fatalf("OutputEvent() error = %v", err)
			}

			golden := filepath.Join("testdata", "golden", tt.name+".golden")

			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(golden), 0755); err != nil {
					t.Fatalf("failed to create golden dir: %v", err)
				}
				if err := os.WriteFile(golden, buf.Bytes(), 0644); err != nil {
					t.Fatalf("failed to update golden file: %v", err)
				}
				t.Logf("updated golden file: %s", golden)
				return
			}

			expected, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("failed to read golden file: %v", err)
			}

			if !bytes.Equal(buf.Bytes(), expected) {
				t.Errorf("output mismatch.\ngot:\n%s\nwant:\n%s", buf.String(), string(expected))
			}
		})
	}
}
```

### 3. Golden ファイルを生成

```bash
go test ./cmd/vrclog -run TestOutputEvent_Golden -update-golden
```

### 4. 既存テストとの共存

既存の `TestOutputPretty` 等のユニットテストはそのまま残し、Golden Files は全体スナップショットとして補完。

## 受け入れ基準

- [ ] `testdata/golden/` ディレクトリが作成されている
- [ ] 各フォーマットの golden ファイルが存在する
- [ ] `go test ./cmd/vrclog` が成功する
- [ ] `-update-golden` フラグで golden ファイルを更新できる
- [ ] 出力変更時に差分が検出される

## テスト

```bash
# 通常テスト（goldenと比較）
go test ./cmd/vrclog -run TestOutputEvent_Golden

# goldenファイルの更新
go test ./cmd/vrclog -run TestOutputEvent_Golden -update-golden

# 意図的に出力を変更してテスト失敗を確認
```

## 注意点/リスク

- 時刻はUTCの固定値を使用（ローカルタイムゾーンによる差異を防ぐ）
- 出力にタイムスタンプが含まれる場合は正規化が必要
- golden ファイルは改行コードに注意（LFで統一）

## 参考

- Go testing patterns: https://go.dev/wiki/TableDrivenTests
- 既存のformat_test.go: `cmd/vrclog/format_test.go`
