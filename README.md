# Koda 🐾

Interactive terminal companion for [steer-runtime](https://github.disney.com/SANCR225/steer-runtime) — manage agent profiles, chat with AI agents, orchestrate agent teams, and configure your environment. All from the terminal.

Part of the Kiro ecosystem:
- **Kiro** — CLI agent runtime (the engine)
- **Kite** — Desktop GUI (Tauri + React)
- **Koda** — Terminal companion (Go + Bubbletea)
- **Steery** — Slack support bot

Koda shares settings with Kite via `~/.kiro/settings/kite.json` — switch between them seamlessly.

---

## Features

### 💬 Chat Mode
Stream responses from any Kiro agent in your terminal. Autocomplete for `/commands` and `@agent` mentions. Switch agents and profiles mid-conversation. Prompt history with ↑↓ arrows. Delegation support — orchestrator `<delegate>` tags spawn sub-sessions automatically.

### 👥 Agent Teams
Orchestrate multiple Kiro agents working in parallel on a single goal. Each worker gets its own git worktree, ACP session, and trust level. AI-assisted plan generation decomposes goals into worker tasks. Dependency chains, result handoff between workers, conflict detection, and three merge strategies (rebase-chain, parallel-merge, PR-per-worker).

### 📊 Interactive TUI Dashboard
10-screen dashboard: profiles, tokens, workspaces, agents (fuzzy search), doctor, rules, MCP servers, sync, clean, and chat. Navigate with single keystrokes.

### 🔧 Setup & Configuration
One-command dependency installer (`koda setup`). Cross-platform: detects brew/apt/winget/choco. MCP server verification and `mcp.json` generation. Masked token input. Profile manifest for kiro-cli.

### 🚀 Profile Management
Install, remove, sync, and list agent profiles. `dev` alias expands to dev-core + dev-web + dev-mobile. Profiles manifest written after every install/sync.

### 🏢 Team Workspaces
List, show, apply, create, and sync team workspaces. One command to configure profiles, rules, and memory banks for your team.

### 🩺 Health & Diagnostics
Quick check, deep doctor (8-point: kiro-cli, node, git, steer-runtime, agents, MCP, tokens, git status), dry-run diff, and one-liner status.

### 🤖 Slack Bot (Steery)
Run a Slack support bot powered by a read-only agent. Responds to @mentions in threads. Socket Mode — no public URL needed.

### 📦 Distribution
One-liner install via curl/PowerShell. Self-update via `koda upgrade`. Cross-compile for macOS (arm64/amd64), Linux, Windows. Publish to GitHub releases.

---

## Install

**macOS / Linux:**
```bash
curl -fsSL https://github.disney.com/raw/SANCR225/steer-runtime/main/tools/install-koda.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://github.disney.com/raw/SANCR225/steer-runtime/main/tools/install-koda.ps1 | iex
```

**From source:**
```bash
git clone git@github.disney.com:SANCR225/Koda.git && cd Koda && make install
```

---

## Quick Start

```bash
koda setup                        # Check & install dependencies
koda install dev                  # Install all dev agents
koda mcp-install                  # Setup MCP servers + tokens
koda                              # Launch TUI dashboard
koda chat --agent orchestrator    # Start chatting
```

---

## Agent Teams

```bash
koda team plan --goal "add refund endpoint"       # AI generates a TeamSpec
koda team init full-stack                          # Or scaffold manually
koda team run spec.json --goal "..." --tui         # Launch with TUI dashboard
koda team merge spec.json                          # Conflict check + merge
```

Team dashboard shows worker cards with live status, context usage, streaming output, and abort controls. Press `enter` on a worker to chat with it directly.

---

## Chat

```bash
koda chat                         # Last-used agent
koda chat --agent orchestrator    # Specific agent
koda chat --debug                 # Log ACP traffic
```

| Feature | How |
|---------|-----|
| Switch agent | `/agent backend` |
| Switch profile | `/profile dev` (filters agents) |
| Mention agent | `@agent_name` + tab |
| Trust level | `/trust autonomous\|supervised\|strict` |
| History | ↑↓ arrows |
| Scroll | pgup/pgdn |
| Clear | `/clear` |
| Exit | `/quit` or ctrl+c |

---

## TUI Dashboard

```bash
koda                              # Launch
```

| Key | Screen |
|-----|--------|
| `p` | Profiles — toggle on/off, enter to apply |
| `t` | Tokens — masked input, auto-advance |
| `w` | Workspaces — browse and apply |
| `a` | Agents — fuzzy search across 41 agents |
| `d` | Doctor — 8-point health check |
| `r` | Rules — toggle and install |
| `m` | MCP — server bundle status |
| `s` | Sync — one-key update |
| `c` | Clean — y/n confirmation |
| `enter` | Chat |
| `q` | Quit |

---

## All Commands

```bash
# Setup
koda setup                          # Install dependencies (node, git, kiro-cli, gh)
koda mcp-install                    # MCP bundles + mcp.json
koda configure                      # Token setup (masked)
koda enable-tools                   # Enable thinking/todo/knowledge

# Profiles
koda install dev-core dev-web       # Install profiles
koda install dev                    # Alias: core + web + mobile
koda remove qa                      # Remove
koda sync                           # Update
koda list [--json]                  # List
koda clean                          # Remove all
koda status                         # One-liner summary

# Health
koda check [--json]                 # Quick check
koda doctor                         # Deep diagnostics
koda diff                           # Dry-run sync

# Chat
koda chat [--agent NAME] [--debug]  # Interactive chat

# Teams
koda team plan --goal TEXT          # AI plan generation
koda team init NAME                 # Scaffold TeamSpec
koda team list                      # List specs
koda team run SPEC --goal TEXT [--tui]  # Launch team
koda team merge SPEC                # Conflict check + merge
koda team status                    # Running team status

# Workspaces
koda workspace list [--json]        # List
koda workspace show NAME [--json]   # Details
koda workspace apply NAME           # Apply config
koda workspace create NAME          # Scaffold
koda workspace sync NAME [--push]   # Git pull/push

# Rules & Prompts
koda rules list | install [--all]
koda prompts list | install [--all]
koda init-memory DIR [--from NAME]

# Slack
koda slack [--agent steery_agent]   # Run Steery bot

# IDE
koda amazonq install|sync|remove DIR

# Other
koda upgrade                        # Self-update
koda version                        # Banner + version
```

---

## Architecture

```
cmd/koda/main.go
internal/
  acp/client.go               # ACP protocol (JSON-RPC over stdio)
  team/                        # Agent Teams engine
    orchestrator.go            # Team lifecycle + dependency resolver
    worker.go                  # Worker state machine + ACP per worker
    worktree.go                # Git worktree manager
    teamspec.go                # TeamSpec schema + Handoff builder
    deps.go                    # Cycle detection + result extraction
    planner.go                 # AI plan generation
    merge.go                   # Conflict detection + merge strategies
  config/paths.go              # Path resolution
  model/                       # Domain types
  ops/                         # Business logic (zero UI deps)
  slack/bot.go                 # Steery Slack bot (Socket Mode)
  cli/                         # Cobra commands
  tui/
    app.go                     # Dashboard (10 screens)
    chat.go                    # Chat (streaming, autocomplete, delegation)
    team.go                    # Team dashboard + worker chat
install.sh / install.ps1       # One-liner installers
Makefile                       # build, test, cross, publish
```

Key design: `ops/` and `team/` have zero UI dependencies. `cli/` and `tui/` are thin wrappers.

---

## Development

```bash
make help                     # All targets
make build                    # Build
make test                     # 10 unit tests
make lint                     # golangci-lint
make cross                    # macOS/Linux/Windows
make publish TAG=v0.2.0       # Tag + upload to steer-runtime releases
```

## Requirements

- Go 1.23+ (building from source only)
- [steer-runtime](https://github.disney.com/SANCR225/steer-runtime) (sibling dir or `--steer-root`)
- [kiro-cli](https://kiro.dev) (for chat and teams)

## License

Internal — Walt Disney Parks & Resorts Online
