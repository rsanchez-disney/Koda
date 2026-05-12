# Koda v0.4.129 — Release Notes

**Date:** 2026-05-12

## Highlights

**Workspace + Fork MCP Levels** — Full support for custom MCP servers at workspace and fork levels, with variable resolution, TUI integration, and user server preservation across upgrades.

## What's New

- **`_source` tracking** — every MCP server in `mcp.json` is tagged with its origin: `global`, `fork`, `workspace:<name>`, or `user`
- **User server preservation** — custom MCPs added by users are never lost during `koda upgrade` or `mcp-install`
- **Workspace MCP support** — reads `workspaces/<team>/mcp/mcp.json` with 3-tier variable resolution (`tokens.env` > `defaults.env` > `variable.default`)
- **`_overrides`** — workspace MCPs can replace global servers (e.g., pre-configured Confluence for a team)
- **`koda mcp status`** — shows all servers grouped by source (global/fork/workspace/user)
- **`koda mcp add [--fork]`** — scaffolds new MCP at workspace or fork level with all required files
- **TUI: Workspace Variables section** — configure workspace MCP variables directly from the TUI (`m` key → tab to section 6)
- **TUI: source labels** — MCP Servers section shows `(fork)`, `(workspace:X)`, `(user)` next to server names
- **Missing var prompt** — warns on workspace activation if required variables are not set
- **`workspace_path` env vars** — supports `${VAR}`, `$VAR`, `%VAR%`, `~` for cross-platform portability
- **Extended `mcp-meta.json`** — fork MCPs can declare `env_required`, `env_secret`, `env_defaults`

## What's Fixed

- **File permissions** — `mcp.json` now written with `0600` (was `0644` — file contains secrets)
- **Path traversal** — workspace names sanitized with `filepath.Base(filepath.Clean())`
- **Infinite loop guard** — `resolveVariables` capped at 50 iterations
- **Workspace index** — `PromptMissingWorkspaceMCPVars` uses leaf workspace (not root ancestor)
- **MCP path resolution** — `ListMCPServers`/`ListMCPServersBySource` read from active workspace path
- **PropagateMCPJson** — called after `GenerateMcpJson` to sync `_source` tags to workspace copies

## Breaking Changes

None. Fully backward compatible — workspaces without `mcp/` folder behave identically to before.
