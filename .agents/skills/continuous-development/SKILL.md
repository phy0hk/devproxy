---
name: continuous-development
description: Carry out broad feature, milestone, or product-completion requests as an ongoing, manifest-tracked engineering queue. Apply repository rules, work in verified slices, and continue independent tasks until done or genuinely blocked.
---

# Continuous Development

Use this skill when the user asks to complete a feature, milestone, product scope, or otherwise directs sustained multi-step implementation. Do not interpret a normal focused request as permission to expand its scope.

## Initialize from repository state

1. Read `RULES.md` and `.ai_context/MANIFEST.md` before implementation. Follow the repository's `AGENTS.md` workflow and applicable project skills as well.
2. Read `.ai_context/REQUIREMENTS.md` and `CAPABILITY-MAP.md` only when changing architecture, module boundaries, or core conventions. Read `.ai_context/memory/CHRONICLE.md` when historical rationale matters.
3. Inspect the relevant task/specification and source/tests only; do not scan unrelated areas. Use Graphify according to `AGENTS.md` when useful, then verify findings in source and tests.
4. Before editing implementation files, update `.ai_context/MANIFEST.md` with the diagnosis, files in scope, and an ordered, testable checklist. Resume unchecked work already recorded there when it belongs to the active request; distinguish unrelated backlog rather than silently taking it on.

## Execute in small slices

- Work on the next independently verifiable checklist item. Keep changes minimal, respect capability boundaries, and do not invent unresolved product decisions or claim incomplete integrations are production-ready.
- Use the repository-mandated runtime and commands from `AGENTS.md`/`RULES.md`; for this repository, use Bun for frontend/backend operations. Select the narrowest relevant check first, then run required project-wide checks before declaring the requested scope complete.
- Add or update focused tests for behavior changes. Treat test, typecheck, lint, and other required check output as the verification record; never report a check as passing unless it was run successfully.
- Immediately after each file edit, update `.ai_context/MANIFEST.md` to mark that file/task complete and list remaining work. Keep the root `TLDR.md` concise and current, with completed work and a dedicated questions/blockers section when relevant.
- If code files change and Graphify is available, refresh it with `graphify update .` and record the result/limitations in the manifest.
- At meaningful verified milestones, append a concise factual entry to `.ai_context/memory/CHRONICLE.md`. Update requirements/version only when requirements or architecture actually change.

## Blockers, progress, and context

- Continue safe, independently actionable work while decisions or external services block other items. Record the exact unresolved decision, external dependency, and affected acceptance criteria in the manifest and `TLDR.md`; ask the user only when further progress genuinely requires input.
- Do not claim overall completion while acceptance criteria or required verification remain open. If a required check fails, record the exact command and relevant failure, fix issues caused by the change, and rerun the check.
- Before a context handoff or when the task grows substantially, persist the current diagnosis, completed files, validation results, blocker, and next exact step in `.ai_context/MANIFEST.md`. A new context should resume from that state without repeating completed work.
- Stop when the requested scope is verified complete, a genuine blocker prevents further safe progress, or the user asks to stop. Leave the manifest clear about remaining repository backlog.

## Runtime limitations

This skill is an instruction set, not an execution controller. Do not claim that Zed automatically runs a Stop hook, blocks final responses, rotates the model context, or enforces a continuation-count limit. Use available tools and persisted repository state to support continuity; report limitations honestly.
