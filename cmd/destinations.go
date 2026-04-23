package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/mdp/qrterminal/v3"
	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/notifypulse/internal/client"
	"github.com/cloudbox-sh/notifypulse/internal/styles"
)

var destinationsCmd = &cobra.Command{
	Use:     "destinations",
	Aliases: []string{"destination", "dest", "d"},
	Short:   "Manage delivery destinations (telegram, discord, slack, email, webhook)",
}

// Keep in sync with the server's CHECK constraint in 0002_notifypulse_schema.sql.
var supportedChannels = []string{"webhook", "email", "telegram", "discord", "slack"}

func init() {
	rootCmd.AddCommand(destinationsCmd)
	destinationsCmd.AddCommand(destinationsListCmd)
	destinationsCmd.AddCommand(destinationsGetCmd)
	destinationsCmd.AddCommand(destinationsCreateCmd)
	destinationsCmd.AddCommand(destinationsDeleteCmd)

	initDestinationsCreateFlags()
}

// ── list ─────────────────────────────────────────────────────────────────

var destinationsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List destinations",
	RunE:    runDestinationsList,
}

func runDestinationsList(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	dests, err := c.ListDestinations(ctx)
	if err != nil {
		return handleAPIError(err, "destination", "")
	}
	return emit(dests, func() {
		if len(dests) == 0 {
			fmt.Println(styles.Dim.Render("no destinations yet — create one with ") +
				styles.Code.Render("notifypulse destinations create"))
			return
		}
		header := lipgloss.JoinHorizontal(lipgloss.Top,
			styles.Header.Width(38).Render("ID"),
			styles.Header.Width(24).Render("NAME"),
			styles.Header.Width(10).Render("CHANNEL"),
			styles.Header.Width(40).Render("CONFIG"),
		)
		fmt.Println(header)
		for _, d := range dests {
			fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top,
				styles.Cell.Width(38).Render(d.ID),
				styles.Cell.Width(24).Render(truncate(d.Name, 22)),
				styles.Cell.Width(10).Render(styles.ChannelColor(d.Channel)),
				styles.Cell.Width(40).Render(truncate(summarizeConfig(d.Channel, d.Config), 38)),
			))
		}
		fmt.Println()
		fmt.Println(styles.Faint.Render(fmt.Sprintf("%d destination(s)", len(dests))))
	})
}

// ── get ──────────────────────────────────────────────────────────────────

var destinationsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show a destination's full configuration",
	Args:  cobra.ExactArgs(1),
	RunE:  runDestinationsGet,
}

func runDestinationsGet(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	d, err := c.GetDestination(ctx, args[0])
	if err != nil {
		return handleAPIError(err, "destination", args[0])
	}
	return emit(d, func() {
		fmt.Println(styles.Highlight.Render(d.Name) + "  " +
			styles.ChannelColor(d.Channel))
		fmt.Println(styles.Dim.Render("id      ") + d.ID)
		fmt.Println(styles.Dim.Render("channel ") + d.Channel)
		fmt.Println(styles.Dim.Render("created ") + d.CreatedAt.Local().Format("2006-01-02 15:04:05"))
		fmt.Println()
		fmt.Println(styles.Accent.Render("config"))
		pretty, _ := json.MarshalIndent(json.RawMessage(d.Config), "  ", "  ")
		fmt.Println("  " + string(pretty))
	})
}

// ── create ───────────────────────────────────────────────────────────────

var (
	destCreateName       string
	destCreateChannel    string
	destCreateConfig     string
	destCreateURL        string
	destCreateEmail      string
	destCreateBindingID  string
	destCreateWebhookURL string
)

func initDestinationsCreateFlags() {
	f := destinationsCreateCmd.Flags()
	f.StringVar(&destCreateName, "name", "", "Destination label (unique per user)")
	f.StringVar(&destCreateChannel, "channel", "", "Channel: webhook, email, telegram, discord, slack")
	f.StringVar(&destCreateConfig, "config", "", "Raw JSON config (escape hatch when shorthand flags don't cover it)")
	f.StringVar(&destCreateURL, "url", "", "Webhook URL (shorthand for channel=webhook)")
	f.StringVar(&destCreateEmail, "email", "", "Destination email (shorthand for channel=email)")
	f.StringVar(&destCreateBindingID, "binding-id", "",
		"Telegram binding id (channel=telegram). Skip to run the interactive QR-deeplink connect flow.")
	f.StringVar(&destCreateWebhookURL, "webhook-url", "",
		"Discord or Slack incoming-webhook URL (channel=discord|slack)")
}

var destinationsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a destination (interactive form or flag-driven)",
	Long: "Create a delivery destination.\n\n" +
		"Common shortcuts:\n" +
		"  notifypulse destinations create --name ops --email ops@example.com\n" +
		"  notifypulse destinations create --name ci-hook --url https://hooks.example.com/x\n" +
		"  notifypulse destinations create --name personal --channel telegram\n" +
		"      (interactive: prints a QR + deeplink, polls for the Start tap)\n" +
		"  notifypulse destinations create --name dev --channel discord \\\n" +
		"     --webhook-url https://discord.com/api/webhooks/…\n\n" +
		"For Telegram, the CLI manages the connect flow with the Notifypulse-\n" +
		"managed bot (@cloudbox_notifypulse_bot). Scan the QR with the Telegram\n" +
		"app on your phone, or click the deeplink if Telegram Desktop is\n" +
		"installed. Pass --binding-id directly to skip the connect flow.\n\n" +
		"For anything unusual, pass raw JSON via --config and --channel.",
	RunE: runDestinationsCreate,
}

func runDestinationsCreate(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	channel := destCreateChannel
	if channel == "" {
		switch {
		case destCreateEmail != "":
			channel = "email"
		case destCreateURL != "":
			channel = "webhook"
		case destCreateBindingID != "":
			channel = "telegram"
		}
	}

	name := destCreateName

	// Have we got enough data to skip the form?
	haveConfig := destCreateConfig != "" || destCreateEmail != "" || destCreateURL != "" ||
		destCreateWebhookURL != "" || destCreateBindingID != ""
	interactive := name == "" || channel == "" || !haveConfig

	if interactive {
		if err := requireFlagsForJSON("--name, --channel, and one of --email/--url/--webhook-url/--binding-id/--config"); err != nil {
			return err
		}
		if err := destinationCreateForm(&name, &channel); err != nil {
			return err
		}
	}

	// Telegram requires a binding_id, which only exists once the user has
	// tapped Start in a chat with the managed bot. If the caller didn't
	// supply one, run the QR-deeplink connect flow now.
	if channel == "telegram" && destCreateBindingID == "" && destCreateConfig == "" {
		bindingID, err := connectTelegram(ctx, c)
		if err != nil {
			return err
		}
		destCreateBindingID = bindingID
	}

	config, err := buildDestinationConfig(channel,
		destCreateEmail, destCreateURL, destCreateWebhookURL,
		destCreateBindingID, destCreateConfig)
	if err != nil {
		return err
	}

	d, err := c.CreateDestination(ctx, client.CreateDestinationRequest{
		Name:    strings.TrimSpace(name),
		Channel: channel,
		Config:  config,
	})
	if err != nil {
		return handleAPIError(err, "destination", "")
	}
	return emitOK("destination", d.ID, d,
		styles.Check()+" destination created "+styles.Faint.Render(d.ID))
}

