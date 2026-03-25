# Koda 🐾

Interactive CLI for managing [steer-runtime](https://github.disney.com/SANCR225/steer-runtime) agent profiles, tokens, workspaces, and IDE integrations.

Part of the Kiro ecosystem: **Kiro** (CLI agent runtime) → **Kite** (desktop GUI) → **Koda** (setup companion).

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

The TUI provides 5 screens:

| Key | Screen | What it does |
|-----|--------|--------------|
| `p` | Profiles | Toggle profiles on/off, enter to apply |
| `t` | Tokens | Masked token input, auto-advance |
| `w` | Workspaces | Browse and apply team workspaces |
| `a` | Agents | Fuzzy search across all 41 agents |
| `s` | Sync | One-key sync of installed profiles |

<!-- TODO: Add TUI screenshots -->

### CLI Commands

```bash
# Profile management
koda install dev-core dev-web       # Install profiles
koda install dev                    # Alias: dev-core + dev-web + dev-mobile
koda remove qa                      # Remove a profile
koda sync                           # Update installed profiles
koda list                           # List all profiles
koda clean                          # Remove everything

# Health
koda check                          # Quick health check
koda doctor                         # Deep health check (MCP, node, git, etc.)

# Configuration
koda configure                      # Interactive token setup (masked input)
koda enable-tools                   # Enable thinking/todo/knowledge in kiro-cli

# Workspaces
koda workspace list                 # List team workspaces
koda workspace show payments-core   # Show workspace details
koda workspace apply payments-core  # Apply workspace config
koda workspace create my-team       # Scaffold new workspace
koda workspace sync payments-core   # Git pull all workspace repos

# Rules & Prompts
koda rules list                     # List available rules
koda rules install --all            # Install all rules
koda prompts list                   # List available prompts
koda prompts install --all          # Install all prompts
koda init-memory ~/myapp            # Initialize project memory bank

# IDE Integrations
koda cursor install ~/myapp         # Install Cursor rules + MCP config
koda cursor init-memory ~/myapp     # Generate project context .mdc
koda cursor remove ~/myapp          # Remove .cursor/
koda amazonq install ~/myapp        # Install Amazon Q rules
koda amazonq remove ~/myapp         # Remove .amazonq/

# JSON output (for scripting/CI)
koda list --json
koda check --json
koda workspace show payments-core --json
```

## Shell Completions

Cobra generates completions automatically:

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
    doctor.go                 # Deep diagnostics
    extras.go                 # Rules, prompts, memory, IDE, agents
    cursor_memory.go          # Cursor project context generation
  cli/                        # Cobra commands
    root.go                   # Root + version + steer-root auto-detect
    commands.go               # install, remove, sync, list, check, clean
    workspace.go              # workspace list/show/apply/create/sync
    extras.go                 # rules, prompts, init-memory, cursor, amazonq
    configure.go              # configure, enable-tools, doctor
  tui/app.go                  # Bubbletea TUI (5 screens)
Makefile                      # build, run, test, lint, cross, release
```

Key design: `ops/` has zero UI dependencies. Both `cli/` and `tui/` call into `ops/`.

## Development

```bash
make help           # Show all targets
make build          # Build binary
make run            # Build + launch TUI
make test           # Run tests
make vet            # Go vet
make fmt            # Format code
make lint           # golangci-lint
make tidy           # go mod tidy
make clean          # Remove bin/
make cross          # Cross-compile
make release        # Tag + cross-compile
```

## Requirements

- Go 1.21+
- [steer-runtime](https://github.disney.com/SANCR225/steer-runtime) repo (sibling directory or `--steer-root`)

## License

Internal — Walt Disney Parks & Resorts Online
