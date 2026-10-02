# 有处：MCP 与个人访问令牌

日期：2026-10-01

状态：已按用户确认的方案 A 写成。用户要求直接实现、中间不再停下来确认。本文是本阶段的实现依据。

依据：同仓库《家庭物品整理与查找需求.md》，《技术方案草稿.md》第 1、2、5、6、8 节，以及已实现的登录、物品和位置、分类和搜索、归位与回收站、照片、家用界面与账号规格。冲突时，会话、Origin、迁移器、SQLite 连接以第一阶段为准；物品、位置、分类、归位、回收站、照片的业务规则和网页契约以各已通过规格为准；本文件覆盖 MCP、个人访问令牌、分类关联的 `source` 列。

本文只覆盖交付顺序中的「MCP 与认证」这一段，按方案 A：Streamable HTTP + 个人访问令牌。OAuth 授权服务器和 Docker 放到部署阶段。

## 1. 目标

本机已经登录的人可以签发个人访问令牌。带令牌的 MCP 客户端查询和整理同一份家庭目录，结果与网页一致。

本阶段结束时：

- 账号页可以创建、查看、撤销接入令牌。明文只在创建成功时出现一次。
- Go 进程在 `/mcp` 提供官方 SDK 的 Streamable HTTP。客户端用 `Authorization: Bearer`。
- 令牌带范围 `read`、`organize`、`write`。默认创建时勾选读取和归类。
- MCP 归类只增加 `source=ai` 的关联，不去掉人工关联。迁移之前已有的关联视为人工。
- 永久删除、改用户名、改密码、签发令牌只能在网页上进行。MCP 没有这些工具。
- 照片通过工具返回 JPEG 字节，不返回依赖网页 Cookie 的 URL。

## 2. 不做的事

不实现 OAuth 授权服务器、资源元数据、动态客户端注册、Client ID Metadata、PKCE、`.well-known` 发现。

不实现 Docker、备份、扫码、HEIC 解码、变更事件表、归类批次撤销、按图搜索、公开无登录的图片地址、stdio 传输、任意 SQL、任意本地路径、服务器命令。

不新增 npm 依赖。不把 Playwright 放进仓库。不启用 CGO。不新增环境变量。`YOUCHU_USERNAME` / `YOUCHU_PASSWORD` 仍只在用户表为空时建号。

不把 Cookie 会话当作 `/mcp` 的凭证。不把令牌放进 URL。不记录令牌明文或 `Authorization` 头。

MCP 不上传照片、不删除照片、不设封面。MCP 不删除位置、不删除分类、不永久删除物品。

登录、限流、Cookie、网页 Origin 校验保持现有行为。现有 JSON 写接口正文上限仍是 32768 字节。令牌接口正文上限 4096 字节。

## 3. 仓库

新增：

```
migrations/006_mcp_auth.sql
internal/mcp/
```

`internal/auth` 负责令牌的签发、哈希查找、列表、撤销。`internal/catalog` 负责 `item_categories.source` 以及归类追加/去掉 AI 关联。`internal/mcp` 用官方 SDK 注册工具，直接调用 `catalog` 和读图函数，不向本进程发 HTTP。`internal/httpapi` 注册 `/api/v1/tokens`、`/api/v1/tokens/{id}`、`/mcp`，先做 Origin（令牌写接口）或 Bearer（MCP），再交给业务。

沿用现有迁移器，只增加 `006` 文件。嵌入版本因此是连续的 `001` 到 `006`。

允许新增纯 Go 模块 `github.com/modelcontextprotocol/go-sdk`，锁定 `v1.8.0`（或实现时 `go get` 得到的、支持 `mcp.NewStreamableHTTPHandler` 与 `mcp.AddTool` 的 v1.8.x，写入 `go.sum`）。其传递依赖一并锁定。`web/package-lock.json` 的依赖项不增加。

实现时更新 `README.md`：说明可以在账号页签发接入令牌，MCP 地址是 `http://127.0.0.1:8080/mcp`，开发时 `http://127.0.0.1:5173/mcp` 由 Vite 转发。第一句改为：本地可以单账号登录，在网页里改用户名和密码、签发接入令牌，登记物品和位置，管理分类，搜索筛选，登记待归位，把物品放进回收站，并给物品加照片。MCP 可用个人访问令牌接入。部署还不在这里。

