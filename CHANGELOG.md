# Changelog

All notable changes to Koda.

## [Unreleased]

### Added
- **Interactive mcp-install** — smart mode detection (first run → assistant, existing config → quick reinstall, `--assistant` → force), server selector, token assistant, GitHub remote manager, `MCPServer` registry (#65)
- **Workspace profile precedence** — workspace-level profiles override global profiles (#66)
- **Kiro IDE integration** — `koda kiro-ide install/sync/remove` CLI + TUI `[k]` screen + doctor check, cross-platform with `FindNodeExe()` for fnm/nvm on Windows (#64)
- **Enterprise Memory Bank** — `services`/`channels` fields in workspace model, `InstallBanks()` merges service/channel docs, TUI shows banks, doctor checks staleness (#63)
- **memory-mcp lifecycle** — container runtime detection, lifecycle management, mcp.json integration, doctor checks (#62)
- **Rule discrimination** — workspace-level rule overrides (#61)
- **Workspace profile loading** — load profiles from workspace directories (#60)
- **Compass MCP** — remote SSE MCP support with token/URL config (#58)
- **Multi-instance GitHub MCP** — per-remote GitHub entries in mcp.json, `@github/*` tool ref expansion (#57, #59)
- **Figma MCP** — server support in mcp.json generation (#55)
- **Nested workspace folders** — support parent/child workspace hierarchy (#56)
- **Tray app** — menu bar tray with workspaces, doctor health, version info, auto-start on login, Windows Registry support (#42-#54)
- **Workspace apply** — auto-clone repos, init memory banks, resolve project paths, hierarchical extends inheritance (#38-#41)

### Fixed
- Doctor checks for multi-instance GitHub remotes (#67)
- Dashboard polish — git stderr suppression + runtime version display (#54)
- Windows tray — ICO format, embedded icon, cross-compilation build tags (#49, #50, #53)
- Tray — removed chat/TUI launch, refresh workspaces on sync (#45)

### Changed
- `MCPInstall` refactored — extracted `CopyMcpBundles()`, `GenerateMcpJson()`, `GenerateMCPConfig()` with `nodeExe` param for absolute paths on Windows (#64, #65)
- `WriteGitHubRemote` — clears stale `GITHUB_API_PATH_<name>` keys on upsert (#65)

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
