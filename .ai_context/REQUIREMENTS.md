# DevProxy Requirements

**Version:** v1.2
**Status:** Active
**Last updated:** 2026-11-17

## 1. Project Objectives

DevProxy is a developer-focused local proxy application that simplifies exposing and routing multiple development services through one application. Its primary goal is to replace repetitive, complex reverse-proxy setup for local testing with a simple dashboard-driven workflow.

The application should make it easy to:

- Register and manage multiple local development service instances.
- Route frontend and backend services through a single local application.
- Start, stop, and inspect the runtime output of development instances.
- Support common development stacks such as Vite, NestJS, Go, and other HTTP-based frontend or backend processes.
- Provide a foundation for a future terminal user interface (TUI) for users who prefer terminal-based workflows.

DevProxy is intentionally focused on simplicity and developer experience rather than replacing the breadth of Nginx as a production-grade web server or reverse proxy.

## 2. Current Repository Baseline

The repository is currently a Go application with these relevant areas:

- `cmd/devproxy/` — primary application entry point.
- `cmd/devproxy-bootstrap/` — bootstrap entry point.
- `internal/config/` — configuration concerns.
- `internal/event/` — application events.
- `internal/logger/` — logging.
- `internal/process/` — managed process lifecycle and output.
- `internal/proxy/` — proxy behavior.
- `internal/proxyserver/` — proxy server concerns.
- `internal/uiserver/` — UI server concerns.
- `internal/logstore/` — bounded, run-aware in-memory process log history.
- `internal/tui/` — Bubble Tea terminal UI.
- `web/` — web-facing assets or dashboard implementation area.
- `docs/PROJECT_DETAILS.md` — project-specific documentation.

## 3. Technology Stack

### Current stack

- **Language:** Go.
- **Module:** `github.com/phy0hk/devproxy`.
- **Application shape:** Go command-line/server application with internal packages for proxying, process management, logging, events, and UI serving.
- **Configuration:** YAML `config.yaml` loaded and validated by the `internal/config` package.
- **Web dashboard:** Embedded browser UI served by the Go application, sharing the process manager and event bus with the TUI.

### Current capability

- A browser-based dashboard for instance and proxy management.
- A TUI that shares core application services with the dashboard rather than duplicating business logic.

### Frontend technology (v1.2)

- The embedded dashboard uses **vendored Tailwind CSS + daisyUI 5** in `internal/uiserver/static/vendor.css`. The CSS is generated with Bun and committed; there is no runtime CDN or build step, so the dashboard works offline.
- Dashboard state is virtualized: the SSE stream is coalesced into 100 ms batches and the log view renders only the visible row window plus overscan.
- Instance consoles keep process stdout/stderr separate from proxy routing telemetry, parse ANSI SGR/OSC sequences, and show run/clear separators.

### TUI technology (v1.2)

- Bubble Tea v1 + Lipgloss + Bubbles, using the dependencies already present in `go.mod`.
- The TUI shares the same event bus and process manager as the dashboard and keeps all state in memory.

## 4. Core Architecture

The architecture should preserve a separation between transport/UI concerns and reusable application services:

1. **Configuration layer** — loads and validates proxy and instance definitions.
2. **Process management layer** — starts, stops, tracks, and captures output from managed development processes.
3. **Log store layer** — keeps bounded per-process stdout/stderr history, run metadata, and monotonic sequences in memory.
4. **Proxy layer** — maps incoming requests to configured local service targets.
5. **Event layer** — communicates lifecycle and runtime state changes across components with resumable stream sequences.
6. **Logging layer** — records application and managed-process output in a consistent way.
7. **HTTP/UI server layer** — exposes the dashboard and management endpoints (`/api/processes`, `/api/processes/{name}/logs`, `/api/routes`, `/events`).
8. **TUI layer** — consumes the same process, proxy, configuration, event, and logging services through terminal-oriented views.

The initial implementation should favor a small, understandable design with explicit interfaces around process lifecycle, proxy routing, and instance state. All history is in memory: a host restart ends the session, and there is no persistence. Persistent storage should not be introduced until the current configuration and lifecycle requirements establish a need for it.

## 5. Functional Requirements

### FR-1: Instance management

The application must support defining development service instances with enough information to identify and run them, including an instance name, command, working directory, environment/configuration, and network target where applicable.

