# Graph Report - devproxy  (2026-11-17)

## Corpus Check
- 67 files · ~58,126 words
- Verdict: corpus is large enough that graph structure adds value.
- Unclassified: 8 file(s) not represented in the graph (top: (none) 5, .css 2, .toml 1)

## Summary
- 970 nodes · 1998 edges · 44 communities (38 shown, 6 thin omitted)
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 180 edges (avg confidence: 0.86)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `f0b2a977`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- devproxy/main.go
- testing.T
- app.js
- devproxy-bootstrap/main.go
- Bus
- testing.T
- Code Review and Quality
- Test-Driven Development
- steps
- store_test.go
- DevProxy Context Manifest
- ansi.js
- API and Interface Design
- Frontend UI Engineering
- Incremental Implementation
- Code Simplification
- Debugging and Error Recovery
- 5. Functional Requirements
- processes
- processes
- Config
- DevProxy Project Details
- Spec-Driven Development
- Planning and Task Breakdown
- DevProxy
- Universal Context Optimization & Liveness Protocol
- Entries
- AGENTS.md
- MVP Roadmap
- Continuous Development
- Initial Project Setup Memo
- DevProxy Rules
- Sprint Memo — Process Lifecycle, Dashboard, and Log Separation
- Bootstrapper Design
- Config Sections
- Example User Experience
- Ponytail: Lazy Senior Dev Mode
- How to Build
- DevProxy Capability Map
- fetch
- github.com/phy0hk/devproxy
- KeyMap
- netError

## God Nodes (most connected - your core abstractions)
1. `Model` - 44 edges
2. `Manager` - 36 edges
3. `Bus` - 30 edges
4. `NewManager()` - 23 edges
5. `readyModel()` - 20 edges
6. `DevProxy Project Details` - 20 edges
7. `Code Review and Quality` - 19 edges
8. `scheduleFrame()` - 18 edges
9. `Config` - 17 edges
10. `NewWithServices()` - 16 edges

## Surprising Connections (you probably didn't know these)
- `Verification evidence (this task)` --references--> `TestManagerStopAllKillsGrandchildren()`  [INFERRED]
  .ai_context/MANIFEST.md → internal/process/manager_processgroup_test.go
- `Continuation Verification (2026-11-16, second session)` --references--> `TestManagerStartEventsCarryRunAndSequence()`  [INFERRED]
  .ai_context/MANIFEST.md → internal/process/manager_test.go
- `Completed This Session (2026-11-17 — v1.2 finish)` --references--> `waitForLogCount()`  [INFERRED]
  .ai_context/MANIFEST.md → internal/process/manager_test.go
- `Slice B — ANSI + SSE batching + dashboard performance` --references--> `renderTerminal()`  [INFERRED]
  .ai_context/MANIFEST.md → internal/uiserver/static/app.js
- `Done` --references--> `runUp()`  [INFERRED]
  TLDR.md → cmd/devproxy/main.go

## Import Cycles
- None detected.

## Communities (44 total, 6 thin omitted)

### Community 0 - "devproxy/main.go"
Cohesion: 0.07
Nodes (35): fetchProcessStatuses(), isFlag(), main(), printProcessStatuses(), printUsage(), resolveUIURL(), run(), runProcess() (+27 more)

### Community 1 - "testing.T"
Cohesion: 0.08
Nodes (43): Checklist, Diagnosis (recorded before edits), 1. Instance restart after stop/exit (root cause fix), 2. Log separation in the dashboard, 3. Dashboard UI with daisyUI + Tailwind, Files Modified, Recommendations / Next Steps, Sprint Memo — Instance Restart Fix, Log Separation, daisyUI/Tailwind Dashboard (+35 more)

### Community 2 - "app.js"
Cohesion: 0.09
Nodes (63): acceptsOutput(), ansiCache, ansiStyle(), appendHighlighted(), appendStyledText(), applyEvent(), applyTheme(), bind() (+55 more)

### Community 3 - "devproxy-bootstrap/main.go"
Cohesion: 0.11
Nodes (26): assetName(), cachedBinaryIsCurrent(), cachedBinaryPath(), checksumForAsset(), downloadBaseURL(), downloadBinary(), downloadToFile(), ensureBinary() (+18 more)

### Community 4 - "Bus"
Cohesion: 0.06
Nodes (36): Root causes confirmed in source, EventType, RequestEvent, ringBuffer, sequenced, Subscriber, subscription, subscriptionsSnapshot (+28 more)

### Community 5 - "testing.T"
Cohesion: 0.05
Nodes (56): TestFindRouteLongestMatch(), TestFindRouteWithRootFallback(), CleanTerminalLine(), NewSanitizer(), SanitizeTerminalText(), TestCleanTerminalLineReportsClears(), TestCleanTerminalLineStripsSGR(), TestSanitizerCarriageReturn() (+48 more)

