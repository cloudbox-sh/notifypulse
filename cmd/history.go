package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/notifypulse/internal/styles"
)

var historyCmd = &cobra.Command{
	Use:     "history",
	Aliases: []string{"notifications", "notifs"},
	Short:   "View recent notifications and their delivery outcomes",
}

var (
	historyListLimit int
)

func init() {
	rootCmd.AddCommand(historyCmd)
	historyCmd.AddCommand(historyListCmd)
	historyCmd.AddCommand(historyGetCmd)

	historyListCmd.Flags().IntVar(&historyListLimit, "limit", 50,
		"Maximum rows to fetch (1-500)")
}

// ── list ─────────────────────────────────────────────────────────────────

var historyListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List recent notifications (newest first)",
	RunE:    runHistoryList,
}

func runHistoryList(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	notifs, err := c.ListNotifications(ctx, historyListLimit)
	if err != nil {
		return handleAPIError(err, "notification", "")
	}
	return emit(notifs, func() {
		if len(notifs) == 0 {
			fmt.Println(styles.Dim.Render("no notifications yet — send one with ") +
				styles.Code.Render("notifypulse notify"))
			return
		}
		header := lipgloss.JoinHorizontal(lipgloss.Top,
			styles.Header.Width(4).Render(""),
			styles.Header.Width(38).Render("ID"),
			styles.Header.Width(20).Render("WHEN"),
			styles.Header.Width(9).Render("SEVERITY"),
			styles.Header.Width(46).Render("TITLE"),
		)
		fmt.Println(header)
		for _, n := range notifs {
			fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top,
				styles.Cell.Width(4).Render(styles.SeverityGlyph(n.Severity)),
				styles.Cell.Width(38).Render(n.ID),
				styles.Cell.Width(20).Render(n.CreatedAt.Local().Format("2006-01-02 15:04:05")),
				styles.Cell.Width(9).Render(colorSeverity(n.Severity)),
				styles.Cell.Width(46).Render(truncate(n.Title, 44)),
			))
		}
		fmt.Println()
		fmt.Println(styles.Faint.Render(fmt.Sprintf("%d notification(s)", len(notifs))))
	})
}

// ── get ──────────────────────────────────────────────────────────────────

var historyGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show a notification with its full delivery log",
	Args:  cobra.ExactArgs(1),
	RunE:  runHistoryGet,
}

func runHistoryGet(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	n, err := c.GetNotification(ctx, args[0])
	if err != nil {
		return handleAPIError(err, "notification", args[0])
	}
	return emit(n, func() {
		fmt.Printf("%s %s\n",
			styles.SeverityGlyph(n.Severity),
			styles.Highlight.Render(n.Title))
		fmt.Println(styles.Dim.Render("id        ") + n.ID)
		fmt.Println(styles.Dim.Render("when      ") + n.CreatedAt.Local().Format("2006-01-02 15:04:05"))
		fmt.Println(styles.Dim.Render("severity  ") + colorSeverity(n.Severity))
		if n.Link != "" {
			fmt.Println(styles.Dim.Render("link      ") + n.Link)
		}
		if n.DedupKey != "" {
			fmt.Println(styles.Dim.Render("dedup_key ") + n.DedupKey)
		}
		if n.Body != "" {
			fmt.Println()
			fmt.Println(styles.Accent.Render("body"))
			fmt.Println("  " + indentLines(n.Body, "  "))
		}

		fmt.Println()
		fmt.Println(styles.Accent.Render(fmt.Sprintf("deliveries (%d)", len(n.Deliveries))))
		if len(n.Deliveries) == 0 {
			fmt.Println(styles.Dim.Render("  none recorded"))
			return
		}
		for _, d := range n.Deliveries {
			line := fmt.Sprintf("  %s %-20s %s  %s",
				styles.DeliveryGlyph(d.Status),
				d.DestinationName,
				styles.ChannelColor(d.Channel),
				styles.Faint.Render(d.DeliveredAt.Local().Format("15:04:05")))
			if d.Error != "" {
				line += "\n      " + styles.Error.Render(truncate(d.Error, 120))
			}
			fmt.Println(line)
		}
	})
}

func colorSeverity(s string) string {
	switch s {
	case "urgent":
		return styles.Error.Render(s)
	case "normal":
		return styles.Info.Render(s)
	case "digest":
		return styles.Dim.Render(s)
	}
	return styles.Dim.Render(s)
}

func indentLines(s, prefix string) string {
	out := ""
	for i, line := range splitLines(s) {
		if i > 0 {
			out += "\n" + prefix
		}
		out += line
	}
	return out
}

func splitLines(s string) []string {
	lines := []string{}
	start := 0
	for i, r := range s {
		if r == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

