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
11-screen dashboard: profiles, tokens, workspaces, agents (fuzzy search), doctor, rules, MCP servers, fork, create workspace, sync, clean, and chat. Navigate with single keystrokes.

### 🔧 Setup & Configuration
One-command dependency installer (`koda setup`). Cross-platform: detects brew/apt/winget/choco. MCP server verification and `mcp.json` generation. Masked token input. Profile manifest for kiro-cli.

### 🚀 Profile Management
Install, remove, sync, and list agent profiles. `dev` alias expands to dev-core + dev-web + dev-mobile. Profiles manifest written after every install/sync.

### 🔀 Fork / Unfork steer-runtime
Teams with forked steer-runtime repos can switch from the default tarball source to their own git fork directly from the dashboard. Sync fetches latest in both modes — tarball re-download or `git pull`.

- **Fork** — press `[f]`, enter fork repo + branch → replaces tarball with `git clone`
- **Unfork** — press `[f]` again → re-downloads canonical tarball
- **Sync** — press `[s]` → fetches latest from tarball or git, then re-installs profiles
- Dashboard shows source: `v0.2.1 (tarball)` or `TEAM/steer-runtime@main (git)`

### 🏢 Team Workspaces
List, apply, and create team workspaces. One command to configure profiles, rules, repos, and memory banks for your team.

**Create workspace** (`[n]` from workspaces screen):
- Full interactive form: name, description, team, jira prefix, profiles, default agent, rules, enable tools
- **Repo discovery** — set a workspace path and koda auto-scans for existing git repos
- **Manual repo add** — add repos not yet cloned (cloned automatically on apply)
- **Auto-PR** — on save, creates a branch, commits, pushes, and opens a PR on the fork
- **Permission guard** — requires git fork mode + write access (checked via GitHub API)

**Apply workspace** (`enter` from workspaces screen):
- Installs profiles, rules, and context
- Clones any workspace repos not yet on disk

### 🩺 Health & Diagnostics
Quick check, deep doctor (8-point: kiro-cli, node, git, steer-runtime, agents, MCP, tokens, git status), dry-run diff, and one-liner status.

### 🤖 Slack Bot (Steery)
Run a Slack support bot powered by a read-only agent. Responds to @mentions in threads. Socket Mode — no public URL needed.

### 🧪 Agent Evals
Score agent output quality with fixtures and rubrics. Structural checks (regex, fast, free) catch broken outputs. LLM-as-judge (`--deep`) scores quality dimensions. Results saved as JSON for trend tracking.

### 📦 Distribution
One-liner install via curl/PowerShell. Self-update via `koda upgrade`. Auto-update enabled by default — daily upgrade + sync at 9 AM (LaunchAgent/cron/Task Scheduler). Cross-compile for macOS (arm64/amd64), Linux, Windows. Publish to GitHub releases.

---

## Install

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.ps1 | iex
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

## Fork / Unfork

Teams that maintain a fork of steer-runtime can switch koda from the default tarball to their fork:

```
Dashboard → [f] Fork → enter repo (e.g. TEAM/steer-runtime) + branch → done
```

Once forked:
- `[s] Sync` runs `git pull --ff-only` instead of re-downloading the tarball
- `[f] Unfork` switches back to the canonical tarball
- Team workspaces can be created and shared via PR

```
Tarball mode:  Runtime: v0.2.1 (tarball)     [f] Fork
Git mode:      Runtime: TEAM/repo@main (git)  [f] Unfork
```

---

## Team Workspaces

### Create a workspace

Requires git fork mode + write access to the fork repo.

```
Dashboard → [w] Workspaces → [n] New
```