### Community 6 - "Code Review and Quality"
Cohesion: 0.07
Nodes (29): 1. Correctness, 2. Readability & Simplicity, 3. Architecture, 4. Security, 5. Performance, Change Descriptions, Change Sizing, Code Review and Quality (+21 more)

### Community 7 - "Test-Driven Development"
Cohesion: 0.07
Nodes (29): Browser Testing with DevTools, Common Rationalizations, DAMP Over DRY in Tests, Decision Guide, Discover the Stack First, Name Tests Descriptively, One Assertion Per Concept, Overview (+21 more)

### Community 8 - "steps"
Cohesion: 0.07
Nodes (29): name, runs-on, steps, GH_TOKEN, jobs, build, name, on (+21 more)

### Community 9 - "store_test.go"
Cohesion: 0.16
Nodes (19): Line, Result, Run, Store, New(), newLineRing(), resolveRun(), line() (+11 more)

### Community 10 - "DevProxy Context Manifest"
Cohesion: 0.08
Nodes (27): Active Task — Stop all instances on quit (2026-11-17), Chosen design, Completed This Session (2026-11-16 — v1.2), Completed This Session (2026-11-17 — v1.2 finish), Continuation Verification (2026-11-16, second session), Continuation Verification (2026-11-17, third session), CPU / event-rate baseline (current code), Current Objective (+19 more)

### Community 11 - "ansi.js"
Cohesion: 0.19
Nodes (15): applySGR(), cloneStyle(), colorValue(), createParser(), feed(), finishCSI(), push(), take() (+7 more)

### Community 12 - "API and Interface Design"
Cohesion: 0.08
Nodes (24): 1. Contract First, 2. Consistent Error Semantics, 3. Validate at Boundaries, 4. Prefer Addition Over Modification, 5. Predictable Naming, 6. Honouring an Idempotency Key, API and Interface Design, Common Rationalizations (+16 more)

### Community 13 - "Frontend UI Engineering"
Cohesion: 0.08
Nodes (24): Accessibility (WCAG 2.1 AA), ARIA Labels, Avoid the AI Aesthetic, Color, Common Rationalizations, Component Architecture, Component Patterns, Design System Adherence (+16 more)

### Community 14 - "Incremental Implementation"
Cohesion: 0.09
Nodes (22): Common Rationalizations, Contract-First Slicing, Implementation Rules, Increment Checklist, Incremental Implementation, Overview, Red Flags, Risk-First Slicing (+14 more)

### Community 15 - "Code Simplification"
Cohesion: 0.09
Nodes (21): 1. Preserve Behavior Exactly, 2. Follow Project Conventions, 3. Prefer Clarity Over Cleverness, 4. Maintain Balance, 5. Scope to What Changed, Code Simplification, Common Rationalizations, Language-Specific Guidance (+13 more)

### Community 16 - "Debugging and Error Recovery"
Cohesion: 0.09
Nodes (21): Build Failure Triage, Common Rationalizations, Debugging and Error Recovery, Error-Specific Patterns, Instrumentation Guidelines, Overview, Red Flags, Runtime Error Triage (+13 more)

### Community 17 - "5. Functional Requirements"
Cohesion: 0.08
Nodes (23): 1. Project Objectives, 2. Current Repository Baseline, 3. Technology Stack, 4. Core Architecture, 5. Functional Requirements, 6. Non-Functional Requirements, 7. Open Decisions, 8. Version History (+15 more)

### Community 18 - "processes"
Cohesion: 0.10
Nodes (19): processes, command, name, working_dir, proxy, host, port, targets (+11 more)

### Community 19 - "processes"
Cohesion: 0.10
Nodes (19): PORT, processes, command, env, name, working_dir, proxy, host (+11 more)

### Community 20 - "Config"
Cohesion: 0.06
Nodes (31): Config, Load(), TestLoadDefaults(), TestLoadProcesses(), writeTestConfig(), NewBus(), TestBusPublish(), TestBusPublishAfterUnsubscribeDoesNotPanic() (+23 more)

### Community 21 - "DevProxy Project Details"
Cohesion: 0.13
Nodes (15): Core Goals, DevProxy Project Details, Event Model, Internal Components, Non-Goals for the First Version, Problem, Proposed Commands, Proposed Config Shape (+7 more)

### Community 22 - "Spec-Driven Development"
Cohesion: 0.14
Nodes (13): Common Rationalizations, Keeping the Spec Alive, Overview, Phase 0: Scope Check, Phase 1: Specify, Phase 2: Plan, Phase 3: Tasks, Phase 4: Implement (+5 more)

### Community 23 - "Planning and Task Breakdown"
Cohesion: 0.15
Nodes (12): 1. Plan read-only, 2. Check existing plans before writing, 3. Map dependencies and slice work, 4. Define implementable tasks, 5. Produce and verify the plan, Overview, Plan Document Structure, Planning and Task Breakdown (+4 more)

