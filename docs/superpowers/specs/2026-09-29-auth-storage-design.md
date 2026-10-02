# 有处：基础登录与存储

日期：2026-09-29

状态：已按审阅修正迁移前缀、每连接 SQLite 参数、代理限流和配置正数校验。本文是第一阶段的实现依据。

依据：同仓库《技术方案草稿.md》的技术栈、登录规则和交付顺序，以及《家庭物品整理与查找需求.md》。冲突时以用户对第一阶段的确认和本文为准。本文只覆盖第一阶段。

## 1. 目标

本机可以启动一个 Go 进程和一套 Vite 开发网页。操作者用部署时创建的唯一账号登录，会话保存在 SQLite。未登录不能读取当前用户。写请求必须来自配置的站点来源。

本阶段结束时，操作者能打开登录页、登录、看到自己的用户名、退出，并重新登录。

## 2. 不做的事

本阶段不实现物品、位置、分类、搜索、待归位、回收站、照片、MCP、OAuth、个人访问令牌、Docker、备份、静态资源托管、密码修改和公开注册。

网页只有登录页和登录后的占位页。占位页不预留物品管理界面。

## 3. 运行形态

两个进程，仅用于开发：

- Go 监听 `YOUCHU_HTTP_ADDR`，默认 `127.0.0.1:8080`，只提供 `/api/v1`。
- Vite 开发服务器提供网页。浏览器只访问 Vite。Vite 把 `/api` 代理到 Go。

浏览器与 Go 之间不启用 CORS。跨源的浏览器请求由 Origin 校验拒绝。

数据文件放在 `YOUCHU_DATA_DIR`，默认 `./data`。目录不存在时创建。数据库文件为该目录下的 `youchu.db`。`data/` 不进入 Git。

启动顺序固定为：校验配置、检查嵌入迁移编号、打开数据库、执行迁移、在用户表为空时创建账号、开始监听。配置不合法或嵌入编号不连续时，不打开数据库、不创建用户、不监听端口。

进程收到 SIGINT 或 SIGTERM 时停止接收新请求，关闭数据库后退出。

## 4. 配置

环境变量如下。未列出的变量不读取。

| 变量 | 默认 | 规则 |
| --- | --- | --- |
| `YOUCHU_HTTP_ADDR` | `127.0.0.1:8080` | Go 监听地址 |
| `YOUCHU_DATA_DIR` | `./data` | SQLite 与后续数据文件的目录 |
| `YOUCHU_PUBLIC_ORIGIN` | 无，必填 | CSRF 允许的精确来源，例如 `http://127.0.0.1:5173`。必须包含方案和主机，不含路径、查询或末尾斜杠 |
| `YOUCHU_COOKIE_SECURE` | `false` | `true` 时会话 Cookie 带 Secure。本机 HTTP 保持 `false`；HTTPS 部署设为 `true` |
| `YOUCHU_SESSION_TTL` | `336h` | 会话绝对有效期，Go duration 语法。必须大于等于 `1s` |
| `YOUCHU_LOGIN_LIMIT` | `10` | 每个限流键在 60 秒内允许的登录请求次数。必须是大于等于 1 的整数 |
| `YOUCHU_USERNAME` | 无 | 仅在用户表为空时必填 |
| `YOUCHU_PASSWORD` | 无 | 仅在用户表为空时必填 |

`YOUCHU_PUBLIC_ORIGIN` 缺失或格式不符时拒绝启动。

`YOUCHU_SESSION_TTL` 缺省为 `336h`。给出的值用 `time.ParseDuration` 解析，必须大于等于 `1s`。`0s`、负数，以及 `500ms` 这类小于 1 秒的正数，都拒绝启动。Cookie 的 `Max-Age` 取该时长的整秒数；因为时长至少 1 秒，`Max-Age` 至少为 1。数据库里的 `expires_at` 使用完整时长，不先截成整秒。

`YOUCHU_LOGIN_LIMIT` 缺省为 `10`。给出的值必须是十进制整数，且大于等于 1。`0`、负数和非整数拒绝启动。

