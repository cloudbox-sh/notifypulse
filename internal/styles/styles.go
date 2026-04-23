// Package styles holds the central Lip Gloss brand kit for the Notifypulse CLI.
//
// Per the Cloudbox design system: Catppuccin Mocha base palette with a single
// Mauve accent (#cba6f7). Every Cloudbox CLI shares these tokens so terminal
// output feels like the rest of the suite.
package styles

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha palette + Cloudbox accent.
var (
	ColorBase     = lipgloss.Color("#1e1e2e")
	ColorMantle   = lipgloss.Color("#181825")
	ColorSurface0 = lipgloss.Color("#313244")
	ColorSurface1 = lipgloss.Color("#45475a")
	ColorText     = lipgloss.Color("#cdd6f4")
	ColorSubtext1 = lipgloss.Color("#bac2de")
	ColorSubtext0 = lipgloss.Color("#a6adc8")
	ColorOverlay0 = lipgloss.Color("#6c7086")

	ColorAccent  = lipgloss.Color("#cba6f7") // Mauve — the Cloudbox accent
	ColorSuccess = lipgloss.Color("#a6e3a1") // Green
	ColorWarning = lipgloss.Color("#f9e2af") // Yellow
	ColorError   = lipgloss.Color("#f38ba8") // Red
	ColorInfo    = lipgloss.Color("#89b4fa") // Blue
)

var (
	Accent    = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	Highlight = lipgloss.NewStyle().Foreground(ColorText).Bold(true)
	Dim       = lipgloss.NewStyle().Foreground(ColorSubtext0)
	Faint     = lipgloss.NewStyle().Foreground(ColorOverlay0)

	Success = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
	Warning = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	Error   = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
	Info    = lipgloss.NewStyle().Foreground(ColorInfo)

	Code = lipgloss.NewStyle().Foreground(ColorAccent)

	Header = lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true).
		Padding(0, 1)

	Cell = lipgloss.NewStyle().
		Foreground(ColorText).
		Padding(0, 1)

	Border = lipgloss.NewStyle().Foreground(ColorSurface0)
)

// SeverityGlyph returns a coloured dot for a notification severity.
func SeverityGlyph(severity string) string {
	switch severity {
	case "urgent":
		return Error.Render("●")
	case "normal":
		return Info.Render("●")
	case "digest":
		return Dim.Render("●")
	default:
		return Faint.Render("○")
	}
}

// DeliveryGlyph returns a coloured glyph for a delivery status.
func DeliveryGlyph(status string) string {
	switch status {
	case "sent":
		return Success.Render("✓")
	case "failed":
		return Error.Render("✗")
	case "skipped":
		return Dim.Render("○")
	case "deduped":
		return Dim.Render("⊘")
	case "partial":
		return Warning.Render("◐")
	default:
		return Faint.Render("?")
	}
}

// ChannelColor returns the severity-neutral colour tag associated with a
// delivery channel, used in list rendering.
func ChannelColor(channel string) string {
	switch channel {
	case "telegram":
		return Info.Render(channel)
	case "discord":
		return lipgloss.NewStyle().Foreground(ColorAccent).Render(channel)
	case "slack":
		return Warning.Render(channel)
	case "email":
		return Success.Render(channel)
	case "webhook":
		return Dim.Render(channel)
	}
	return Dim.Render(channel)
}

// Check returns a green ✓.
func Check() string { return Success.Render("✓") }

// Cross returns a red ✗.
func Cross() string { return Error.Render("✗") }
