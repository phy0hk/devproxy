# TLDR

## Current work

- Quit behavior fixed: `q` and Ctrl+C (plus server startup errors, SIGHUP) now stop every managed instance, including grandchildren of shell wrappers. The tracked `./devproxy` binary was stale and has been rebuilt from current source. Working tree changes are uncommitted.

```text
process stdout/stderr ──▶ manager ──▶ logstore (5,000 lines, current+previous run)
                              │
                              └──▶ event bus ──▶ SSE batch (100 ms, resumable)
                                                    ├──▶ dashboard (virtualized, ANSI)
                                                    └──▶ TUI (Bubble Tea, shared services)
```

## Done

- Run-aware in-memory log history with `GET /api/processes/{name}/logs`.
- Batched, resumable SSE (`id`, `Last-Event-ID`, 100 ms coalescing).
- Dashboard rewritten with vendored offline Tailwind/daisyUI, themes, ANSI rendering, virtualized rows, run separators, search/copy, responsive + keyboard checks.
- TUI rewritten with Bubble Tea messages, min-size/resize, run switching, filter, non-blocking actions, clean quit.
- Event bus safe unsubscribe/replay and shared bus/manager between dashboard and TUI.
- Whole-process-tree shutdown: instances run in their own process groups; `StopAll` sends SIGTERM then SIGKILL to the group, and `runUp` shuts down on every exit path.
- Verification: Go tests/vet/build, Bun ANSI tests, headless-Chromium dashboard smoke, tmux TUI smoke, CPU/SSE measurements recorded in the manifest and chronicle.

## Blockers / questions

- Log capacity/run retention are programmatic defaults; exposing them in `config.yaml` still needs approval.
- The tracked `devproxy` binary is modified in the working tree; don't commit it.
- Stopped processes exit with code `-1` (existing kill behavior).
