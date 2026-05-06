# Koda — Kiro IDE Integration

This guide covers the `koda kiro-ide` commands that install and sync steering files, skills,
agents, prompts, context, rules, and hooks for use with Kiro IDE.

## Overview

Kiro IDE reads configuration from two locations:

| Level     | Path                    | Contents                              |
|-----------|-------------------------|---------------------------------------|
| User      | `~/.kiro/`             | Steering files, skills, MCP config    |
| Workspace | `<project>/.kiro/`     | Agents, prompts, context, rules, hooks |

`koda kiro-ide` manages both levels, sourcing content from your steer-runtime profiles.

## Commands

### `koda kiro-ide install [workspace-dir]`

Full installation of all Kiro IDE configuration.

```bash
# User-level only (steering + skills)
koda kiro-ide install

# User-level + workspace-level (agents, prompts, context, rules, hooks)
koda kiro-ide install /path/to/project

# Specific profiles only
koda kiro-ide install --profiles dev-core,dev-web /path/to/project
```

**What it does:**

1. Resolves which profiles to install (explicit `--profiles` > active workspace > all discovered)
2. Installs steering files to `~/.kiro/steering/`
3. Installs skills to `~/.kiro/skills/`
4. If workspace-dir provided:
   - Copies agents (with path rewriting) to `<project>/.kiro/agents/`
   - Copies prompts to `<project>/.kiro/prompts/`
   - Copies context files to `<project>/.kiro/context/`
   - Installs workspace rules to `<project>/.kiro/rules/`
   - Generates default hooks in `<project>/.kiro/hooks/`
5. Copies MCP server bundles and generates `mcp.json`
6. Removes orphaned files from previously installed profiles

### `koda kiro-ide sync [workspace-dir]`

Updates installed files from the latest profile sources. Same as install but skips hooks and MCP
(those are static after initial setup).

```bash
koda kiro-ide sync
koda kiro-ide sync /path/to/project
koda kiro-ide sync --profiles dev-core
```

Stale files from profiles that are no longer selected are automatically cleaned up.

### `koda kiro-ide list`

Lists available profiles and shows the active workspace context.

```bash
koda kiro-ide list
```

### `koda kiro-ide remove [workspace-dir]`

Removes all generated `.kiro` content from a workspace directory (hooks, agents, prompts, context).

```bash
koda kiro-ide remove /path/to/project
```

## Profile Resolution

Profiles are resolved in this priority order:

1. **Explicit `--profiles` flag** — highest priority
2. **Active workspace profiles** — from `koda workspace apply`
3. **All discovered profiles** — fallback when nothing else is configured

Use the `dev` alias to expand to all dev profiles (`dev-core`, `dev-web`, `dev-mobile`, etc.).

## Orphan Cleanup

When you change workspaces or narrow your profile selection, files from previously installed
profiles are automatically removed on the next `install` or `sync`. This prevents stale steering
files from accumulating and confusing Kiro IDE.

**What gets cleaned:**

- `~/.kiro/steering/*.md` — steering files not in any selected profile
- `~/.kiro/skills/` — skill files and directories not in any selected profile
- `<project>/.kiro/agents/*.json` — agents not in any selected profile
- `<project>/.kiro/prompts/*.md` — prompts not in any selected profile
- `<project>/.kiro/context/` — context files not from shared, profiles, or workspace banks

**Observability:** Removed files are logged to stdout (unless running in quiet/TUI mode):

```
🔄 Syncing Kiro IDE config (stale files from removed profiles will be cleaned)
  removing orphan: old-dotnet-rules.md
  removing orphan: legacy-skill.md
  cleaned 2 orphaned steering file(s)
✅ Synced: 5 steering, 3 skills
```

## User-Managed Files

If you want to add custom steering files, skills, or context that koda should never touch, prefix
them with `local-`:

```
~/.kiro/steering/local-my-team-rules.md       ✅ preserved on sync
~/.kiro/steering/local-project-conventions.md  ✅ preserved on sync
~/.kiro/steering/my-custom-rules.md            ❌ removed on next sync
```

The `local-` prefix convention applies to all directories managed by orphan cleanup:

- `~/.kiro/steering/local-*.md`
- `~/.kiro/skills/local-*.md`
- `<project>/.kiro/agents/local-*.json`
- `<project>/.kiro/prompts/local-*.md`
- `<project>/.kiro/context/local-*`

## Workspace-Level .gitignore

Install automatically adds these entries to your project's `.gitignore`:

```
.kiro/agents/
.kiro/prompts/
.kiro/context/
.kiro/hooks/
```

These directories contain generated content that should not be committed.

## Typical Workflows

### First-time setup

```bash
koda setup                              # Install dependencies
koda workspace apply my-workspace       # Activate a workspace
koda kiro-ide install /path/to/project  # Full install
```

### After switching workspaces

```bash
koda workspace apply new-workspace
koda kiro-ide sync /path/to/project     # Updates files, removes stale ones
```

### After pulling steer-runtime updates

```bash
koda sync                               # Pull latest steer-runtime
koda kiro-ide sync /path/to/project     # Propagate to Kiro IDE files
```

### Adding personal steering

```bash
# Create a local file that won't be removed on sync
echo "# My Rules" > ~/.kiro/steering/local-my-rules.md
```
