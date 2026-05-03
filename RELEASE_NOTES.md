# Koda v0.4.114 — Release Notes

**Date:** 2026-05-03

## Highlights

**Graceful process cleanup during upgrade** — Koda now safely manages running processes when performing an upgrade. Disposable processes (sub-agents, tray, MCP servers) are automatically terminated, while active chat sessions prompt the user before stopping and trigger an auto-save via SIGTERM.

## What's New

### Added
- **Graceful process cleanup during upgrade** — auto-kill disposable processes (sub-agents, tray, MCP) during upgrade, prompt user before stopping active chat sessions, send SIGTERM to sessions triggering auto-save before exit (#183)
- **SIGTERM auto-save handler** — chat TUI saves session to `autosave.json` on SIGTERM; extracted shared `saveSession` method from `/save` command
- **RestartTray** now checks and announces before killing; 5s grace period with SIGKILL fallback for hung processes

### Fixed
- `gracefulKillProcess` polls with signal 0 instead of `Wait()` (works for non-child processes) (#183)
- `saveSession` uses value receiver for consistency with bubbletea pattern (#183)
- SIGTERM autosave uses `autosave-<pid>.json` to avoid collisions (#183)
- `KillOrphanProcesses` uses graceful kill for sessions, hard kill for sub-agents (#183)

## Commits

- `3a3f70e` feat: graceful process cleanup during upgrade with auto-save
- `031dafe` fix: address PR #183 review issues

## Files Changed

- `internal/ops/ps.go` — process management with graceful kill logic
- `internal/ops/upgrade.go` — upgrade flow with process cleanup
- `internal/tui/chat.go` — SIGTERM handler and saveSession extraction
