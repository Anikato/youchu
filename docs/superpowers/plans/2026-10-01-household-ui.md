# 家用界面与账号 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把网页做成家用纸质目录的样子，重排页头和物品页，并加上改用户名、改密码。

**Architecture:** 不新增迁移。`internal/auth` 负责改 `users` 行、校验当前密码、删掉当前令牌以外的会话。`internal/httpapi` 给 `GET|PATCH /api/v1/me` 和 `POST /api/v1/me/password` 做 Origin、会话、正文和字段。网页新增 `theme.css` 变量，改 CSS Modules 与页头、登录、账号、物品列表和物品表单。不新增 npm 包。

**Tech Stack:** 现有 Go 1.27.1、`modernc.org/sqlite` v1.60.1、React 19.3.0、Vite 8.3.1、TypeScript 7.0.2、React Router 8.4.0、TanStack Query 5.104.0。不启用 CGO，不新增环境变量。

规格：`docs/superpowers/specs/2026-10-01-household-ui-design.md`。

提交：本目录还不是 Git 仓库。每个提交步骤先检查 `git rev-parse --is-inside-work-tree`。不是仓库就跳过 commit，**不要** `git init`。有仓库但没有 `user.name` 就停下来问用户，不要改 Git 配置，也不要编造作者。不要提交 `/data/`、`web/node_modules/` 或 `web/dist/`。

---

## 文件职责

- `internal/auth/user.go`：`UpdateUsername`、`ChangePassword`（同一事务更新哈希并 `DELETE FROM sessions WHERE token_hash <> ?`）。
- `internal/auth/session.go`：可把删其它会话放在这里，与 `tokenHash` 同包。
- `internal/httpapi/server.go`：`me` 改为 GET/PATCH；注册 `/api/v1/me/password`。
- `internal/httpapi/server_test.go`：规格第 11 节。
- `web/src/theme.css`：CSS 变量和 `html, body, #root`。
- `web/src/main.tsx`：引入 `theme.css`。
- `web/src/styles.module.css`：登录面板、页头、主/次按钮、列表行、`--thumb`。
- `web/src/App.tsx`：登录页结构；`/account` 路由。
- `web/src/pages.tsx`：页头、账号页、物品列表查找、物品表单顺序、「更多说明」、删除移出表单。
- `web/src/api.ts`：`updateUsername`、`changePassword`。
- `README.md`、`docs/工作记录.md`。

路由不要写成 Go 1.22 的 `"PATCH /path"` 方法模式。沿用 `HandleFunc` 加 `switch r.Method`。

写账号接口失败顺序：Origin → 会话 → 正文大小 → 查询字符串 → JSON 形状 → 字段。正文上限 4096。`writeFields` 已在 `catalog_http.go` 同包，直接用。`queryRejected` 同样。

`currentUser` 只返回 bool。PATCH/POST 需要当前 Cookie 值才能保留会话，在 handler 里自己读 `youchu_session`，用 `auth.Lookup` 判会话，再把 token 传给 `ChangePassword`。

---

### Task 1: 改用户名和改密码接口

**Files:**
- Modify: `internal/auth/user.go`
- Modify: `internal/auth/session.go`（若把 `DeleteOtherSessions` 放这里）
- Modify: `internal/httpapi/server.go`
- Modify: `internal/httpapi/server_test.go`

- [ ] **Step 1: 写失败测试**

在 `internal/httpapi/server_test.go` 的现有登录测试之后追加。`assertInvalidFields` 在 `catalog_test.go` 同包，可以直接调用。不要用 `assertCode` 去解带 `fields` 的正文。

```go
func TestPatchMeUsername(t *testing.T) {
	_, h, _ := testHandler(t, false)
	login := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	cookie := login.Result().Cookies()[0].Value
	rec := request(h, http.MethodPatch, "/api/v1/me", `{"username":"kevin"}`, "http://127.0.0.1:5173", "203.0.113.5:1", cookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"username":"kevin"`) {
		t.Fatalf("patch=%d %s", rec.Code, rec.Body.String())
	}
	me := request(h, http.MethodGet, "/api/v1/me", "", "", "203.0.113.5:1", cookie)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"username":"kevin"`) {
		t.Fatalf("me=%d %s", me.Code, me.Body.String())
	}
	old := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.6:1", "")
	assertCode(t, old, http.StatusUnauthorized, "invalid_credentials")
	ok := request(h, http.MethodPost, "/api/v1/session", `{"username":"kevin","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.6:1", "")
	if ok.Code != http.StatusOK {
		t.Fatalf("login kevin=%d %s", ok.Code, ok.Body.String())
	}
}

