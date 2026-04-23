package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/notifypulse/internal/client"
	"github.com/cloudbox-sh/notifypulse/internal/styles"
)

var recipientsCmd = &cobra.Command{
	Use:     "recipients",
	Aliases: []string{"recipient", "r"},
	Short:   "Manage recipients — named fan-out groups of destinations",
	Long: "A recipient is a stable name (e.g. 'on-call') that resolves to one or\n" +
		"more destinations at send time. Pointing your notifications at recipients\n" +
		"rather than destinations lets you re-route later without redeploying code.",
}

func init() {
	rootCmd.AddCommand(recipientsCmd)
	recipientsCmd.AddCommand(recipientsListCmd)
	recipientsCmd.AddCommand(recipientsGetCmd)
	recipientsCmd.AddCommand(recipientsCreateCmd)
	recipientsCmd.AddCommand(recipientsDeleteCmd)
	recipientsCmd.AddCommand(recipientsBindCmd)
	recipientsCmd.AddCommand(recipientsUnbindCmd)

	recipientsCreateCmd.Flags().StringVar(&recipCreateName, "name", "", "Recipient name (unique per user)")
	recipientsCreateCmd.Flags().StringSliceVar(&recipCreateDests, "destination", nil,
		"Destination name to bind at creation (repeatable)")
	recipientsBindCmd.Flags().StringSliceVar(&recipBindDests, "destination", nil,
		"Destination names to add (repeatable, required)")
}

// ── list ─────────────────────────────────────────────────────────────────

var recipientsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List recipients",
	RunE:    runRecipientsList,
}

func runRecipientsList(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	recs, err := c.ListRecipients(ctx)
	if err != nil {
		return handleAPIError(err, "recipient", "")
	}
	return emit(recs, func() {
		if len(recs) == 0 {
			fmt.Println(styles.Dim.Render("no recipients yet — create one with ") +
				styles.Code.Render("notifypulse recipients create"))
			return
		}
		header := lipgloss.JoinHorizontal(lipgloss.Top,
			styles.Header.Width(24).Render("NAME"),
			styles.Header.Width(10).Render("COUNT"),
			styles.Header.Width(50).Render("DESTINATIONS"),
		)
		fmt.Println(header)
		for _, r := range recs {
			names := make([]string, 0, len(r.Destinations))
			for _, d := range r.Destinations {
				names = append(names, d.Name+styles.Faint.Render(" ["+d.Channel+"]"))
			}
			destList := "—"
			if len(names) > 0 {
				destList = joinWithSep(names, "  ")
			}
			fmt.Println(lipgloss.JoinHorizontal(lipgloss.Top,
				styles.Cell.Width(24).Render(truncate(r.Name, 22)),
				styles.Cell.Width(10).Render(fmt.Sprintf("%d", len(r.Destinations))),
				styles.Cell.Width(50).Render(truncate(destList, 48)),
			))
		}
		fmt.Println()
		fmt.Println(styles.Faint.Render(fmt.Sprintf("%d recipient(s)", len(recs))))
	})
}

// ── get ──────────────────────────────────────────────────────────────────

var recipientsGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Show a recipient's bound destinations",
	Args:  cobra.ExactArgs(1),
	RunE:  runRecipientsGet,
}

func runRecipientsGet(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	r, err := c.GetRecipient(ctx, args[0])
	if err != nil {
		return handleAPIError(err, "recipient", args[0])
	}
	return emit(r, func() {
		fmt.Println(styles.Highlight.Render(r.Name))
		fmt.Println(styles.Dim.Render("id       ") + r.ID)
		fmt.Println(styles.Dim.Render("created  ") + r.CreatedAt.Local().Format("2006-01-02 15:04:05"))
		fmt.Println(styles.Dim.Render("bindings ") + fmt.Sprintf("%d", len(r.Destinations)))
		fmt.Println()
		if len(r.Destinations) == 0 {
			fmt.Println(styles.Warning.Render("! no destinations bound — notifications sent to this recipient will fail"))
			fmt.Println(styles.Dim.Render("  add one: ") +
				styles.Code.Render("notifypulse recipients bind "+r.Name+" --destination <name>"))
			return
		}
		fmt.Println(styles.Accent.Render("destinations"))
		for _, d := range r.Destinations {
			fmt.Printf("  %s %-24s %s\n",
				styles.ChannelColor(d.Channel),
				d.Name,
				styles.Faint.Render(d.ID))
		}
	})
}

// ── create ───────────────────────────────────────────────────────────────

var (
	recipCreateName  string
	recipCreateDests []string
)

var recipientsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a recipient, optionally binding destinations at creation",
	RunE:  runRecipientsCreate,
}

