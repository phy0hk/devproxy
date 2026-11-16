# DevProxy Context Manifest

## Latest Chronicle Entry

`.ai_context/memory/CHRONICLE.md` → `2026-11-16 — Context v2 migration`

## Requirements Version

v1.1 (see `.ai_context/REQUIREMENTS.md`)

## Current Objective

Context v2 migration is complete. Next objective: harden process lifecycle reliability and add server-side log history.

## Files to Touch

- `internal/event/bus.go` (fix potential goroutine leak in `replayHistory` when the subscriber channel is closed)
- `internal/uiserver/` (add a server-side log history endpoint so reconnects restore recent lines)
- `internal/uiserver/static/app.js` (per-run instance IDs to label restart boundaries)

## Next Tasks

- [ ] Fix the server-side goroutine leak in `replayHistory` when a subscriber channel is closed
- [ ] Add a server-side log history endpoint (for example `GET /api/processes/{name}/logs`)
- [ ] Add per-run instance IDs to events so restart boundaries are visible in the terminal
- [ ] Verify with tests and a live smoke test

## Completed This Session (2026-11-16 — Context v2 migration)

- [x] Create `RULES.md`
- [x] Create `.ai_context/memory/CHRONICLE.md` from the legacy memos
- [x] Move legacy memos to `.ai_context/memory/archives/`
- [x] Create `.ai_context/CAPABILITY-MAP.md`
- [x] Rewrite `.ai_context/MANIFEST.md` in the v2 format
- [x] Fill the `AGENTS.md` project context with the project name, requirements, and related data
- [x] Create root `TLDR.md`
- [x] Verify the migration (structure, links, no application code changed)

## Remaining Repository Backlog

Carried from `.ai_context/REQUIREMENTS.md` open decisions and the prior sprint:

- Add health checks for managed processes, then build out the real TUI.
- Resolve open decisions: config schema, persistence scope, port allocation, and any non-localhost access restrictions.
