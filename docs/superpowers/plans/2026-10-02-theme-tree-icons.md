# 外观、位置树与图标 Implementation Plan

> **For agentic workers:** Execute inline. User asked to finish, push, and deploy without mid-stream confirmation.

**Goal:** 默认暗色、账号页切浅色和三种强调色；位置 `icon`；列表按路径缩进。

**Architecture:** 外观只在 `localStorage` + `html` 属性。位置 `icon` 走迁移 007 与现有 catalog 写入。树形只用已有 `path.length`。

**Tech Stack:** 现有 Go、React、CSS 变量。无新 npm。

规格：`docs/superpowers/specs/2026-10-02-theme-tree-icons-design.md`。

任务：1 迁移与 API；2 外观 CSS 与账号页；3 树与图标 UI；4 测试、文档、推送、部署。实现时跳过任务间确认。提交一次后推 `main`，等 Actions 出镜像，再在 Alis `/data/youchu` `docker compose pull && up -d`。
