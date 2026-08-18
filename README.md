# DevProxy

DevProxy is a developer-focused reverse proxy and process runner.

It is meant to replace repetitive local development setup like opening multiple terminals for frontend, backend, and proxy commands. A project defines one `config.yaml`, then DevProxy starts the required services, proxies requests, and streams logs to a web UI, CLI, or optional TUI.

## Goal

```sh
devproxy up
```

Instead of:

```sh
pnpm dev
pnpm start:dev
# plus manual proxy setup
```

## Current Commands

```sh
devproxy up --config config.yaml              # start processes, proxy, and web UI
devproxy process list --config config.yaml    # list process status through the UI API
devproxy process start frontend
devproxy process stop frontend
devproxy process restart frontend
devproxy validate --config config.yaml
devproxy version
devproxy help
```

`up` is the default command, so this also works:

```sh
devproxy --config config.yaml
```

`--tui` is accepted but currently falls back to normal terminal logs while the real TUI is still planned.

## Web UI

The UI server runs on `server.ui`. It currently provides:

- process cards with state, PID, exit code, working directory, and last error,
- start/stop/restart buttons for each configured process,
- event tabs for `All`, `Proxy`, `Process`, `Stdout`, `Stderr`, and `Errors`,
- live event streaming from `/events`.

The UI files are embedded from:

```text
internal/uiserver/static/index.html
internal/uiserver/static/app.css
internal/uiserver/static/app.js
```

## Process Control API

```http
GET  /api/processes
POST /api/processes/{name}/start
POST /api/processes/{name}/stop
POST /api/processes/{name}/restart
```

## Bootstrapper

A small bootstrapper command lives at `cmd/devproxy-bootstrap`.

Build it with:

```sh
go build -o devproxy-bootstrap ./cmd/devproxy-bootstrap
```

When it runs, it:

1. creates `.devproxy/bin`,
2. adds `.devproxy/` to `.gitignore` if missing,
3. checks whether the platform-specific DevProxy binary already exists,
4. downloads it from GitHub releases if missing,
5. forwards all arguments to the real DevProxy binary.

Default release URL base:

```text
https://github.com/phy0hk/devproxy/releases/latest/download
```

Override it with:

```sh
DEVPROXY_DOWNLOAD_BASE_URL=https://example.com/devproxy ./devproxy-bootstrap up
```

The bootstrapper verifies downloaded binaries against `checksums.txt` from the same release URL base. For local testing only, checksum verification can be skipped:

```sh
DEVPROXY_SKIP_CHECKSUM=1 ./devproxy-bootstrap up
```

Expected release asset names:

```text
devproxy-linux-amd64
devproxy-linux-arm64
devproxy-darwin-amd64
devproxy-darwin-arm64
devproxy-windows-amd64.exe
devproxy-windows-arm64.exe
```

## Release Workflow

A GitHub Actions release workflow lives at `.github/workflows/release.yml`. Pushing a tag like `v0.1.0` builds Linux, macOS, and Windows binaries, generates `checksums.txt`, and publishes all assets to the GitHub release.

## Planned Features

- Reverse proxy with path-based routing.
- Project-level `config.yaml` that can be committed to each repo.
- Process manager for frontend, backend, and any other local services.
- Combined service logs.
- Web UI for service status and logs.
- Full TUI mode, for example `devproxy up --tui`.
- Real TUI mode.
- Health checks for managed processes.
- Request history storage for the UI.

## Example Config Direction

```yaml
server:
  proxy:
    host: 127.0.0.1
    port: 8082
  ui:
    host: 127.0.0.1
    port: 8081

processes:
  - name: frontend
    command: pnpm dev
    working_dir: ./frontend

  - name: backend
    command: pnpm start:dev
    working_dir: ./backend

proxy:
  targets:
    - name: frontend
      path: /
      url: http://127.0.0.1:3000

    - name: api
      path: /api
      url: http://127.0.0.1:4000
      rewrite: true
```

## Project Details

See [`docs/PROJECT_DETAILS.md`](docs/PROJECT_DETAILS.md) for the full problem statement, solution design, config shape, architecture, bootstrapper design, and MVP roadmap.

## Current Status

This project currently has:

- YAML config loading and validation,
- reverse proxy routing,
- process startup/shutdown from `processes`,
- process start/stop/restart controls,
- stdout/stderr process log capture,
- process status tracking,
- process status API at `/api/processes`,
- request and process event streaming,
- embedded web dashboard with process cards and event tabs,
- CLI process control commands,
- checksum-verifying bootstrapper command under `cmd/devproxy-bootstrap`,
- GitHub release workflow,
- route matching, event bus, config, process, and UI API tests.

The next recommended step is to add health checks for managed processes, then build the real TUI.