`web/vite.config.ts` 增加把 `/mcp` 转到 `http://127.0.0.1:8080` 的代理，与 `/api` 并列。

## 4. 表

`006_mcp_auth.sql`：

```sql
CREATE TABLE access_tokens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK (name <> ''),
    token_hash TEXT NOT NULL UNIQUE,
    token_prefix TEXT NOT NULL,
    scopes TEXT NOT NULL,
    created_at TEXT NOT NULL
);

ALTER TABLE item_categories
    ADD COLUMN source TEXT NOT NULL DEFAULT 'human'
    CHECK (source IN ('human', 'ai'));
```

`access_tokens.id` 插入时不指定 `id`。已提交的删除不会把该 id 再分配给新行。回滚、未提交的插入不在本阶段测试里。

时间列使用 UTC 的 RFC3339Nano 文本。

`scopes` 存规范化后的逗号串，只能是下列之一：

- `read`
- `read,organize`
- `read,write`
- `read,organize,write`

顺序固定为 `read`、`organize`、`write`。库里不出现其它拼法。

`token_hash` 是完整令牌的 SHA-256 十六进制摘要，算法与 `sessions.token_hash` 相同（`crypto/sha256`，小写 hex）。`token_prefix` 是完整令牌的前 12 个字符，只用于列表识别，不加唯一约束。

已有 `item_categories` 行在 `ADD COLUMN` 后 `source` 为 `human`。网页创建或 PATCH 替换分类时写入 `human`。MCP 归类写入 `ai`。

主键仍是 `(item_id, category_id)`。同一物品同一分类只能有一行，不能同时既是人工又是 AI。

## 5. 个人访问令牌

### 5.1 明文格式

`crypto/rand` 生成 32 字节，令牌为 `yc_` 后接 `base64.RawURLEncoding`。与会话令牌相同的编码，只多固定前缀 `yc_`。

创建成功的 JSON 带完整 `token`。之后任何接口都不再返回完整值。列表和撤销响应没有 `token` 键。

### 5.2 范围

| 范围 | 能力 |
| --- | --- |
| `read` | 查询物品、位置、分类、待归位、回收站；读取照片字节 |
| `organize` | 新建分类；给物品追加 AI 分类；去掉该物品上的 AI 分类 |
| `write` | 新建/编辑物品和位置；编辑分类名称或父级；把物品放入回收站；恢复；新建/完成/去掉待归位事项 |

每张令牌必须包含 `read`。`organize` 和 `write` 可选。没有单独的 `destructive` 范围。永久删除不在 MCP 里，也不靠令牌范围打开。

改密码不撤销接入令牌。改用户名不影响令牌。退出网页会话不影响令牌。撤销某张令牌立刻使它的 Bearer 失效。

最多 20 张未撤销令牌。超过时创建返回 409 `token_limit`，短句「最多 20 个接入令牌」。撤销是删除行，腾出名额。

名称：去掉首尾空白后 1 到 80 个 Unicode 码点，不能含控制字符或换行。空名「名称不能为空」；超长「名称不能超过 80 个字符」；控制字符「名称不能包含控制字符」。允许中间空格，例如「电脑上的 Cursor」。大小写保留。

### 5.3 HTTP

网页会话接口，Cookie 名仍是 `youchu_session`。

`GET /api/v1/tokens`：已登录。不检查 Origin。有任何查询字符串则 400 `invalid_fields`，`不支持的参数`。无会话 401。成功 200：

```json
{"data":[{"id":1,"name":"电脑上的 Cursor","token_prefix":"yc_abcdefghij","scopes":["read","organize"],"created_at":"..."}]}
```

`data` 按 `id` 升序。没有令牌时是 `"data":[]`，不是 `null`。`scopes` 永远是 JSON 数组。

`POST /api/v1/tokens`：写失败顺序 Origin → 会话 → 正文 4096 → 查询字符串 → JSON → 字段 → 张数上限。正文：

```json
{"name":"电脑上的 Cursor","scopes":["read","organize"]}
```

只认 `name` 和 `scopes`。其它键忽略。`name` 缺失或 `null`：400 `invalid_body`。`scopes` 缺失、`null`、不是数组、含未知值、重复、不含 `read`：400 `invalid_fields`，`fields.scopes` 为「权限范围不正确」。成功 201，对象含 `id`、`name`、`token`、`token_prefix`、`scopes`、`created_at`。