`YOUCHU_PUBLIC_ORIGIN`、`YOUCHU_COOKIE_SECURE`、`YOUCHU_SESSION_TTL` 和 `YOUCHU_LOGIN_LIMIT` 在打开数据库之前校验。不合法时进程以非零状态退出。用户名和密码要等迁移完成、确认用户表为空之后再校验，因为已有用户时不再读取这两个变量。

用户表为空时，`YOUCHU_USERNAME` 与 `YOUCHU_PASSWORD` 都必须存在。用户名去掉首尾空白后长度为 1 到 64，且不能包含空白字符。密码不去掉空白，长度至少 8，至多 128。不满足则拒绝启动，并且不创建用户。

用户表已有记录时，启动过程不读取这两个变量，也不修改已有密码或用户名。账号创建成功后，操作者应从长期运行环境中去掉明文密码。进程不把密码写入日志。

布尔值只接受 `true` 和 `false`。

## 5. 仓库布局

```
cmd/youchu/main.go
internal/config/
internal/migrate/
internal/auth/
internal/httpapi/
migrations/001_auth.sql
web/
docs/superpowers/specs/2026-09-29-auth-storage-design.md
```

`migrations/*.sql` 通过 `go:embed` 打进二进制。`web/` 是独立的 npm 项目。

依赖在实施时选用当时相互兼容的稳定版本，并提交 `go.sum` 与 `web/package-lock.json`。版本不使用浮动的 `latest`。前端使用 React、TypeScript、Vite、React Router、TanStack Query 和 CSS Modules。后端使用 Go 标准库 `net/http`、`database/sql` 和 `modernc.org/sqlite`。密码哈希使用 `golang.org/x/crypto/argon2`。

## 6. 迁移

数据库文件用 `file:` URI 打开。外键、忙等超时和 WAL 由驱动在每条物理连接建立时应用，使用写在程序里的参数 `_foreign_keys=on`、`_busy_timeout=5000`、`_journal_mode=WAL`。这些参数不来自环境变量。`database/sql` 会按需新建连接，所以不能只在启动时执行一次 `PRAGMA`。`*sql.DB` 调用 `SetMaxOpenConns(4)`。本阶段写入都是短事务。

连接是否生效，用新连接验证，而不是只检查第一条连接。测试把 `SetMaxIdleConns(0)`，使归还的连接被关闭。连续两次取出连接，每次都读到 `PRAGMA foreign_keys` 为 `1`、`PRAGMA busy_timeout` 为 `5000`。第二条连接插入一条 `user_id` 不存在的 `sessions` 行时，外键约束失败。

迁移前，程序创建 `schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL)`（已存在则保持不变）。这张表不占用迁移版本号。

启动时先读取已执行版本，确认可以升级后，再按编号执行剩余迁移。每个迁移单独一个事务，并在同一事务写入自己的 `schema_migrations` 行。`applied_at` 使用 UTC 的 RFC3339 文本。

规则：

- 嵌入文件名格式为 `NNN_描述.sql`。全部嵌入版本必须正好是从 `001` 到最大版本的连续编号。这个检查在打开数据库之前完成。缺号或重复编号时拒绝启动，不创建数据库文件，也不写入 `schema_migrations`。用临时 SQL 文件测试缺号和重复，不往生产迁移里放坏文件。
- 已执行版本必须是嵌入版本的连续前缀 `1..k`，`k` 可以为 0。只执行 `k+1` 到最大版本。数据库已有 `001`、程序新增 `002` 时，只执行 `002`。
- 没有剩余迁移时直接启动，不重复执行已有版本。
- 数据库中的最大版本高于嵌入的最大版本时拒绝启动。已执行版本缺号，或含有程序不认识的版本时，也拒绝启动。拒绝时不执行迁移，也不改写已有的 `schema_migrations` 行。
- 任一迁移失败则回滚该迁移并退出，不继续服务。

`001_auth.sql` 创建：

- `users`：`id INTEGER PRIMARY KEY CHECK (id = 1)`、`username TEXT NOT NULL UNIQUE`、`password_hash TEXT NOT NULL`、`created_at TEXT NOT NULL`、`updated_at TEXT NOT NULL`。插入时显式使用 `id = 1`，因此全库只有一行用户。
- `sessions`：自增 `id`、`token_hash TEXT NOT NULL UNIQUE`、`user_id INTEGER NOT NULL REFERENCES users(id)`、`expires_at TEXT NOT NULL`、`created_at TEXT NOT NULL`。`token_hash` 是会话令牌的 SHA-256 十六进制摘要。

