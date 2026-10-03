# 主屏幕、拍照与动效 Implementation Plan

> **For agentic workers:** Execute inline in this session. User skipped review checkpoints and asked to deploy when done.

**Goal:** Safari standalone home screen, camera/album photo pickers including on item create, a few GSAP entrances.

**Architecture:** Static manifest and PNG icons in `web/public`. Client-only pending files on create, then existing photo POST. GSAP via `useGSAP` on login, app shell, and two lists.

**Tech Stack:** React 19, GSAP + `@gsap/react`, existing Go SPA file server.

Execute: Go SPA test first, then web files, `go test ./...`, `cd web && npm run build`, Playwright in `/tmp`, commit, push, deploy Alis.
