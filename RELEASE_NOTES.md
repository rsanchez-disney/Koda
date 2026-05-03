# Koda v0.4.115 — Release Notes

**Date:** 2026-05-03

## Highlights

**MCP toggle and chrome-devtools defaults fixed** — Two bugs in the MCP subsystem are resolved: the TUI space-bar toggle for MCP servers now targets the correct section, and the chrome-devtools server is disabled by default as intended.

## What's Fixed

- **TUI MCP toggle** — space bar checked `mcpSection==3` instead of `4`, so toggling servers on/off never worked (`internal/tui/app.go`)
- **chrome-devtools default** — MCP server entry was missing `Disabled: true`, causing it to load on every session (`internal/ops/mcp.go`)

## Commits

- `dc9ebcb` fix: MCP toggle wrong section and chrome-devtools not disabled by default (#179)

## Files Changed

- `internal/tui/app.go` — corrected mcpSection comparison from 3 to 4
- `internal/ops/mcp.go` — added `Disabled: true` to chrome-devtools server entry
