# 添加物品时搜索位置 Implementation Plan

> **For agentic workers:** Execute inline in this session. User skipped review checkpoints and asked to deploy when done.

**Goal:** Item create/edit location picker filters by name and code; tapping a hit adds it.

**Architecture:** Client-side filter on the already-fetched flat location list. No API change.

**Tech Stack:** React item form. `filterLocations` in `web/src/locationSearch.ts`. Node built-in test in `/tmp`. Playwright in `/tmp`.

Execute: failing `filterLocations` tests, implement matcher, replace `ItemLocationsField` select, CSS, `npm run build`, Playwright, commit, push, deploy Alis.
