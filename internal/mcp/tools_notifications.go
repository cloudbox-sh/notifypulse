package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// registerNotificationTools wires the read-only delivery-log tools.
//
// list_notifications / get_notification
//
// Useful when an agent wants to verify the previous send actually reached
// the human, or when the user asks "did the alert go out?".
func registerNotificationTools(srv *mcpserver.MCPServer, d *deps) {
	// ── list_notifications ─────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("list_notifications",
			mcp.WithDescription(
				"List recent notifications (newest first). Each row includes the title, "+
					"severity, dedup_key, and created_at — drill into a single row with "+
					"get_notification to see the per-destination delivery rows. "+
					"REST: GET /v1/notifications",
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum rows to return. Server caps and defaults apply when omitted."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			limit := req.GetInt("limit", 0)
			ns, err := d.api.ListNotifications(ctx, limit)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(ns, dashLink(d.baseURL, "/app/history")), nil
		},
	)

	// ── get_notification ───────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("get_notification",
			mcp.WithDescription(
				"Fetch one notification with its full delivery log — one row per "+
					"destination it was fanned out to, including channel, status (sent / "+
					"failed / skipped), and the channel-specific error if any. Use this to "+
					"answer \"did the alert reach Werner on Telegram?\". "+
					"REST: GET /v1/notifications/{id}",
			),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("Notification UUID."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			n, err := d.api.GetNotification(ctx, id)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(n, dashLink(d.baseURL, "/app/history")), nil
		},
	)
}