func TestPatchMeUsernameFields(t *testing.T) {
	_, h, _ := testHandler(t, false)
	login := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	cookie := login.Result().Cookies()[0].Value
	origin := "http://127.0.0.1:5173"
	blank := request(h, http.MethodPatch, "/api/v1/me", `{"username":"  "}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, blank, map[string]string{"username": "用户名不能为空"})
	space := request(h, http.MethodPatch, "/api/v1/me", `{"username":"a b"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, space, map[string]string{"username": "用户名不能包含空白"})
	long := request(h, http.MethodPatch, "/api/v1/me", `{"username":"`+strings.Repeat("名", 65)+`"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, long, map[string]string{"username": "用户名过长"})
	nullName := request(h, http.MethodPatch, "/api/v1/me", `{"username":null}`, origin, "203.0.113.5:1", cookie)
	assertCode(t, nullName, http.StatusBadRequest, "invalid_body")
	missing := request(h, http.MethodPatch, "/api/v1/me", `{}`, origin, "203.0.113.5:1", cookie)
	assertCode(t, missing, http.StatusBadRequest, "invalid_body")
	same := request(h, http.MethodPatch, "/api/v1/me", `{"username":"ada"}`, origin, "203.0.113.5:1", cookie)
	if same.Code != http.StatusOK || !strings.Contains(same.Body.String(), `"username":"ada"`) {
		t.Fatalf("same=%d %s", same.Code, same.Body.String())
	}
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	db, h, _ := testHandler(t, false)
	a := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	b := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.6:1", "")
	cookieA := a.Result().Cookies()[0].Value
	cookieB := b.Result().Cookies()[0].Value
	rec := request(h, http.MethodPost, "/api/v1/me/password", `{"current_password":"correct-horse","new_password":"new-horse-1"}`, "http://127.0.0.1:5173", "203.0.113.5:1", cookieA)
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("change=%d %s", rec.Code, rec.Body.String())
	}
	meA := request(h, http.MethodGet, "/api/v1/me", "", "", "203.0.113.5:1", cookieA)
	if meA.Code != http.StatusOK {
		t.Fatalf("session A=%d %s", meA.Code, meA.Body.String())
	}
	meB := request(h, http.MethodGet, "/api/v1/me", "", "", "203.0.113.6:1", cookieB)
	assertCode(t, meB, http.StatusUnauthorized, "unauthenticated")
	old := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.7:1", "")
	assertCode(t, old, http.StatusUnauthorized, "invalid_credentials")
	ok := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"new-horse-1"}`, "http://127.0.0.1:5173", "203.0.113.7:1", "")
	if ok.Code != http.StatusOK {
		t.Fatalf("new login=%d %s", ok.Code, ok.Body.String())
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("sessions=%d want 2 (kept A plus new login)", n)
	}
}

func TestChangePasswordFields(t *testing.T) {
	db, h, _ := testHandler(t, false)
	login := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	cookie := login.Result().Cookies()[0].Value
	origin := "http://127.0.0.1:5173"
	var hashBefore string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE id = 1`).Scan(&hashBefore); err != nil {
		t.Fatal(err)
	}
	wrong := request(h, http.MethodPost, "/api/v1/me/password", `{"current_password":"wrong-horse","new_password":"new-horse-1"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, wrong, map[string]string{"current_password": "当前密码不正确"})
	same := request(h, http.MethodPost, "/api/v1/me/password", `{"current_password":"correct-horse","new_password":"correct-horse"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, same, map[string]string{"new_password": "新密码不能与当前密码相同"})
	short := request(h, http.MethodPost, "/api/v1/me/password", `{"current_password":"correct-horse","new_password":"short"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, short, map[string]string{"new_password": "密码至少 8 个字符"})
	var hashAfter string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE id = 1`).Scan(&hashAfter); err != nil {
		t.Fatal(err)
	}
	if hashAfter != hashBefore {
		t.Fatal("hash changed after field errors")
	}
	other := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, origin, "203.0.113.8:1", "")
	if other.Code != http.StatusOK {
		t.Fatal("other session lost")
	}
	emptyCurrent := request(h, http.MethodPost, "/api/v1/me/password", `{"current_password":"","new_password":"new-horse-1"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, emptyCurrent, map[string]string{"current_password": "请输入当前密码"})
}

func TestAccountWriteGuardOrder(t *testing.T) {
	_, h, _ := testHandler(t, false)
	login := request(h, http.MethodPost, "/api/v1/session", `{"username":"ada","password":"correct-horse"}`, "http://127.0.0.1:5173", "203.0.113.5:1", "")
	cookie := login.Result().Cookies()[0].Value
	origin := "http://127.0.0.1:5173"
	oversize := strings.Repeat("a", 4100)
	bigUser := `{"username":"` + oversize + `"}`
	bigPass := `{"current_password":"correct-horse","new_password":"` + oversize + `"}`

	for _, tc := range []struct {
		method, path, body, origin, cookie string
		status                             int
		code                               string
	}{
		{http.MethodPatch, "/api/v1/me", `{"username":"kevin"}`, "http://evil.example", cookie, 403, "origin_rejected"},
		{http.MethodPost, "/api/v1/me/password", `{"current_password":"correct-horse","new_password":"new-horse-1"}`, "http://evil.example", cookie, 403, "origin_rejected"},
		{http.MethodPatch, "/api/v1/me", `{"username":"kevin"}`, origin, "", 401, "unauthenticated"},
		{http.MethodPost, "/api/v1/me/password", `{"current_password":"correct-horse","new_password":"new-horse-1"}`, origin, "", 401, "unauthenticated"},
		{http.MethodPatch, "/api/v1/me", bigUser, "http://evil.example", cookie, 403, "origin_rejected"},
		{http.MethodPost, "/api/v1/me/password", bigPass, "http://evil.example", cookie, 403, "origin_rejected"},
		{http.MethodPatch, "/api/v1/me", bigUser, origin, "", 401, "unauthenticated"},
		{http.MethodPost, "/api/v1/me/password", bigPass, origin, "", 401, "unauthenticated"},
		{http.MethodPut, "/api/v1/me", `{"username":"kevin"}`, origin, cookie, 404, "not_found"},
		{http.MethodPost, "/api/v1/me", `{"username":"kevin"}`, origin, cookie, 404, "not_found"},
		{http.MethodGet, "/api/v1/me/password", "", origin, cookie, 404, "not_found"},
	} {
		rec := request(h, tc.method, tc.path, tc.body, tc.origin, "203.0.113.5:1", tc.cookie)
		assertCode(t, rec, tc.status, tc.code)
	}

	query := request(h, http.MethodPatch, "/api/v1/me?foo=1", `{"username":"kevin"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, query, map[string]string{"foo": "不支持的参数"})
	queryPass := request(h, http.MethodPost, "/api/v1/me/password?foo=1", `{"current_password":"correct-horse","new_password":"new-horse-1"}`, origin, "203.0.113.5:1", cookie)
	assertInvalidFields(t, queryPass, map[string]string{"foo": "不支持的参数"})

	tooBig := request(h, http.MethodPatch, "/api/v1/me", bigUser, origin, "203.0.113.5:1", cookie)
	assertCode(t, tooBig, http.StatusRequestEntityTooLarge, "body_too_large")
	tooBigPass := request(h, http.MethodPost, "/api/v1/me/password", bigPass, origin, "203.0.113.5:1", cookie)
	assertCode(t, tooBigPass, http.StatusRequestEntityTooLarge, "body_too_large")

	me := request(h, http.MethodGet, "/api/v1/me", "", "", "203.0.113.5:1", cookie)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"username":"ada"`) {
		t.Fatalf("username changed after guard failures: %s", me.Body.String())
	}
}
```

`TestChangePasswordRevokesOtherSessions` 里改密后会话数：当前 A 保留，B 删除，随后新登录再加 1，故为 2。若实现先删再插，以 HTTP 行为为准：A 仍 200，B 401，新密码能登录。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/httpapi -count=1 -run 'TestPatchMeUsername|TestPatchMeUsernameFields|TestChangePasswordRevokesOtherSessions|TestChangePasswordFields|TestAccountWriteGuardOrder'`

Expected: FAIL，PATCH `/api/v1/me` 现在走 `me` 的 GET-only 分支，返回 404。

- [ ] **Step 3: 实现**

`internal/auth/user.go` 增加：

```go
func UpdateUsername(ctx context.Context, db *sql.DB, username string, now time.Time) error {
	stamp := now.UTC().Format(time.RFC3339Nano)
	_, err := db.ExecContext(ctx, `UPDATE users SET username = ?, updated_at = ? WHERE id = 1`, username, stamp)
	return err
}

func ChangePassword(ctx context.Context, db *sql.DB, keepToken, currentPassword, newPassword string, now time.Time) error
```

立即事务照抄 `internal/catalog/tx.go` 的 `withImmediate`：`db.Conn` → `BEGIN IMMEDIATE` → 成功则 `COMMIT`，否则 `ROLLBACK`。不要 `sql.Tx` 套一层 `BEGIN IMMEDIATE`。密码流程：

1. 立即事务。
2. `SELECT password_hash FROM users WHERE id = 1`。
3. `auth.Verify(hash, currentPassword)` 失败则回滚并返回一个可识别错误（例如 `var ErrCurrentPassword = errors.New("current password")`）。
4. `hash, err := Hash(newPassword)`。
5. `UPDATE users SET password_hash = ?, updated_at = ? WHERE id = 1`。
6. `DELETE FROM sessions WHERE token_hash <> ?`，参数为当前 cookie 的 `tokenHash(keepToken)`（与 `Create`/`Lookup` 同一函数，必须在 `auth` 包内）。
7. Commit。

HTTP：`server.go` 的 `me` 改为：

```go
func (h *handler) me(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getMe(w, r)
	case http.MethodPatch:
		h.patchMe(w, r)
	default:
		h.notFound(w, r)
	}
}
```

把现有 GET 逻辑移到 `getMe`。`New` 里增加 `mux.HandleFunc("/api/v1/me/password", h.mePassword)`。`mePassword` 只接受 POST。

`patchMe` / `postPassword` 顺序：

1. `originOK`
2. 读 cookie；无 cookie 或 `Lookup` 得到 `ErrUnauthenticated` → 401 `unauthenticated`
3. `readLimited(..., 4096)`
4. `r.URL.RawQuery != ""` → `writeFields(w, queryRejected(r))`
5. 解码。`username` / `current_password` / `new_password` 用 `*string`。缺失或非字符串 → `invalid_body`
6. 字段短句按规格第 7 节。用户名：`TrimSpace` 后判空、64、空白。密码：当前空字符串、新长度、新等于当前。格式过了再 `ChangePassword`；`ErrCurrentPassword` → `fields.current_password`「当前密码不正确」
7. 成功：PATCH 200 `{"username": name}`；POST 204 空正文

字段错误可同时带 `current_password` 与 `new_password` 的格式问题。当前密码不正确只在格式都通过之后出现，且不要夹带其它 fields 键。

`utf8.RuneCountInString` 算用户名和密码长度。用户名中间空白用 `unicode.IsSpace`。

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/httpapi ./internal/auth -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

先 `git rev-parse --is-inside-work-tree`。不是仓库就跳过。不要 `git init`。

---

### Task 2: 纸色主题、登录页、页头

**Files:**
- Create: `web/src/theme.css`
- Modify: `web/src/main.tsx`
- Modify: `web/src/styles.module.css`
- Modify: `web/src/App.tsx`
- Modify: `web/src/pages.tsx`（`AppShell`）

- [ ] **Step 1: 主题文件**

`web/src/theme.css`：

```css
:root {
  --paper: #f4efe6;
  --paper-raised: #fffdf8;
  --ink: #2c261f;
  --ink-muted: #6b6258;
  --line: #d9d0c4;
  --clay: #8f4d36;
  --clay-press: #743d2b;
  --danger: #9f1239;
  --radius: 0.75rem;
  --radius-sm: 0.5rem;
  --font-display: "Songti SC", "STSong", "Noto Serif SC", "Noto Serif CJK SC", serif;
  --font-ui: "PingFang SC", "Hiragino Sans GB", "Noto Sans SC", "Noto Sans CJK SC", system-ui, sans-serif;
  --page-max: 40rem;
  --login-max: 22rem;
  --control-min: 2.75rem;
  --thumb: 4.5rem;
  --space: 1rem;
}

html,
body {
  margin: 0;
  background: var(--paper);
  color: var(--ink);
  font-family: var(--font-ui);
}

#root {
  min-height: 100%;
}

html,
body,
#root {
  min-height: 100%;
}
```

`main.tsx` 增加 `import "./theme.css";`

- [ ] **Step 2: 改 CSS Modules**

把 `.page` / `.appPage` / `.title` / `.field` / `.button` / `.error` / `.header` / `.nav` / `.itemLink` / `.itemThumb` / `.fileButton` 的硬编码色和 2px 圆角换成变量。新增：

```css
.loginPage {
  box-sizing: border-box;
  min-height: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1.5rem 1rem;
}

.loginPanel {
  box-sizing: border-box;
  width: 100%;
  max-width: var(--login-max);
  background: var(--paper-raised);
  border: 1px solid var(--line);
  border-radius: var(--radius);
  padding: 1.5rem;
}

.tagline {
  margin: 0 0 1.25rem;
  color: var(--ink-muted);
}

.brand {
  font-family: var(--font-display);
  font-size: 1.35rem;
  font-weight: 600;
  color: inherit;
  text-decoration: none;
  min-height: var(--control-min);
  display: inline-flex;
  align-items: center;
}

.buttonPrimary {
  font: inherit;
  padding: 0.4rem 0.9rem;
  min-height: var(--control-min);
  border: none;
  border-radius: var(--radius-sm);
  background: var(--clay);
  color: var(--paper-raised);
}

.buttonPrimary:active {
  background: var(--clay-press);
}

.nav a[aria-current="page"] {
  font-weight: 600;
  box-shadow: inset 0 -2px 0 var(--clay);
}

.user {
  margin: 0;
  margin-left: auto;
}

.header {
  position: sticky;
  top: 0;
  z-index: 2;
  background: var(--paper);
  border-bottom: 1px solid var(--line);
  padding: 0.35rem 0 0.6rem;
  margin: -1.25rem -1rem 1.25rem;
  padding-left: 1rem;
  padding-right: 1rem;
}
```

`.appPage` 的 `margin`/`padding` 要和 sticky 页头的负边距对齐，保证 390 不横滑。`.itemThumb` 宽高改为 `var(--thumb)`。`.error` 用 `var(--danger)`。输入框焦点 `outline: 2px solid var(--clay)`。`.button` 保持次要：透明底、`var(--line)` 边框、`var(--ink)` 字。

登录按钮、之后任务里的查找/新增/保存用 `buttonPrimary`。本任务先把登录按钮和页头改掉。

- [ ] **Step 3: 登录页**

`App.tsx` 的 `LoginPage`：`main.loginPage` > `div.loginPanel` > `h1.title`「有处」> `p.tagline`「家里的东西在哪儿」> 表单。登录按钮 `styles.buttonPrimary`。字段和 autocomplete 不变。

- [ ] **Step 4: AppShell**

去掉退出按钮和回收站链接。结构：

```tsx
<header className={styles.header}>
  <Link className={styles.brand} to="/" aria-label="有处">有处</Link>
  <nav className={styles.nav} aria-label="主要">
    <Link to="/" aria-current={itemHere ? "page" : undefined}>物品</Link>
    <Link to="/locations" aria-current={here("/locations") ? "page" : undefined}>位置</Link>
    <Link to="/categories" aria-current={here("/categories") ? "page" : undefined}>分类</Link>
    <Link to="/returns" aria-current={here("/returns") ? "page" : undefined}>待归位</Link>
  </nav>
  <Link className={styles.user} to="/account" aria-current={pathname === "/account" ? "page" : undefined}>
    {username}
  </Link>
</header>
```

`itemHere`：`pathname === "/" || pathname.startsWith("/items/")`。`here(prefix)`：`pathname === prefix || pathname.startsWith(prefix + "/")`。`/trash` 不高亮任何主入口。退出 mutation 暂时可留在账号页任务再用；本任务页头不再调用它。

`useLocation` 已有。页头 `flex-wrap`。390 下品牌、导航、用户名可换行，导航链接 `min-height: 2.75rem`。

- [ ] **Step 5: 构建**

Run: `cd web && npm run build`

Expected: PASS。`package-lock.json` 依赖项不增加。

- [ ] **Step 6: Commit**

不是 Git 仓库就跳过。不要 `git init`。

---

### Task 3: 账号页

**Files:**
- Modify: `web/src/api.ts`
- Modify: `web/src/pages.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: API**

```ts
export async function updateUsername(username: string): Promise<Me> {
  return sendJSON<Me>("/api/v1/me", "PATCH", { username }, "无法保存用户名");
}

export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  const response = await request(
    "/api/v1/me/password",
    { method: "POST", body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }) },
    "无法修改密码",
  );
  if (!response.ok) throw await parseError(response, "无法修改密码");
}
```

204 不要 `response.json()`。

- [ ] **Step 2: AccountPage**

在 `pages.tsx` 导出 `AccountPage`。四块：用户名表单、密码表单、`Link`「回收站」到 `/trash`、退出按钮。

用户名框受控，初值来自 `useQuery(["me"])` 的 `username`。保存成功：`queryClient.setQueryData(["me"], { username })`，提示「用户名已保存」。字段错误 `error.fields.username`。

密码：三个受控框。提交时若新密码与确认不一致，set 本地错误「两次输入的新密码不一致」，return。成功清空三框，提示「密码已修改」。`fields.current_password` / `fields.new_password` 显示在对应框下。`autocomplete` 按规格。按钮 `buttonPrimary`。退出沿用现有 `logout` mutation（从 AppShell 移到本页）。

- [ ] **Step 3: 路由**

`App.tsx`：`import { AccountPage, ...}`，在 `RequireAuth` 内 `<Route path="/account" element={<AccountPage />} />`。

AppShell 仍从 `me.data.username` 取名，改名后缓存更新则页头跟着变。

- [ ] **Step 4: 构建**

Run: `cd web && npm run build`

Expected: PASS

- [ ] **Step 5: Commit**

不是仓库就跳过。

---

### Task 4: 物品列表与物品表单排布

**Files:**
- Modify: `web/src/pages.tsx`
- Modify: `web/src/styles.module.css`（若还缺 `.itemName` / 主按钮用在列表）

- [ ] **Step 1: 物品列表**

查找 input：`type="search"`，`aria-label="找家里的东西"`，`placeholder="找家里的东西"`。提交按钮 `buttonPrimary`「查找」。同一 `searchRow` 右侧 `Link.buttonPrimary` 到 `/items/new`，文案「新增」（目标仍 `/items/new`）。「只看待定位」/「全部物品」用 `.button`，移到查找行下面。`details` 摘要仍是「筛选」。

`.itemLinkBody` 内名称一块、其下 `meta`/`path`。缩略图已由 CSS `--thumb` 变大。

位置列表「新增位置」、分类列表「新增分类」改为 `buttonPrimary`，可见文案「新增」，`to` 仍是 `/locations/new` 与 `/categories/new`。

- [ ] **Step 2: ItemFields 顺序与更多说明**

```tsx
function ItemFields({ draft, onChange, error, extrasOpen }: {
  draft: ItemDraft;
  onChange: (draft: ItemDraft) => void;
  error: ApiError | null;
  extrasOpen: boolean;
}) {
  // 名称
  // ItemLocationsField
  // ItemCategoriesField
  // <details className={styles.filters} open={extrasOpen}>
  //   <summary>更多说明</summary>
  //   别名、型号、规格、数量说明、备注
  // </details>
}
```

新增页：`<ItemFields extrasOpen={false} />`。编辑页：`extrasOpen` 为 alias/model/spec/quantityNote/note 任一非空。`details` 无受控 `onToggle` 也行：用 `key` 或原生 `open` 初始值。注意 React 里 `open={false}` 会一直受控收起。要用**非受控**初始：`<details defaultOpen={extrasOpen}>`。

保存按钮改为 `buttonPrimary`。`DeleteConfirm` 从保存所在 `<form>` 挪到表单后面、照片区和待归位之后、页尾。照片区和待归位仍在保存之后、删除之前。

- [ ] **Step 3: 构建**

Run: `cd web && npm run build`

Expected: PASS。不要改 `filters.ts` 的地址栏规则，不要改照片上传逻辑。FileList 仍须在清空 input 前拷到数组。

- [ ] **Step 4: Commit**

不是仓库就跳过。

---

### Task 5: README、工作记录、浏览器验收

**Files:**
- Modify: `README.md`
- Modify: `docs/工作记录.md`
- Modify: 若验收发现缺口，只改网页样式或文案，不动已通过的 Go 测试契约

- [ ] **Step 1: README**

第一句改为：本地可以单账号登录，在网页里改用户名和密码，登记物品和位置，管理分类，搜索筛选，登记待归位，把物品放进回收站，并给物品加照片。MCP 和部署还不在这里。

补一句：登录后点用户名进入账号，可改用户名和密码；回收站和退出也在账号页。

开发入口仍是 `http://127.0.0.1:5173`，Origin 与首次账号说明保留。

