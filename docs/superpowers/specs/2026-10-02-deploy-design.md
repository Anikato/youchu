# 有处：部署

日期：2026-10-02

状态：已按用户确认的方案 A 写成。用户要求直接实现、推送到 GitHub，中间不再停下来确认。本文是本阶段的实现依据。

依据：同仓库《家庭物品整理与查找需求.md》《技术方案草稿.md》第 1、2、6、7、8 节，以及已实现的登录、物品和位置、分类和搜索、归位与回收站、照片、家用界面与账号、MCP 与个人访问令牌规格。冲突时，会话、Origin、迁移器、SQLite 连接以第一阶段为准；业务规则以各已通过规格为准；本文件覆盖静态托管、反向代理后的登录限流、容器镜像、Compose 数据目录、GitHub 仓库与自动构建。

本文只覆盖交付顺序中的「部署和端到端验证」。OAuth 授权服务器放到后续可选工作。

## 1. 目标

公开 GitHub 仓库保存源码。GitHub Actions 构建多架构镜像并推到 Docker Hub。目标机用 Docker Compose 跑一个容器。本机 Nginx 做 HTTPS 和反代。数据与 Compose 文件同级，备份时打包整个目录。

本阶段结束时：

- 一个 Go 进程在生产环境同时提供网页 `/`、接口 `/api/v1`、MCP `/mcp`。运行时不需要 Node.js。
- 开发形态仍是 Vite + Go 两个进程，浏览器继续只开 `http://127.0.0.1:5173`。
- 目标机目录含 `docker-compose.yml`、`.env`、`data/`。Compose 把容器端口映射到 `127.0.0.1:主机端口`。TLS 由 Nginx 处理。
- 登录限流在 `YOUCHU_TRUST_PROXY=true` 时按 `X-Forwarded-For` 最左边的地址计。默认关闭，现有开发行为不变。
- 仓库 `https://github.com/Anikato/youchu` 公开。推送 `main` 后，若已配置 Docker Hub token，Actions 推送 `l97312/youchu`。token 未配置时构建仍可跑，跳过登录和推送。
- 程序内不做备份。文档写明停服后打包整个部署目录。

## 2. 不做的事

不实现 OAuth 授权服务器、资源元数据、动态客户端注册、PKCE、`.well-known` 发现。

不实现程序内备份、扫码、HEIC 解码、变更事件表、归类批次撤销、按图搜索、公开无登录的图片地址。

不在 Compose 里带 Nginx 或 Certbot。不在容器内做 TLS。

不新增 npm 依赖。不把 Playwright 放进仓库。不启用 CGO。`YOUCHU_USERNAME` / `YOUCHU_PASSWORD` 仍只在用户表为空时建号。

不把 Docker Hub token 写进仓库、`.env.example` 或文档正文。token 只放在 GitHub Actions Secret `DOCKERHUB_TOKEN`。

不把 `data/`、`web/node_modules/`、`.env` 提交进 Git。

## 3. 仓库与作者

首次在本目录 `git init`。本地（仅此仓库）作者：

- `user.name`：`Anikato`
- `user.email`：`110054022+Anikato@users.noreply.github.com`

默认分支 `main`。用已登录的 `gh` 创建公开仓库 `Anikato/youchu` 并推送。

`.gitignore` 至少包含：`/data/`、`*.db`、`*.db-wal`、`*.db-shm`、`/web/node_modules/`、`/web/dist/`、`.env`、`.DS_Store`。

## 4. 新增环境变量

`YOUCHU_TRUST_PROXY`：`true` 或 `false`，缺省 `false`（与空字符串相同）。其它值拒绝启动，错误发生在打开数据库之前。解析规则与 `YOUCHU_COOKIE_SECURE` 相同。

现有变量含义不变。生产 Compose 中：

| 变量 | 生产值 |
| --- | --- |
| `YOUCHU_HTTP_ADDR` | 容器内 `0.0.0.0:8080`（由 Compose `environment` 写入，不靠操作者手填） |
| `YOUCHU_DATA_DIR` | 容器内 `/data`（由 Compose 写入） |
| `YOUCHU_PUBLIC_ORIGIN` | `https://` 加上站点主机，无路径、查询、末尾斜杠 |
| `YOUCHU_COOKIE_SECURE` | `true` |
| `YOUCHU_TRUST_PROXY` | `true` |
| `YOUCHU_USERNAME` / `YOUCHU_PASSWORD` | 仅空库首次启动需要 |

开发默认仍是 `YOUCHU_HTTP_ADDR=127.0.0.1:8080`、`YOUCHU_COOKIE_SECURE=false`、`YOUCHU_TRUST_PROXY=false`。

