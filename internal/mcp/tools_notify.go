package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/cloudbox-sh/notifypulse/internal/client"
)

// registerNotifyTool wires the headline tool — the whole point of the MCP.
//
// notify(title, body?, severity?, link?, dedup_key?, to | destinations[])
//
// One JSON body, server-side fan-out, returns the per-destination outcome
// list inline so an agent can immediately see whether the human got the
// message on every channel.
func registerNotifyTool(srv *mcpserver.MCPServer, d *deps) {
	srv.AddTool(
		mcp.NewTool("notify",
			mcp.WithDescription(
				"Send a notification. Server-side fan-out: one call reaches the human on "+
					"every channel they listen on. Pass either `to` (a recipient name — fans "+
					"out to every destination bound to that recipient) or `destinations` "+
					"(a list of destination names/IDs — direct send), or both. `dedup_key` "+
					"collapses repeat notifications within a 5-minute window — useful when "+
					"the same alert can fire from multiple sources. "+
					"\n\n"+
					"This is the headline workflow: when a user says \"tell Werner …\" or "+
					"\"page on-call …\", this is the tool to call. The response includes a "+
					"`deliveries` array with the per-destination status (sent / failed / "+
					"skipped) and the channel-specific error if any. "+
					"REST: POST /v1/notify",
			),
			mcp.WithString("title",
				mcp.Required(),
				mcp.Description("Short headline. Shown as the message subject / push-notification title across every channel."),
			),
			mcp.WithString("body",
				mcp.Description("Optional longer text. Plain-text; channel adapters render it as appropriate (Slack mrkdwn, Discord embed body, email plain+HTML)."),
			),
			mcp.WithString("to",
				mcp.Description("Recipient name to fan out to. Mutually compatible with `destinations` — both can be set."),
			),
			mcp.WithArray("destinations",
				mcp.Description("Destination names or IDs to send to directly. Mutually compatible with `to`."),
				mcp.Items(map[string]any{"type": "string"}),
			),
			mcp.WithString("severity",
				mcp.Description("urgent | normal | digest. Default normal. Drives ntfy priority, Telegram disable_notification, etc."),
				mcp.Enum("urgent", "normal", "digest"),
			),
			mcp.WithString("link",
				mcp.Description("Optional URL. Rendered as a clickable button on Slack/Discord/Teams; appended to the body on email/Telegram."),
			),
			mcp.WithString("dedup_key",
				mcp.Description("Idempotency key — repeats with the same key inside a 5-minute window are dropped (returned as deduped:true with an empty deliveries array)."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			title, err := req.RequireString("title")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			to := req.GetString("to", "")
			dests := req.GetStringSlice("destinations", nil)
			if to == "" && len(dests) == 0 {
				return mcp.NewToolResultError(
					"need at least one of `to` (recipient name) or `destinations` (destination names/IDs)",
				), nil
			}

			body := client.NotifyRequest{
				To:           to,
				Destinations: dests,
				Title:        title,
				Body:         req.GetString("body", ""),
				Severity:     req.GetString("severity", ""),
				Link:         req.GetString("link", ""),
				DedupKey:     req.GetString("dedup_key", ""),
			}

			resp, err := d.api.Notify(ctx, body)
			if err != nil {
				return apiErrorResult(err), nil
			}

			link := dashLink(d.baseURL, "/app/history")
			if resp.ID != "" {
				link = dashLink(d.baseURL, fmt.Sprintf("/app/history#%s", resp.ID))
			}
			return jsonResultWithLink(resp, link), nil
		},
	)
}
