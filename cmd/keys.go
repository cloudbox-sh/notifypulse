package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/notifypulse/internal/styles"
)

var keysCmd = &cobra.Command{
	Use:     "keys",
	Aliases: []string{"key"},
	Short:   "Manage API keys (list, create, revoke)",
	Long: "API key management lives under /api/* (session-authed), so each call\n" +
		"opens a short-lived session using your stored email and a password\n" +
		"prompt. To avoid repeated prompts, run these commands in a batch.",
}

func init() {
	rootCmd.AddCommand(keysCmd)
	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysCreateCmd)
	keysCmd.AddCommand(keysRevokeCmd)

	keysCreateCmd.Flags().StringVar(&keyCreateName, "name", "", "Label for the new key (default: hostname-qualified)")
}

// ── list ─────────────────────────────────────────────────────────────────

var keysListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List API keys (raw secrets are never shown again after creation)",
	RunE:    runKeysList,
}

func runKeysList(cmd *cobra.Command, args []string) error {
	c, _, err := openSession("list api keys")
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()
	defer func() { _ = c.LogoutSession(ctx) }()

	keys, err := c.ListAPIKeysSession(ctx)
	if err != nil {
		return handleAPIError(err, "key", "")
	}
	return emit(keys, func() {
		if len(keys) == 0 {
			fmt.Println(styles.Dim.Render("no keys yet — create one with ") +
				styles.Code.Render("notifypulse keys create"))
			return
		}
		header := lipgloss.JoinHorizontal(lipgloss.Top,
			styles.Header.Width(38).Render("ID"),
			styles.Header.Width(20).Render("NAME"),
			styles.Header.Width(12).Render("PREFIX"),
			styles.Header.Width(20).Render("LAST USED"),
			styles.Header.Width(20).Render("CREATED"),
		)
		fmt.Println(header)
		for _, k := range keys {
			lastUsed := "never"
			if k.LastUsed != nil {
				lastUsed = k.LastUsed.Local().Format("2006-01-02 15:04:05")
			}
			fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top,
				styles.Cell.Width(38).Render(k.ID),
				styles.Cell.Width(20).Render(truncate(k.Name, 18)),
				styles.Cell.Width(12).Render(k.Prefix+"…"),
				styles.Cell.Width(20).Render(lastUsed),
				styles.Cell.Width(20).Render(k.CreatedAt.Local().Format("2006-01-02 15:04:05")),
			))
		}
		fmt.Println()
		fmt.Println(styles.Faint.Render(fmt.Sprintf("%d key(s)", len(keys))))
	})
}

// ── create ───────────────────────────────────────────────────────────────

var keyCreateName string

var keysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new API key (the raw secret is shown once — save it!)",
	RunE:  runKeysCreate,
}

func runKeysCreate(cmd *cobra.Command, args []string) error {
	c, _, err := openSession("create api key")
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()
	defer func() { _ = c.LogoutSession(ctx) }()

	name := keyCreateName
	if name == "" {
		name = defaultKeyName()
	}

	created, err := c.CreateAPIKeySession(ctx, name)
	if err != nil {
		return handleAPIError(err)
	}
	return emitOK("key", created.ID, created,
		styles.Check()+" key created  "+
			styles.Accent.Render(created.RawKey)+"\n"+
			styles.Warning.Render("  ! save this secret now — it will never be shown again"))
}

// ── revoke ───────────────────────────────────────────────────────────────

var keysRevokeCmd = &cobra.Command{
	Use:   "revoke <id>",
	Short: "Revoke an API key by ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runKeysRevoke,
}

func runKeysRevoke(cmd *cobra.Command, args []string) error {
	c, _, err := openSession("revoke api key")
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()
	defer func() { _ = c.LogoutSession(ctx) }()

	if !jsonOutput {
		confirm := false
		if err := huh.NewConfirm().
			Title("Revoke key " + args[0] + "?").
			Description("Requests authenticating with this key will start failing. Destinations and recipients are not affected — they're owned by the user, not the key.").
			Affirmative("Revoke").Negative("Cancel").
			Value(&confirm).WithTheme(huh.ThemeCatppuccin()).Run(); err != nil {
			return err
		}
		if !confirm {
			fmt.Println(styles.Dim.Render("aborted"))
			return nil
		}
	}
	if err := c.RevokeAPIKeySession(ctx, args[0]); err != nil {
		return handleAPIError(err, "key", args[0])
	}
	return emitOK("", args[0], nil, styles.Check()+" key revoked")
}
