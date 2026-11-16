# TLDR

## Current work

- Migrated `.ai_context/` from the legacy memo layout to the v2 context structure and recorded project data in `AGENTS.md`.

## Done

- Added `RULES.md` (non-negotiable constraints).
- Created `.ai_context/memory/CHRONICLE.md`; archived legacy memos to `.ai_context/memory/archives/`.
- Added `.ai_context/CAPABILITY-MAP.md`.
- Rewrote `.ai_context/MANIFEST.md` in the v2 format.
- Filled the `AGENTS.md` project context with the project name, requirements, and related data.
- Added this `TLDR.md`.

## Structure

```text
RULES.md                     # non-negotiable constraints
TLDR.md                      # this file
AGENTS.md                    # workflow + project context
.ai_context/
├── MANIFEST.md              # active task + liveness checklist
├── REQUIREMENTS.md          # v1.1 requirements + architecture
├── CAPABILITY-MAP.md        # module boundaries + capability status
└── memory/
    ├── CHRONICLE.md         # milestone history
    └── archives/            # legacy memos (2026-10-03)
```

## Blockers / questions

- None. No application code was changed by this migration.
