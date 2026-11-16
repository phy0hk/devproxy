# DevProxy Rules

Non-negotiable constraints for this repository. Read before making changes. If a request conflicts with a rule, stop and surface the conflict instead of working around it.

## Scope and safety

- DevProxy is a local development tool only. It must not be described or treated as a production security boundary or as a replacement for Nginx, Traefik, or Caddy.
- Do not introduce persistent storage, remote orchestration, or multi-host scheduling until the current configuration and lifecycle requirements establish a need for it.
- Keep the default developer workflow simple, and prefer clear, actionable errors over silent failure.

## Architecture boundaries

- Keep core process, proxy, configuration, event, and logging logic independent of any specific UI so the dashboard, CLI, and future TUI share the same services.
- Keep instance process output (stdout/stderr) separate from proxy routing telemetry in the event stream and in the UI.
- Keep dashboard assets Go-embedded with no required build step unless offline use becomes an explicit requirement.
- Add or update tests around process lifecycle, routing, configuration validation, and error paths whenever behavior changes.
- Do not invent unresolved product decisions or claim incomplete integrations are production-ready.

## Collaboration and network security

- Require the unguessable session token in WebSocket invite paths.
- Validate incoming message formats and sizes at the collaboration boundary.
- Make the shared address and token explicit to users; LAN collaboration is unsuitable for untrusted networks.
- Keep session history and acknowledged-operation guarantees consistent with the requirements: the initial design is in-memory and a host restart ends the session.
- Do not describe last-arriving snapshots or host sequencing as conflict-free collaboration.

## Tooling and conventions

- Use Bun as the JavaScript/TypeScript runtime for all frontend and backend operations.
- Use the existing Go module `github.com/phy0hk/devproxy` and keep package organization compatible with the current layout.
- Keep `.ai_context/MANIFEST.md` and the root `TLDR.md` current as work proceeds.
- Never report a check (test, build, lint, vet) as passing unless it was actually run successfully.
