package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/cloudbox-sh/notifypulse/internal/client"
)

// registerRecipientTools wires the six recipient tools.
//
// list_recipients / get_recipient / create_recipient / delete_recipient
// bind_destinations / unbind_destination
//
// Recipients are named bundles of destinations — sending to a recipient
// fans out to every bound destination. A recipient can have zero
// destinations (useful for staging an on-call name before the channels
// are ready).
func registerRecipientTools(srv *mcpserver.MCPServer, d *deps) {
	// ── list_recipients ────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("list_recipients",
			mcp.WithDescription(
				"List every recipient on the current account. Each recipient is a named "+
					"fan-out group of destinations. The list view returns id, name, "+
					"created_at — call get_recipient for the bound destinations. "+
					"REST: GET /v1/recipients",
			),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			rs, err := d.api.ListRecipients(ctx)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(rs, dashLink(d.baseURL, "/app/recipients")), nil
		},
	)

	// ── get_recipient ──────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("get_recipient",
			mcp.WithDescription(
				"Fetch one recipient by name with the full set of destinations bound to "+
					"it. Use this before notifying so you know which channels will fire. "+
					"REST: GET /v1/recipients/{name}",
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Recipient name (unique per account)."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			r, err := d.api.GetRecipient(ctx, name)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(r, dashLink(d.baseURL, "/app/recipients")), nil
		},
	)

	// ── create_recipient ───────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("create_recipient",
			mcp.WithDescription(
				"Create a recipient and optionally bind destinations at creation. "+
					"`destinations` is a list of destination names or IDs — leave it empty "+
					"to create the recipient first and bind later via bind_destinations. "+
					"REST: POST /v1/recipients",
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Recipient name (unique per account). Cannot be changed later."),
			),
			mcp.WithArray("destinations",
				mcp.Description("Optional list of destination names or IDs to bind at creation."),
				mcp.Items(map[string]any{"type": "string"}),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			dests := req.GetStringSlice("destinations", nil)
			r, err := d.api.CreateRecipient(ctx, client.CreateRecipientRequest{
				Name:         name,
				Destinations: dests,
			})
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(r, dashLink(d.baseURL, "/app/recipients")), nil
		},
	)

	// ── delete_recipient ───────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("delete_recipient",
			mcp.WithDescription(
				"Delete a recipient. Bound destinations are NOT deleted — the destinations "+
					"survive and can be bound to other recipients. Past notifications keep a "+
					"snapshot of the recipient name in History. "+
					"Requires confirm:true. "+
					"REST: DELETE /v1/recipients/{name}",
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Recipient name."),
			),
			confirmField(),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if r := requireConfirm(req); r != nil {
				return r, nil
			}
			name, err := req.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := d.api.DeleteRecipient(ctx, name); err != nil {
				return apiErrorResult(err), nil
			}
			return mcp.NewToolResultText("recipient deleted: " + name), nil
		},
	)

	// ── bind_destinations ──────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("bind_destinations",
			mcp.WithDescription(
				"Add one or more destinations to an existing recipient. Idempotent — "+
					"binding a destination already bound to the recipient is a no-op. "+
					"REST: POST /v1/recipients/{name}/destinations",
			),
			mcp.WithString("recipient",
				mcp.Required(),
				mcp.Description("Recipient name to bind onto."),
			),
			mcp.WithArray("destinations",
				mcp.Required(),
				mcp.Description("Destination names or IDs to bind."),
				mcp.Items(map[string]any{"type": "string"}),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("recipient")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			dests := req.GetStringSlice("destinations", nil)
			if len(dests) == 0 {
				return mcp.NewToolResultError("`destinations` must contain at least one entry"), nil
			}
			bound, err := d.api.BindDestinations(ctx, name, dests)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(map[string]any{
				"recipient":    name,
				"destinations": bound,
			}, dashLink(d.baseURL, "/app/recipients")), nil
		},
	)

	// ── unbind_destination ─────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("unbind_destination",
			mcp.WithDescription(
				"Remove one destination from a recipient. The destination itself is not "+
					"deleted. Use delete_destination if you want to remove the destination "+
					"entirely. "+
					"REST: DELETE /v1/recipients/{name}/destinations/{destination}",
			),
			mcp.WithString("recipient",
				mcp.Required(),
				mcp.Description("Recipient name."),
			),
			mcp.WithString("destination",
				mcp.Required(),
				mcp.Description("Destination name or ID to unbind."),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("recipient")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			dest, err := req.RequireString("destination")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := d.api.UnbindDestination(ctx, name, dest); err != nil {
				return apiErrorResult(err), nil
			}
			return mcp.NewToolResultText("unbound " + dest + " from " + name), nil
		},
	)
}