`DELETE /api/v1/tokens/{id}`：写失败顺序 Origin → 会话 → 查询字符串（DELETE 不读正文；有查询字符串则 400）→ 路径 id。id 不是正整数且 Origin、会话已过：404 `not_found`（与其它删除路径一致：先 Origin、会话，再解析 id）。成功 204。不存在 404。不要求 version。

`PUT` / `PATCH` `/api/v1/tokens` 以及 `GET` `/api/v1/tokens/{id}` 返回 404 `not_found`。

错误 Origin 即使带会话也是 403 `origin_rejected`，不创建、不删除。无会话且 Origin 正确是 401。错误 Origin 加超限正文是 403 不是 413。无会话加超限正文是 401 不是 413。

路径不要写成 Go 1.22 的 `"PATCH /path"` 方法模式。沿用 `HandleFunc` 加 `switch r.Method`。

### 5.4 Bearer

`Authorization` 必须是 `Bearer` 加一个空格加完整令牌，scheme 大小写不敏感。缺头、不是 Bearer、空令牌、哈希不匹配：HTTP 401，JSON `{"code":"unauthenticated","message":"未登录"}`，响应头：

```
WWW-Authenticate: Bearer realm="youchu"
```

`/mcp` 忽略 Cookie。只有 Cookie、没有合法 Bearer 时仍是 401。同时有 Cookie 和 Bearer 时只认 Bearer。

令牌不出现在查询字符串。`/mcp?token=` 仍按无令牌处理（先 401），不把查询里的值当凭证。

## 6. `/mcp`

官方 SDK `mcp.NewStreamableHTTPHandler`。`getServer` 可以每次返回同一 `*mcp.Server`。生产使用 `StreamableHTTPOptions{Stateless: true}`，避免跨请求泄漏会话 goroutine。不要把 JSONResponse 设死为唯一模式；客户端 `Accept` 同时带 `application/json` 和 `text/event-stream` 时由 SDK 选择。

服务实现名 `youchu`，版本字符串 `0.1.0`。Instructions（工具说明之外的服务器说明）固定为：

> 有处是家庭物品目录。查询可以直接做。改数据、归类、放入回收站必须是用户刚刚明确要求的操作。不要把对话里的口头答应当成授权。归类只增加 AI 分类，不去掉人工分类。永久删除只能在网页上进行。

### 6.1 HTTP 入口顺序

对 `/mcp` 的每个请求：

1. 若 `Origin` 头存在：主机名不是 `127.0.0.1`、也不是 `localhost`、也不等于 `YOUCHU_PUBLIC_ORIGIN` 解析得到的主机，则 403 `origin_rejected`。`YOUCHU_PUBLIC_ORIGIN` 整段相等也接受（部署后网页来源）。缺 `Origin` 的本地客户端直接进入下一步。
2. 解析 Bearer。失败则 401，带 `WWW-Authenticate`。
3. 其余交给 SDK。

浏览器跨站带攻击者站点 Origin 的请求在第 1 步被拒。本机 Inspector（Origin 主机为 `127.0.0.1` 或 `localhost`）可以通过，仍必须带令牌。

`/mcp` 不走网页那套「Origin 必须等于 PublicOrigin」规则，因为 MCP 客户端不是 Vite 页面。

### 6.2 工具错误

认证失败用 HTTP 状态码，不进入 JSON-RPC。

工具内部失败返回 `CallToolResult.IsError = true`，`Content` 里一段 JSON 文本，形状与网页错误一致：`code`、`message`，需要时有 `fields`。不要把业务错误提升成 JSON-RPC 协议错误。

范围不够：`code` 为 `forbidden`，`message` 为「当前令牌没有这项权限」。不执行写操作。

版本冲突、未找到、字段错误的 `code` / 短句与对应 HTTP 接口相同。

成功时 `Content` 为一段 JSON 文本，对象形状与对应 GET 接口相同（物品含 `categories[].source`、`photos`，列表含 `data`、`total`、`limit`、`offset`）。空数组写成 `[]`，可选空字段仍是 `null`，与网页 JSON 相同。

### 6.3 范围与工具

没有 `read` 的令牌发不出来，因此所有工具都先检查对应范围。

