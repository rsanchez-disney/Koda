# Koda Setup — MCP Server Installation

This guide walks through setting up MCP servers using `koda mcp-install`. The command verifies
pre-built server bundles, lets you choose which servers to install, configures required tokens
inline, and generates `~/.kiro/settings/mcp.json`.

## Prerequisites

- Koda binary installed (`koda version` works)
- steer-runtime available (auto-cloned on first run, or `--steer-root`)
- Node.js installed (for MCP server bundles)

## Quick Start

```bash
koda setup          # Install all dependencies first
koda mcp-install    # Setup MCP servers
```

## Command Modes

`koda mcp-install` adapts its behavior based on your current configuration:

| Scenario                         | Behavior                                                                |
|----------------------------------|-------------------------------------------------------------------------|
| No `mcp.json` exists (first run) | Interactive assistant — server selection, token prompts, GitHub remotes |
| `mcp.json` already has servers   | Quick reinstall — re-verifies and regenerates config silently           |
| `--assistant` flag used          | Forces interactive assistant regardless of existing config              |
| Non-TTY (CI, piped input)        | Installs all verified servers without prompting                         |

```bash
koda mcp-install               # Auto-detects: assistant on first run, quick reinstall after
koda mcp-install --assistant   # Force interactive assistant to reconfigure servers/tokens
echo "" | koda mcp-install     # Non-TTY: install all verified servers silently
```

## Interactive Assistant Flow

The assistant runs automatically on first install (no existing `mcp.json`), or when you pass
`--assistant`. It walks you through three phases:

### 1. Bundle Verification

Koda scans `~/.kiro/tools/mcp-servers/` for pre-built server bundles and reports which ones are
ready:

```
🔍 Verifying MCP server bundles...
  ✓ jira-mcp
  ✓ confluence-mcp
  ✓ mermaid-diagram-mcp
  ✓ bruno-mcp
  ✓ mywiki-mcp
  ✓ figma-mcp
  ✓ github-mcp
  ✓ context7-mcp

✅ 8 MCP servers available
```

### 2. Server Selection

A numbered toggle list lets you pick which servers to install. All verified servers default to
enable.

```
🔧 Select MCP servers to install (toggle with number, Enter to confirm):

  [1] ✓ jira
  [2] ✓ confluence
  [3] ✓ mermaid
  [4] ✓ bruno
  [5] ✓ mywiki
  [6] ✓ figma
  [7] ✓ github
  [8] ✓ context7
  [9] ✓ compass

Toggle (1-9, a=all, n=none, Enter=confirm):
```

Controls:

- Type numbers separated by commas to toggle (e.g., `5,6` disables mywiki and figma)
- `a` — enable all verified servers
- `n` — disable all servers
- `Enter` — confirm selection and proceed

### 3. Token Configuration

Koda prompts only for tokens required by your selected servers. Existing values are shown masked.
Press Enter to keep the current value, or type a new one (input is hidden).

```
🔑 Configure tokens for selected servers

  Jira PAT [abcde1...k8z9]:
    Hint: https://jira.disney.com/secure/ViewProfile.jspa → Personal Access Tokens
    New value (Enter to keep):
    ⏭ Kept

  Confluence PAT [not set]:
    Hint: https://confluence.disney.com/plugins/personalaccesstokens/usertokens.action
    New value: ********
    ✓ Updated
```

### 4. GitHub Remotes (if github selected)

If you selected the GitHub server, Koda shows existing remotes and lets you add new ones:

```
🐙 GitHub Remotes

  Current remotes:
    • disney (github.disney.com) abcde1...k8z9

  Add GitHub remote? (name or Enter to skip): public
    Host for 'public' (e.g., github.com): github.com
    Token for 'public': ********
    API path for 'public' (Enter to skip):
    ✓ Added remote 'public'

  Add GitHub remote? (name or Enter to skip):
```

Multiple remotes generate separate `github-<name>` entries in the config. A single remote uses the
plain `github` key.

### 5. Config Generation

Koda generates `~/.kiro/settings/mcp.json` with only your selected servers and injects tokens into
agent configurations:

```
🔧 Generating mcp.json...
  ✓ /Users/dev/.kiro/settings/mcp.json

  Servers included:
    • jira
    • confluence
    • mermaid
    • bruno
    • github-disney
    • github-public
    • context7
    • compass

✅ MCP servers ready (8 servers configured)
```

## Quick Reinstall Mode

When `mcp.json` already contains at least one server, running `koda mcp-install` without
`--assistant` performs a quick reinstall: it re-verifies bundles, selects all verified servers,
reads existing tokens and GitHub remotes, and regenerates the config. No prompts.

To reconfigure servers or tokens after initial setup, use:

```bash
koda mcp-install --assistant
```

## Non-Interactive Mode

When stdin is not a TTY (CI, scripts, piped input), Koda falls back to installing all verified
servers without prompting:

```bash
echo "" | koda mcp-install
```

This preserves backward compatibility with automated workflows.

## Available Servers

| Server     | Bundle              | Tokens Required  | Notes                                      |
|------------|---------------------|------------------|--------------------------------------------|
| jira       | jira-mcp            | `JIRA_PAT`       |                                            |
| confluence | confluence-mcp      | `CONFLUENCE_PAT` | Also needs `CONFLUENCE_URL` env var        |
| mermaid    | mermaid-diagram-mcp | —                | No tokens needed                           |
| bruno      | bruno-mcp           | —                | No tokens needed                           |
| mywiki     | mywiki-mcp          | `MYWIKI_PAT`     | Also needs `MYWIKI_URL` env var            |
| figma      | figma-mcp           | `FIGMA_TOKEN`    |                                            |
| github     | github-mcp          | —                | Tokens managed via GitHub Remotes flow     |
| context7   | context7-mcp        | —                | Installed via npm from public registry     |
| compass    | —                   | `COMPASS_TOKEN`  | SSE transport, needs `COMPASS_URL` env var |

## Re-running

- `koda mcp-install` — quick reinstall with current settings (if config exists)
- `koda mcp-install --assistant` — re-enter the interactive flow to change server selection,
  update tokens, or manage GitHub remotes

The config file is overwritten each time.

## Troubleshooting

- **"0 MCP servers available"** — steer-runtime hasn't been synced yet. Run `koda sync` first.
- **Bundle missing for a server** — the server shows `(bundle missing)` in the selector and is
  excluded from the default selection. Run `koda sync --update` to re-download bundles.
- **context7 install fails** — requires `npm` on your PATH. Run `koda setup` to install Node.js.
- **Compass not in config** — compass only appears if `COMPASS_TOKEN` is set. Enter it during the
  token prompt.
