# Koda 🐾

Interactive terminal companion for [steer-runtime](https://github.disney.com/SANCR225/steer-runtime) — manage agent profiles, chat with AI agents, orchestrate agent teams, and configure your environment. All from the terminal.

Part of the Kiro ecosystem:
- **Kiro CLI** (`kiro-cli`) — Agent runtime engine
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
12-screen dashboard: main dashboard, profiles, tokens, env vars, workspaces, agents (fuzzy search), doctor, rules, MCP servers, fork, create/edit workspace, and clean confirmation. Sync runs inline from the dashboard. Chat launches a separate session. Navigate with single keystrokes.

### 🔧 Setup & Configuration
One-command dependency installer (`koda setup`). Cross-platform: detects brew/apt/winget/choco. MCP server verification and `mcp.json` generation. Masked token input with paste support and PAT URL hints. Profile manifest for kiro-cli.

### 🚀 Profile Management
Install, remove, sync, and list agent profiles. `dev` alias expands to dev-core + dev-web + dev-mobile. Profiles manifest written after every install/sync.

### 🔀 Fork / Unfork steer-runtime
Teams with forked steer-runtime repos can switch from the default tarball source to their own git fork directly from the dashboard. See [Fork / Unfork](#fork--unfork) for details.

### 🏢 Team Workspaces
List, apply, create, and edit team workspaces. One command to configure profiles, rules, repos, and memory banks for your team. See [Team Workspaces](#team-workspaces) for details.

### 🩺 Health & Diagnostics
Quick check (`koda check`), deep doctor (`koda doctor`) with 9-point diagnostics: kiro-cli, node, git, steer-runtime, agents, MCP servers (with per-server diagnostics), tokens, git status, gh-auth. Dry-run diff (`koda diff`) previews what sync would change. One-liner status (`koda status`). In the TUI, navigate checks with `j/k`, press `f` to auto-fix failing checks (e.g., `gh auth login`).

### 📈 Usage Stats
Track prompt scoring and token usage over time with `koda stats`. Defaults to the last 7 days.

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

> If steer-runtime isn't found locally, Koda auto-clones it to `~/.kiro/steer-runtime` on first run.

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

## Global Flags

| Flag | Description |
|------|-------------|
| `--steer-root PATH` | Path to steer-runtime (auto-detected from CWD, parent, or `~/.kiro/steer-runtime`) |
| `--project PATH` | Target project directory (default: `~/.kiro`) |
| `--json` | JSON output (where supported) |

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

### Edit a workspace

**TUI** — press `[e]` on a workspace to open the edit form pre-populated with existing values. Press `ctrl+e` to open the raw JSON in `$EDITOR`.

**CLI:**
```bash
koda workspace edit payments-core   # Open in $EDITOR
```

### Share with your team

Workspaces are shared via the fork repo:
1. Creator saves → PR is auto-created on the fork
2. Team lead reviews and merges the PR
3. Teammates run `[s] Sync` → workspace appears in their list
4. Teammates apply the workspace → repos are cloned, profiles installed

---

## Env Vars

Manage non-secret environment variables (URLs, paths) separately from tokens.

**TUI** — press `[e]` from dashboard:
```
▸ GITHUB_URL          https://github.disney.com
    type to edit...█
    GitHub Enterprise URL
  CONFLUENCE_URL      https://confluence.disney.com
  MYWIKI_URL          https://mywiki.disney.com
  JIRA_URL            https://jira.disney.com
  GITHUB_API_PATH     /api/v3
```

- `j/k` navigate, type to edit, `enter` saves all
- `n` to add custom env vars, `d` to delete custom ones
- Known vars have sensible defaults — override only when needed

**CLI:**
```bash
koda env list                              # Show all env vars
koda env get GITHUB_URL                    # Get a single value
koda env set CUSTOM_VAR=my-value           # Set any env var
```

Stored in `~/.kiro/env.vars`. MCP server config reads URLs from here instead of hardcoding.

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
| `t` | Tokens — masked input, paste support, PAT URL hints |
| `a` | Agents — fuzzy search across all agents |
| `e` | Env Vars — manage URLs and config |
| `d` | Doctor — 9-point health check, per-server MCP diagnostics, `f` to fix |
| `r` | Rules — toggle and install |
| `w` | Workspaces — browse, apply, `e` to edit, `n` to create |
| `m` | MCP — server bundle status |
| `f` | Fork/Unfork — switch steer-runtime source |
| `s` | Sync — fetch latest + re-install profiles (inline action) |
| `c` | Clean — y/n confirmation |
| `enter` | Chat (launches separate session) |
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
koda diff                           # Dry-run sync (preview changes)

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
koda workspace edit NAME            # Edit in $EDITOR
koda workspace sync NAME [--push]   # Git pull/push across workspace repos

# Env Vars
koda env list                       # Show all env vars
koda env get KEY                    # Get value
koda env set KEY=VALUE              # Set value

# Rules & Prompts
koda rules list | install [--all]
koda prompts list | install [--all]
koda init-memory DIR [--from NAME]

# Stats
koda stats [--days N]               # Prompt scoring & token usage (default: 7 days)

# Slack
koda slack [--agent steery_agent]   # Run Steery bot

# Evals
koda eval AGENT                     # Structural checks
koda eval AGENT --deep              # + LLM quality scoring
koda eval --all --save              # Run all, save results
koda eval --profile critical --json # CI mode
koda eval --list                    # List fixtures

# IDE Integration
koda amazonq install|sync|remove DIR  # Manage Amazon Q agent config in a project

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
  eval/                        # Agent evaluation framework
    runner.go                  # Fixture loading + structural eval
    scorer.go                  # LLM-as-judge quality scoring
    reporter.go                # Results output + JSON export
    types.go                   # Eval domain types
  model/                       # Domain types
  ops/                         # Business logic (zero UI deps)
    steer.go                   # Sync, fork, unfork, tarball download
    ghauth.go                  # GitHub user identity + repo permissions
    workspace_create.go        # Create workspace, scan repos, clone, publish PR
    workspaces.go              # List, get, apply workspaces
    envvars.go                 # Env vars store (read/write/get)
    doctor.go                  # 9-point health diagnostics
    autoupdate.go              # LaunchAgent/cron/Task Scheduler management
    scorer.go                  # Prompt scoring + token usage tracking
    upgrade.go                 # Self-update from GitHub releases
  slack/bot.go                 # Steery Slack bot (Socket Mode)
  cli/                         # Cobra commands
  tui/
    app.go                     # Dashboard (12 screens)
    chat.go                    # Chat (streaming, autocomplete, delegation)
    team.go                    # Team dashboard + worker chat
    create_workspace.go        # Workspace creation form
install.sh / install.ps1       # One-liner installers
Makefile                       # build, test, cross, publish
```

Key design: `ops/`, `team/`, and `eval/` have zero UI dependencies. `cli/` and `tui/` are thin wrappers.

---

## Development

```bash
make help                     # All targets
make build                    # Build
make test                     # Run tests
make lint                     # golangci-lint
make cross                    # macOS/Linux/Windows
make publish-steer TAG=v0.x.x STEER_ROOT=../steer-runtime  # Publish steer-runtime tarball
make release TAG=v0.x.x       # Tag + cross-compile + publish Koda
```

## Requirements

- Go 1.25+ (building from source only)
- [steer-runtime](https://github.disney.com/SANCR225/steer-runtime) (auto-cloned, or sibling dir, or `--steer-root`)
- [kiro-cli](https://kiro.dev) (for chat and teams)
- [gh](https://cli.github.com/) (for fork, workspace PR creation)

---

## License

Internal — Walt Disney Parks & Resorts Online
