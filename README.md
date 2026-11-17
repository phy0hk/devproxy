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
devproxy up --config config.yaml --tui        # start the terminal UI instead
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

`--tui` starts the terminal UI instead of plain terminal logs, and both the TUI and dashboard share the same process manager and event bus. Use `--no-tui` or `--no-ui` to disable either surface.

## Terminal UI

```sh
devproxy up --tui
```

The TUI shows process state and run-aware logs side by side. It enforces a minimum of 72×16, reflows on resize, and supports:

```text
↑/↓ or j/k   move selection        s / x / r   start / stop / restart
v            switch log view       [ / ]       older / newer run
/            filter lines          c           clear the current view
?            toggle help           q / ctrl+c  quit and stop children
```

## Web UI

The UI server runs on `server.ui`. It currently provides:

- process cards with state, PID, exit code, working directory, and last error,
- start/stop/restart buttons for each configured process,
- separate `stdout`, `stderr`, `lifecycle`, and `proxy` views with run/clear separators,
- ANSI-styled, virtualized log output that restores server-side history after a reload or reconnect,
- run selection (current/previous), visible-text filtering, copy, clear-view, light/dark/system themes, and a responsive layout,
- live event batches streamed from `/events` with SSE resume support.

The UI files are embedded from:

```text
internal/uiserver/static/index.html
internal/uiserver/static/app.css
internal/uiserver/static/app.js
internal/uiserver/static/ansi.js
internal/uiserver/static/vendor.css
```

## Process Control API

```http
GET  /api/processes
GET  /api/processes/{name}/logs?run=current|previous|<id>&limit=1000&before=<seq>
POST /api/processes/{name}/start
POST /api/processes/{name}/stop
POST /api/processes/{name}/restart
GET  /api/routes
GET  /events
```

`/events` sends one `devproxy` SSE event per ~100 ms batch, carries the last sequence as `id`, and accepts `Last-Event-ID` to replay retained history without duplicates. Log history is in memory (5,000 lines per process, current + previous run) and ends when the host process exits.

## Bootstrapper

A bootstrapper command lives at `cmd/devproxy-bootstrap`.

Build it as a static, smaller binary with:

```sh
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -buildid=" -o devproxy-bootstrap ./cmd/devproxy-bootstrap
```

`CGO_ENABLED=0` is important for NixOS and other systems that cannot run generic dynamically linked Linux binaries out of the box.

When it runs, it:

1. creates `.devproxy/v1/bin`,
2. adds `.devproxy/` to `.gitignore` if missing,
3. checks whether the platform-specific DevProxy binary already exists,
4. downloads it from GitHub releases if missing,
5. verifies `checksums.txt`,
6. forwards all arguments to the real DevProxy binary.

If your environment allows scripts, you can copy one of these much smaller repo bootstrappers instead of the Go binary:

```text
scripts/bootstrap/devproxy.sh
scripts/bootstrap/devproxy.ps1
```

The scripts are tiny and download/run the real DevProxy binary the same way. If your organization blocks `.ps1` or `.sh`, use the native `devproxy-bootstrap-*` executable from the release instead; it avoids PowerShell/shell script policy restrictions. On Windows, the `.exe` does not need `chmod`.

Default release URL base for the current bootstrapper:

```text
https://github.com/phy0hk/devproxy/releases/download/v1
```

Bootstrappers are pinned to a major release channel. The current bootstrapper is pinned to `v1`, so it only downloads DevProxy binaries from the `v1` release. A future breaking `v2` would require updating the bootstrapper to a `v2` bootstrapper.

Override it with:

```sh
DEVPROXY_DOWNLOAD_BASE_URL=https://example.com/devproxy ./devproxy-bootstrap up
```

The bootstrapper verifies downloaded binaries against `checksums.txt` from the same release URL base. It also checks whether the cached `.devproxy/v1/bin/...` binary still matches the current `v1` checksum. If the `v1` release was updated, the bootstrapper automatically downloads the newer `v1` binary.

For local testing only, checksum verification can be skipped:

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

A GitHub Actions workflow lives at `.github/workflows/release.yml`. It runs on pushes to `main`/`master`, pull requests, manual `workflow_dispatch`, and semver tags like `v1.2.3`.

Behavior:

- Pull requests build and upload artifacts only.
- Pushes to `main`/`master` automatically update the `v1` release channel.
- Manual workflow runs also update the `v1` release channel.
- Semver tags like `v1.2.3` publish an exact versioned release.

The `v1` channel release is what the current bootstrapper uses. This lets projects keep the same v1 bootstrapper while receiving compatible v1 DevProxy updates automatically.

## Planned Features

- Health checks for managed processes.
- Optional configuration for log capacity and run retention.
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
- stdout/stderr process log capture with ANSI rendering,
- run-aware in-memory log history (current + previous run),
- process status API at `/api/processes` and paged logs at `/api/processes/{name}/logs`,
- batched, resumable request and process event streaming,
- offline embedded web dashboard with a virtualized log view,
- shared-services terminal UI (`devproxy up --tui`),
- CLI process control commands,
- checksum-verifying bootstrapper command under `cmd/devproxy-bootstrap`,
- GitHub release workflow,
- route matching, event bus, config, process, log store, TUI, and UI API tests.

The next recommended step is to add health checks for managed processes.

## How to Build

### Linux

```bash
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o devproxy-bootstrap-linux-amd64 ./cmd/devproxy-bootstrap
```

### Mac OS

```bash
GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o devproxy-bootstrap-mac-amd64  ./cmd/devproxy-bootstrap
```

### Window

```bash
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o devproxy-bootstrap-windows-amd64.exe ./cmd/devproxy-bootstrap
```
