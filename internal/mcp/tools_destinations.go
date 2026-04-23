package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/cloudbox-sh/notifypulse/internal/client"
)

// registerDestinationTools wires the four destination tools.
//
// list_destinations / get_destination / create_destination / delete_destination
//
// create_destination keeps the schema small for the common channels by
// flattening shorthand fields (`email`, `webhook_url`, `binding_id`) plus a
// `config_json` escape hatch for any channel the schema doesn't cover.
func registerDestinationTools(srv *mcpserver.MCPServer, d *deps) {
	// ── list_destinations ──────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("list_destinations",
			mcp.WithDescription(
				"List every destination on the current account. A destination is a bound "+
					"channel — \"Werner's Telegram\", \"#ops in our Slack\", etc. Returns id, "+
					"name, channel, config (channel-specific JSON), created_at. "+
					"REST: GET /v1/destinations",
			),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			ds, err := d.api.ListDestinations(ctx)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(ds, dashLink(d.baseURL, "/app/destinations")), nil
		},
	)

	// ── get_destination ────────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("get_destination",
			mcp.WithDescription(
				"Fetch one destination by id with its full configuration. "+
					"REST: GET /v1/destinations/{id}",
			),
			mcp.WithString("id", mcp.Required(), mcp.Description("Destination UUID.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			dest, err := d.api.GetDestination(ctx, id)
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(dest, dashLink(d.baseURL, "/app/destinations")), nil
		},
	)

	// ── create_destination ─────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("create_destination",
			mcp.WithDescription(
				"Create a destination. Pass a name, a channel, and the channel-specific "+
					"config — either via the shorthand fields below or as raw JSON in "+
					"`config_json`. Channels: webhook (url), email (email), telegram "+
					"(binding_id — must be obtained via the dashboard's Connect-Telegram "+
					"flow first; there is no API path to bind a chat), discord/slack/"+
					"mattermost/teams (webhook_url), ntfy (config_json with topic+server). "+
					"REST: POST /v1/destinations",
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Destination name. Must be unique per account; will be referenced from `notify` calls."),
			),
			mcp.WithString("channel",
				mcp.Required(),
				mcp.Description("Channel type."),
				mcp.Enum("webhook", "email", "telegram", "discord", "slack", "mattermost", "teams", "ntfy"),
			),
			mcp.WithString("email",
				mcp.Description("Email address. Required when channel=email (ignored otherwise)."),
			),
			mcp.WithString("webhook_url",
				mcp.Description(
					"Incoming-webhook URL. Required when channel=webhook|discord|slack|"+
						"mattermost|teams (ignored otherwise). For webhook it's any HTTPS endpoint "+
						"that accepts a JSON POST; for the others it's the channel-native "+
						"incoming-webhook URL.",
				),
			),
			mcp.WithString("binding_id",
				mcp.Description("Telegram binding UUID. Required when channel=telegram. Get one from the dashboard's Connect-Telegram flow."),
			),
			mcp.WithString("config_json",
				mcp.Description(
					"Optional raw JSON config — wins over the shorthand fields when both "+
						"are present. Use this for ntfy ({topic, server, token}) and any future "+
						"channel the schema doesn't cover yet.",
				),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, err := req.RequireString("name")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			channel, err := req.RequireString("channel")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			cfg, err := buildConfig(channel,
				req.GetString("config_json", ""),
				req.GetString("email", ""),
				req.GetString("webhook_url", ""),
				req.GetString("binding_id", ""),
			)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			dest, err := d.api.CreateDestination(ctx, client.CreateDestinationRequest{
				Name:    name,
				Channel: channel,
				Config:  cfg,
			})
			if err != nil {
				return apiErrorResult(err), nil
			}
			return jsonResultWithLink(dest, dashLink(d.baseURL, "/app/destinations")), nil
		},
	)

	// ── delete_destination ─────────────────────────────────────────────────
	srv.AddTool(
		mcp.NewTool("delete_destination",
			mcp.WithDescription(
				"Permanently delete a destination. Recipients that bound this destination "+
					"lose the binding (the recipient itself is not deleted). Past delivery "+
					"rows in History keep the destination name as a snapshot. "+
					"Requires confirm:true (see the field's description). "+
					"REST: DELETE /v1/destinations/{id}",
			),
			mcp.WithString("id", mcp.Required(), mcp.Description("Destination UUID.")),
			confirmField(),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if r := requireConfirm(req); r != nil {
				return r, nil
			}
			id, err := req.RequireString("id")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if err := d.api.DeleteDestination(ctx, id); err != nil {
				return apiErrorResult(err), nil
			}
			return mcp.NewToolResultText("destination deleted: " + id), nil
		},
	)
}

// buildConfig assembles the channel-specific config blob the server expects.
// Raw config_json wins over shorthand fields when both are supplied.
func buildConfig(channel, raw, email, webhookURL, bindingID string) (json.RawMessage, error) {
	if raw != "" {
		if !json.Valid([]byte(raw)) {
			return nil, fmt.Errorf("config_json is not valid JSON")
		}
		return json.RawMessage(raw), nil
	}
	switch channel {
	case "email":
		if email == "" {
			return nil, fmt.Errorf("email channel requires `email`")
		}
		return mustJSON(map[string]string{"email": email})
	case "webhook":
		if webhookURL == "" {
			return nil, fmt.Errorf("webhook channel requires `webhook_url`")
		}
		return mustJSON(map[string]string{"url": webhookURL})
	case "telegram":
		if bindingID == "" {
			return nil, fmt.Errorf("telegram channel requires `binding_id` — connect a chat in the dashboard first")
		}
		return mustJSON(map[string]string{"binding_id": bindingID})
	case "discord", "slack", "mattermost", "teams":
		if webhookURL == "" {
			return nil, fmt.Errorf("%s channel requires `webhook_url`", channel)
		}
		return mustJSON(map[string]string{"webhook_url": webhookURL})
	case "ntfy":
		return nil, fmt.Errorf("ntfy channel requires `config_json` with at least {topic, server}")
	}
	return nil, fmt.Errorf("unsupported channel %q — pass `config_json` with raw JSON", channel)
}

func mustJSON(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return b, nil
}
