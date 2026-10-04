# 列表多选与物品分页 Implementation Plan

> **For agentic workers:** Execute inline in this session. User skipped remaining questions and asked to deploy when done.

**Goal:** Current-page multi-select on item, location, trash, and return lists; item home page size and page numbers; undo for deletes.

**Architecture:** Add `child_count` to location JSON. UI selection mode calls existing per-record APIs sequentially. No batch routes.

**Tech Stack:** Go catalog JSON, React list pages, CSS modules. Node built-in tests in `/tmp`. Playwright in `/tmp`.

Execute: failing `child_count` and filter tests, implement helpers and UI, `go test` + `npm run build`, Playwright, commit, push, deploy Alis.
