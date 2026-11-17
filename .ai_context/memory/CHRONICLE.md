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

### 2026-11-17 — v1.2 completion: log history, batched SSE, dashboard and TUI

- **Objective:** Finish and verify the v1.2 work: run-aware log history, bounded/coalesced SSE, redesigned offline dashboard, and a usable shared-services TUI.
- **Files modified (this session):** `internal/logstore/store.go`, `internal/logstore/store_test.go`, `internal/process/manager.go`, `internal/process/manager_test.go`, `internal/uiserver/static/app.js`, `.ai_context/REQUIREMENTS.md`, `.ai_context/CAPABILITY-MAP.md`, `.ai_context/MANIFEST.md`, `TLDR.md`, `README.md`, `.ai_context/memory/CHRONICLE.md`.
- **Technical additions:** Log queries now sort by sequence before paging so concurrent stdout/stderr insertion order cannot skew `before`/`next_before`; output sequence assignment moved inside the manager's output lock so store order, publish order, and sequence order agree. Dashboard status now reports "showing X–Y of Z" to distinguish lines from separator entries. Requirements v1.2, capability map, and README document the log store, resumable batched SSE, offline vendored assets, and the active TUI.
- **Verification:** `go test -count=1 ./...` passed; `go vet ./...` clean; ANSI parser 9/9 Bun tests pass; `go build ./...` succeeds. Headless Chromium smoke (1280/1024/768/390 widths): no console/page errors, no horizontal overflow, keyboard tabs, search, stop/start/restart, mid-run reload history, previous-run continuity, clear separators, ANSI styling. TUI smoke in tmux at 110×32 and 60×14: min-size message, resize recovery, selection, restart, previous run, filter, `q` and Ctrl-C both stop children. At ~1,000 lines/s: SSE 53 frames/5 s (~10.6/s) and 4.87 MB/5 s; `devproxy` CPU 4.0% without an SSE client and 13.0% with one; steady-state browser TaskDuration 0.70 s over 5 s with 0 long tasks (one transient ~189 ms task during the initial 5,000-line history parse); heap stable under GC.
- **Known gap:** The manipulated processes exit with code `-1` when stopped or restarted; this is existing kill behavior, not a regression. The tracked `devproxy` binary is modified in the working tree and should not be committed.
- **Anti-patterns avoided:** No config-schema change; all history remains in memory; no new dependency; no claim of conflict-free collaboration; dashboard assets stay Go-embedded and offline.

### 2026-11-17 — Stop every managed instance on quit

- **Objective:** Make `q`/Ctrl+C (and any shutdown path) stop all running instances, including descendants of shell wrappers.
- **Files modified:** `internal/process/processgroup_unix.go` (new), `internal/process/processgroup_windows.go` (new), `internal/process/manager.go`, `internal/process/manager_processgroup_test.go` (new), `cmd/devproxy/main.go`, `.ai_context/REQUIREMENTS.md`, `.ai_context/CAPABILITY-MAP.md`, `.ai_context/MANIFEST.md`, `TLDR.md`, `.ai_context/memory/CHRONICLE.md`.
- **Diagnosis:** `shellCommand` used `sh -c <cmd>` with no `Setpgid`; `Manager.stop` cancelled the `exec.CommandContext`, killing only the direct `sh` child. Reproduced with `sleep 1000 & echo $! > pidfile; wait`: after SIGINT to the devproxy PID alone, three `sleep` grandchildren survived as orphans. Earlier tmux smoke tests masked this because session teardown SIGHUP'd the whole process group. A second leak existed in `runUp`: a server bind failure after `StartAll` returned an error without calling `srv.Shutdown`.
- **Technical additions:** Each managed command now gets its own process group on Unix (`Setpgid: true`); stop signals the whole group with SIGTERM, escalates to SIGKILL after 5 s, and defers context cancellation. Windows uses a job-free direct-child fallback behind build tags. `runUp` now funnels every exit path through a single `srv.Shutdown` call, and `signal.NotifyContext` also handles SIGHUP so a closed terminal cleans up instances.
- **Verification:** New reproduction test failed before the fix and passes after. `go test -count=1 ./...` passes, `go vet ./...` clean, `GOOS=windows go build ./...` succeeds. Live checks: SIGINT to devproxy alone left 0 orphans; pty-driven `q` and Ctrl+C each exited devproxy with 0 orphans; a deliberate port-bind failure left the process count unchanged.
- **Anti-patterns avoided:** No persistent process registry or external supervisor added; no config-schema change; no new dependency; process group ownership replaces ad-hoc PID tracking.
- **Follow-up (same day):** User reported TUI `q`/Ctrl+C still left instances running while non-TUI Ctrl+C worked. Diagnosed as two combined factors: (1) the tracked `./devproxy` binary was stale (11:14 build, before the fix), and (2) Bubble Tea uses raw mode, so TUI Ctrl+C is a key event and `q` a key — neither raises a process-group SIGINT, so both rely solely on `Shutdown` → `StopAll`; non-TUI Ctrl+C raises a real SIGINT that killed children even under the old code, masking the bug. Rebuilt the tracked binary; re-verified TUI `q`, TUI Ctrl+C, non-TUI SIGINT, and non-TUI SIGTERM all leave 0 orphans. Confirmed Bubble Tea 1.3.10 `Program.Send` is a no-op after termination, so TUI event forwarding cannot block shutdown.
