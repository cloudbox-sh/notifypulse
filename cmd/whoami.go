package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/notifypulse/internal/config"
	"github.com/cloudbox-sh/notifypulse/internal/styles"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show which account + key the CLI is using",
	RunE:  runWhoami,
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

func runWhoami(cmd *cobra.Command, args []string) error {
	resolved, err := config.Resolve(apiURLFlag)
	if err != nil {
		return err
	}
	if resolved.APIKey == "" {
		return config.ErrNotAuthenticated
	}

	// Load the raw disk config for the metadata we cached at login time
	// (key id/name — these aren't exposed through a /v1 endpoint today).
	disk, _ := config.Load()

	payload := map[string]any{
		"api_url":     resolved.APIURL,
		"auth_source": resolved.Source,
		"user_email":  firstNonEmpty(resolved.UserEmail, disk.UserEmail),
		"key_prefix":  firstNonEmpty(resolved.KeyPrefix, disk.KeyPrefix),
		"key_name":    disk.KeyName,
		"key_id":      disk.KeyID,
	}

	return emit(payload, func() {
		if email := firstNonEmpty(resolved.UserEmail, disk.UserEmail); email != "" {
			fmt.Println(styles.Highlight.Render(email))
		}
		fmt.Println(styles.Dim.Render("api  ") + resolved.APIURL)
		fmt.Println(styles.Dim.Render("auth ") + resolved.Source)
		if prefix := firstNonEmpty(resolved.KeyPrefix, disk.KeyPrefix); prefix != "" {
			keyLine := prefix + "…"
			if disk.KeyName != "" {
				keyLine += "  " + styles.Faint.Render(disk.KeyName)
			}
			fmt.Println(styles.Dim.Render("key  ") + keyLine)
		}
	})
}
