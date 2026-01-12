# 計画: Shell Completion サブコマンドの追加

## 目的

`vrclog completion bash|zsh|fish|powershell` コマンドを追加し、シェル補完スクリプトを生成できるようにする。

## 背景

CLIのユーザー体験向上のため、Cobraの標準機能を使用してシェル補完を実装する。
これにより、コマンドやフラグのTAB補完が可能になる。

## 変更範囲

- `cmd/vrclog/completion.go` (新規作成)
- `cmd/vrclog/main.go` (AddCommand追加)

## 実装手順

### 1. completion.go を新規作成

```go
package main

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for vrclog.

To load completions:

Bash:
  $ source <(vrclog completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ vrclog completion bash > /etc/bash_completion.d/vrclog
  # macOS:
  $ vrclog completion bash > $(brew --prefix)/etc/bash_completion.d/vrclog

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ vrclog completion zsh > "${fpath[1]}/_vrclog"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ vrclog completion fish | source

  # To load completions for each session, execute once:
  $ vrclog completion fish > ~/.config/fish/completions/vrclog.fish

PowerShell:
  PS> vrclog completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> vrclog completion powershell > vrclog.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletion(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
```

### 2. main.go を確認

`init()` で `rootCmd.AddCommand(completionCmd)` が呼ばれることを確認。
completion.go の init() で追加するため、main.go の変更は不要。

## 受け入れ基準

- [ ] `vrclog completion bash` が補完スクリプトを出力する
- [ ] `vrclog completion zsh` が補完スクリプトを出力する
- [ ] `vrclog completion fish` が補完スクリプトを出力する
- [ ] `vrclog completion powershell` が補完スクリプトを出力する
- [ ] `vrclog completion invalid` がエラーになる
- [ ] `vrclog completion` (引数なし) が使用方法を表示する
- [ ] `vrclog --help` に completion が表示される

## テスト

```bash
# 基本動作確認
go run ./cmd/vrclog completion bash > /dev/null && echo "bash: OK"
go run ./cmd/vrclog completion zsh > /dev/null && echo "zsh: OK"
go run ./cmd/vrclog completion fish > /dev/null && echo "fish: OK"
go run ./cmd/vrclog completion powershell > /dev/null && echo "powershell: OK"

# エラーケース
go run ./cmd/vrclog completion invalid 2>&1 | grep -q "invalid argument"
```

## 注意点/リスク

- Cobraの組み込み completion コマンドとの競合を避けるため、デフォルトを無効化する場合は `rootCmd.CompletionOptions.DisableDefaultCmd = true` を設定
- fish の `includeDescription` は true を推奨（補完候補に説明を表示）

## 参考

- Cobra completion ドキュメント: https://github.com/spf13/cobra/blob/main/shell_completions.md
- 既存のCLI構造: `cmd/vrclog/tail.go`, `cmd/vrclog/parse.go`
