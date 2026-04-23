# Notifypulse MCP server

A [Model Context Protocol](https://modelcontextprotocol.io) server that lets
AI agents (Claude Desktop, Claude Code, Cursor, Copilot, …) drive Notifypulse
through typed tool calls instead of UI clicks or hand-written curl. Lives
inside the same Go binary as the CLI — start it with `notifypulse mcp serve`.

## Wire it into your agent

### Claude Desktop / Claude Code

Add to `~/.claude/mcp_servers.json` (Claude Code) or `~/Library/Application Support/Claude/claude_desktop_config.json` (Claude Desktop):

```json
{
  "mcpServers": {
    "notifypulse": {
      "command": "notifypulse",
      "args": ["mcp", "serve"]
    }
  }
}
```

### Cursor

`~/.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "notifypulse": {
      "command": "notifypulse",
      "args": ["mcp", "serve"]
    }
  }
}
```

After saving, restart the agent. The 13 tools below should appear in the
agent's tool picker.

## Auth

The MCP inherits the CLI's API key. Run `notifypulse login` once and both
surfaces share `~/.config/cloudbox/notifypulse.json`.

For headless environments, set env vars in the agent's config block:

| Variable | Purpose |
|---|---|
| `NOTIFYPULSE_API_KEY` | API key (`np_<prefix>_<secret>`) |
| `NOTIFYPULSE_API_URL` | Override base URL (default `https://notifypulse.cloudbox.sh`) |
| `NOTIFYPULSE_BASIC_USER` | Optional. HTTP basic-auth user when the deploy is behind a `SITE_PASSWORD` gate |
| `NOTIFYPULSE_BASIC_PASS` | Optional. Matching password |

The key never reaches the LLM — it sits inside the MCP subprocess.

## Tools (v1: 13)

**Destinations**

- `list_destinations` — every destination with channel + config
- `get_destination` — one destination by id
- `create_destination` — webhook / email / telegram / discord / slack / mattermost / teams / ntfy. Shorthand fields (`email`, `webhook_url`, `binding_id`) cover the common cases; `config_json` is the escape hatch for ntfy and any future channel
- `delete_destination` — destructive; requires `confirm:true`

**Recipients**

- `list_recipients`, `get_recipient`
- `create_recipient` — name + optional initial destination bindings
- `delete_recipient` — destructive (recipient only; bound destinations survive); requires `confirm:true`
- `bind_destinations` — add destinations to a recipient (idempotent)
- `unbind_destination` — remove one destination from a recipient

**Notify** (the headline workflow — send a notification from chat)

- `notify` — title + (`to` recipient | `destinations[]`) + optional body / severity / link / dedup_key. Returns the per-destination delivery log inline so the agent can immediately confirm the human got the message

**Delivery log**

- `list_notifications` — recent sends, newest first
- `get_notification` — one notification with its full per-destination delivery log

## Safety

Every destructive tool (`delete_destination`, `delete_recipient`) requires
the agent to set `confirm: true`. The tool description tells the model to
surface this as an explicit yes/no to the user before flipping the flag.

`unbind_destination` is **not** gated by confirm — it just severs a binding,
the destination itself survives.

Every successful tool result includes a dashboard link
(`→ https://notifypulse.cloudbox.sh/app/...`) so the human reading the
agent's transcript can verify the change without leaving their chat.

## Telegram destinations

The server uses the managed-bot model — Notifypulse owns the bot identity
(`@cloudbox_notifypulse_bot`) and binds individual chats via a deeplink
flow. There is **no API path to bind a chat**: the user must connect a
Telegram chat from the dashboard's Connect-Telegram flow first, and the
resulting `binding_id` is what `create_destination` consumes when
`channel=telegram`. This is a Telegram-platform constraint, not a
Notifypulse one.

## Out of scope (deferred)

- API key CRUD (server-side `/api/keys` is session-authed only today; keys
  are minted via `notifypulse login` from the CLI)
- Per-recipient severity routing (model exists, no API surface yet)
- Quiet hours (not modelled)
- MCP `resources` and `prompts`
- HTTP / SSE transport (stdio only — every current agent runtime supports it)