时间使用 UTC 的 RFC3339 文本。

## 7. 账号与密码

首次启动且用户表为空时，在一个事务里插入 `id = 1` 的用户。密码以 argon2id 的 PHC 字符串存储，参数为 memory 64 MiB、iterations 3、parallelism 4、salt 16 字节、输出 32 字节。明文密码不入库、不进日志。两个进程同时对空库启动时，后写入的一方会因主键冲突退出，不把对方创建的账号改成自己的密码。

进程启动时用相同的 argon2id 参数生成一次虚拟哈希，密码随机产生并立即丢弃。登录时用户名不存在，就校验这个虚拟哈希，再返回与密码错误相同的响应。用户名比较区分大小写。登录请求里的用户名去掉首尾空白；密码不去掉空白。去掉空白后用户名为空，或密码字段为空字符串，返回 400 `invalid_body`。密码只受 4 KiB 正文上限约束，登录时不再套用创建账号时的 128 字符上限。

没有注册接口，没有默认用户名或默认密码，没有修改密码接口。

## 8. 会话

登录成功时用 `crypto/rand` 生成 32 字节令牌，Cookie 中放置 RawURLEncoding 形式，数据库只存 SHA-256 十六进制摘要。

Cookie 名称为 `youchu_session`。属性为 `Path=/`、`HttpOnly`、`SameSite=Lax`，不设置 `Domain`。`Secure` 由 `YOUCHU_COOKIE_SECURE` 决定。`Max-Age` 等于会话有效期的整秒数；有效期至少 1 秒，所以 `Max-Age` 至少为 1。

有效期从创建时起算，访问接口不会延长。过期会话在读取时删除，并视为未登录。登录不撤销该用户的其他会话，以便手机和电脑同时登录。退出只删除当前 Cookie 对应的会话。

令牌、Cookie 值和密码哈希不写入日志。

## 9. HTTP 接口

请求和响应正文为 JSON，字符集 UTF-8。错误正文为：

```json
{ "code": "invalid_credentials", "message": "用户名或密码错误" }
```

`message` 给人读，判断以 `code` 为准。

| 方法与路径 | 成功 | 说明 |
| --- | --- | --- |
| `POST /api/v1/session` | 200，正文 `{"username":"..."}`，并设置 Cookie | 登录。失败不设置会话 Cookie |
| `DELETE /api/v1/session` | 204，空正文，并清除 Cookie | 退出。没有会话时同样返回 204 |
| `GET /api/v1/me` | 200，正文 `{"username":"..."}` | 读取当前用户 |

状态与 `code`：

| 状态 | code | 何时 |
| --- | --- | --- |
| 400 | `invalid_body` | JSON 无法解析；用户名或密码缺失、不是字符串；去掉空白后用户名为空；密码为空字符串 |
| 401 | `invalid_credentials` | 用户名或密码不匹配。两种情况使用同一句 message |
| 401 | `unauthenticated` | `GET /api/v1/me` 没有有效会话 |
| 403 | `origin_rejected` | 写请求的 Origin 与 `YOUCHU_PUBLIC_ORIGIN` 不一致，或缺少 Origin |
| 429 | `rate_limited` | 登录请求超过频率限制 |
| 413 | `body_too_large` | 请求正文超过 4 KiB |

`POST` 与 `DELETE` 先检查 Origin，再拒绝超过 4 KiB 的正文。通过这两项之后，只有 `POST /api/v1/session` 进入登录限流。`GET /api/v1/me` 不检查 Origin。没有正文的 `DELETE` 视为长度 0。

Origin 必须与配置值逐字符相同。不使用 Referer 代替 Origin，不支持多个来源。

登录限流统计每一次 `POST /api/v1/session`，包括即将成功的请求。键是 `RemoteAddr` 经 `SplitHostPort` 得到的地址，不读取 `X-Forwarded-For` 或其他转发头。`RemoteAddr` 没有端口时，用原字符串作为键。窗口是固定窗口：某个键的第一次计入请求打开 60 秒窗口，窗口内超过 `YOUCHU_LOGIN_LIMIT` 次即拒绝，窗口结束后下一次请求打开新窗口。计数在进程内存中，重启后清零。超过限制时不校验密码。Origin 不匹配的请求在限流之前拒绝，不计入次数。

