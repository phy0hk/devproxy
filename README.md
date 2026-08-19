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
- event tabs for `All`, `Proxy`, `Process`, `Stdout`, `Stderr`, `Errors`, plus one tab per configured process such as `frontend` or `backend`,
- terminal-style colored event output streamed from `/events`.

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