## 5. 登录限流与转发头

`YOUCHU_TRUST_PROXY=false`（默认）：限流键仍是 `RemoteAddr` 经 `SplitHostPort` 得到的地址。不读 `X-Forwarded-For`。现有 `TestLoginRateLimitIsPerRemoteAddr` 继续成立。

`YOUCHU_TRUST_PROXY=true`：若请求带非空 `X-Forwarded-For`，取按逗号分割后的**第一段**（去掉首尾空白）作为限流键；该段若含端口则去掉端口。头缺失或第一段为空时，回退到 `RemoteAddr`。Origin 不匹配的请求仍在限流之前拒绝，不计入次数。

Compose 必须把端口发到 `127.0.0.1`，只有本机 Nginx 能连上容器端口。因此开启信任转发头时，伪造 `X-Forwarded-For` 需要先到达本机。

Nginx 示例为每个代理请求设置 `X-Forwarded-For`。不要改写浏览器带来的 `Origin`。

## 6. 网页嵌入

新增包 `internal/webui`：

```
internal/webui/dist/.gitkeep
internal/webui/fs.go
```

`//go:embed all:dist` 嵌入 `dist` 目录。仓库里的 `dist` 只有占位文件，没有 `index.html`。Docker 构建把 `web/dist` 的产物拷进这个目录后再编译 Go。

`httpapi.New` 签名保持 `(db, cfg, dummyHash, now)`，默认使用 `webui.Dist`。测试若要注入文件系统，同包使用未导出构造。

静态与 SPA 规则（仅 GET 与 HEAD）：

1. 路径等于 `/api` 或前缀 `/api/`：现有 JSON `404 not_found`，不返回网页。
2. 路径等于 `/mcp` 或前缀 `/mcp/`：已由 MCP 处理器或 JSON `404` 处理，不返回网页。
3. 嵌入目录中没有 `index.html`：`/` 与其它非接口路径仍是 JSON `404`（本机 `go run` 保持现状，网页由 Vite 提供）。
4. 有 `index.html`：先按嵌入目录找精确文件（去掉开头 `/`）。找到则按文件返回。找不到且路径最后一段含 `.`（视为静态资源）则 JSON `404`。否则返回 `index.html`，`Content-Type` 含 `text/html`。

POST/PUT/PATCH/DELETE 打到网页路径仍是 JSON `404`。

接口、Cookie、MCP 契约不变。`/api/v1/photos/{id}/original` 等现有路由优先于 SPA。

## 7. 容器镜像

`Dockerfile` 多阶段：

1. `node:22-bookworm`：`web/` 里 `npm ci` 然后 `npm run build`。
2. `golang:1.27.1-bookworm`：`CGO_ENABLED=0` 编译 `./cmd/youchu`。把前端 `dist` 拷到 `internal/webui/dist` 后再 `go build`。
3. `gcr.io/distroless/static-debian12`：只放二进制。入口 `/youchu`。不启用 CGO。

`.dockerignore` 排除 `/data`、`web/node_modules`、`web/dist`、`.git`、`.env`。

镜像名 `l97312/youchu`。标签 `latest` 与 `sha-<完整 github.sha>`。平台 `linux/amd64` 和 `linux/arm64`。

## 8. 目标机 Compose

仓库 `deploy/`：

```
deploy/docker-compose.yml
deploy/.env.example
deploy/nginx.conf.example
```

操作者把 `deploy/` 里的文件拷到目标机一个空目录（或只拷 compose 与 env 示例），同级建 `data/`。

`docker-compose.yml`：

- 服务名 `youchu`
- `image: l97312/youchu:${YOUCHU_IMAGE_TAG:-latest}`
- `restart: unless-stopped`
- `ports: ["127.0.0.1:${YOUCHU_HOST_PORT:-8080}:8080"]`
- `env_file: .env`
- `environment` 固定 `YOUCHU_HTTP_ADDR=0.0.0.0:8080`、`YOUCHU_DATA_DIR=/data`
- `volumes: ["./data:/data"]`
- `stop_grace_period: 10s`（进程已处理 SIGTERM，5 秒内 Shutdown）

`.env.example` 列出：`YOUCHU_PUBLIC_ORIGIN`、`YOUCHU_COOKIE_SECURE=true`、`YOUCHU_TRUST_PROXY=true`、`YOUCHU_USERNAME`、`YOUCHU_PASSWORD`、`YOUCHU_IMAGE_TAG=latest`、`YOUCHU_HOST_PORT=8080`。不含 token，不含真实域名。

备份：`docker compose stop`，打包该目录（含 `data/`），再 `docker compose start`。运行时直接复制 WAL 中的数据库不保证一致。程序不实现自动备份。

