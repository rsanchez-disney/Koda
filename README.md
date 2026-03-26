# Koda 🐾

Interactive terminal companion for [steer-runtime](https://github.disney.com/SANCR225/steer-runtime) — manage agent profiles, chat with AI agents, configure tokens, and apply team workspaces. All from the terminal.

Part of the Kiro ecosystem:
- **Kiro** — CLI agent runtime (the engine)
- **Kite** — Desktop GUI (Tauri + React)
- **Koda** — Terminal companion (Go + Bubbletea)

Koda shares settings with Kite via `~/.kiro/settings/kite.json` — switch between them seamlessly.

## Install

**macOS / Linux** (one-liner):
```bash
curl -fsSL https://github.disney.com/raw/SANCR225/steer-runtime/main/tools/install-koda.sh | bash
```

**Windows** (PowerShell):
```powershell
irm https://github.disney.com/raw/SANCR225/steer-runtime/main/tools/install-koda.ps1 | iex
```

**From source:**
```bash
git clone git@github.disney.com:SANCR225/Koda.git
cd Koda && make install
```

**Self-update:**
```bash
koda upgrade
```

## Quick Start

```bash
koda setup                        # Check & install dependencies (node, git, kiro-cli)
koda install dev                  # Install all dev agents
koda mcp-install                  # Setup MCP servers + tokens
koda configure                    # Configure tokens (masked input)
koda                              # Launch TUI dashboard
```

## Chat Mode

Koda wraps `kiro-cli` via the ACP protocol — same engine as Kite, in your terminal.

```bash
koda chat                         # Chat with last-used agent
koda chat --agent orchestrator    # Chat with specific agent
koda chat --agent backend         # Chat with backend specialist
```

Inside chat:
- Type messages and get streaming responses
- `/profile dev` — switch profile, filters agents (dev = dev-core + dev-web + dev-mobile)
- `/agent backend` — switch agent mid-conversation
- `@agent_name` — mention agents with tab-complete
- `/clear` — clear history
- `/quit` — exit
- `pgup`/`pgdn` — scroll
- Delegation: orchestrator `<delegate>` tags are intercepted and run as sub-sessions

## Interactive TUI

```bash
koda                              # Launch dashboard
```

| Key | Screen | What it does |
|-----|--------|--------------|
| `p` | Profiles | Toggle profiles on/off, enter to apply |
| `t` | Tokens | Masked token input, auto-advance |
| `w` | Workspaces | Browse and apply team workspaces |
| `a` | Agents | Fuzzy search across all 41 agents |
| `d` | Doctor | 8-point health check |
| `r` | Rules | Toggle and install coding rules |
| `m` | MCP | MCP server bundle status |
| `s` | Sync | One-key sync of installed profiles |
| `c` | Clean | Remove all with y/n confirmation |
| `enter` | Chat | Launch chat mode |
| `q` | Quit | Exit |

## CLI Commands

```bash
# Setup & Dependencies
koda setup                          # Check & install missing deps
koda mcp-install                    # Verify MCP bundles, install context7, generate mcp.json
koda configure                      # Interactive token setup (masked input)
koda enable-tools                   # Enable thinking/todo/knowledge in kiro-cli

# Profile Management
koda install dev-core dev-web       # Install profiles
koda install dev                    # Alias: dev-core + dev-web + dev-mobile
koda remove qa                      # Remove a profile
koda sync                           # Update installed profiles
koda list [--json]                  # List all profiles
koda clean                          # Remove everything
koda status                         # One-liner: profiles, agents, tokens, branch

# Health & Diagnostics
koda check [--json]                 # Quick health check
koda doctor                         # Deep check (kiro-cli, node, git, MCP, tokens)
koda diff                           # Preview what sync would change

# Chat
koda chat [--agent NAME]            # Interactive chat with streaming

# Workspaces
koda workspace list [--json]        # List team workspaces
koda workspace show NAME [--json]   # Show workspace details
koda workspace apply NAME           # Apply workspace config
koda workspace create NAME          # Scaffold new workspace
koda workspace sync NAME [--push]   # Git pull/push all workspace repos

# Rules & Prompts
koda rules list                     # List available rules
koda rules install --all            # Install all rules
koda prompts list                   # List available prompts
koda prompts install --all          # Install all prompts
koda init-memory DIR [--from NAME]  # Initialize project memory bank

# IDE Integrations
koda amazonq install DIR            # Install Amazon Q rules
koda amazonq sync DIR               # Update Amazon Q rules
koda amazonq remove DIR             # Remove .amazonq/

# Other
koda upgrade                        # Self-update from latest release
koda version                        # Print version with banner
```

## JSON Output

For scripting and CI:
```bash
koda list --json
koda check --json
koda workspace show payments-core --json
```

## Shell Completions

```bash
koda completion bash > /usr/local/etc/bash_completion.d/koda   # Bash
koda completion zsh > "${fpath[1]}/_koda"                      # Zsh
koda completion fish > ~/.config/fish/completions/koda.fish    # Fish
```

## Architecture

```
cmd/koda/main.go              # Entry point
internal/
  acp/client.go               # ACP protocol client (JSON-RPC over stdio)
  config/paths.go             # Path resolution (~/.kiro, steer-root)
  model/                      # Domain types (Agent, Profile, Workspace, Token)
  ops/                        # Business logic (zero UI deps)
    profiles.go               # Install/remove/sync
    tokens.go                 # Read/write/inject tokens
    health.go                 # Health checks
    workspaces.go             # Workspace management
    mcp.go                    # MCP install + profiles manifest
    doctor.go                 # Deep diagnostics
    settings.go               # Shared settings with Kite
    setup.go                  # Dependency checker/installer
    status.go                 # One-liner status
    diff.go                   # Dry-run sync diff
    upgrade.go                # Self-update
    extras.go                 # Rules, prompts, memory, amazonq, agents
  cli/                        # Cobra commands
  tui/
    app.go                    # Dashboard TUI (10 screens)
    chat.go                   # Chat TUI (ACP streaming, delegation, autocomplete)
install.sh                    # macOS/Linux installer
install.ps1                   # Windows installer
Makefile                      # build, test, cross, publish
```

Key design: `ops/` has zero UI dependencies. Both `cli/` and `tui/` call into `ops/`.

## Development

```bash
make help                     # Show all targets
make build                    # Build binary
make run                      # Build + launch TUI
make test                     # Run tests (10 unit tests)
make vet                      # Go vet
make lint                     # golangci-lint
make cross                    # Cross-compile (macOS arm64/amd64, Linux, Windows)
make publish TAG=v0.1.0       # Tag + build + upload to steer-runtime releases
```

## Requirements

- Go 1.23+ (for building from source)
- [steer-runtime](https://github.disney.com/SANCR225/steer-runtime) repo (sibling directory or `--steer-root`)
- [kiro-cli](https://github.disney.com/SANCR225/steer-runtime) (for chat mode)

## License

Internal — Walt Disney Parks & Resorts Online
