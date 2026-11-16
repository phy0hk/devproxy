## Read order and scope

1. Read `RULES.md` for non-negotiable project constraints.
2. Read `.ai_context/MANIFEST.md` for the active task, files in scope, and liveness checklist.
3. Read `.ai_context/REQUIREMENTS.md` only when changing architecture, requirements, interfaces, or core conventions.
4. Read `.ai_context/memory/CHRONICLE.md` only when historical rationale is needed.
5. Read only source files, tests, and documentation relevant to the active task. Do not scan unrelated areas of the repository.

Keep `.ai_context/MANIFEST.md` current: record the diagnosis before editing, mark each file complete immediately after editing it, and leave a clear list of remaining work.

## TLDR

- On every prompt, create or update the root `TLDR.md` with the smallest possible summary of current and completed work; include a brief visual description when it communicates the work better.

## Project context

- **Project name:** DevProxy
- **Repository:** `github.com/phy0hk/devproxy` (https://github.com/phy0hk/devproxy), default branch `main`.
- **What it is:** A developer-focused local reverse proxy and process runner. A single `config.yaml` defines the services to start, the routes to proxy, and where logs and status are viewed, replacing repetitive multi-terminal local setup with one `devproxy up` command.
- **Stack:** Go (module `github.com/phy0hk/devproxy`); embedded web dashboard in `internal/uiserver/static` (Tailwind + daisyUI via CDN, no build step); TUI scaffolding in `internal/tui`; Bun is the mandated JavaScript/TypeScript runtime.
- **Scope:** Local development only. Not a production security boundary and not a replacement for Nginx, Traefik, or Caddy.
- **Requirements:** v1.1 in `.ai_context/REQUIREMENTS.md` (objectives, architecture, FR-1..FR-9, non-functional requirements, open decisions).
- **Capability map:** `.ai_context/CAPABILITY-MAP.md` (module boundaries and capability status).
- **Non-negotiable constraints:** `RULES.md`.
- **Project documentation:** `docs/PROJECT_DETAILS.md` (problem, solution, config shape, roadmap) and `README.md` (commands, bootstrapper, release workflow).

## Graphify

Treat `graphify-out/` as a navigation cache, not as a substitute for source code or tests. For repository-analysis tasks, use this order:

1. Check freshness by comparing the report's built commit with `git rev-parse HEAD`; if the working tree has code changes or the cache is stale, run `graphify update .` when the CLI is available.
2. Start with a scoped `graphify query`, `graphify explain`, or `graphify path` query to identify relevant files and relationships; use `GRAPH_REPORT.md` only for broad orientation.
3. Inspect the identified source files and tests directly before editing or validating. Record the useful query/result and any graph limitation in `.ai_context/MANIFEST.md` when the task uses Graphify.

For cross-language flows, do not infer a missing relationship from a missing graph edge. Query each language boundary separately, then verify the handoff in source and tests. If Graphify is unavailable, stale, or insufficient, continue with targeted source inspection and note that fallback. Never treat Graphify output as proof that the application is correct.

If code files are modified and the Graphify CLI is available, run `graphify update .` after the edits and record the refresh in the manifest.

## Security and collaboration constraints

- Require the unguessable session token in WebSocket invite paths.
- Validate incoming message formats and sizes at the collaboration boundary.
- Make the shared address and token explicit to users; LAN collaboration is unsuitable for untrusted networks.
- Keep session history and acknowledged-operation guarantees consistent with the requirements: the initial design is in-memory and a host restart ends the session.
- Do not describe last-arriving snapshots or host sequencing as conflict-free collaboration.

## Skill use

Use the project-local skill that matches the request. In particular:

- `token-context-optimizer` for every request, before task-specific work.
- `spec-driven-development` and `planning-and-task-breakdown` for unclear or substantial new work.
- `incremental-implementation` for changes spanning multiple files or slices.
- `test-driven-development` for behavior changes and bug fixes.
- `debugging-and-error-recovery` for failures or unexpected behavior.
- `frontend-ui-engineering` for frontend interface work.
- `api-and-interface-design` for public or cross-module contracts.
- `code-review-and-quality` before merging or when reviewing a diff.
- `graphify` when using the repository knowledge graph.
- Use bun as the JavaScript/TypeScript runtime for all frontend and backend operations.
