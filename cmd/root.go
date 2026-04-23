// Package cmd wires every Notifypulse CLI subcommand into a single Cobra tree.
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/cloudbox-sh/notifypulse/internal/client"
	"github.com/cloudbox-sh/notifypulse/internal/config"
	"github.com/cloudbox-sh/notifypulse/internal/styles"
)

// Version is set via -ldflags at release time by GoReleaser. For plain
// `go build` / `go install` the init below derives a best-effort value
// from Go's embedded build info.
var Version = "dev"

func resolveVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return strings.TrimPrefix(v, "v")
	}
	var rev, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if rev == "" {
		return "dev"
	}
	short := rev
	if len(short) > 12 {
		short = short[:12]
	}
	if modified == "true" {
		short += "-dirty"
	}
	return "dev+" + short
}

// Root-level flags.
var (
	apiURLFlag  string
	jsonOutput  bool
	debugFlag   bool
	verboseFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "notifypulse",
	Short: "Notifypulse — one API for Telegram / Discord / Slack / email / webhook",
	Long: styles.Accent.Render("Notifypulse") + " — one API for Telegram, Discord, Slack, email, and webhooks.\n\n" +
		styles.Dim.Render("Anything the dashboard can do, this CLI can do too — and vice versa.\n"+
			"Agent-friendly: your AI assistant can drive it directly.\n\n"+
			"Docs: https://notifypulse.cloudbox.sh/docs"),
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute is the single entry point called from main.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	if Version == "dev" {
		Version = resolveVersion()
	}

	rootCmd.PersistentFlags().StringVar(&apiURLFlag, "api-url", "",
		"Notifypulse API base URL (overrides config + NOTIFYPULSE_API_URL)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false,
		"Emit machine-parsable JSON instead of styled tables (scripts + AI agents)")
	rootCmd.PersistentFlags().BoolVarP(&debugFlag, "debug", "d", false,
		"Log HTTP request/response summary to stderr (NDJSON when --json is set)")
	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false,
		"Show extended detail for commands that support it")
}

// ── output helpers ────────────────────────────────────────────────────────

// emit is the output helper used by every subcommand. In --json mode it
// writes a single JSON document to stdout; otherwise it runs the passed
// human-readable renderer.
func emit(data any, human func()) error {
	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}
	if human != nil {
		human()
	}
	return nil
}

// emitOK is shorthand for mutation responses — create/update/delete.
func emitOK(entity, id string, payload any, humanMsg string) error {
	if jsonOutput {
		out := map[string]any{"ok": true}
		if id != "" {
			out["id"] = id
		}
		if entity != "" && payload != nil {
			out[entity] = payload
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	fmt.Println(humanMsg)
	return nil
}

// requireFlagsForJSON is called before any interactive form runs. In --json
// mode we can't show a TUI, so we tell the caller which flags to pass.
func requireFlagsForJSON(required string) error {
	if !jsonOutput {
		return nil
	}
	return fmt.Errorf("--json mode requires flags (%s) — interactive forms are disabled", required)
}

// signalCtx returns a context cancelled on Ctrl-C so long-running commands
// can shut down cleanly.
func signalCtx() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// ── client construction ───────────────────────────────────────────────────

// newClient builds an authenticated API client. Returns ErrNotAuthenticated
// when no API key is available.
func newClient() (*client.Client, *config.Resolved, error) {
	r, err := config.Resolve(apiURLFlag)
	if err != nil {
		return nil, nil, err
	}
	if r.APIKey == "" {
		return nil, r, config.ErrNotAuthenticated
	}
	return client.New(r.APIURL, r.APIKey).
		WithBasicAuth(os.Getenv("NOTIFYPULSE_BASIC_USER"), os.Getenv("NOTIFYPULSE_BASIC_PASS")).
		WithDebug(debugFlag, jsonOutput), r, nil
}

// newAnonClient returns a client without requiring a key. Used by login/signup.
func newAnonClient() (*client.Client, *config.Resolved, error) {
	r, err := config.Resolve(apiURLFlag)
	if err != nil {
		return nil, nil, err
	}
	return client.New(r.APIURL, r.APIKey).
		WithBasicAuth(os.Getenv("NOTIFYPULSE_BASIC_USER"), os.Getenv("NOTIFYPULSE_BASIC_PASS")).
		WithDebug(debugFlag, jsonOutput), r, nil
}

// ── error formatting ──────────────────────────────────────────────────────

// listCmdForKind maps an entity kind to its list subcommand. Used to build
// helpful "run X to see available IDs" hints on 404s.
func listCmdForKind(kind string) string {
	switch kind {
	case "destination":
		return "destinations list"
	case "recipient":
		return "recipients list"
	case "notification":
		return "history"
	case "key":
		return "keys list"
	}
	return kind + "s list"
}

// handleAPIError turns a raw API error into a friendly, context-aware one.
func handleAPIError(err error, ctx ...string) error {
	if err == nil {
		return nil
	}

	kind, ref := "", ""
	if len(ctx) >= 1 {
		kind = ctx[0]
	}
	if len(ctx) >= 2 {
		ref = ctx[1]
	}

	var ae *client.APIError
	if !errors.As(err, &ae) {
		if kind != "" && ref != "" {
			return fmt.Errorf("%s %q: %w", kind, ref, err)
		}
		return err
	}

	switch ae.Status {
	case http.StatusUnauthorized:
		fmt.Fprintln(os.Stderr, styles.Error.Render("✗ not authenticated")+
			" — run "+styles.Code.Render("notifypulse login")+" to reauthenticate.")
		return errors.New("not authenticated")

	case http.StatusForbidden:
		return errors.New(ae.Message)

	case http.StatusNotFound:
		if kind != "" && ref != "" {
			listCmd := listCmdForKind(kind)
			return fmt.Errorf("%s %q not found — run %s to see available IDs",
				kind, ref, styles.Code.Render("notifypulse "+listCmd))
		}
		if ae.Message != "" {
			return errors.New(ae.Message)
		}
		return errors.New("not found")

	case http.StatusConflict:
		return errors.New(ae.Message)

	default:
		if ae.Status >= 500 {
			return fmt.Errorf("server error (HTTP %d) — try again in a moment", ae.Status)
		}
		if ae.Message != "" {
			return errors.New(ae.Message)
		}
		return fmt.Errorf("request failed (HTTP %d)", ae.Status)
	}
}

// ── small shared utilities used by multiple subcommands ──────────────────

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func strp(s string) *string { return &s }

func requireNonEmpty(field string) func(string) error {
	return func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
}
