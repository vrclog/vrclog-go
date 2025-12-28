package main

import (
	"strings"

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
	Args:                  cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Usage()
		}

		root := cmd.Root()
		out := cmd.OutOrStdout()

		switch args[0] {
		case "bash":
			return root.GenBashCompletionV2(out, true)
		case "zsh":
			return root.GenZshCompletion(out)
		case "fish":
			return root.GenFishCompletion(out, true)
		case "powershell":
			return root.GenPowerShellCompletionWithDesc(out)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

// completeEventTypes returns a completion function for event type flags.
// It supports comma-separated values and excludes already-selected types.
// Returns full values (prefix + candidate) for reliable cross-shell behavior.
func completeEventTypes(flagName string) func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		parts := strings.Split(toComplete, ",")
		prefix := strings.Join(parts[:len(parts)-1], ",")
		if prefix != "" {
			prefix += ","
		}
		current := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))

		// Track already-used values
		used := make(map[string]struct{})
		addUsed := func(v string) {
			v = strings.ToLower(strings.TrimSpace(v))
			if v != "" {
				used[v] = struct{}{}
			}
		}

		// Values from current input
		for _, p := range parts[:len(parts)-1] {
			addUsed(p)
		}

		// Values already set on the flag (for repeated flag usage)
		if vals, err := cmd.Flags().GetStringSlice(flagName); err == nil {
			for _, v := range vals {
				addUsed(v)
			}
		}

		// Build candidates from valid event types
		allTypes := ValidEventTypeNames()
		var candidates []string
		for _, t := range allTypes {
			if _, ok := used[t]; ok {
				continue
			}
			if strings.HasPrefix(t, current) {
				candidates = append(candidates, prefix+t)
			}
		}

		return candidates, cobra.ShellCompDirectiveNoSpace | cobra.ShellCompDirectiveNoFileComp
	}
}

// registerEventTypeCompletion registers completion for an event type flag.
func registerEventTypeCompletion(cmd *cobra.Command, flagName string) {
	_ = cmd.RegisterFlagCompletionFunc(flagName, completeEventTypes(flagName))
}
