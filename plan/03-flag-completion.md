# 計画: --include-types/--exclude-types のカスタム補完

## 目的

`--include-types` と `--exclude-types` フラグで、イベントタイプ（`player_join`, `player_left`, `world_join`）のTAB補完を有効にする。

## 背景

イベントタイプは固定・少数（3種類）のため、補完により誤入力を防止できる。
カンマ区切りの入力にも対応し、既に入力済みの値は候補から除外する。

## 変更範囲

- `cmd/vrclog/completion.go` (補完関数を追加)
- `cmd/vrclog/tail.go` (補完関数を登録)
- `cmd/vrclog/parse.go` (補完関数を登録)

## 実装手順

### 1. completion.go に補完関数を追加

```go
package main

import (
	"strings"

	"github.com/spf13/cobra"
)

// completeEventTypes provides completion for event type flags.
// Supports comma-separated values and excludes already-selected types.
func completeEventTypes(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Split by comma to handle "player_join,pla<TAB>" case
	parts := strings.Split(toComplete, ",")
	prefix := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))

	// Track already-used values
	used := make(map[string]struct{})
	for _, p := range parts[:len(parts)-1] {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			used[p] = struct{}{}
		}
	}

	// Build completion candidates
	allTypes := []string{"player_join", "player_left", "world_join"}
	var candidates []string
	for _, t := range allTypes {
		if _, ok := used[t]; ok {
			continue // Skip already-used
		}
		if strings.HasPrefix(t, prefix) {
			candidates = append(candidates, t)
		}
	}

	return candidates, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
}

// registerEventTypeCompletion registers completion for an event type flag.
func registerEventTypeCompletion(cmd *cobra.Command, flagName string) {
	_ = cmd.RegisterFlagCompletionFunc(flagName, completeEventTypes)
}
```

### 2. tail.go に補完登録を追加

`init()` 関数の末尾に追加：

```go
func init() {
	// ... 既存のフラグ定義 ...

	// Register completion for event type flags
	registerEventTypeCompletion(tailCmd, "include-types")
	registerEventTypeCompletion(tailCmd, "exclude-types")
}
```

### 3. parse.go に補完登録を追加

`init()` 関数の末尾に追加：

```go
func init() {
	// ... 既存のフラグ定義 ...

	// Register completion for event type flags
	registerEventTypeCompletion(parseCmd, "include-types")
	registerEventTypeCompletion(parseCmd, "exclude-types")
}
```

## 受け入れ基準

- [ ] `vrclog tail --include-types <TAB>` で3つの候補が表示される
- [ ] `vrclog tail --include-types player_<TAB>` で `player_join`, `player_left` が表示される
- [ ] `vrclog tail --include-types player_join,<TAB>` で `player_left`, `world_join` が表示される（player_joinは除外）
- [ ] `vrclog parse --include-types <TAB>` も同様に動作する
- [ ] `--exclude-types` も同様に動作する

## テスト

シェル補完のテストは実際のシェルで確認が必要。以下の手順で確認：

```bash
# 1. 補完スクリプトを読み込み
source <(go run ./cmd/vrclog completion bash)

# 2. 補完をテスト
vrclog tail --include-types <TAB><TAB>
# 期待: player_join  player_left  world_join

vrclog tail --include-types player_<TAB>
# 期待: player_join  player_left

vrclog tail --include-types player_join,<TAB><TAB>
# 期待: player_left  world_join
```

## 注意点/リスク

- `RegisterFlagCompletionFunc` はフラグ定義後に呼び出す必要がある
- `ShellCompDirectiveNoSpace`: カンマの後に自動でスペースを入れない
- `ShellCompDirectiveNoFileComp`: ファイル補完を無効化

## 参考

- Cobra custom completions: https://github.com/spf13/cobra/blob/main/shell_completions.md#completions-for-flags
- 既存のイベントタイプ定義: `cmd/vrclog/eventtypes.go`
