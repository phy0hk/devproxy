# DevProxy Capability Map

Module boundaries and capabilities for DevProxy. Read this when changing architecture, module boundaries, or core conventions. Verify against source and tests before relying on any entry here.

| Capability | Package / area | Status | Notes |
| --- | --- | --- | --- |
| CLI entrypoint and flags | `cmd/devproxy/` | Active | `up`, `process`, `validate`, `version`, `help`; `--tui` is accepted but falls back to terminal logs. |
| Bootstrapper | `cmd/devproxy-bootstrap/`, `scripts/bootstrap/` | Active | Downloads and checksum-verifies the pinned `v1` release binary, then forwards args. |
| Configuration load and validation | `internal/config/` | Active | YAML `config.yaml`: `server.proxy`, `server.ui`, `processes`, `proxy.targets`. |
| Process lifecycle | `internal/process/` | Active | Start/stop/restart, status tracking, stdout/stderr capture; the manager owns cancellation. |
| Reverse proxy routing | `internal/proxy/`, `internal/proxyserver/` | Active | Path-prefix matching with optional rewrite. |
| Event bus | `internal/event/` | Active | Shared stream for lifecycle, output, and proxy telemetry. |
| Logging | `internal/logger/` | Active | Request and process log formatting and publishing. |
| HTTP/UI server | `internal/uiserver/` | Active | Dashboard plus `/api/processes` and `/events` (SSE); static assets are Go-embedded. |
| TUI | `internal/tui/` | Partial | Terminal UI scaffolding exists; `--tui` still falls back to terminal logs. |
| Health checks | `internal/process/` | Planned | Readiness checks for managed processes. |
| Server-side log history | `internal/uiserver/` | Planned | Restore recent log lines after a reconnect. |

## Capability boundaries

- Core process, proxy, configuration, event, and logging capabilities must remain usable without the web dashboard (FR-8).
- The dashboard and any future TUI are consumers of core capabilities, not owners of business logic.
- Instance process output stays separate from proxy routing telemetry across every interface.