Interactive form:
```
┌──────────────────────────────────────────────────────────────┐
│  Create Workspace         tab=next  ctrl+s=save  esc=back    │
│                                                              │
│  ▸ Name:        payments-core                                │
│    Description: Payments backend team                        │
│    Team:        payments                                     │
│    Jira Prefix: DPAY                                         │
│    Profiles:    [✓] dev-core  [✓] dev-web  [ ] qa            │
│    Agent:       orchestrator                                 │
│    Rules:       [✓] conventional_commit                      │
│    Tools:       enabled                                      │
│                                                              │
│    Repos Path:  ~/Workspace/Payments                         │
│    Repositories:                                             │
│      [✓] TEAM/payment-service  (local)                       │
│      [✓] TEAM/payment-web      (local)                       │
│      [✓] TEAM/payment-mobile   (clone on apply)              │
│      [+] org/repo-name...                                    │
└──────────────────────────────────────────────────────────────┘
```

On save (`ctrl+s`):
1. Scaffolds `workspaces/<name>/` with `workspace.json`
2. Creates branch `workspace/<name>`, commits, pushes
3. Opens a PR on the fork targeting main
4. Returns to main branch

### Apply a workspace

```
Dashboard → [w] Workspaces → select → enter
```

Installs profiles, rules, context, and clones any repos not yet on disk.

### Share with your team

Workspaces are shared via the fork repo:
1. Creator saves → PR is auto-created on the fork
2. Team lead reviews and merges the PR
3. Teammates run `[s] Sync` → workspace appears in their list
4. Teammates apply the workspace → repos are cloned, profiles installed

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
| `w` | Workspaces — browse, apply, `n` to create new |
| `a` | Agents — fuzzy search across all agents |
| `d` | Doctor — 8-point health check |
| `r` | Rules — toggle and install |
| `m` | MCP — server bundle status |
| `f` | Fork/Unfork — switch steer-runtime source |
| `s` | Sync — fetch latest + re-install profiles |
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
koda sync                           # Update (tarball re-download or git pull)
koda sync --update                  # Force download latest steer-runtime
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
koda workspace apply NAME           # Apply config + clone repos
koda workspace create NAME          # Scaffold (TUI recommended)
koda workspace sync NAME [--push]   # Git pull/push

# Rules & Prompts
koda rules list | install [--all]
koda prompts list | install [--all]
koda init-memory DIR [--from NAME]

# Slack
koda slack [--agent steery_agent]   # Run Steery bot

# Evals
koda eval AGENT                     # Structural checks
koda eval AGENT --deep              # + LLM quality scoring
koda eval --all --save              # Run all, save results
koda eval --profile critical --json # CI mode
koda eval --list                    # List fixtures

# IDE
koda amazonq install|sync|remove DIR

# Auto-update
koda auto-update enable             # Daily upgrade + sync (enabled by default)
koda auto-update disable            # Remove scheduled job
koda auto-update status             # Check if enabled

# Other
koda upgrade                        # Self-update binary
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
  config/settings.go           # SteerSettings (repo, branch, source)
  model/                       # Domain types
  ops/                         # Business logic (zero UI deps)
    steer.go                   # Sync, fork, unfork, tarball download
    ghauth.go                  # GitHub user identity + repo permissions
    workspace_create.go        # Create workspace, scan repos, clone, publish PR
    workspaces.go              # List, get, apply workspaces
  slack/bot.go                 # Steery Slack bot (Socket Mode)
  cli/                         # Cobra commands
  tui/
    app.go                     # Dashboard (11 screens)
    chat.go                    # Chat (streaming, autocomplete, delegation)
    team.go                    # Team dashboard + worker chat
    create_workspace.go        # Workspace creation form
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
make publish-steer TAG=v0.2.1 STEER_ROOT=../steer-runtime  # Publish steer-runtime
make release TAG=v0.2.1       # Tag + cross-compile + publish Koda
```

## Requirements

- Go 1.23+ (building from source only)
- [steer-runtime](https://github.disney.com/SANCR225/steer-runtime) (sibling dir or `--steer-root`)
- [kiro-cli](https://kiro.dev) (for chat and teams)
- [gh](https://cli.github.com/) (for fork, workspace PR creation)

## License

Internal — Walt Disney Parks & Resorts Online
