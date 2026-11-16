---
name: graphify
description: Use this skill when working with the repository's Graphify knowledge graph or when querying the codebase through Graphify. Follow the project's graph guidance and keep graphify-out current after code changes.
---

# Graphify Knowledge Graph

This repository maintains a Graphify knowledge graph in `graphify-out/`. Use it as a code-reading cache and navigation aid: query its indexed knowledge before broad source exploration, but never assume the cache is complete or current.

## Working with Graphify

- For repository-analysis tasks, first compare the commit in `graphify-out/GRAPH_REPORT.md` with `git rev-parse HEAD`. Run `graphify update .` when the cache is stale or code files have changed; this update is AST-only and does not incur API cost.
- Before broad source exploration, run a scoped `graphify query`, `graphify explain`, or `graphify path` query. Use `graphify-out/GRAPH_REPORT.md` for broad orientation, not detailed conclusions.
- If `graphify-out/wiki/index.md` exists, navigate the wiki from that index rather than reading raw files.
- Use graph results to select relevant files and relationships, then inspect source files and tests directly before implementing or validating changes. Record the useful query/result and known limitations in `.ai_context/MANIFEST.md` for the active task.
- For cross-language flows, query the frontend and backend nodes separately when no direct path exists; a missing edge is not evidence that the integration is absent.
- If the graph or relevant cached memory is missing, Graphify cannot be queried, the graph is stale, or its result is insufficient, continue with targeted source inspection and document the fallback. Missing graph coverage is not evidence that the code or behavior does not exist.
- When working on Graphify itself—including its generation, configuration, or correctness—follow the repository's existing Graphify guidance. Do not treat the graph as evidence when evaluating the graph's own correctness.
- After modifying code files in the session, run `graphify update .` when the CLI is available to keep the cache current and record the refresh in the manifest.

ws://172.17.0.1:43365/collaborate/527d81e3168af0dbf01e7760a823296eafc23c7f46f5a1b1