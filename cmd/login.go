package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/notifypulse/internal/client"
	"github.com/cloudbox-sh/notifypulse/internal/config"
	"github.com/cloudbox-sh/notifypulse/internal/styles"
)

// hostname-qualified key name makes it easy to see which machine minted it.
func defaultKeyName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "cli"
	}
	return "cli-" + host
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate against the Notifypulse API",
	Long: "Prompts for email + password, mints a CLI API key, and stores it at\n" +
		"~/.config/cloudbox/notifypulse.json.\n\n" +
		"If you already have a raw API key from the dashboard, pass --with-key to\n" +
		"skip the email/password flow and just paste it.",
	RunE: runLogin,
}

var loginWithKey bool

func init() {
	loginCmd.Flags().BoolVar(&loginWithKey, "with-key", false,
		"Paste an existing raw API key instead of logging in with email+password")
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	c, resolved, err := newAnonClient()
	if err != nil {
		return err
	}

	if jsonOutput {
		return fmt.Errorf("--json mode does not support interactive login — set NOTIFYPULSE_API_KEY instead")
	}

	fmt.Println(styles.Accent.Render("→ notifypulse login") +
		styles.Dim.Render(" ("+resolved.APIURL+")"))

	ctx, cancel := signalCtx()
	defer cancel()

	if loginWithKey {
		return loginWithRawKey(ctx, c, resolved)
	}
	return loginWithPassword(ctx, c, resolved)
}

// loginWithPassword uses email/password to open a short-lived session, mints
// a CLI API key through /api/keys, and then tears the session down. The raw
// key is stored locally; the session cookie is discarded.
func loginWithPassword(ctx context.Context, c *client.Client, resolved *config.Resolved) error {
	var email, password string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Email").
				Value(&email).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("email is required")
					}
					if !strings.Contains(s, "@") {
						return fmt.Errorf("that doesn't look like an email")
					}
					return nil
				}),
			huh.NewInput().
				Title("Password").
				EchoMode(huh.EchoModePassword).
				Value(&password).
				Validate(requireNonEmpty("password")),
		),
	).WithTheme(huh.ThemeCatppuccin())
	if err := form.Run(); err != nil {
		return err
	}

	user, err := c.Login(ctx, strings.TrimSpace(email), password)
	if err != nil {
		return handleAPIError(err)
	}

	created, err := c.CreateAPIKeySession(ctx, defaultKeyName())
	if err != nil {
		// Best-effort session cleanup even on failure.
		_ = c.LogoutSession(ctx)
		return fmt.Errorf("mint CLI key: %w", err)
	}

	// Session has done its job — discard it so we don't leave a hanging cookie.
	_ = c.LogoutSession(ctx)

	cfg := &config.Config{
		APIURL:    resolved.APIURL,
		APIKey:    created.RawKey,
		KeyID:     created.ID,
		KeyName:   created.Name,
		KeyPrefix: created.Prefix,
		UserEmail: user.Email,
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	path, _ := config.Path()
	fmt.Fprintln(os.Stderr, styles.Check()+" logged in as "+
		styles.Highlight.Render(user.Email)+
		styles.Dim.Render(" (key "+created.Prefix+"… stored at "+path+")"))
	return nil
}

// loginWithRawKey stores a pre-existing raw key without going through the
// session flow. The key must exist server-side already.
func loginWithRawKey(ctx context.Context, c *client.Client, resolved *config.Resolved) error {
	var raw string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("API key").
				Description("Paste a raw key minted in the dashboard — starts with np_").
				EchoMode(huh.EchoModePassword).
				Value(&raw).
				Validate(func(s string) error {
					s = strings.TrimSpace(s)
					if s == "" {
						return fmt.Errorf("key is required")
					}
					if !strings.HasPrefix(s, "np_") {
						return fmt.Errorf("keys start with 'np_'")
					}
					return nil
				}),
		),
	).WithTheme(huh.ThemeCatppuccin())
	if err := form.Run(); err != nil {
		return err
	}
	raw = strings.TrimSpace(raw)

	// Validate the key by calling a cheap authenticated endpoint.
	verify := client.New(resolved.APIURL, raw).
		WithBasicAuth(os.Getenv("NOTIFYPULSE_BASIC_USER"), os.Getenv("NOTIFYPULSE_BASIC_PASS")).
		WithDebug(debugFlag, jsonOutput)
	if _, err := verify.ListDestinations(ctx); err != nil {
		return fmt.Errorf("key rejected: %w", handleAPIError(err))
	}

	prefix := ""
	if parts := strings.SplitN(strings.TrimPrefix(raw, "np_"), "_", 2); len(parts) == 2 {
		prefix = parts[0]
	}

	cfg := &config.Config{
		APIURL:    resolved.APIURL,
		APIKey:    raw,
		KeyPrefix: prefix,
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	path, _ := config.Path()
	fmt.Fprintln(os.Stderr, styles.Check()+" key saved "+
		styles.Dim.Render("("+prefix+"… at "+path+")"))
	return nil
}