- [ ] **Step 2: 工作记录**

更新日期 2026-10-01。当前进度：登录到照片五段之后，插入了家用界面与账号；下一段仍是 MCP 与认证。不要把 MCP、OAuth、Docker、HEIC 写成已完成。记录：无新迁移；`PATCH /api/v1/me`、`POST /api/v1/me/password`；页头不再放回收站和退出。

- [ ] **Step 3: Go 测试与前端构建**

Run: `go test ./... -count=1`

Run: `cd web && npm run build`

Expected: 全绿。`package-lock.json` 依赖项不增加。`migrations/` 仍只有 `001`–`005`。

- [ ] **Step 4: 浏览器验收**

用仓库外 Playwright（`~/.cursor/skills/webapp-testing/scripts/with_server.py`），脚本放 `/tmp`，不要写进仓库。两个服务：

```bash
YOUCHU_PUBLIC_ORIGIN=http://127.0.0.1:5173 \
YOUCHU_USERNAME=ada \
YOUCHU_PASSWORD=correct-horse \
YOUCHU_DATA_DIR=<全新临时目录> \
go run ./cmd/youchu
```

和 `cd web && npm run dev`。浏览器只打开 `http://127.0.0.1:5173`。

按规格第 12 节走完：

1. 登录页有「有处」和「家里的东西在哪儿」。ada / correct-horse 进入。页头有物品、位置、分类、待归位、ada；没有「回收站」「退出」。
2. 点 ada 进账号，改用户名为 kevin，页头变成 kevin。改密码为 `new-horse-1`，见「密码已修改」。退出。ada 登录失败。kevin + `new-horse-1` 进入。
3. 查找框 placeholder 为「找家里的东西」。新增只填名称的物品。名称在最上，位置和分类在「更多说明」外，「更多说明」默认收起。保存后有照片区和待归位。页尾能移到回收站。账号页「回收站」看得到它。
4. `page.set_viewport_size({"width": 390, "height": 844})`，登录页、物品列表、物品页、账号页 `document.documentElement.scrollWidth == 390`。页头主入口可点。

把验收步骤和结果写入 `docs/工作记录.md`。

若 390 横滑：只改 CSS（`min-width: 0`、`max-width: 100%`、路径换行），不要改业务规则。

- [ ] **Step 5: Commit**

不是仓库就跳过。不要 `git init`。

---

## 计划自检

- 规格第 3 节视觉 → Task 2
- 第 4 节页头路由 → Task 2、Task 3
- 第 5 节登录 → Task 2
- 第 6 节账号页 → Task 3
- 第 7 节 HTTP → Task 1
- 第 8–9 节物品列表和表单 → Task 4
- 第 10 节其它页主按钮 → Task 4
- 第 11 节测试 → Task 1、Task 5
- 第 12 节验收 → Task 5
- 第 13 节 README / 无 git init → Task 5、各 Task 的 commit 步骤

无 TBD。不新增迁移、不新增 npm、不改照片 FileList 快照、不改物品 JSON。