// connectTelegram drives the QR-deeplink connect flow:
//  1. POST /v1/telegram/connect to mint a single-use token
//  2. render the deeplink URL as a terminal QR (scan with phone) + print the
//     URL itself (click on desktop)
//  3. poll every 2s until the binding lands or the token expires
func connectTelegram(ctx context.Context, c *client.Client) (string, error) {
	resp, err := c.IssueTelegramConnect(ctx)
	if err != nil {
		return "", handleAPIError(err, "telegram-connect", "")
	}

	// Friendly preamble — only when stdout is a terminal. JSON mode reaches
	// here only via --binding-id (handled above), so a TTY is the safe
	// assumption, but be defensive.
	if !jsonOutput {
		fmt.Println()
		fmt.Println(styles.Accent.Render("Connect Telegram"))
		fmt.Println(styles.Faint.Render("Scan with the Telegram app on your phone, or open the link below if Telegram Desktop is installed."))
		fmt.Println()
		qrterminal.GenerateWithConfig(resp.URL, qrterminal.Config{
			Level:     qrterminal.M,
			Writer:    os.Stdout,
			BlackChar: qrterminal.BLACK,
			WhiteChar: qrterminal.WHITE,
			QuietZone: 1,
		})
		fmt.Println(styles.Faint.Render(resp.URL))
		fmt.Println()
		fmt.Println(styles.Faint.Render(
			"Tap Start in the chat with @" + resp.BotUsername + ". For groups/channels, " +
				"add the bot and post: /start@" + resp.BotUsername + " " + resp.Token[:8] + "…",
		))
		fmt.Println()
		fmt.Println(styles.Faint.Render("Waiting…"))
	}

	// Poll every 2 seconds. Server enforces a 15-min token TTL but bound
	// the wait here too so a backgrounded CLI doesn't hang forever.
	deadline := time.Now().Add(15 * time.Minute)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	for {
		status, err := c.PollTelegramConnect(ctx, resp.Token)
		if err != nil {
			return "", handleAPIError(err, "telegram-connect", "")
		}
		switch status.Status {
		case "consumed":
			if !jsonOutput {
				fmt.Println(styles.Check() + " telegram chat bound " + styles.Faint.Render(status.BindingID))
				fmt.Println()
			}
			return status.BindingID, nil
		case "expired", "unknown":
			return "", fmt.Errorf("connect link expired before it was used — try again")
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timed out waiting for Telegram Start tap (15 min)")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-tick.C:
		}
	}
}

// destinationCreateForm walks the user through a channel-aware TUI form.
// It writes back to *name / *channel and the package-level shorthand flag
// vars (so buildDestinationConfig can pick them up uniformly).
func destinationCreateForm(name, channel *string) error {
	chOpts := make([]huh.Option[string], 0, len(supportedChannels))
	for _, ch := range supportedChannels {
		chOpts = append(chOpts, huh.NewOption(ch, ch))
	}
	if *channel == "" {
		*channel = "email"
	}

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Name").
				Description("Unique label shown in dashboards and logs.").
				Value(name).Validate(requireNonEmpty("name")),
			huh.NewSelect[string]().Title("Channel").
				Options(chOpts...).Value(channel),
		),
		// Email ----------------------------------------------------------
		huh.NewGroup(
			huh.NewInput().Title("Email address").
				Value(&destCreateEmail).
				Validate(func(s string) error {
					if !strings.Contains(s, "@") {
						return fmt.Errorf("not an email")
					}
					return nil
				}),
		).WithHideFunc(func() bool { return *channel != "email" }),
		// Webhook --------------------------------------------------------
		huh.NewGroup(
			huh.NewInput().Title("Webhook URL").
				Description("Notifypulse POSTs a JSON body with title/body/severity/link.").
				Value(&destCreateURL).Validate(requireNonEmpty("url")),
		).WithHideFunc(func() bool { return *channel != "webhook" }),
		// Telegram has no form fields — the QR-deeplink connect flow runs
		// after the form closes (runDestinationsCreate calls connectTelegram)
		// and writes the binding id back into destCreateBindingID.
		// Discord --------------------------------------------------------
		huh.NewGroup(
			huh.NewInput().Title("Discord webhook URL").
				Description("Channel Settings → Integrations → Webhooks → New Webhook.").
				Value(&destCreateWebhookURL).Validate(requireNonEmpty("webhook url")),
		).WithHideFunc(func() bool { return *channel != "discord" }),
		// Slack ----------------------------------------------------------
		huh.NewGroup(
			huh.NewInput().Title("Slack webhook URL").
				Description("https://hooks.slack.com/services/… — from Slack App → Incoming Webhooks.").
				Value(&destCreateWebhookURL).Validate(requireNonEmpty("webhook url")),
		).WithHideFunc(func() bool { return *channel != "slack" }),
	).WithTheme(huh.ThemeCatppuccin()).Run()
	return err
}

