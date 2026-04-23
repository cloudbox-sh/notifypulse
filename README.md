# notifypulse

One CLI to drive [Notifypulse][np]: a single REST relay that fans out a
notification to the recipient's existing Telegram / Discord / Slack / email
inbox or any webhook.

Two surfaces, full parity — anything the dashboard can do, this CLI can do
too (and vice versa). Agent-friendly: all subcommands accept flags and speak
`--json` so your AI assistant can drive them directly.

[np]: https://notifypulse.cloudbox.sh

## Install

```sh
# from source:
go install github.com/cloudbox-sh/notifypulse@latest
```

## Getting started

```sh
# 1. Log in (prompts for email + password, mints a CLI API key)
notifypulse login

# 2. Wire up a destination
notifypulse destinations create --name ops --channel slack \
  --webhook-url https://hooks.slack.com/services/...

# 3. Create a recipient that fans out to one or more destinations
notifypulse recipients create --name on-call --destination ops

# 4. Send
notifypulse notify --to on-call --title 'DB at 90% capacity' --severity urgent
```

## Commands

```
notifypulse login            # email/password → mints CLI key
notifypulse signup           # self-serve account creation (if enabled)
notifypulse logout           # clear local config
notifypulse whoami           # show which account/key is active

notifypulse status           # counts + 24h delivery stats

notifypulse destinations
  list | get <id> | create | delete <id>

notifypulse recipients
  list | get <name> | create | delete <name>
  bind <name> --destination ...
  unbind <name> <destination>

notifypulse notify --title ... (--to <recipient> | --destination <name> ...)
notifypulse history list | get <id>

notifypulse keys list | create | revoke <id>

notifypulse completion <shell>
notifypulse version
```

## Flags

- `--json` — machine-parsable JSON output; disables interactive prompts.
- `--debug` / `-d` — log HTTP request/response summaries to stderr.
- `--verbose` / `-v` — extra detail on commands that support it.
- `--api-url` — override the API base URL (defaults to prod).

## Environment

- `NOTIFYPULSE_API_URL` — alternate API base (self-hosting, staging, etc.).
- `NOTIFYPULSE_API_KEY` — use a raw key directly without logging in.
- `NOTIFYPULSE_BASIC_USER` / `NOTIFYPULSE_BASIC_PASS` — clears the preview-gate
  HTTP Basic Auth when the deploy is still password-protected.

## Exit codes

`notify` returns specific codes to make scripting clean:

| Code | Meaning |
|------|---------|
| 0    | All deliveries sent |
| 2    | Partial delivery (some failed) |
| 3    | All deliveries failed |
| 4    | Deduped (skipped — same dedup_key within 5m) |

`status` exits non-zero when any delivery failed in the last 24 hours.

## Configuration

Local state lives at `~/.config/cloudbox/notifypulse.json` (respects
`XDG_CONFIG_HOME`). It holds the API URL + the minted API key; chmod 0600.
# notifypulse
