package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/notifypulse/internal/client"
	"github.com/cloudbox-sh/notifypulse/internal/config"
	"github.com/cloudbox-sh/notifypulse/internal/styles"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a one-screen overview — counts + last 24h deliveries",
	Long: "Prints destination / recipient / key counts plus notifications-sent and\n" +
		"delivery success/failure counts for the last 24 hours.\n\n" +
		"Exits non-zero when any deliveries failed in the last 24h — safe to pipe.",
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// /api/dashboard is only exposed under session auth, so we open a
	// short-lived session using the stored email + a password prompt.
	c, resolved, err := openSession("status overview")
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()
	defer func() { _ = c.LogoutSession(ctx) }()

	stats, err := c.DashboardSession(ctx)
	if err != nil {
		return handleAPIError(err)
	}

	degraded := stats.DeliveriesFailed24h > 0

	if err := emit(stats, func() {
		headline := styles.Success.Render("● all deliveries healthy")
		if degraded {
			headline = styles.Warning.Render("● delivery failures in last 24h")
		}
		fmt.Println(headline)
		fmt.Println(styles.Dim.Render("api  ") + resolved.APIURL)
		fmt.Println()

		fmt.Printf("  %s  %s destinations\n",
			styles.Faint.Render("●"), styles.Highlight.Render(fmt.Sprintf("%d", stats.Destinations)))
		fmt.Printf("  %s  %s recipients\n",
			styles.Faint.Render("●"), styles.Highlight.Render(fmt.Sprintf("%d", stats.Recipients)))
		fmt.Printf("  %s  %s api keys\n",
			styles.Faint.Render("●"), styles.Highlight.Render(fmt.Sprintf("%d", stats.APIKeys)))

		fmt.Println()
		fmt.Println(styles.Accent.Render("last 24h"))
		fmt.Printf("  %s  %s notifications sent\n",
			styles.Info.Render("●"), styles.Highlight.Render(fmt.Sprintf("%d", stats.Notifications24h)))
		fmt.Printf("  %s  %s deliveries succeeded\n",
			styles.Success.Render("✓"), styles.Success.Render(fmt.Sprintf("%d", stats.DeliveriesSent24h)))
		failed := fmt.Sprintf("%d", stats.DeliveriesFailed24h)
		if stats.DeliveriesFailed24h == 0 {
			fmt.Printf("  %s  %s deliveries failed\n", styles.Faint.Render("○"), styles.Dim.Render(failed))
		} else {
			fmt.Printf("  %s  %s deliveries failed\n", styles.Error.Render("✗"), styles.Error.Render(failed))
		}
	}); err != nil {
		return err
	}

	if degraded {
		os.Exit(1)
	}
	return nil
}

// openSession opens a short-lived /api/* session using the stored email +
// a password prompt. Used by the two surfaces the server only exposes under
// session auth: /api/dashboard (status) and /api/keys (keys subcommands).
//
// Caller is responsible for calling .LogoutSession on the returned client.
func openSession(purpose string) (*client.Client, *config.Resolved, error) {
	if jsonOutput {
		return nil, nil, fmt.Errorf("--json mode does not support the password prompt that %s needs", purpose)
	}
	c, resolved, err := newAnonClient()
	if err != nil {
		return nil, nil, err
	}
	disk, _ := config.Load()
	email := disk.UserEmail
	if email == "" {
		return nil, nil, fmt.Errorf("no stored email — run 'notifypulse login' first so %s can reauthenticate", purpose)
	}

	fmt.Fprintln(os.Stderr, styles.Dim.Render("→ password required for "+purpose+" — ")+
		styles.Highlight.Render(email))

	var password string
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Password").EchoMode(huh.EchoModePassword).
				Value(&password).Validate(requireNonEmpty("password")),
		),
	).WithTheme(huh.ThemeCatppuccin()).Run(); err != nil {
		return nil, nil, err
	}

	ctx, cancel := signalCtx()
	defer cancel()
	if _, err := c.Login(ctx, strings.TrimSpace(email), password); err != nil {
		return nil, nil, handleAPIError(err)
	}
	return c, resolved, nil
}