func runRecipientsCreate(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	name := recipCreateName
	dests := recipCreateDests

	interactive := name == ""
	if interactive {
		if err := requireFlagsForJSON("--name"); err != nil {
			return err
		}
		all, _ := c.ListDestinations(ctx)
		opts := make([]huh.Option[string], 0, len(all))
		for _, d := range all {
			opts = append(opts, huh.NewOption(
				fmt.Sprintf("%s [%s]", d.Name, d.Channel), d.Name))
		}
		fields := []huh.Field{
			huh.NewInput().Title("Recipient name").
				Description("Stable identifier used in POST /v1/notify 'to' field.").
				Value(&name).Validate(requireNonEmpty("name")),
		}
		if len(opts) > 0 {
			fields = append(fields, huh.NewMultiSelect[string]().
				Title("Destinations to bind").
				Description("Bindings can be changed any time with 'recipients bind / unbind'.").
				Options(opts...).Value(&dests))
		}
		if err := huh.NewForm(huh.NewGroup(fields...)).WithTheme(huh.ThemeCatppuccin()).Run(); err != nil {
			return err
		}
	}

	r, err := c.CreateRecipient(ctx, client.CreateRecipientRequest{
		Name:         name,
		Destinations: dests,
	})
	if err != nil {
		return handleAPIError(err, "recipient", "")
	}
	msg := styles.Check() + " recipient created " + styles.Faint.Render(r.ID)
	if len(dests) == 0 {
		msg += "\n" + styles.Warning.Render("  (no destinations bound — run 'recipients bind' before sending)")
	}
	return emitOK("recipient", r.ID, r, msg)
}

// ── bind / unbind ────────────────────────────────────────────────────────

var recipBindDests []string

var recipientsBindCmd = &cobra.Command{
	Use:   "bind <name>",
	Short: "Add destinations to a recipient",
	Args:  cobra.ExactArgs(1),
	RunE:  runRecipientsBind,
}

func runRecipientsBind(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	dests := recipBindDests
	if len(dests) == 0 {
		if err := requireFlagsForJSON("--destination (repeatable)"); err != nil {
			return err
		}
		all, _ := c.ListDestinations(ctx)
		opts := make([]huh.Option[string], 0, len(all))
		for _, d := range all {
			opts = append(opts, huh.NewOption(
				fmt.Sprintf("%s [%s]", d.Name, d.Channel), d.Name))
		}
		if len(opts) == 0 {
			return fmt.Errorf("no destinations exist yet — create one first with 'destinations create'")
		}
		if err := huh.NewForm(huh.NewGroup(
			huh.NewMultiSelect[string]().Title("Bind destinations").
				Options(opts...).Value(&dests).
				Validate(func(s []string) error {
					if len(s) == 0 {
						return fmt.Errorf("pick at least one")
					}
					return nil
				}),
		)).WithTheme(huh.ThemeCatppuccin()).Run(); err != nil {
			return err
		}
	}

	bound, err := c.BindDestinations(ctx, args[0], dests)
	if err != nil {
		return handleAPIError(err, "recipient", args[0])
	}
	return emitOK("destinations", args[0], map[string]any{"destinations": bound},
		styles.Check()+" bound "+fmt.Sprintf("%d destination(s) to %s", len(bound), args[0]))
}

var recipientsUnbindCmd = &cobra.Command{
	Use:   "unbind <name> <destination>",
	Short: "Remove one destination from a recipient",
	Args:  cobra.ExactArgs(2),
	RunE:  runRecipientsUnbind,
}

func runRecipientsUnbind(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()
	if err := c.UnbindDestination(ctx, args[0], args[1]); err != nil {
		return handleAPIError(err, "recipient", args[0])
	}
	return emitOK("", "", nil,
		styles.Check()+" unbound "+styles.Faint.Render(args[1]+" from "+args[0]))
}

// ── delete ───────────────────────────────────────────────────────────────

var recipientsDeleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"rm"},
	Short:   "Delete a recipient (destinations are not deleted)",
	Args:    cobra.ExactArgs(1),
	RunE:    runRecipientsDelete,
}

func runRecipientsDelete(cmd *cobra.Command, args []string) error {
	c, _, err := newClient()
	if err != nil {
		return err
	}
	ctx, cancel := signalCtx()
	defer cancel()

	if !jsonOutput {
		confirm := false
		if err := huh.NewConfirm().
			Title("Delete recipient " + args[0] + "?").
			Description("Sends targeting this recipient will start failing. Destinations are not affected.").
			Affirmative("Delete").Negative("Cancel").
			Value(&confirm).WithTheme(huh.ThemeCatppuccin()).Run(); err != nil {
			return err
		}
		if !confirm {
			fmt.Println(styles.Dim.Render("aborted"))
			return nil
		}
	}
	if err := c.DeleteRecipient(ctx, args[0]); err != nil {
		return handleAPIError(err, "recipient", args[0])
	}
	return emitOK("", args[0], nil, styles.Check()+" recipient deleted")
}

// joinWithSep is a stdlib-style strings.Join that tolerates pre-rendered
// ANSI colour codes in the entries.
func joinWithSep(xs []string, sep string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += sep
		}
		out += x
	}
	return out
}
