---
name: token-context-optimizer
version: 2.0.0
description: Optimized context tracking, liveness state management, and memory compaction standard. Use when managing multi-turn coding sessions, tracking task state, enforcing project constraints, or preventing context bloat.
author: Context Engineering Architecture Group
license: MIT
compatibility: Universal (Claude Code, Gemini CLI, Cursor, Cline, OpenClaw, Codex)
allowed-tools: read_file write_file list_directory search_files execute_command
context: inline
effort: high
metadata:
  category: development
  subcategory: context-engineering
  rules-path: RULES.md
  context-dir: .ai_context
---

# Universal Context Optimization & Liveness Protocol

## Execution Standard & Model Behavioral Instructions
You are an expert AI software architect operating under a strict token-minimization and liveness-saving context protocol. You must follow the exact reading sequences, real-time state persistence rules, and final cleanup steps outlined below.

---

## 1. READ PROTOCOL (Strict Order)

To minimize token usage and avoid reading unneeded codebase files, execute reads strictly in the following sequence:

1. READ BOUNDARIES FIRST: Read RULES.md (or .ai_context/RULES.md) to establish absolute constraints, anti-patterns, and non-negotiable coding conventions.
2. READ CURRENT STATE: Read .ai_context/MANIFEST.md to check active sprint goals, completed tasks, and the immediate execution checklist.
3. READ ARCHITECTURE (Conditional): Read .ai_context/REQUIREMENTS.md ONLY if the task alters high-level system design, domain models, or external dependencies.
4. READ HISTORY (Conditional): Read .ai_context/memory/CHRONICLE.md ONLY if past architectural rationale or recent session handoffs are explicitly required for the current task.
5. READ TARGET FILES ONLY: Read ONLY the specific project files listed under Files to Touch in .ai_context/MANIFEST.md. Do NOT perform broad directory scans or read unreferenced codebase files unless required for active error diagnosis.

---

## 2. REAL-TIME LIVENESS SAVING PROTOCOL

To ensure session state is NEVER lost if API disconnects, rate limits, or context truncations occur mid-task, write state updates back to .ai_context/MANIFEST.md in real-time:

1. POST-DIAGNOSIS UPDATE: Immediately after analyzing a task or forming an execution plan, update .ai_context/MANIFEST.md with your concrete diagnosis and itemized step-by-step checklist under In-Progress Tasks.
2. POST-FILE EDIT UPDATE: Immediately after editing or creating any codebase file, update .ai_context/MANIFEST.md to mark the finished item [x], record the exact relative path modified, and present remaining items [ ].
3. RESUMPTION CHECK: Upon starting any new session, inspect .ai_context/MANIFEST.md first. If incomplete checklist items exist, pick up execution precisely from the last uncompleted item without repeating completed steps.

---

## 3. CONTEXT PRUNING AND PRE-COMPACTION RULES

Before executing context handoffs or appending to history, apply these non-generative cleanup rules:

- Tool Result Pruning: Retain only the most recent output of duplicate file reads. Replace historical read outputs with [Retained Latest File State].
- Resolved Error Purging: When a build failure or test error is fixed, replace verbose trace logs in working memory with [Resolved: <Error Type>].
- Targeted Output Truncation: Truncate raw logs or file outputs longer than 100 lines, retaining only key structural headers and bottom lines.

---

## 4. TASK COMPLETION & MEMORY COMPACTION PROTOCOL

When all itemized tasks in .ai_context/MANIFEST.md are verified complete, execute the following three finalization steps:

### STEP 4.1: APPEND TO CHRONICLE
Append a concise update entry to .ai_context/memory/CHRONICLE.md:
- Task ID & Timestamp: Record execution date and objective.
- Files Modified: List exact relative file paths changed.
- Technical Additions: Document new exports, structural changes, and key trade-offs made.
- Anti-Patterns Avoided: Note any specific constraints from RULES.md that influenced implementation decisions.

Compaction Trigger: If CHRONICLE.md exceeds 4,000 tokens, summarize older task entries into an Epoch Summary block at the top of the file, moving raw entries to .ai_context/memory/archives/.

### STEP 4.2: UPDATE REQUIREMENTS (If Architecture Changed)
If system specifications, interfaces, or technical dependencies changed during development, update .ai_context/REQUIREMENTS.md and increment the version tag (e.g., v1.0 -> v1.1).

### STEP 4.3: REGENERATE MANIFEST
Wipe .ai_context/MANIFEST.md and re-initialize it fresh for the next iteration cycle:
- Latest Chronicle Entry: Relative pointer to the newly appended section in CHRONICLE.md.
- Requirements Version: Current version from REQUIREMENTS.md.
- Current Objective: High-level target for the upcoming sprint.
- Files to Touch: Empty array or minimal initial set for the next task.
- Next Tasks: Primary checklist items for the upcoming phase.
