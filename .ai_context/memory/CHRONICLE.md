# DevProxy Chronicle

Append-only record of verified milestones and their rationale. Newest entries at the bottom. When this file exceeds roughly 4,000 tokens, summarize older entries into the Epoch Summary and move raw entries to `.ai_context/memory/archives/`.

## Epoch Summary

- **2026-10-03 — Context baseline.** Created `.ai_context/` with `MANIFEST.md`, `REQUIREMENTS.md` (v1.0), and `memos/`. No core code changed.
- **2026-10-03 — Process lifecycle + dashboard.** Detached process commands from the request context, made restart work after exit, isolated stdout/stderr from proxy telemetry, and shipped a local-only daisyUI/Tailwind dashboard (no build step). Added restart regression tests.
- **2026-10-03 — Restart/log/UI hardening.** Root-caused the UI start/restart failure to request-context cancellation, added ownership checks in `stop`/`wait`, fixed SSE named-event handling, and documented the event contract. Full `go test ./...` passed.
- **2026-11-16 — Context v2 migration.** Migrated `.ai_context/` from the memo layout to the v2 structure (`RULES.md`, `memory/CHRONICLE.md`, `CAPABILITY-MAP.md`, `TLDR.md`). No application code changed.

## Entries

### 2026-10-03 11:43 — Initial project setup (MEMO_20261003_1143)

- **Objective:** Establish the context and memory baseline for DevProxy.
- **Files modified:** `.ai_context/MANIFEST.md`, `.ai_context/REQUIREMENTS.md`, `.ai_context/memos/MEMO_20261003_1143.md`.
- **Technical additions:** Requirements v1.0 (objectives, stack, architecture, FR-1..FR-9, non-functional requirements, open decisions).
- **Notes:** Repository baseline inspected; no core application code was written.

### 2026-10-03 12:14 — Process lifecycle, dashboard, log separation (MEMO_20261003_1214)

- **Objective:** Restart reliability, instance output isolation, and the embedded dashboard.
- **Files modified:** `internal/process/manager.go`, `internal/process/manager_test.go`, `internal/uiserver/static/index.html`, `internal/uiserver/static/app.css`, `internal/uiserver/static/app.js`, `.ai_context/MANIFEST.md`.
- **Technical additions:** Process commands detached from the starting request context; restart works after natural exit and after explicit stop; waiters only clear the instance they own; `process.output` carries an explicit `stream` field; local-only daisyUI/Tailwind dashboard with no new runtime dependency.
- **Verification:** `go test ./internal/process` passed.

### 2026-10-03 12:35 — Restart fix, log separation, daisyUI/Tailwind dashboard (MEMO_20261003_1235)

- **Objective:** Fix instance restart, isolate per-instance logs, and finalize the dashboard UI framework.
- **Files modified:** `internal/process/manager.go`, `internal/process/manager_test.go`, `internal/uiserver/static/index.html`, `internal/uiserver/static/app.js`, `internal/uiserver/static/app.css`, `.ai_context/MANIFEST.md`.
- **Technical additions:** `Start` owns cancellation via `context.Background()`; `Restart` handles both live and already-exited instances; ownership checks in `stop`/`wait`; SSE consumer uses `addEventListener("devproxy", ...)`; views for stdout/stderr/lifecycle/proxy; client buffers capped at 400 lines (no server-side history endpoint yet).
- **Verification:** `go test -count=1 ./...`, `go vet ./internal/process ./internal/uiserver`, `node --check internal/uiserver/static/app.js`, plus a live smoke test.
- **Recommendations:** Server-side log history endpoint; per-run instance IDs; optional Vite build step if CDN usage is unacceptable.

### 2026-11-16 — Context v2 migration

- **Objective:** Migrate `.ai_context/` from the legacy memo layout to the v2 context structure and record the project name, requirements, and related data in `AGENTS.md`.
- **Files modified:** `RULES.md` (new), `.ai_context/memory/CHRONICLE.md` (new), `.ai_context/memory/archives/*` (moved legacy memos), `.ai_context/CAPABILITY-MAP.md` (new), `.ai_context/MANIFEST.md` (rewritten), `AGENTS.md` (project context), `TLDR.md` (new).
- **Technical additions:** Legacy memos archived under `.ai_context/memory/archives/`; requirements v1.1 carried forward unchanged.
- **Anti-patterns avoided:** No application code touched; no new runtime dependency; dashboard assets remain Go-embedded.
