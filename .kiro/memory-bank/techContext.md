# Tech Context

## Stack
- **Language**: Go 1.22+
- **TUI**: Bubbletea + Lipgloss
- **CLI**: Cobra
- **Communication**: ACP (JSON-RPC 2.0 over stdio) to kiro-cli
- **Tray**: systray (macOS/Windows/Linux)

## Git Remotes
| Remote | URL | Purpose |
|--------|-----|---------|
| `origin` | `git@github.disney.com-sancr225:SANCR225/Koda.git` | Primary (Disney GHE) |
| `public` | `git@github.com-rsanchez-disney:rsanchez-disney/Koda.git` | Public mirror (github.com) |
| `personal` | `git@github.com-arianthox:arianthox/Koda.git` | Backup (private, github.com) |

To sync personal backup: `git push personal main`

## Shared Settings
- `~/.kiro/settings/kite.json` — shared with Kite (activeProfile, lastAgent, steerRuntime)
- `~/.kiro/settings/profiles.json` — profile manifest
- `~/.kiro/settings/mcp.json` — MCP server config
- `~/.kiro/settings/usage.jsonl` — usage/scoring log (shared with Kite)
- `~/.kiro/settings/sessions.db` — SQLite sessions (Kite)

## Build
```bash
make build          # Build binary
make install        # Build + install to ~/.local/bin
make test           # Run tests
```
