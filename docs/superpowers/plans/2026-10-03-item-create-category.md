# 添加物品时新建分类 Implementation Plan

> **For agentic workers:** Execute inline in this session. User skipped review checkpoints and asked to deploy when done.

**Goal:** Create a root category from the item form and attach it to the draft; keep existing reparent so it can become a child later.

**Architecture:** Reuse `POST /api/v1/categories` with name only. UI in `ItemCategoriesField`. HTTP test pins reparent leaving `item_categories` unchanged.

**Tech Stack:** Existing Go catalog + React item form.

Execute: reparent regression test, item-form create UI, `go test ./...`, `cd web && npm run build`, Playwright in `/tmp`, commit, push, deploy Alis.
