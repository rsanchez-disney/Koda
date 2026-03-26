# Koda 🐾

Interactive terminal companion for [steer-runtime](https://github.disney.com/SANCR225/steer-runtime) — manage agent profiles, chat with AI agents, configure tokens, and apply team workspaces.

## Install

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/rsanchez-disney/Koda/main/install.ps1 | iex
```

## Verify

```bash
koda version
```

## Usage

```bash
koda setup                        # Check dependencies
koda install dev                  # Install dev agents
koda mcp-install                  # Setup MCP servers + tokens
koda                              # Launch interactive dashboard
koda chat --agent orchestrator    # Chat with an agent
```

## Update

```bash
koda self-update
```

## Uninstall

```bash
rm $(which koda)
```

---

Binaries are published as [GitHub Releases](https://github.com/rsanchez-disney/Koda/releases). Source code is maintained internally.
