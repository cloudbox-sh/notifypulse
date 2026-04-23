package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/notifypulse/internal/client"
	"github.com/cloudbox-sh/notifypulse/internal/styles"
)

var (
	notifyTitle    string
	notifyBody     string
	notifyTo       string
	notifyDests    []string
	notifySeverity string
	notifyLink     string
	notifyDedup    string
	notifyBodyFile string
)

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Send a notification (the main event)",
	Long: "Fan out a notification to a recipient or an ad-hoc destination list.\n\n" +
		"Examples:\n" +
		"  notifypulse notify --to on-call --title 'DB at 90% capacity' --severity urgent\n" +
		"  notifypulse notify --destination ops-slack --title 'deploy complete'\n" +
		"  echo 'long body' | notifypulse notify --to team --title 'weekly digest' --body-file -\n" +
		"  notifypulse notify --to me --title 'build broken' --link https://ci.example/runs/42 \\\n" +
		"     --dedup-key 'ci-build-42' --severity urgent",
	RunE: runNotify,
}

func init() {
	rootCmd.AddCommand(notifyCmd)

	f := notifyCmd.Flags()
	f.StringVar(&notifyTitle, "title", "", "Notification title (required)")
	f.StringVar(&notifyBody, "body", "", "Notification body (plain text)")
	f.StringVar(&notifyBodyFile, "body-file", "", "Read body from file (use '-' for stdin)")
	f.StringVar(&notifyTo, "to", "", "Recipient name to fan out to")
	f.StringSliceVar(&notifyDests, "destination", nil,
		"Destination name/id to send to (repeatable; alternative to --to)")
	f.StringVar(&notifySeverity, "severity", "", "urgent | normal | digest (default normal)")
	f.StringVar(&notifyLink, "link", "", "URL to attach (shown as a button on Discord/Slack)")
	f.StringVar(&notifyDedup, "dedup-key", "",
		"Swallow duplicates with the same key within 5 minutes")
}

