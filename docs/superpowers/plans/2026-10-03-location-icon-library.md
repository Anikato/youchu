# 位置自传 SVG 图标库 Implementation Plan

> **For agentic workers:** Execute inline in this session. User skipped review checkpoints and asked to deploy when done.

**Goal:** Household SVG icon library in SQLite; pick on location create/edit; manage on the account page.

**Architecture:** `location_icons` stores sanitized SVG text. Locations gain `custom_icon_id`. HTTP `/api/v1/location-icons` plus existing location write. Web picker and account section share the list.

**Tech Stack:** Go 1.22+ stdlib `encoding/xml`, React 19, existing CSS modules.

Execute TDD: HTTP/catalog tests first, then code, then web, then `go test ./...` and `cd web && npm run build`, then browser QA in `/tmp`, commit, push, deploy Alis.