只读工具需要 `read`。归类工具需要 `organize`。写入工具需要 `write`。只读工具在只有 `organize` 或只有 `write` 的令牌上仍然可用，因为令牌必有 `read`。

工具名称、参数、范围：

**读取（`read`）**

| 工具 | 参数 | 行为 |
| --- | --- | --- |
| `youchu_search_items` | `q`、`placement`、`in_location`、`in_location_descendants`、`category`、`category_match`、`category_descendants`、`uncategorized`、`limit`、`offset` 均为可选 | 与 `GET /api/v1/items` 同一套筛选和分页。默认 `limit=30`，上限 100。不提供 `location=`（那是位置页直连列表）。互斥与字段错误短句与第三阶段相同 |
| `youchu_get_item` | `id` 必填 | 活物品详情。回收站中的 id 为未找到 |
| `youchu_list_locations` | `parent`、`flat`、`limit`、`offset` 可选 | 与 `GET /api/v1/locations` 相同。`parent` 与 `flat` 互斥 |
| `youchu_get_location` | `id` 必填 | 位置详情 |
| `youchu_list_categories` | `parent`、`flat`、`limit`、`offset` 可选 | 与 `GET /api/v1/categories` 相同 |
| `youchu_get_category` | `id` 必填 | 分类详情 |
| `youchu_list_return_tasks` | `limit`、`offset` 可选 | 未完成、活物品上的事项，与 `GET /api/v1/return-tasks` 相同 |
| `youchu_get_return_task` | `id` 必填 | 事项详情 |
| `youchu_list_trash` | `limit`、`offset` 可选 | 与 `GET /api/v1/trash` 相同 |
| `youchu_get_trash_item` | `id` 必填 | 回收站详情 |
| `youchu_get_photo` | `id` 必填；`variant` 可选 `thumbnail`（默认）或 `original` | 返回 `ImageContent`，`MIMEType` 为 `image/jpeg`，`Data` 为文件字节。物品在回收站仍可读。缺文件与网页读图相同（500、非 JPEG）。不要返回 URL |

**归类（`organize`）**

| 工具 | 参数 | 行为 |
| --- | --- | --- |
| `youchu_create_category` | `name` 必填；`parent_id` 可选 | 与 `POST /api/v1/categories` 相同 |
| `youchu_add_item_categories` | `item_id` 必填；`category_ids` 必填非空整数数组 | 活物品。已存在的关联（无论 human/ai）跳过。其余插入 `source=ai`。不改物品 `version` / `updated_at`。成功返回该物品详情 |
| `youchu_remove_ai_item_categories` | `item_id` 必填；`category_ids` 必填非空整数数组 | 只删除 `source=ai` 的行。若其中任一 id 当前是 `human`：不删任何行，`invalid_fields`，`fields.category_ids` 为「不能去掉人工分类」。若某 id 该物品上没有：`invalid_fields`，「该物品没有这个分类」。不改物品 `version` / `updated_at`。成功返回该物品详情 |

`category_ids` 不是数组或含非正整数：`invalid_fields`，「分类格式不正确」。重复 id 视为「同一分类只能关联一次」。分类 id 在库中不存在：`所选分类不存在`。物品缺失或在回收站：未找到，即使分类参数也不合法也先认物品（与归位事项「先认物品」一致：物品 404 优先于字段）。实现顺序：鉴权范围 → 物品存在且未删除 → 字段 → 写入。

**写入（`write`）**

| 工具 | 参数 | 行为 |
| --- | --- | --- |
| `youchu_create_item` | 与 POST 物品相同：`name` 必填；`alias`、`model`、`spec`、`quantity_note`、`note`、`locations` 可选。不要接受 `categories` | 新建物品。分类用归类工具追加 |
| `youchu_update_item` | `id`、`version` 必填；可改字段与 PATCH 物品相同。不要接受 `categories` | 不替换分类。省略的字段含义与 PATCH 相同 |
| `youchu_create_location` | 与 POST 位置相同 | 新建位置 |
| `youchu_update_location` | `id`、`version` 必填；可改字段与 PATCH 位置相同 | 编辑位置 |
| `youchu_update_category` | `id`、`version` 必填；`name` 与/或 `parent_id` | 与 PATCH 分类相同 |
| `youchu_trash_item` | `id`、`version` 必填 | 与 `DELETE /api/v1/items/{id}?version=` 相同，软删除 |
| `youchu_restore_item` | `id`、`version` 必填 | 与 `POST /api/v1/trash/{id}/restore` 相同 |
| `youchu_create_return_task` | `item_id` 必填；`part_note`、`reason`、`destination_note` 可选 | 与 POST 归位事项相同 |
| `youchu_complete_return_task` | `id`、`version` 必填 | 完成事项 |
| `youchu_delete_return_task` | `id`、`version` 必填 | 去掉未完成事项 |