## 9. Nginx

`deploy/nginx.conf.example` 给本机 Nginx 作片段，不是可直接替换的整份站点配置。要求：

- `proxy_pass http://127.0.0.1:8080;`（端口与 `YOUCHU_HOST_PORT` 一致）
- `proxy_set_header Host $host;`
- `proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;`
- `proxy_set_header X-Forwarded-Proto $scheme;`
- 不设置 `Origin`
- `client_max_body_size 41m;`（应用层照片上限是 40 MiB）
- `location /mcp` 另写：`proxy_buffering off;`、`proxy_http_version 1.1;`、读写超时至少 `3600s`，并转发 `Authorization`

证书、`server_name`、监听 443 由操作者现有 Nginx 管理。

## 10. GitHub Actions

`.github/workflows/docker.yml`：

- 触发：`push` 到 `main`，以及 `workflow_dispatch`
- `actions/checkout`、`docker/setup-qemu-action`、`docker/setup-buildx-action`
- `DOCKERHUB_TOKEN` 非空时：`docker/login-action` 用户名 `l97312`，密码为该 Secret，然后 `push: true`
- token 为空时：不登录，`push: false`，仍然构建（可只构建 `linux/amd64` 以节省时间；有 token 时构建 amd64 与 arm64）
- 有 token 时标签：`l97312/youchu:latest` 与 `l97312/youchu:sha-${{ github.sha }}`

不在工作流里写 token。文档说明到 GitHub 仓库 Settings → Secrets and variables → Actions 增加 `DOCKERHUB_TOKEN`。

## 11. 测试

Go 测试，不模拟 SQLite，不把 Docker 或 Nginx 当作本机必跑项（本机可能没有 Docker）。

配置：

- 缺省 `TrustProxy == false`
- `YOUCHU_TRUST_PROXY=yes` 拒绝启动

限流：

- 默认：同一 `RemoteAddr`、不同 `X-Forwarded-For`，第 11 次仍按 `RemoteAddr` 限流
- `TrustProxy=true`：同一转发地址 10 次之后第 11 次 `429 rate_limited`；换一个转发地址不共用该桶

网页：

- 无 `index.html` 时 `GET /` 为 JSON `404 not_found`
- 注入含 `index.html` 与 `assets/app.js` 的文件系统：`GET /` 与 `GET /account` 返回 HTML；`GET /assets/app.js` 返回该文件；`GET /api/v1/does-not-exist` 仍是 JSON `404`；`GET /missing.js` 为 JSON `404`

`go test ./...` 与 `cd web && npm run build` 必须通过。不增加 `package-lock.json` 里的包数量。

## 12. README 与工作记录

README 第一句改为：本地可以单账号登录，在网页里改用户名和密码、签发接入令牌，登记物品和位置，管理分类，搜索筛选，登记待归位，把物品放进回收站，并给物品加照片。MCP 可用个人访问令牌接入。生产用 Docker Compose，本机 Nginx 反代。

README 保留现有两进程开发说明。增加：

- 公开仓库地址
- 镜像 `l97312/youchu`
- 拷贝 `deploy/`、复制 `.env.example` 为 `.env`、填写来源与首次账号、`mkdir data`、`docker compose up -d`
- Nginx 看 `deploy/nginx.conf.example`
- 备份：停服后打包整个目录
- Actions Secret `DOCKERHUB_TOKEN`；未配置时推送不会发布镜像

工作记录：本段（部署）完成；嵌入迁移仍是 `001`–`006`；Git 仓库已建；下一段不再是交付顺序里的必做切片。不要把 OAuth 或 HEIC 写成已完成。

## 13. 验收

1. `GET /` 在嵌入了前端的二进制上返回登录页 HTML；未嵌入时本机 `go run` 仍 JSON 404，网页走 Vite。
2. 生产 `YOUCHU_PUBLIC_ORIGIN` 与浏览器 HTTPS 来源一致时，登录写请求通过 Origin 检查。
3. Compose 端口只绑 `127.0.0.1`。
4. 停服复制部署目录后，数据文件在 `data/` 下。
5. 工作流文件存在；无 token 时不试图登录 Docker Hub。
6. 网页与 MCP 路径规则不变：Cookie 不能当 `/mcp` 凭证。

## 14. 不变量

- 仓库中没有 OAuth 授权服务器，没有 HEIC 解码，没有 `change_events`。
- 没有新的 npm 依赖，Playwright 不在仓库。
- 迁移仍是 `001`–`006`。
- Docker Hub 用户名是 `l97312`，GitHub 用户是 `Anikato`，镜像是 `l97312/youchu`。
