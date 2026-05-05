# Koda v0.4.115 — Release Notes

**Date:** 2026-05-05

## Highlights

**Multi-workspace sessions** — Work in multiple workspaces in parallel without overwriting `~/.kiro/` configuration. Each `koda chat --ws <name>` session gets its own isolated runtime directory.

## What's New

- **Multi-workspace sessions** — `koda chat --ws <name>` spawns isolated sessions with `KIRO_HOME` pointing to workspace-specific runtime dir (#191)
- **Workspace lifecycle** — `koda workspace remove <name>` / `koda workspace prune` for cleanup
- **MCP propagation** — `koda sync` propagates `mcp.json` to all materialized workspaces
- **TUI workspace checklist** — multi-select with `space`=toggle, `p`=primary (#191)
- **IDE plugins** — `koda ide install/status/update` CLI commands + TUI IDE Plugins screen (#189)
- **8 IDE support** — VS Code, Cursor, IntelliJ, WebStorm, PyCharm, Rider, GoLand, Android Studio
- **steer-plugins in releases** — steer.vsix bundled with Koda releases

## What's Fixed

- Sibling profiles with same agent names showing as installed (false positive) (#190)
- reconcile activeWorkspace with workspace.json to prevent drift (#186)
- MCP toggle wrong section and chrome-devtools not disabled by default (#179)
- native MCP install wizard in TUI env vars screen (#185)
- bufio reader for upgrade session prompt (#188)

## Commits

See full diff: v0.4.114...v0.4.115
