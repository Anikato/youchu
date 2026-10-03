# 添加物品体验 Implementation Plan

> **For agentic workers:** Execute inline in this session. User skipped review checkpoints and asked to deploy when done.

**Goal:** Item create stays on the form after save, remembers last location/category, shows save/photo progress, mobile sticky save, list defaults to newest first.

**Architecture:** `sort` on existing `GET /api/v1/items`. UI state in localStorage and React. Photos still upload after create.

**Tech Stack:** Go catalog + React item form.

Execute: HTTP sort tests, catalog/MCP/query parse, web form/list/sticky save, `go test ./...`, `cd web && npm run build`, Playwright in `/tmp`, commit, push, deploy Alis.
