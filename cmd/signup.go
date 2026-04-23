package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/notifypulse/internal/config"
	"github.com/cloudbox-sh/notifypulse/internal/styles"
)

var signupCmd = &cobra.Command{
	Use:   "signup",
	Short: "Create a new Notifypulse account and mint a CLI key",
	Long: "Creates a new account, establishes a session, mints a CLI API key,\n" +
		"and stores it locally — the same end-state as 'login'.\n\n" +
		"Signups may be disabled server-side (NOTIFYPULSE_SIGNUPS_ENABLED).",
	RunE: runSignup,
}

func init() {
	rootCmd.AddCommand(signupCmd)
}

func runSignup(cmd *cobra.Command, args []string) error {
	c, resolved, err := newAnonClient()
	if err != nil {
		return err
	}
	if jsonOutput {
		return fmt.Errorf("--json mode does not support interactive signup")
	}

	fmt.Println(styles.Accent.Render("→ notifypulse signup") +
		styles.Dim.Render(" ("+resolved.APIURL+")"))

	var email, name, password, confirm string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Email").Value(&email).Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return fmt.Errorf("email is required")
				}
				if !strings.Contains(s, "@") {
					return fmt.Errorf("that doesn't look like an email")
				}
				return nil
			}),
			huh.NewInput().Title("Name").Description("Displayed in the dashboard (optional).").Value(&name),
			huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).
				Value(&password).Validate(func(s string) error {
				if len(s) < 8 {
					return fmt.Errorf("password must be at least 8 characters")
				}
				return nil
			}),
			huh.NewInput().Title("Confirm password").EchoMode(huh.EchoModePassword).
				Value(&confirm).Validate(func(s string) error {
				if s != password {
					return fmt.Errorf("passwords do not match")
				}
				return nil
			}),
		),
	).WithTheme(huh.ThemeCatppuccin())
	if err := form.Run(); err != nil {
		return err
	}

	ctx, cancel := signalCtx()
	defer cancel()

	user, err := c.Signup(ctx, strings.TrimSpace(email), password, strings.TrimSpace(name))
	if err != nil {
		return handleAPIError(err)
	}

	created, err := c.CreateAPIKeySession(ctx, defaultKeyName())
	if err != nil {
		_ = c.LogoutSession(ctx)
		return fmt.Errorf("mint CLI key: %w", err)
	}
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
	fmt.Fprintln(os.Stderr, styles.Check()+" account created for "+
		styles.Highlight.Render(user.Email)+
		styles.Dim.Render(" (key "+created.Prefix+"… stored at "+path+")"))
	return nil
}
