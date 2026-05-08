# Koda v0.5.0 — Release Notes

**Date:** 2026-05-08

## Highlights

**Multi-prefix jira_prefix** — Workspaces can now specify multiple JIRA prefixes as an array, enabling teams that span multiple JIRA projects.

## What's New

- **Multi-prefix jira_prefix** — `StringOrSlice` type supports `string | string[]` for workspace jira_prefix (#196)

## What's Fixed

- `dev` alias now expands to all 9 dev-* profiles (#197)
