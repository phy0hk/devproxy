# DevProxy Context Manifest

## Latest Memo Pointer

`.ai_context/memos/MEMO_20261003_1235.md`

## Requirements Version

v1.1 (see `.ai_context/REQUIREMENTS.md`)

## Current Objective

Harden instance observability: give each process run a stable instance ID and add a server-side log history endpoint so dashboard reconnects can restore recent output.

## Files to Touch

- `internal/event/event.go` (add instance/run ID to `ProcessEvent`)
- `internal/process/manager.go` (assign run ID on start, attach to events)
- `internal/uiserver/api.go` (new log history endpoint + active run ID in statuses)
- `internal/uiserver/static/app.js` (consume run IDs, fetch history on load/reconnect)
- `internal/uiserver/static/index.html` (only if new UI elements are needed)

## Next Tasks

- [ ] Add per-run instance ID to process events and status payload.
- [ ] Implement `GET /api/processes/{name}/logs` (bounded ring buffer per instance).
- [ ] Label restart boundaries in the terminal with the new run IDs.
- [ ] Verify dashboard manually in a browser (tabs, clear, error toasts, reconnect).
- [ ] Decide whether to replace CDN Tailwind/daisyUI with a build step.
