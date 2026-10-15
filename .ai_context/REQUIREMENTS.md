# DevProxy Requirements

**Version:** v1.1  
**Status:** Active  
**Last updated:** 2026-10-03

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
- `web/` — web-facing assets or dashboard implementation area.
- `docs/PROJECT_DETAILS.md` — project-specific documentation.

## 3. Technology Stack

### Current stack

- **Language:** Go.
- **Module:** `github.com/phy0hk/devproxy`.
- **Application shape:** Go command-line/server application with internal packages for proxying, process management, logging, events, and UI serving.
- **Configuration:** Existing repository configuration files and the `internal/config` package.
- **Web dashboard:** Existing or planned web UI served through the Go application; implementation details must be confirmed from the current code before extending it.

### Planned capability

- A browser-based dashboard for instance and proxy management.
- A future TUI sharing core application services with the dashboard rather than duplicating business logic.

### Frontend technology (resolved 2026-10-03)

- The embedded dashboard uses **Tailwind CSS + daisyUI** loaded from CDN in `internal/uiserver/static/index.html` (no build step; assets remain Go-embedded).
- Dashboard requires internet access for the CDN assets; a build step (e.g. Vite + daisyUI 5) may replace the CDN approach if offline use becomes a requirement.
- Instance consoles must keep process stdout/stderr separate from proxy routing telemetry.

## 4. Core Architecture

The architecture should preserve a separation between transport/UI concerns and reusable application services:

1. **Configuration layer** — loads and validates proxy and instance definitions.
2. **Process management layer** — starts, stops, tracks, and captures output from managed development processes.
3. **Proxy layer** — maps incoming requests to configured local service targets.
4. **Event layer** — communicates lifecycle and runtime state changes across components.
5. **Logging layer** — records application and managed-process output in a consistent way.
6. **HTTP/UI server layer** — exposes the dashboard and management endpoints.
7. **Future TUI layer** — consumes the same process, proxy, configuration, event, and logging services through terminal-oriented views.

The initial implementation should favor a small, understandable design with explicit interfaces around process lifecycle, proxy routing, and instance state. Persistent storage should not be introduced until the current configuration and lifecycle requirements establish a need for it.

## 5. Functional Requirements

### FR-1: Instance management

The application must support defining development service instances with enough information to identify and run them, including an instance name, command, working directory, environment/configuration, and network target where applicable.

### FR-2: Process lifecycle

Users must be able to start and stop a configured instance. The application must track whether an instance is stopped, starting, running, stopping, or failed, and must handle process termination without leaving stale state.

### FR-3: Runtime output

Users must be able to view standard output and standard error from managed frontend and backend processes. Output should be associated with the correct instance and remain useful for diagnosing local development failures.

### FR-4: Proxy routing

Users must be able to route requests received by DevProxy to one or more configured local services. Routing configuration must be simple enough for local testing and must clearly report invalid or unreachable targets.

### FR-5: Dashboard

The web dashboard must provide a simple view for inspecting configured instances, current lifecycle state, proxy configuration, and recent or live logs. It must expose the core management operations without requiring users to write Nginx configuration.

### FR-6: Configuration

The application must load configuration through the repository's established configuration mechanism, validate required fields, and return actionable errors for invalid definitions.

### FR-7: Observability and errors

The application must log meaningful application events and surface actionable errors for failed starts, failed stops, invalid routes, unavailable targets, and configuration errors.

### FR-8: TUI readiness

Core business logic must remain usable without the web dashboard so a future TUI can call the same instance, process, proxy, event, and logging services.

### FR-9: Safety and scope

DevProxy is intended for local development and testing. It must not be described or treated as a production security boundary or a full replacement for Nginx.

## 6. Non-Functional Requirements

- Keep the default workflow simple for developers.
- Prefer clear errors over silent failure.
- Avoid coupling core process and proxy logic to a specific UI.
- Keep changes compatible with the existing Go module and package organization.
- Add tests around lifecycle, routing, configuration validation, and error paths as implementation progresses.

## 7. Open Decisions

The following decisions require confirmation from the current code and product direction before implementation:

- Exact configuration schema for instances and routes.
- Whether configuration is file-only or also persisted through the dashboard.
- Log retention and streaming strategy (client-side buffers only; no server-side history endpoint yet).
- Port allocation and collision behavior.
- Authentication or access restrictions, if DevProxy may bind beyond localhost.
- TUI framework and command structure.

## 8. Version History

| Version | Date       | Change                                                                                                                   |
| ------- | ---------- | ------------------------------------------------------------------------------------------------------------------------ |
| v1.0    | 2026-10-03 | Initial requirements baseline created from the product brief and current repository structure.                           |
| v1.1    | 2026-10-03 | Resolved dashboard frontend technology (Tailwind + daisyUI via CDN); recorded log-separation rule for instance consoles. |
