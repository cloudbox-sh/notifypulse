package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/notifypulse/internal/styles"
)

var completionStdout bool

var completionCmd = &cobra.Command{
	Use:       "completion [bash|zsh|fish|powershell]",
	Short:     "Install or print a shell completion script",
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	Args:      cobra.ExactArgs(1),
	Long: "Install a shell completion script for notifypulse.\n\n" +
		"By default the script is written to your shell's standard completion\n" +
		"directory and this command prints the one-line snippet needed to\n" +
		"activate it. Pass --stdout to print the script to stdout instead.\n\n" +
		"Examples:\n" +
		"  notifypulse completion zsh\n" +
		"  notifypulse completion bash --stdout > /etc/bash_completion.d/notifypulse\n" +
		"  notifypulse completion powershell --stdout >> $PROFILE",
	RunE: runCompletion,
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	completionCmd.Flags().BoolVar(&completionStdout, "stdout", false,
		"Print the completion script to stdout instead of writing a file")
	rootCmd.AddCommand(completionCmd)
}

func runCompletion(cmd *cobra.Command, args []string) error {
	shell := args[0]
	if completionStdout {
		return writeCompletionScript(shell, os.Stdout)
	}
	if shell == "powershell" {
		return fmt.Errorf("powershell auto-install is not supported — use:\n" +
			"  notifypulse completion powershell --stdout >> $PROFILE")
	}

	path, enable, err := completionTarget(shell)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create completion directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("write completion script: %w", err)
	}
	defer f.Close()
	if err := writeCompletionScript(shell, f); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, styles.Check()+" installed "+shell+" completion at "+
		styles.Highlight.Render(path))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, styles.Dim.Render("To enable:"))
	fmt.Fprintln(os.Stderr, enable)
	return nil
}

func completionTarget(shell string) (path, enable string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve home directory: %w", err)
	}
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zsh", "completions", "_notifypulse"),
			"  Add to your ~/.zshrc (once), then open a new shell:\n" +
				"    fpath=(~/.zsh/completions $fpath)\n" +
				"    autoload -U compinit && compinit", nil
	case "bash":
		return filepath.Join(home, ".local", "share", "bash-completion", "completions", "notifypulse"),
			"  Already on bash-completion's XDG search path.\n" +
				"  Open a new shell, or source the file directly:\n" +
				"    source " + filepath.Join(home, ".local", "share", "bash-completion", "completions", "notifypulse"), nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "completions", "notifypulse.fish"),
			"  Already on fish's completion path. Open a new shell to pick it up.", nil
	}
	return "", "", fmt.Errorf("unsupported shell: %s", shell)
}

func writeCompletionScript(shell string, w io.Writer) error {
	switch shell {
	case "bash":
		return rootCmd.GenBashCompletionV2(w, true)
	case "zsh":
		return rootCmd.GenZshCompletion(w)
	case "fish":
		return rootCmd.GenFishCompletion(w, true)
	case "powershell":
		return rootCmd.GenPowerShellCompletionWithDesc(w)
	}
	return fmt.Errorf("unsupported shell: %s", shell)
}
