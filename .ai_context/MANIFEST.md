# DevProxy Context Manifest

## Latest Chronicle Entry

`.ai_context/memory/CHRONICLE.md` → `2026-11-17 — Stop every managed instance on quit`

## Requirements Version

v1.2 (active; see `.ai_context/REQUIREMENTS.md`)

## Current Objective

DevProxy v1.2: server-side run-aware log history, bounded/coalesced SSE with a redesigned offline dashboard, and a usable, resize-aware TUI that shares core services with the dashboard.

## Active Task — Stop all instances on quit (2026-11-17)

### Diagnosis (recorded before edits)

- User wants every managed instance stopped when quitting via `q` or Ctrl+C.
- Reproduced with `sleep 1000 & sleep 1000` and `sh -c "sleep 1000 & wait"` commands in a `setsid` session; sending SIGINT to only the devproxy PID left 3 orphaned `sleep 1000` grandchildren alive (reparented to PID 1, still in devproxy's old PGID).
- Root cause: `shellCommand` uses `sh -c <cmd>` with no `Setpgid`; `Manager.stop` cancels the `exec.CommandContext`, which kills only the direct `sh` child. Grandchildren (the real dev servers spawned by wrappers like `npm run dev`) survive.
- Secondary leak: in `cmd/devproxy/main.go` `runUp`, the `serverErr` early-return path returns without `srv.Shutdown`, so a bind failure after `StartAll` leaves processes running.
- tmux smoke tests masked the bug because session teardown SIGHUP'd the whole process group.
- Chosen fix: start each child in its own process group (`SysProcAttr{Setpgid: true}` on Unix) and terminate the whole group (SIGTERM → 5s → SIGKILL) in `Manager.stop`; guard `runUp`'s error path with `Shutdown`.

### Checklist

- [x] Write failing reproduction test in `internal/process/manager_processgroup_test.go` (orphaned grandchild survives StopAll); confirmed RED before fix, GREEN after.
- [x] Add platform-specific process-group helpers (`internal/process/processgroup_unix.go`, `processgroup_windows.go`).
- [x] Wire helpers into `Manager.Start` and `Manager.stop`.
- [x] Ensure `runUp` calls `srv.Shutdown` on every exit path that started processes.
- [x] Run focused tests, full `go test ./...`, `go vet ./...`, Windows cross-build; re-run live SIGINT/pty `q`/Ctrl+C reproductions.
- [x] Update TLDR, CHRONICLE, requirements/capability map; Graphify refreshed (`graphify update .` → 970 nodes, 1,998 edges, 44 communities across 67 files).

### Verification evidence (this task)

- RED: `TestManagerStopAllKillsGrandchildren` failed with "grandchild process ... is still alive after StopAll" before the fix; GREEN in 0.01 s after.
- `go test -count=1 ./...` all packages pass; `go vet ./...` exit 0; `GOOS=windows GOARCH=amd64 go build ./...` exit 0.
- Live: SIGINT to devproxy PID only → 0 orphaned `sleep` processes (previously 3). Pty-driven `q` and Ctrl+C in `--tui` mode each exited devproxy 0 with 0 remaining instances. Deliberate UI/proxy port-bind failure left the running-instance count unchanged. SIGHUP added to `signal.NotifyContext` and live-verified: 5 → 0 instances.
- Follow-up report: TUI quit still appeared broken while non-TUI Ctrl+C worked. Root cause of the *difference*: Bubble Tea runs the terminal in raw mode (ISIG off), so TUI Ctrl+C is a key event and `q` is a key — neither generates a process-group SIGINT, so both depend entirely on `Shutdown` → `StopAll`. Non-TUI Ctrl+C generates a real SIGINT to the whole foreground group, which killed children even with the old buggy `StopAll`. The tracked `./devproxy` binary was stale (built 11:14, before the fix); rebuilt from current source. All four paths re-verified with the rebuilt binary: TUI `q`, TUI Ctrl+C, non-TUI SIGINT, non-TUI SIGTERM → 0 orphans each. `program.Send` is a no-op after termination (Bubble Tea 1.3.10 `tea.go:774`), so the TUI event goroutine cannot block shutdown.

### Verified baseline

- `git rev-parse HEAD`: `f0b2a977db7d2c7557591333d8b0a388af60045c` (branch `main`, clean tree).
- Graphify CLI is now installed (`graphify 0.9.101`) and `graphify-out/` was generated with `graphify update .` (AST-only; 845 nodes, 1,580 edges, 44 communities across 57 files). Graph queries are available; source/tests inspection remains the source of truth.
- `go test ./...` baseline already fails for two pre-existing reasons unrelated to this task:
  - `internal/config/config_test.go:140` uses `ServerAddress` where `UIConfig` is required (test compile error).
  - `cmd/devproxy-bootstrap` expects invalid `version:` YAML to fall back to empty; the small parser returns `[broken`.
  These will be noted separately from v1.2 test results.

### CPU / event-rate baseline (current code)

Method: built `cmd/devproxy` to `/tmp/opencode/devproxy-bench/devproxy-baseline`, ran a process emitting ~1,000 stdout lines/s in 100-line bursts, sampled `/proc/<pid>/stat` over 5 s, and streamed `/events` with `curl -N --max-time 5`.

| Metric | Baseline |
| --- | --- |
| Idle CPU (no process output, no SSE client) | 0.00% over 5 s |
| CPU with noisy process, no SSE client | 0.60% over 5 s |
| SSE events received in 5 s (one client) | 3,185 (~637/s; bus also drops under bursts) |
| SSE bytes in 5 s | 518,822 |
| SSE frames/flushes in 5 s | ~3,186 (~637 flushes/s) |
| RSS while noisy | ~14.3 MB |

### Root causes confirmed in source

1. **Dashboard client**: `internal/uiserver/static/app.js` calls `renderOutput()` on every `devproxy` SSE event. `renderOutput()` rebuilds the tab strip with `innerHTML`, rebuilds the entire terminal with `innerHTML` from up to 400 buffered lines, and forces `scrollTop = scrollHeight`. At ~637 events/s that is ~637 full DOM rebuilds/s, matching the reported freeze. There is no `requestAnimationFrame`, no batching, no dirty-component tracking, and no hidden-tab pause.
2. **SSE server**: `internal/uiserver/event.go` writes and flushes one HTTP chunk per event; no coalescing. At 1,000 lines/s this creates roughly 1,000 flushes/s per client.
3. **No history**: `internal/process/manager.go` publishes output only; nothing retains per-process lines. A reload/late client starts at live output. `internal/uiserver/static/app.js` keeps only 400 client-side lines.
4. **No runs/sequence**: `event.ProcessEvent` has no run ID or sequence. Restart boundaries cannot be distinguished.
5. **Bus leak/race**: `internal/event/bus.go` starts `replayHistory` in a goroutine and closes subscriber channels in `Unsubscribe`; `Publish` can send on a closed channel, and replay happens asynchronously. History replay is also oldest-first, so a slow subscriber buffer fills with stale events.
6. **TUI is disconnected**: `cmd/devproxy/main.go` creates one bus/manager for the TUI and calls `server.New(cfg)`, which creates a second bus/manager. The TUI therefore manages different process instances than the dashboard. `internal/tui/tui.go`'s `listenEvents` command mutates a copied model from inside a blocking goroutine instead of sending messages to Bubble Tea's update loop, so the UI never re-renders from events. Layout math is fixed-width, no minimum-size message, and start/stop/restart mutate state from goroutines without a safe message path.

### Chosen design

- Add `internal/logstore`: bounded per-process ring buffer (default 5,000 lines), run metadata (default retain current + previous run), monotonic sequence lookup, and paging by run / limit / before-sequence. Programmatic `Options` only; **no config-schema change** (requires approval).
- Add `RunID` and `Sequence` to process events. Assign a new run ID on every successful start; tag output and lifecycle events.
- Add `GET /api/processes/{name}/logs?run=current|previous|<id>&limit=<n>&before=<seq>`.
- Make SSE resumable and batched: `id: <sequence>`, honour `Last-Event-ID`, replay bus history after that sequence without duplicates, coalesce live events on a 100 ms ticker per client.
- Fix the bus by removing the async replay goroutine, guarding subscriber sends/closes with a mutex, copying snapshots instead of mutating them, and replaying history in sequence order.
- Refactor `internal/uiserver` so the caller can inject the shared bus and process manager; `cmd/devproxy` uses one set of services for proxy, dashboard, and TUI.
- Rewrite the dashboard with vendored Tailwind + daisyUI 5 CSS (generated with Bun, committed; no runtime build), light/dark/system theme, status actions, run separators, stream filter, search, copy, clear-view, route table, mobile layout.
- Rewrite the TUI with the existing Bubble Tea/Lipgloss/Bubbles stack (already present in `go.mod`; no new dependency), event commands instead of model mutation from goroutines, min-size guard, resize-aware layout, run switching, filter, and clean shutdown.
- Keep everything in memory. No persistence, no non-localhost auth change, no shadcn/ui switch.

## Files to Touch

- `internal/event/event.go`, `internal/event/id.go`, `internal/event/bus.go`, `internal/event/bus_test.go`
- `internal/logstore/store.go`, `internal/logstore/store_test.go` (new)
- `internal/process/manager.go`, `internal/process/manager_test.go`
- `internal/uiserver/api.go`, `internal/uiserver/event.go`, `internal/uiserver/server.go`, `internal/uiserver/event_test.go`, tests
- `internal/uiserver/static/{index.html,app.js,app.css,ansi.js,vendor.css}` (assets Go-embedded) + `internal/uiserver/ansi.test.js`
- `internal/tui/{tui.go,tui_test.go,keys.go,styles.go,layout.go,layout_test.go,ansi.go,ansi_test.go}`
- `cmd/devproxy/main.go`
- `.ai_context/{REQUIREMENTS.md,CAPABILITY-MAP.md,MANIFEST.md}`, `TLDR.md`, `README.md`, `.ai_context/memory/CHRONICLE.md`

## Continuation Verification (2026-11-16, second session)

- Read `RULES.md`, `.ai_context/MANIFEST.md`, `TLDR.md`, `REQUIREMENTS.md`, `CAPABILITY-MAP.md`.
- Graphify CLI is now installed but `graphify-out/` was absent at resume; fallback to targeted source/tests inspection. `graphify update .` must run after edits.
- Working tree contains uncommitted v1.2 work from the previous session (`git status`, 17 modified files + `internal/logstore/`, `internal/uiserver/event_test.go`, `vendor.css`).
- Test baseline at resume: `internal/event`, `internal/logstore`, `internal/uiserver` pass; `internal/process` had one flaky timeout (`TestManagerStartEventsCarryRunAndSequence`, passed on isolated/count reruns); two pre-existing unrelated failures remain (`internal/config` test compile typo, bootstrap invalid-YAML fallback).
- Verified complete: Slice A (A1–A6), server-side SSE coalescing (B1), dashboard redesign skeleton + vendored Tailwind/daisyUI (`vendor.css`), theme toggle markup, shared services wiring in `cmd/devproxy/main.go`, Bubble Tea/Lipgloss decision recorded in REQUIREMENTS.
- Not complete: ANSI parsing/rendering (no code anywhere; bug persists), true batched SSE payloads (currently one frame per event per flush), run separators/previous-run continuity in the dashboard, performance re-measurement, TUI rewrite (`listenEvents` still mutates a copied model from a blocking goroutine, `mu` is nil, no ANSI sanitising, no min-size/resize correctness tests).
- No code was rewritten in this verification step.

## Continuation Verification (2026-11-17, third session)

- Full suite now green: `go test ./...` passes (including the two previously failing config/bootstrap tests), `go vet ./...` clean, `bun test internal/uiserver/ansi.test.js` 9/9 pass.
- Confirmed complete by inspection: D2 pure layout/key/sanitizer tests (`internal/tui/{layout,ansi,tui}_test.go`), C5 inline pre-paint theme in `index.html`.
- Dashboard smoke via headless Chromium (Playwright, 1280×900 + 1024/768/390 widths): no console/page errors, connected SSE, ANSI styled spans, 32 virtualized rows/296 DOM nodes at ~1,000 lines/s, no horizontal overflow, keyboard tab activation works, search filters, dark theme persists across reload with no flash, system theme removes the attribute.
- New SSE measurement at ~1,000 stdout lines/s, 5 s one curl client: 53 frames (was ~3,186), 4,865,081 bytes (no drops; sequences continuous).
- CPU of `devproxy` at ~1,000 lines/s (corrected ticks formula): idle/no SSE client 4.0% (was 0.6% before server-side history), one SSE client 13.0%.
- Steady-state browser after history settles (5 s observation): 0 long tasks, TaskDuration 0.70 s, ScriptDuration 0.33 s, LayoutDuration 0.07 s; JS heap stable (GC reclaimed allocations). Immediately after first history load there is a transient ~189 ms longest task while 5,000 ANSI lines are parsed once; steady state is clean.
- UI wording fix: log status now labels virtual-scroll positions as entries (they include run/clear separators) instead of rows.
- Graphify refreshed after edits: `graphify update .` → 954 nodes, 1,926 edges, 46 communities across 64 files (AST-only; 37/64 cache hits).

## Remaining Tasks (updated)

### Slice A — finish/validate history

- [x] A1–A6 verified done.
- [x] A7: Added no-gap/no-duplicate batched SSE resume tests (`event_test.go`).
- [x] A8: Live log view now loads current + previous run and renders run/clear separators.

### Slice B — ANSI + SSE batching + dashboard performance

- [x] B4: `static/ansi.js` parser (SGR, 256/truecolor, OSC, CSI clear/movement/K, CR overwrite, split chunks, hostile input) + 9 Bun tests in `internal/uiserver/ansi.test.js`.
- [x] B5: SSE now writes one JSON array frame per 100 ms flush with the last sequence as `id`; resume/coalesce tests updated.
- [x] B6: `app.js` renders styled spans via createElement/textContent, run/clear separators, visible-text search/copy, capped pending queue.
- [x] B7: Measured after-change CPU/event rate + browser-side task/memory checks (3rd session notes above); clarified line/entry count label in `renderTerminal`.

### Slice C — dashboard polish

- [x] C4: Headless-Chromium smoke at 1280/1024/768/390 widths: no console errors, no horizontal overflow, keyboard tabs, search, stop/start/restart, reload history, previous run, clear separators, ANSI styling, no raw escapes.
- [x] C5: Inline pre-paint theme selection (system default, persisted choice, no flash); dark/system toggles verified across reload.

### Slice D — TUI

- [x] D1: Replaced `internal/tui` update loop with Bubble Tea messages (`p.Send`), no model mutation from goroutines; added `ansi.go`, `layout.go`, `separatorStyle`.
- [x] D2: Pure layout/key/filter/sanitizer helpers + tests at several terminal sizes.
- [x] D3: Resize-aware layout, min-size message, run switching, filter, non-blocking actions, clean quit, Ctrl-C stops children (live-verified in tmux).
- [x] D4: Live tmux smoke: 110×32 render, 60×14 min-size message, resize back, j/k selection, restart to run-3, `[` previous run, `/` filter, `q` and Ctrl-C both stop children and close listeners.

### Slice E — Documentation + verification

- [x] E1: `go test ./...`, `go vet ./...`, `bun test internal/uiserver/ansi.test.js` all pass.
- [x] E2: Live smoke at ~1,000 lines/s with ANSI clears: mid-run reload restores 5,000 lines, restart boundary shows current+previous runs, TUI resize/quit, CPU recorded above.
- [x] E3: Updated `TLDR.md`, `CHRONICLE.md`, `CAPABILITY-MAP.md`, `REQUIREMENTS.md` v1.2, `README.md`.
- [x] E4: Final report delivered (changes, tests, perf, gaps, open decisions).

## Completed This Session (2026-11-16 — v1.2)

- [x] Read RULES, MANIFEST, REQUIREMENTS, CAPABILITY-MAP.
- [x] Recorded diagnosis + baseline CPU/event measurements above.
- [x] Resumed previous session, verified existing work against tests, recorded gaps.

## Completed This Session (2026-11-17 — v1.2 finish)

- [x] Verified all tests/vet/build/Bun and dashboard/TUI live behavior.
- [x] Fixed log paging order for concurrent stdout/stderr (`sort` by sequence) + regression test; moved output sequence assignment inside the manager output lock.
- [x] Clarified dashboard status wording (`showing X–Y of Z`).
- [x] Raised `waitForLogCount` test deadline to 10 s to remove the pre-existing full-suite flake.
- [x] Updated requirements v1.2, capability map, README, TLDR, chronicle, and manifest.
- [x] Recorded CPU, SSE, browser-task/memory, dashboard, and TUI smoke evidence above.

## Remaining Repository Backlog

- Health checks for managed processes.
- Open decisions: config schema, persistence scope, port allocation, non-localhost access restrictions, whether to expose log capacity/retention in `config.yaml`.
