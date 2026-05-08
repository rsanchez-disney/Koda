# Changelog

All notable changes to Koda.

## [Unreleased]

---

## [0.5.0] — 2026-05-08

### Added
- **Multi-prefix jira_prefix** — `StringOrSlice` type supports `string | string[]` for workspace jira_prefix (#196)

### Fixed
- `dev` alias now expands to all 9 dev-* profiles (#197)

---

## [0.4.123] — 2026-05-06

### Added
- **Auto-configure kiroCLIPath** — IDE plugin install now auto-sets `kiroCLIPath` in settings (#194)

### Fixed
- Clean orphaned steering/skills/agents on sync (#192)
- Correct `enable-tools` setting name (#193)

---

## [0.4.115] — 2026-05-05

### Added
- **Multi-workspace sessions** — `koda chat --ws <name>` spawns isolated sessions with `KIRO_HOME` pointing to `~/.kiro/workspaces/<name>/` (#191)
- **Workspace lifecycle** — `koda workspace remove <name>` dematerializes + cleans settings; `koda workspace prune` removes stale (30d) workspaces (#191)
- **MCP propagation** — `koda sync` propagates global `mcp.json` to all materialized workspaces (#191)
- **TUI workspace checklist** — multi-select with `space`=toggle active, `p`=set primary (#191)
- **IDE plugins CLI** — `koda ide install/status/update` commands (#189)
- **IDE Plugins TUI screen** — detect and install steer plugins for 8 IDEs (#187)
- **8 IDE support** — VS Code, Cursor, IntelliJ, WebStorm, PyCharm, Rider, GoLand, Android Studio (#187)
- **steer-plugins in releases** — steer.vsix bundled with Koda release assets (#187)

### Fixed
- Sibling profiles with same agent names showing as installed — false positive (#190)
- reconcile activeWorkspace with workspace.json to prevent drift (#186)
- native MCP install wizard added to TUI env vars screen (#185)
- MCP toggle wrong section and chrome-devtools not disabled by default (#179)
- restore [c] Reset menu entry, add gh auth token fallback for GHE
- download IDE plugins from public Koda releases (no GHE auth needed)
- bufio reader for upgrade session prompt (#188)

---

## [0.4.114] — 2026-05-03

### Added
- **Graceful process cleanup during upgrade** — auto-kill disposable processes (sub-agents, tray, MCP) during upgrade, prompt user before stopping active chat sessions, send SIGTERM to sessions triggering auto-save before exit (#183)
- **SIGTERM auto-save handler** — chat TUI saves session to `autosave.json` on SIGTERM; extracted shared `saveSession` method from `/save` command
- **RestartTray** now checks and announces before killing; 5s grace period with SIGKILL fallback for hung processes

### Fixed
- `gracefulKillProcess` polls with signal 0 instead of `Wait()` (works for non-child processes) (#183)
- `saveSession` uses value receiver for consistency with bubbletea pattern (#183)
- SIGTERM autosave uses `autosave-<pid>.json` to avoid collisions (#183)
- `KillOrphanProcesses` uses graceful kill for sessions, hard kill for sub-agents (#183)

---

## [0.4.113] — 2026-05-03

### Fixed
- Sync installs global profile before workspace overlay agents, matching the workspace apply flow; also deduplicates DetectInstalled output (#182)

---

## [0.1.0] — 2026-03-28

Initial release with install, sync, remove, workspace, doctor, chat, TUI dashboard.