func runNotify(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	interactive := notifyTitle == "" || (notifyTo == "" && len(notifyDests) == 0)
	if interactive {
		if err := requireFlagsForJSON("--title and one of --to / --destination"); err != nil {
			return err
		}
		if err := notifyForm(ctx, c); err != nil {
			return err
		}
	}

	body := notifyBody
	if notifyBodyFile != "" {
		b, err := readBodyFile(notifyBodyFile)
		if err != nil {
			return err
		}
		body = b
	}

	severity := notifySeverity
	if severity == "" {
		severity = "normal"
	}
	if !validSeverity(severity) {
		return fmt.Errorf("invalid severity %q — must be urgent, normal, or digest", severity)
	}

	// Validate local destination refs before sending so a typo doesn't
	// produce a round-trip 400.
	if len(notifyDests) > 0 {
		if err := resolveDestinationIDs(ctx, c, notifyDests); err != nil {
			return err
		}
	}

	resp, err := c.Notify(ctx, client.NotifyRequest{
		Title:        strings.TrimSpace(notifyTitle),
		Body:         body,
		To:           strings.TrimSpace(notifyTo),
		Destinations: notifyDests,
		Severity:     severity,
		Link:         strings.TrimSpace(notifyLink),
		DedupKey:     strings.TrimSpace(notifyDedup),
	})
	if err != nil {
		return handleAPIError(err)
	}

	// Exit codes for scripts: 0=sent, 2=partial, 3=failed, 4=deduped.
	exitCode := 0
	switch {
	case resp.Deduped:
		exitCode = 4
	case resp.Status == "failed":
		exitCode = 3
	case resp.Status == "partial":
		exitCode = 2
	}

	if err := emit(resp, func() {
		glyph := styles.DeliveryGlyph(resp.Status)
		if resp.Deduped {
			glyph = styles.DeliveryGlyph("deduped")
		}
		fmt.Printf("%s %s  %s\n",
			glyph, colorNotifyStatus(resp.Status, resp.Deduped),
			styles.Faint.Render(resp.ID))

		if resp.Deduped {
			fmt.Println(styles.Dim.Render("  skipped — same dedup key seen within 5 minutes"))
			return
		}
		for _, d := range resp.Deliveries {
			line := fmt.Sprintf("  %s %-20s %s",
				styles.DeliveryGlyph(d.Status),
				d.DestinationName,
				styles.ChannelColor(d.Channel))
			if d.Error != "" {
				line += "  " + styles.Error.Render(truncate(d.Error, 80))
			}
			fmt.Println(line)
		}
	}); err != nil {
		return err
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}

// notifyForm prompts for missing fields, pulling the user's recipients and
// destinations for the selector.
func notifyForm(ctx context.Context, c *client.Client) error {
	recs, _ := c.ListRecipients(ctx)
	dests, _ := c.ListDestinations(ctx)

	mode := "recipient"
	if len(notifyDests) > 0 && notifyTo == "" {
		mode = "destinations"
	}

	recOpts := make([]huh.Option[string], 0, len(recs))
	for _, r := range recs {
		label := r.Name
		if len(r.Destinations) > 0 {
			label += fmt.Sprintf("  (%d dests)", len(r.Destinations))
		}
		recOpts = append(recOpts, huh.NewOption(label, r.Name))
	}
	destOpts := make([]huh.Option[string], 0, len(dests))
	for _, d := range dests {
		destOpts = append(destOpts, huh.NewOption(d.Name+"  ["+d.Channel+"]", d.Name))
	}

	sev := notifySeverity
	if sev == "" {
		sev = "normal"
	}
	target := notifyTo

	modeField := huh.NewSelect[string]().Title("Send mode").Options(
		huh.NewOption("To a recipient (fan-out)", "recipient"),
		huh.NewOption("To one or more destinations directly", "destinations"),
	).Value(&mode)

	titleField := huh.NewInput().Title("Title").Value(&notifyTitle).
		Validate(requireNonEmpty("title"))
	bodyField := huh.NewText().Title("Body (optional)").Value(&notifyBody).Lines(4)
	severityField := huh.NewSelect[string]().Title("Severity").Options(
		huh.NewOption("urgent", "urgent"),
		huh.NewOption("normal", "normal"),
		huh.NewOption("digest", "digest"),
	).Value(&sev)
	linkField := huh.NewInput().Title("Link (optional)").
		Description("URL attached to the notification (shown as a button on chat apps).").
		Value(&notifyLink)

	var targetField huh.Field
	if len(recOpts) > 0 {
		targetField = huh.NewSelect[string]().
			Title("Recipient").Options(recOpts...).Value(&target)
	} else {
		targetField = huh.NewInput().Title("Recipient name").
			Value(&target).Validate(requireNonEmpty("recipient"))
	}

	var destField huh.Field
	if len(destOpts) > 0 {
		destField = huh.NewMultiSelect[string]().
			Title("Destinations").Options(destOpts...).Value(&notifyDests).
			Validate(func(s []string) error {
				if len(s) == 0 {
					return fmt.Errorf("pick at least one destination")
				}
				return nil
			})
	} else {
		return fmt.Errorf("no destinations exist yet — create one with 'destinations create'")
	}

	groups := []*huh.Group{
		huh.NewGroup(modeField, titleField, bodyField, severityField, linkField),
		huh.NewGroup(targetField).WithHideFunc(func() bool { return mode != "recipient" }),
		huh.NewGroup(destField).WithHideFunc(func() bool { return mode != "destinations" }),
	}
	if err := huh.NewForm(groups...).WithTheme(huh.ThemeCatppuccin()).Run(); err != nil {
		return err
	}

	notifySeverity = sev
	if mode == "recipient" {
		notifyTo = target
		notifyDests = nil
	} else {
		notifyTo = ""
	}
	return nil
}

func readBodyFile(path string) (string, error) {
	if path == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(b), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(b), nil
}

func validSeverity(s string) bool {
	switch s {
	case "urgent", "normal", "digest":
		return true
	}
	return false
}

func colorNotifyStatus(status string, deduped bool) string {
	if deduped {
		return styles.Dim.Render("deduped")
	}
	switch status {
	case "sent":
		return styles.Success.Render("sent")
	case "partial":
		return styles.Warning.Render("partial")
	case "failed":
		return styles.Error.Render("failed")
	}
	return styles.Dim.Render(status)
}