### Community 24 - "DevProxy"
Cohesion: 0.14
Nodes (12): Bootstrapper, Current Commands, Current Status, DevProxy, Example Config Direction, Goal, Planned Features, Process Control API (+4 more)

### Community 25 - "Universal Context Optimization & Liveness Protocol"
Cohesion: 0.20
Nodes (9): 1. READ PROTOCOL (Strict Order), 2. REAL-TIME LIVENESS SAVING PROTOCOL, 3. CONTEXT PRUNING AND PRE-COMPACTION RULES, 4. TASK COMPLETION & MEMORY COMPACTION PROTOCOL, Execution Standard & Model Behavioral Instructions, STEP 4.1: APPEND TO CHRONICLE, STEP 4.2: UPDATE REQUIREMENTS (If Architecture Changed), STEP 4.3: REGENERATE MANIFEST (+1 more)

### Community 27 - "Entries"
Cohesion: 0.22
Nodes (8): 2026-10-03 11:43 — Initial project setup (MEMO_20261003_1143), 2026-10-03 12:14 — Process lifecycle, dashboard, log separation (MEMO_20261003_1214), 2026-10-03 12:35 — Restart fix, log separation, daisyUI/Tailwind dashboard (MEMO_20261003_1235), 2026-11-16 — Context v2 migration, 2026-11-17 — v1.2 completion: log history, batched SSE, dashboard and TUI, DevProxy Chronicle, Entries, Epoch Summary

### Community 28 - "AGENTS.md"
Cohesion: 0.29
Nodes (6): Graphify, Project context, Read order and scope, Security and collaboration constraints, Skill use, TLDR

### Community 29 - "MVP Roadmap"
Cohesion: 0.29
Nodes (7): MVP Roadmap, Phase 1: Stable proxy foundation, Phase 2: Process manager, Phase 3: Web UI MVP, Phase 4: CLI polish, Phase 5: TUI, Phase 6: Bootstrapper

### Community 30 - "Continuous Development"
Cohesion: 0.33
Nodes (5): Blockers, progress, and context, Continuous Development, Execute in small slices, Initialize from repository state, Runtime limitations

### Community 31 - "Initial Project Setup Memo"
Cohesion: 0.33
Nodes (5): Initial Project Setup Memo, Initialization Record, Next Gate, Repository Baseline Inspected, Setup Steps Completed

### Community 32 - "DevProxy Rules"
Cohesion: 0.33
Nodes (5): Architecture boundaries, Collaboration and network security, DevProxy Rules, Scope and safety, Tooling and conventions

### Community 34 - "Sprint Memo — Process Lifecycle, Dashboard, and Log Separation"
Cohesion: 0.40
Nodes (4): Files, Implemented, Sprint Memo — Process Lifecycle, Dashboard, and Log Separation, Verification

### Community 35 - "Bootstrapper Design"
Cohesion: 0.40
Nodes (5): Behavior, Bootstrapper Design, Bootstrapper options, Example cache paths, Release naming

### Community 36 - "Config Sections"
Cohesion: 0.40
Nodes (5): Config Sections, `processes`, `proxy.targets`, `server.proxy`, `server.ui`

### Community 37 - "Example User Experience"
Cohesion: 0.40
Nodes (5): Example User Experience, Install globally, Run through repo bootstrapper, Run with a config path, Run with TUI

### Community 38 - "Ponytail: Lazy Senior Dev Mode"
Cohesion: 0.50
Nodes (3): Before you write, Ponytail: Lazy Senior Dev Mode, The smallest complete change

### Community 39 - "How to Build"
Cohesion: 0.50
Nodes (4): How to Build, Linux, Mac OS, Window

## Knowledge Gaps
- **1 isolated node(s):** `github.com/phy0hk/devproxy`
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 407 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **6 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Slice B — ANSI + SSE batching + dashboard performance` connect `DevProxy Context Manifest` to `app.js`?**
  _High betweenness centrality (0.073) - this node is a cross-community bridge._
- **Are the 12 inferred relationships involving `NewManager()` (e.g. with `TestManagerStopAllKillsGrandchildren()` and `TestManagerInitialStatuses()`) actually correct?**
  _`NewManager()` has 12 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/phy0hk/devproxy` to the rest of the system?**
  _1 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `devproxy/main.go` be split into smaller, more focused modules?**
  _Cohesion score 0.0673616680032077 - nodes in this community are weakly interconnected._
- **Should `testing.T` be split into smaller, more focused modules?**
  _Cohesion score 0.08038914490527393 - nodes in this community are weakly interconnected._
- **Should `app.js` be split into smaller, more focused modules?**
  _Cohesion score 0.09027777777777778 - nodes in this community are weakly interconnected._
- **Should `devproxy-bootstrap/main.go` be split into smaller, more focused modules?**
  _Cohesion score 0.1051693404634581 - nodes in this community are weakly interconnected._