没有 `youchu_purge_item`、`youchu_delete_location`、`youchu_delete_category`、`youchu_upload_photo`、`youchu_delete_photo`、`youchu_set_photo_first`。若模型调用未注册名称，SDK 按未知工具处理。

工具注解：只读工具 `readOnlyHint=true`。归类与写入 `readOnlyHint=false`。`youchu_trash_item` 的 `destructiveHint=true`（进回收站，可恢复）。其余写入 `destructiveHint=false`。

### 6.4 并发与一致性

网页 PATCH 物品若带 `categories` 数组，仍整表替换该物品的分类，新插入的行 `source=human`。这会清掉未出现在数组里的 AI 关联。这是网页原有语义，本阶段保持。

MCP 与网页改同一件物品的名称或位置时，仍用 `version`。MCP 归类不增加 `version`，因此不会单独制造物品版本冲突。

同一业务规则（长度、树循环、占用、软删除隐藏）全部走 `internal/catalog`，禁止在 MCP 包复制 SQL。

## 7. 物品 JSON 的 `source`

`itemCategoryJSON` 增加必填键 `source`，值只能是 `"human"` 或 `"ai"`。物品详情、物品列表、回收站详情里每一条分类都带这个键。不能省略。没有分类时仍是 `"categories":[]`。

网页表单不展示来源，也不按来源筛选。TypeScript `ItemCategory` 增加 `source: "human" | "ai"`，保存物品时 PATCH 仍只送 `category_id`（现有形状）。网页保存仍把提交的集合写成人工关联。

现有断言如果核对分类对象原文，必须带上 `"source":"human"`。这是本阶段要改的断言，不是行为回退。

## 8. 账号页

在密码区块和「回收站」链接之间增加「接入令牌」。

- 列表：名称、`token_prefix`、范围（中文：读取 / 归类 / 写入）、创建时间、按钮「撤销」。
- 空列表文案：「还没有接入令牌」。
- 表单：名称；三个复选框，标签「读取」「归类」「写入」。打开页面时「读取」和「归类」勾选，「写入」不勾选。「读取」不能取消；取消时提交前提示「至少需要读取权限」，不发请求。
- 主按钮「创建令牌」，`buttonPrimary`。
- 创建成功：在表单下显示完整令牌（只读输入框），说明「请立刻复制，关闭或刷新后无法再看」。列表出现新行且没有完整令牌。再创建另一张则替换这次的明文提示。
- 撤销：立即删除，不必二次确认（令牌可再签发；与永久删除物品不同）。失败显示接口短句。

390 宽账号页 `document.documentElement.scrollWidth` 仍为 390。令牌区块允许换行，不横向撑开。

## 9. 测试

Go 测试使用临时数据目录和 `httptest`，通过 HTTP 验证，不模拟 SQLite。至少覆盖：