// buildDestinationConfig assembles the channel-specific JSON config blob.
// Raw --config wins over shorthand flags when both are supplied.
func buildDestinationConfig(channel, email, webhookURL, incomingURL, bindingID, raw string) (json.RawMessage, error) {
	if raw != "" {
		if !json.Valid([]byte(raw)) {
			return nil, fmt.Errorf("--config is not valid JSON")
		}
		return json.RawMessage(raw), nil
	}
	switch channel {
	case "email":
		if email == "" {
			return nil, fmt.Errorf("email channel requires --email")
		}
		return mustJSON(map[string]string{"email": email}), nil
	case "webhook":
		if webhookURL == "" {
			return nil, fmt.Errorf("webhook channel requires --url")
		}
		return mustJSON(map[string]string{"url": webhookURL}), nil
	case "telegram":
		if bindingID == "" {
			return nil, fmt.Errorf("telegram channel requires --binding-id (or run without it for the interactive QR-deeplink connect flow)")
		}
		return mustJSON(map[string]string{"binding_id": bindingID}), nil
	case "discord", "slack":
		if incomingURL == "" {
			return nil, fmt.Errorf("%s channel requires --webhook-url", channel)
		}
		return mustJSON(map[string]string{"webhook_url": incomingURL}), nil
	}
	return nil, fmt.Errorf("unsupported channel %q — pass --config with raw JSON", channel)
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// summarizeConfig renders a short, human-readable preview of a destination's
// config for list output. Secrets (bot token) are masked.
func summarizeConfig(channel string, raw json.RawMessage) string {
	switch channel {
	case "email":
		var cfg struct {
			Email string `json:"email"`
		}
		_ = json.Unmarshal(raw, &cfg)
		return cfg.Email
	case "webhook":
		var cfg struct {
			URL string `json:"url"`
		}
		_ = json.Unmarshal(raw, &cfg)
		return cfg.URL
	case "telegram":
		var cfg struct {
			BotToken string `json:"bot_token"`
			ChatID   string `json:"chat_id"`
		}
		_ = json.Unmarshal(raw, &cfg)
		return "chat=" + cfg.ChatID + "  token=" + maskToken(cfg.BotToken)
	case "discord", "slack":
		var cfg struct {
			WebhookURL string `json:"webhook_url"`
		}
		_ = json.Unmarshal(raw, &cfg)
		return cfg.WebhookURL
	}
	return string(raw)
}

func maskToken(t string) string {
	if len(t) <= 6 {
		return "***"
	}
	return t[:3] + "…" + t[len(t)-3:]
}

// ── delete ───────────────────────────────────────────────────────────────

var destinationsDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a destination",
	Args:    cobra.ExactArgs(1),
	RunE:    runDestinationsDelete,
}

func runDestinationsDelete(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	if !jsonOutput {
		confirm := false
		if err := huh.NewConfirm().
			Title("Delete destination " + args[0] + "?").
			Description("Any recipients bound to it will lose this destination.").
			Affirmative("Delete").Negative("Cancel").
			Value(&confirm).WithTheme(huh.ThemeCatppuccin()).Run(); err != nil {
			return err
		}
		if !confirm {
			fmt.Println(styles.Dim.Render("aborted"))
			return nil
		}
	}
	if err := c.DeleteDestination(ctx, args[0]); err != nil {
		return handleAPIError(err, "destination", args[0])
	}
	return emitOK("", args[0], nil, styles.Check()+" destination deleted")
}

// resolveDestinationIDs takes a mixed slice of UUIDs and names and returns
// the caller's preferred forms (names accepted by /v1/notify's
// `destinations` array). Used by notify.go to validate against the user's
// current destination list before sending.
func resolveDestinationIDs(ctx context.Context, c *client.Client, refs []string) error {
	if len(refs) == 0 {
		return nil
	}
	dests, err := c.ListDestinations(ctx)
	if err != nil {
		return handleAPIError(err, "destination", "")
	}
	known := map[string]bool{}
	for _, d := range dests {
		known[d.ID] = true
		known[d.Name] = true
	}
	for _, r := range refs {
		if !known[r] {
			return fmt.Errorf("unknown destination %q — run %s",
				r, styles.Code.Render("notifypulse destinations list"))
		}
	}
	return nil
}