开发时浏览器通过 Vite 代理访问 Go，Go 看到的 `RemoteAddr` 通常是 Vite 进程的地址，因此这些登录请求共享一个限流桶。第一阶段接受按代理地址合并的限流。正式部署若前面还有反向代理，应使用只信任指定代理的配置，或在代理层限流。本阶段不实现这两种方式，也不把未经信任配置的转发头当作浏览器地址。

未知路径返回 404，正文 code 为 `not_found`。

## 10. 网页

界面文字使用中文。

- `/login`：用户名、密码和登录按钮。密码框使用 `type="password"`。输入框分别标明用户名和密码，并设置 `autocomplete="username"` 与 `autocomplete="current-password"`。失败时显示接口返回的 message。
- `/`：已登录时显示用户名和退出按钮。未登录时进入 `/login`。已登录时访问 `/login` 则进入 `/`。

登录和当前用户请求使用 TanStack Query。路由使用 React Router。样式使用 CSS Modules 和少量 CSS 变量，只用基础表单控件。

开发时打开 `http://127.0.0.1:5173`，并让 `YOUCHU_PUBLIC_ORIGIN` 等于该来源。不使用 `localhost` 与 `127.0.0.1` 混用。

## 11. 测试

Go 测试使用临时数据目录和 `httptest`，覆盖：

1. 只嵌入 `001` 时，迁移执行两次后，`001` 仍只有一行，表结构仍在。
2. 嵌入迁移缺号或重复编号时拒绝启动，不写入 `schema_migrations`。
3. 数据库已记录 `001`，嵌入文件为 `001` 和 `002` 时，只执行 `002`，`001` 仍只有一行。
4. 数据库中的最大版本高于嵌入的最大版本时拒绝启动，且不改写已有的 `schema_migrations` 行。
5. 已执行版本不是从 1 开始的连续前缀时拒绝启动。
6. 在 `001` 已执行的库上把 `SetMaxIdleConns(0)`，连续取出的两条连接都读到 `foreign_keys = 1` 和 `busy_timeout = 5000`。第二条连接插入引用不存在用户的 `sessions` 行时，外键约束失败。
7. `YOUCHU_SESSION_TTL` 为 `0s`、`-1s` 或 `500ms`，以及 `YOUCHU_LOGIN_LIMIT` 为 `0` 或 `-1` 时，配置校验失败。失败发生在打开数据库和创建用户之前。
8. 没有用户且缺少账号环境变量时，初始化失败且用户表仍为空。
9. 首次初始化写入唯一用户；再次初始化即使用不同的密码环境变量，哈希也不变。
10. 正确密码返回 200 和 Cookie；随后 `GET /api/v1/me` 返回该用户名。
11. 错误密码返回 401 和 `invalid_credentials`，响应不带会话 Cookie。
12. 不存在的用户名与错误密码返回相同的 code 和 message。
13. 无 Cookie 或过期 Cookie 时，`GET /api/v1/me` 返回 401。
14. 写请求缺少 Origin 或 Origin 不匹配时返回 403，且不创建会话。
15. 同一 `RemoteAddr` 的第 11 次登录请求返回 429。另一个 `RemoteAddr` 的第 1 次请求不受前一个地址的计数影响。测试直接设置 `RemoteAddr`，不把 Vite 代理下多个浏览器共享一桶当成失败。
16. `DELETE /api/v1/session` 之后，原 Cookie 不能再访问 `GET /api/v1/me`。另一个仍有效的会话不受影响。

网页在实现时用浏览器实际完成登录、看到用户名、退出、再次打开受保护页面这些步骤。本阶段不添加浏览器自动化套件。

## 12. 验收

同时满足以下条件才算本阶段完成：

- 第 11 节的 Go 测试通过。
- 浏览器中可以用环境变量创建的账号登录并退出。
- 仓库中没有默认密码、没有物品业务代码、没有 MCP 和 OAuth 实现。
- `go.sum` 与 `web/package-lock.json` 已提交，记录实际使用的版本。