1. 已有 `001`–`005` 的库启动后执行 `006`。再次启动不重复执行。`access_tokens` 表存在且 `id` 为 AUTOINCREMENT。`item_categories` 有 `source` 列。先插入一条不带 `source` 的旧库不可再出现；本阶段用迁移后的库插入分类关联，不写 `source` 时默认 `human`。
2. 已登录 `POST /api/v1/tokens` `{"name":"电脑上的 Cursor","scopes":["read","organize"]}` 返回 201，`token` 以 `yc_` 开头，`token_prefix` 为其前 12 字符，`scopes` 为 `["read","organize"]`。库中 `token_hash` 不是明文。随后 `GET /api/v1/tokens` 的 `data` 含该行且原文没有 `"token":`。
3. `scopes` 为 `["organize"]`、`["write"]`、`["read","read"]`、`["read","foo"]`、不是数组，均 400 `fields.scopes`「权限范围不正确」。名称 `"  "` 为「名称不能为空」。第 21 张 409 `token_limit`。
4. `DELETE /api/v1/tokens/{id}` 204 后，原 Bearer 调 `/mcp` 为 401。错误 Origin 的 POST/DELETE 为 403，行数不变。无会话 401。POST 带查询字符串 400。超过 4096 字节：已登录 413；错误 Origin 加超限 403；无会话加超限 401。
5. `PUT /api/v1/tokens`、`GET /api/v1/tokens/1` 为 404 `not_found`。
6. 网页创建物品并关联分类后，`GET` 物品原文含 `"source":"human"`。MCP `youchu_add_item_categories` 追加另一个分类后，该条 `"source":"ai"`，人工那条仍是 `"human"`，物品 `version` 不变。再对人工分类调用 `youchu_remove_ai_item_categories`：错误「不能去掉人工分类」，两行都在。
7. 无 `Authorization` 的 `POST /mcp` 为 401，带 `WWW-Authenticate`。错误 Origin（例如 `https://evil.example`）即使带合法 Bearer 也是 403。缺 Origin、带合法 Bearer 可以 `initialize`。Cookie 会话、无 Bearer 访问 `/mcp` 为 401。
8. 只有 `read` 的令牌调用 `youchu_create_item` 得到工具错误 `forbidden`，物品表无新行。带 `write` 的令牌创建「遥控车」后，`GET /api/v1/items/{id}`（网页会话）看得到同一件，名称一致。
9. 两个独立客户端都能完成查询：① `mcp.Client` + `mcp.StreamableClientTransport`（HTTP 头带 Bearer）；② 原始 `http.Post` JSON-RPC `initialize` 然后 `tools/call` `youchu_search_items`。两者都查到第 8 步创建的物品。记录所用 SDK 版本为 `go.mod` 中的 `go-sdk` 版本。
10. `youchu_get_photo` 对已上传 JPEG 返回 `ImageContent` 且能按 `image/jpeg` 解码。同一张图用网页会话 `GET .../thumbnail` 也是 200。MCP 调用没有 `youchu_purge_item` 工具（`tools/list` 名称集合不含它）。带 `write` 的令牌也不能靠未知工具删库。
11. 现有登录、物品、分类、归位、照片、改用户名密码测试仍然通过。分类原文断言补上 `source`。

网页不把浏览器自动化套件放进仓库。实现时在浏览器里实际完成第 10 节。

## 10. 验收

同时满足：

- 第 9 节的 Go 测试通过。
- `cd web && npm run build` 通过。`web/package-lock.json` 的依赖项不增加。嵌入迁移是 `001` 到 `006`。
- 桌面宽度，`http://127.0.0.1:5173`：登录后点用户名进入账号。创建名称为「测试令牌」、范围读取+归类的令牌，能看到一次完整 `yc_` 开头的值。刷新后完整值消失，列表仍有前缀。撤销后列表为空。
- 用该流程新签发一张带读取+写入的令牌，在 Go 测试外用原始 HTTP 对 `http://127.0.0.1:8080/mcp` 调用 `youchu_search_items` 得到 与网页物品列表一致的名称集合（可在实现时的 Playwright 或 `/tmp` 脚本里做；不进仓库）。
- 390×844：账号页（含令牌区块）`scrollWidth` 为 390。
- 仓库中没有 OAuth 授权服务器、没有 Docker、没有 HEIC 解码、没有 `change_events`、没有 `item_categories` 以外的归类批次表。有 `item_categories.source` 和 `/mcp`。

## 11. 给实现的边界

- 目录还不是 Git 仓库。实现结束不要 `git init`，也不要提交。
- 本机 Git 若没有 `user.name` / `user.email`，不要发明作者信息，也不要改 Git 配置。
- 浏览器验收打开 `http://127.0.0.1:5173`，不要用 `localhost`。MCP 客户端连 `http://127.0.0.1:8080/mcp` 或经 Vite 的 `/mcp`。
- Playwright 若使用，放在仓库外（例如 `/tmp`）。
- 不要重做登录、物品位置、分类搜索、归位回收站、照片或家用界面，除非本阶段必须改到现有接口（物品分类 JSON 增加 `source`、网页插入分类时写 `human`、Vite 代理 `/mcp`、账号页增加令牌区块）。
