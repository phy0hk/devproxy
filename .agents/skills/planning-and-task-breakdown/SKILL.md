---
name: planning-and-task-breakdown
description: Break clear requirements into ordered, verifiable implementation tasks. Use when a task is too large to start, needs scope estimates, or can be parallelized.
---

# Planning and Task Breakdown

## Overview

Decompose work into small, verifiable tasks with explicit acceptance criteria. A good task breakdown makes dependencies visible and gives each task a clear completion check.

## When to Use

- A specification or clear requirements need to be broken into implementable units.
- A task feels too large or vague to start.
- Work needs to be estimated, communicated, or parallelized.
- Implementation order is unclear.

Do not use for single-file changes with obvious scope or when the specification already provides well-defined tasks.

## Process

### 1. Plan read-only

Before writing implementation code, inspect the specification and only the relevant codebase sections. Identify existing patterns, dependencies, risks, and unknowns. Follow applicable project instructions and read protocols. Do not implement during planning.

### 2. Check existing plans before writing

Use the project's designated task tracker if its instructions specify one; otherwise use `tasks/todo.md` for the checklist. Save the explanatory plan to `tasks/plan.md`. Create `tasks/` if needed.

Before writing either file, check whether it exists and contains unchecked tasks. If it contains incomplete work for a different effort, stop and ask the user how to proceed. If it is the same effort being revised, update it in place; never overwrite incomplete work for unrelated tasks.

### 3. Map dependencies and slice work

Identify which components depend on others and order foundational work first. Prefer vertical, end-to-end slices over horizontal layers. Parallelize only independent work; identify shared contracts that must be agreed first. Include checkpoints after every two or three tasks and between major phases.

### 4. Define implementable tasks

Each task should normally be small enough for one focused session and touch no more than about five files. Split tasks that span independent subsystems, need multiple sessions, or have more than three acceptance criteria.

For every task, record:

- **Description:** What outcome the task delivers.
- **Acceptance criteria:** Specific, testable conditions (use checklist items).
- **Verification:** Relevant focused test/build commands and any manual checks.
- **Dependencies:** Task numbers or `None`.
- **Files likely touched:** Known paths, when identifiable.
- **Estimated scope:** XS (1 file), S (1–2), M (3–5), or L (5–8). Break down XL (8+) work.

Do not invent commands, paths, or requirements. Mark unknowns as open questions and ask the user when a decision is necessary to plan safely.

### 5. Produce and verify the plan

Use `tasks/plan.md` for overview, architecture decisions, ordered phases, risks, and open questions. Use the task-list target for the actionable tasks and checkpoints. If an external tracker is designated, create one item per task and record its identifiers in the plan rather than duplicating the checklist.

Before presenting the plan, confirm that dependencies are ordered, every task has acceptance criteria and verification, checkpoints are present, and no task is oversized. Report assumptions and unresolved questions. Do not begin implementation unless the user asks to proceed or project workflow explicitly authorizes it.

## Task Format

```markdown
## Task [N]: [Short descriptive title]

**Description:** [Outcome]

**Acceptance criteria:**
- [ ] [Specific, testable condition]

**Verification:**
- [ ] [Focused test/build command]
- [ ] [Manual check, if needed]

**Dependencies:** [Task numbers or None]

**Files likely touched:**
- `path/to/file`

**Estimated scope:** [XS/S/M/L]
```

## Plan Document Structure

```markdown
# Implementation Plan: [Name]

## Overview
[Summary]

## Architecture Decisions
- [Decision and rationale]

## Task List
### Phase 1: [Name]
- [ ] Task 1: ...

### Checkpoint: [Name]
- [ ] [Verification checkpoint]

## Risks and Mitigations
| Risk | Impact | Mitigation |
|---|---|---|

## Open Questions
- [Question]
```

## Verification

- Every task has explicit acceptance criteria and verification steps.
- Dependencies and implementation order are clear.
- Checkpoints exist between major phases.
- The task-list target follows project instructions and is not overwriting unrelated incomplete work.
- No task exceeds the recommended scope without further decomposition.
- Open questions and assumptions are visible to the user.
