# DevProxy Context Manifest

## Latest Memo Pointer

`.ai_context/memos/MEMO_20261003_1235.md`

## Requirements Version

v1.1 (see `.ai_context/REQUIREMENTS.md`)

## Current Objective

Fix event listener leaks and improve performance of the proxy app.

## Files to Touch

- `internal/uiserver/static/app.js` (fix event listener accumulation in renderProcesses, renderTabs)
- `internal/event/bus.go` (fix potential goroutine leak in replayHistory when channel is closed)
- `internal/uiserver/event.go` (verify SSE cleanup)

## Next Tasks

- [x] Fix client-side event listener accumulation in `renderProcesses()` - use event delegation
- [x] Fix client-side event listener accumulation in `renderTabs()` - use event delegation
- [ ] Fix server-side goroutine leak in `replayHistory` when subscriber channel is closed
- [ ] Verify all fixes with tests and manual testing
- [ ] Create memo documenting the fixes