### FR-2: Process lifecycle

Users must be able to start and stop a configured instance. The application must track whether an instance is stopped, starting, running, stopping, or failed, and must handle process termination without leaving stale state. Stopping an instance — including on `q`/Ctrl+C shutdown or a startup error — must terminate its whole process tree, not just the shell wrapper, so no descendant dev server is orphaned.

### FR-3: Runtime output

Users must be able to view standard output and standard error from managed frontend and backend processes. Output must be associated with the correct instance and remain useful for diagnosing local development failures. The server keeps bounded in-memory history per process (default 5,000 lines, current plus previous run), exposes it through `GET /api/processes/{name}/logs`, and the dashboard restores it after a reload or reconnect. ANSI styling is rendered, and screen-clear sequences become visible separators instead of erasing the retained history.

### FR-4: Proxy routing

Users must be able to route requests received by DevProxy to one or more configured local services. Routing configuration must be simple enough for local testing and must clearly report invalid or unreachable targets.

### FR-5: Dashboard

The web dashboard must provide a simple view for inspecting configured instances, current lifecycle state, proxy configuration, and recent or live logs. It must expose the core management operations without requiring users to write Nginx configuration. It supports light/dark/system themes, run selection, visible-text filtering and copy, keyboard-accessible tabs, and a responsive layout down to mobile widths.

### FR-6: Configuration

The application must load configuration through the repository's established configuration mechanism, validate required fields, and return actionable errors for invalid definitions.

### FR-7: Observability and errors

The application must log meaningful application events and surface actionable errors for failed starts, failed stops, invalid routes, unavailable targets, and configuration errors.

### FR-8: TUI readiness

Core business logic must remain usable without the web dashboard so the TUI can call the same instance, process, proxy, event, and logging services. The TUI is enabled with `--tui` or `server.tui.enabled`, enforces a minimum terminal size, supports resize, run switching, filtering, and start/stop/restart without blocking the update loop, and shares one process manager and event bus with the dashboard.

### FR-10: Stream resumability

The event stream must batch live events (about 10 flushes per second), carry the sequence of the last delivered event as the SSE `id`, accept `Last-Event-ID` on reconnect, and replay retained history without duplicates from that point. The dashboard must not render raw ANSI escape bytes or claim conflict-free collaboration semantics.

### FR-9: Safety and scope

DevProxy is intended for local development and testing. It must not be described or treated as a production security boundary or a full replacement for Nginx.

## 6. Non-Functional Requirements

- Keep the default workflow simple for developers.
- Prefer clear errors over silent failure.
- Avoid coupling core process and proxy logic to a specific UI.
- Keep changes compatible with the existing Go module and package organization.
- Add tests around lifecycle, routing, configuration validation, and error paths as implementation progresses.
- Keep dashboard work bounded under sustained output: coalesce SSE, virtualize rows, and cap client/server buffers.

## 7. Open Decisions

The following decisions require confirmation from the current code and product direction before implementation:

- Exact configuration schema for instances and routes.
- Whether configuration is file-only or also persisted through the dashboard.
- Whether log buffer capacity and run retention should be exposed in `config.yaml` (v1.2 uses programmatic defaults only; changing the schema requires approval).
- Port allocation and collision behavior.
- Authentication or access restrictions, if DevProxy may bind beyond localhost.
- **Resolved 2026-11-16 — TUI framework:** Bubble Tea v1 + Lipgloss + Bubbles (already present in `go.mod`, so no new dependency). The TUI consumes the same process manager and event bus as the dashboard; it does not own business logic.
- **Resolved 2026-11-17 — Dashboard assets:** Tailwind + daisyUI 5 are vendored and Go-embedded; no CDN or runtime build is required.

## 8. Version History

| Version | Date       | Change                                                                                                                   |
| ------- | ---------- | ------------------------------------------------------------------------------------------------------------------------ |
| v1.0    | 2026-10-03 | Initial requirements baseline created from the product brief and current repository structure.                           |
| v1.1    | 2026-10-03 | Resolved dashboard frontend technology (Tailwind + daisyUI via CDN); recorded log-separation rule for instance consoles. |
| v1.2    | 2026-11-17 | Added server-side run-aware log history, resumable batched SSE, ANSI rendering, vendored offline Tailwind/daisyUI, active shared-services TUI, and FR-10 stream resumability. |
