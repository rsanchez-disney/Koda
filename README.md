# Koda

Interactive CLI for managing [steer-runtime](https://github.disney.com/SANCR225/steer-runtime) agent profiles, tokens, workspaces, and IDE integrations.

Part of the Kiro ecosystem: **Kiro** (CLI agent runtime) > **Kite** (desktop GUI) > **Koda** (setup companion).

## Install

```bash
make build          # Build bin/koda
make install        # Copy to ~/go/bin/
```

Or cross-compile for all platforms:

```bash
make cross          # macOS (arm64/amd64), Linux, Windows
```

## Usage

### Interactive TUI

```bash
koda                              # Launch interactive dashboard
```

The TUI provides 9 screens:

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

### CLI Commands

```bash
# Profile management
koda install dev-core dev-web       # Install profiles
koda install dev                    # Alias: dev-core + dev-web + dev-mobile
koda remove qa                      # Remove a profile
koda sync                           # Update installed profiles
koda list [--json]                  # List all profiles
koda clean                          # Remove everything
koda status                         # One-liner: profiles, agents, tokens, branch

# Health
koda check [--json]                 # Quick health check
koda doctor                         # Deep health check (MCP, node, git, etc.)
koda diff                           # Preview what sync would change

# Configuration
koda configure                      # Interactive token setup (masked input)
koda enable-tools                   # Enable thinking/todo/knowledge in kiro-cli
koda mcp-install                    # Verify MCP bundles, install context7, generate mcp.json

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
koda upgrade                        # Self-update from GitHub releases
koda version                        # Print version
```

### JSON Output

For scripting and CI:

```bash
koda list --json
koda check --json
koda workspace show payments-core --json
```

## Shell Completions

```bash
# Bash
koda completion bash > /usr/local/etc/bash_completion.d/koda

# Zsh
koda completion zsh > "${fpath[1]}/_koda"

# Fish
koda completion fish > ~/.config/fish/completions/koda.fish
```

## Architecture

```
cmd/koda/main.go              # Entry point
internal/
  config/paths.go             # Path resolution (~/.kiro, steer-root)
  model/                      # Domain types (Agent, Profile, Workspace, Token)
  ops/                        # Business logic (zero UI deps)
    profiles.go               # Install/remove/sync
    tokens.go                 # Read/write/inject tokens
    health.go                 # Health checks
    workspaces.go             # Workspace management
    mcp.go                    # MCP install + profiles manifest
    doctor.go                 # Deep diagnostics
    status.go                 # One-liner status
    diff.go                   # Dry-run sync diff
    upgrade.go                # Self-update
    extras.go                 # Rules, prompts, memory, amazonq, agents
  cli/                        # Cobra commands
  tui/app.go                  # Bubbletea TUI (9 screens)
Makefile
```

Key design: `ops/` has zero UI dependencies. Both `cli/` and `tui/` call into `ops/`.

## Development

```bash
make help           # Show all targets
make build          # Build binary
make run            # Build + launch TUI
make test           # Run tests (10 unit tests)
make vet            # Go vet
make fmt            # Format code
make lint           # golangci-lint
make cross          # Cross-compile
make release TAG=v0.1.0  # Tag + cross-compile
```

## Requirements

- Go 1.23+
- [steer-runtime](https://github.disney.com/SANCR225/steer-runtime) repo (sibling directory or `--steer-root`)

## License

Internal — Walt Disney Parks & Resorts Online
