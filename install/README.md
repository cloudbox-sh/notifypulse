# Installer scripts

Sources for the `curl | sh` + PowerShell installers advertised on the Notifypulse landing page and README.

| File | Hosted at | Use |
|------|-----------|-----|
| `notifypulse.sh` | `https://get.cloudbox.sh/notifypulse` | `curl -sSL https://get.cloudbox.sh/notifypulse \| sh` |
| `notifypulse.ps1` | `https://get.cloudbox.sh/notifypulse.ps1` | `irm https://get.cloudbox.sh/notifypulse.ps1 \| iex` |

Both scripts:

1. Detect OS + architecture.
2. Resolve the version (env `INSTALL_VERSION`, or the latest GitHub Release).
3. Download the matching GoReleaser archive from the Release assets.
4. Verify SHA-256 against the published `checksums.txt`.
5. Extract the binary to `/usr/local/bin` (with sudo) / `$HOME/.local/bin` on Unix, or `%LOCALAPPDATA%\Programs\Notifypulse` on Windows.
6. Warn if the install dir isn't on `PATH` (Unix) or update the user `Path` variable (Windows).

## Publishing to `get.cloudbox.sh`

The `get.cloudbox.sh` origin is owned by the Cloudbox landing infrastructure (separate repo). On each release of this CLI, copy the updated script there. Until automation is wired up, manual copy is fine — the script changes rarely.

## Environment overrides

| Variable | Description |
|----------|-------------|
| `INSTALL_VERSION` | Pin a specific tag (`v0.2.0`). Default: latest release. |
| `INSTALL_DIR` | Override install target. |

## Testing locally

```sh
# Linux / macOS
sh install/notifypulse.sh

# Windows PowerShell
pwsh -File install/notifypulse.ps1
```
