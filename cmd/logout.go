package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/notifypulse/internal/config"
	"github.com/cloudbox-sh/notifypulse/internal/styles"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Forget the stored API key (does not revoke it server-side)",
	Long: "Clears the local config file. The API key is NOT revoked server-side —\n" +
		"run 'notifypulse keys revoke <id>' first if you want the key itself dead.",
	RunE: runLogout,
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}

func runLogout(cmd *cobra.Command, args []string) error {
	if err := config.Clear(); err != nil {
		return fmt.Errorf("clear config: %w", err)
	}
	if jsonOutput {
		return emit(map[string]any{"ok": true}, nil)
	}
	fmt.Fprintln(os.Stderr, styles.Check()+" logged out")
	return nil
}
