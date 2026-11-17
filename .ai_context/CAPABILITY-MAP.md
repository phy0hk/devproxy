# DevProxy Capability Map

Module boundaries and capabilities for DevProxy. Read this when changing architecture, module boundaries, or core conventions. Verify against source and tests before relying on any entry here.

| Capability | Package / area | Status | Notes |
| --- | --- | --- | --- |
| CLI entrypoint and flags | `cmd/devproxy/` | Active | `up`, `process`, `validate`, `version`, `help`; `--tui`/`--no-tui`/`--no-ui` override config. |
| Bootstrapper | `cmd/devproxy-bootstrap/`, `scripts/bootstrap/` | Active | Downloads and checksum-verifies the pinned `v1` release binary, then forwards args. |
| Configuration load and validation | `internal/config/` | Active | YAML `config.yaml`: `server.proxy`, `server.ui`, `server.tui`, `processes`, `proxy.targets`. |
| Process lifecycle | `internal/process/` | Active | Start/stop/restart, status tracking, stdout/stderr capture, per-run metadata; the manager owns cancellation. Each instance runs in its own process group and stop/quit terminates the whole tree (SIGTERM → SIGKILL fallback), including grandchildren. |
| Reverse proxy routing | `internal/proxy/`, `internal/proxyserver/` | Active | Path-prefix matching with optional rewrite. |
| Event bus | `internal/event/` | Active | Bounded history, sequence-aware replay, resumable subscribers, safe unsubscribe; process events carry run ID and sequence. |
| Log store | `internal/logstore/` | Active | In-memory per-process ring (default 5,000 lines), current + previous run, paging by run/limit/before-sequence. |
| Logging | `internal/logger/` | Active | Request and process log formatting and publishing. |
| HTTP/UI server | `internal/uiserver/` | Active | Dashboard plus `/api/processes`, `/api/processes/{name}/logs`, `/api/routes`, and batched resumable `/events` (SSE); static assets are Go-embedded. |
| Dashboard | `internal/uiserver/static/` | Active | Offline vendored Tailwind + daisyUI, light/dark/system themes, virtualized ANSI log view, run separators, search/copy, responsive layout, keyboard-accessible tabs. |
| ANSI rendering | `internal/uiserver/static/ansi.js`, `internal/tui/ansi.go` | Active | SGR/256/truecolor/OSC parsing, CR overwrite, clear markers, split-chunk handling; dashboard and TUI strip hostile sequences. |
| TUI | `internal/tui/` | Active | Bubble Tea/Lipgloss/Bubbles; shares the bus and manager with the dashboard, enforces minimum size, resize-aware, run switching, filter, clean quit. |
| Health checks | `internal/process/` | Planned | Readiness checks for managed processes. |
| Persistence | — | Not planned for v1 | All history is in memory; a host restart ends the session. |

## Capability boundaries

- Core process, proxy, configuration, event, and logging capabilities must remain usable without the web dashboard (FR-8).
- The dashboard and TUI are consumers of core capabilities, not owners of business logic; `cmd/devproxy` wires one shared bus and process manager into both.
- Instance process output stays separate from proxy routing telemetry across every interface.
- SSE batches are transport framing only; they do not change the ordering or retention guarantees of the log store.
- Do not describe last-arriving snapshots or host sequencing as conflict-free collaboration, and do not describe DevProxy as a production security boundary.
