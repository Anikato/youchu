# 物品和位置 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在已有单账号登录之上，登记区域、固定储物位、移动容器和物品，并在中文网页里按位置查看、修改和永久删除。

**Architecture:** `migrations/002_items_locations.sql` 增加三张表，`id` 使用 `AUTOINCREMENT`。业务事务在 `internal/catalog`，用 `BEGIN IMMEDIATE` 做创建、修改和删除。`internal/httpapi` 先处理 Origin、会话、正文大小和 JSON 类型，再调用 catalog。网页仍由 Vite 提供，不新增依赖。

**Tech Stack:** 现有 Go 1.27.1、`modernc.org/sqlite` v1.60.1、React 19.3.0、Vite 8.3.1、TypeScript 7.0.2、React Router 8.4.0、TanStack Query 5.104.0。不新增 Go 模块或 npm 包。

规格：`docs/superpowers/specs/2026-09-29-items-locations-design.md`。

提交：本目录还不是 Git 仓库，也没有 `user.name`。每个提交步骤先检查 `git rev-parse --is-inside-work-tree` 和 `git config user.name`。仓库不存在就先 `git init`。用户名为空就停下来问用户，不要改 Git 配置，也不要编造作者。不要提交 `/data/`、`web/node_modules/` 或 `web/dist/`。

---

## 文件职责

- `migrations/002_items_locations.sql`：三张表和索引。
- `internal/catalog/errors.go`：可区分的业务错误。
- `internal/catalog/validate.go`：字符数、空白、控制字符。
- `internal/catalog/tx.go`：`BEGIN IMMEDIATE` 与只读事务。
- `internal/catalog/location.go`：位置的创建、列表、读取、修改、删除、路径。
- `internal/catalog/item.go`：物品和直接位置关联。
- `internal/httpapi/decode.go`：区分省略、`null` 和类型错误。
- `internal/httpapi/catalog_http.go`：物品和位置路由。
- `internal/httpapi/server.go`：注册路由；登录仍用 4096 字节上限，新写接口用 32768。
- `internal/httpapi/catalog_test.go`：规格第 8 节的 HTTP 测试。
- `web/src/api.ts`：物品和位置请求。
- `web/src/pages.tsx`：登录后的页面。
- `web/src/App.tsx`：路由。
- `web/src/styles.module.css`：登录后的宽度和控件高度。
- `README.md`：说明当前可以登记物品和位置。

路由不要写成 Go 1.22 的 `"GET /path"` 方法模式。那种写法对错误方法返回 405，规格要求 404。沿用现在的 `HandleFunc("/api/v1/...")`，在处理函数里用 `switch r.Method`，其余走 `notFound`。

`writeError` 继续只写 `code` 和 `message`。字段错误另写 `writeFields`，因为 `fields` 是对象，不能塞进 `map[string]string`。

---

### Task 1: 迁移 002

**Files:**
- Create: `migrations/002_items_locations.sql`
- Test: `internal/httpapi/catalog_test.go`

- [ ] **Step 1: 写失败测试**

`internal/httpapi/catalog_test.go`：

```go
package httpapi

import (
	"context"
	"io/fs"
	"strings"
	"testing"
	"time"

	"youchu/internal/migrate"
	"youchu/migrations"
)

func TestMigration002(t *testing.T) {
	db := migratedDB(t)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 2`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("version 2 rows = %d", n)
	}
	for _, name := range []string{"locations", "items"} {
		var sqlText string
		if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = ?`, name).Scan(&sqlText); err != nil {
			t.Fatal(name, err)
		}
		if !strings.Contains(sqlText, "AUTOINCREMENT") {
			t.Fatalf("%s missing AUTOINCREMENT: %s", name, sqlText)
		}
	}
	var indexSQL string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'locations_code_unique'`).Scan(&indexSQL); err != nil {
		t.Fatal(err)
	}
	files, err := loadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(context.Background(), db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 2`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("second apply rows = %d", n)
	}
}

func loadEmbedded() ([]migrate.File, error) {
	var files []migrate.File
	err := fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		version, err := migrate.ParseVersion(path)
		if err != nil {
			return err
		}
		body, err := fs.ReadFile(migrations.FS, path)
		if err != nil {
			return err
		}
		files = append(files, migrate.File{Version: version, Name: path, SQL: string(body)})
		return nil
	})
	return files, err
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/httpapi -run TestMigration002 -count=1`

Expected: FAIL。表或 `AUTOINCREMENT` 还不存在。

- [ ] **Step 3: 写入迁移，并改成真实的嵌入读取**

`migrations/002_items_locations.sql`：

```sql
CREATE TABLE locations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK (name <> ''),
    type TEXT NOT NULL CHECK (type IN ('area', 'fixed', 'movable')),
    code TEXT CHECK (code IS NULL OR code <> ''),
    parent_id INTEGER REFERENCES locations(id),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (parent_id IS NULL OR parent_id <> id),
    CHECK (
        (type = 'area' AND code IS NULL)
        OR (type IN ('fixed', 'movable') AND code IS NOT NULL)
    ),
    CHECK (type <> 'fixed' OR parent_id IS NOT NULL)
);

CREATE UNIQUE INDEX locations_code_unique ON locations(code) WHERE code IS NOT NULL;
CREATE INDEX locations_parent_id ON locations(parent_id);

CREATE TABLE items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK (name <> ''),
    alias TEXT CHECK (alias IS NULL OR alias <> ''),
    model TEXT CHECK (model IS NULL OR model <> ''),
    spec TEXT CHECK (spec IS NULL OR spec <> ''),
    quantity_note TEXT CHECK (quantity_note IS NULL OR quantity_note <> ''),
    note TEXT CHECK (note IS NULL OR note <> ''),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX items_name_id ON items(name, id);

CREATE TABLE item_locations (
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    location_id INTEGER NOT NULL REFERENCES locations(id),
    note TEXT CHECK (note IS NULL OR note <> ''),
    PRIMARY KEY (item_id, location_id)
);

CREATE INDEX item_locations_location_id ON item_locations(location_id);
```

- [ ] **Step 4: 测试通过**

Run: `go test ./internal/httpapi -run 'TestMigration002|TestLoginAndMe' -count=1`

Expected: PASS。已有登录测试仍然通过。

- [ ] **Step 5: Commit**

```bash
git add migrations/002_items_locations.sql internal/httpapi/catalog_test.go
git commit -m "feat: 增加物品和位置表"
```

---

### Task 2: 文本校验与 JSON 类型

**Files:**
- Create: `internal/catalog/errors.go`
- Create: `internal/catalog/validate.go`
- Create: `internal/catalog/validate_test.go`
- Create: `internal/httpapi/decode.go`
- Create: `internal/httpapi/decode_test.go`

- [ ] **Step 1: 写失败测试**

`internal/catalog/validate_test.go` 断言：

- `"  厨房\n"` 的名称错误是「名称不能包含控制字符」。
- `"  厨房  "` 得到 `"厨房"`。
- 81 个 `a` 得到「名称过长」。
- 编号 `" 001 "` 得到 `"001"`。
- 编号 `"0 01"` 得到「编号不能包含空白」。
- 备注 `"一行\n二行"` 通过，并保留换行。
- 备注里的 `U+0000` 得到「备注不能包含控制字符」。

`internal/httpapi/decode_test.go` 里的物品解码测试：

```go
func TestDecodeItemLocations(t *testing.T) {
	cases := []struct {
		body    string
		typeErr bool
		nullSet bool
		set     bool
	}{
		{`{"name":"钳","locations":"x"}`, true, false, false},
		{`{"name":"钳","locations":[1]}`, true, false, false},
		{`{"name":"钳","locations":[null]}`, true, false, false},
		{`{"name":"钳","locations":null}`, false, true, true},
		{`{"name":"钳"}`, false, false, false},
		{`{"name":"钳","version":1.0}`, true, false, false},
		{`{"name":"钳","version":0}`, false, false, false},
	}
	for _, tc := range cases {
		got, err := decodeItemWrite([]byte(tc.body))
		if tc.typeErr {
			if !errors.Is(err, errInvalidBody) {
				t.Fatalf("%s err=%v", tc.body, err)
			}
			continue
		}
		if err != nil || got.LocationsNull != tc.nullSet || got.LocationsSet != tc.set {
			t.Fatalf("%s got=%+v err=%v", tc.body, got, err)
		}
	}
	loc, err := decodeLocationWrite([]byte(`{"parent_id":null}`))
	if err != nil || !loc.ParentNull {
		t.Fatalf("parent null: %+v %v", loc, err)
	}
}
```

名称校验的失败测试直接调用 `catalog.NormalizeName`、`catalog.NormalizeCode` 和 `catalog.NormalizeNote`，断言上面列出的短句和规范化结果。`NormalizeName("  厨房  ")` 返回 `"厨房"` 和空错误。81 个 `a` 返回「名称过长」。`"  厨房\n"` 返回「名称不能包含控制字符」。`NormalizeCode(" 001 ")` 返回 `"001"`。`NormalizeCode("0 01")` 返回「编号不能包含空白」。`NormalizeNote("一行\n二行")` 保留换行。含 `U+0000` 的备注返回「备注不能包含控制字符」。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/catalog ./internal/httpapi -run 'TestValidate|TestDecode' -count=1`

Expected: FAIL，函数未定义。

- [ ] **Step 3: 实现校验和解析**

`internal/catalog/errors.go`：

```go
package catalog

import "errors"

var (
	ErrNotFound   = errors.New("not found")
	ErrVersion    = errors.New("version conflict")
	ErrParent     = errors.New("invalid parent")
	ErrCycle      = errors.New("location cycle")
	ErrInUse      = errors.New("location in use")
	ErrCodeTaken  = errors.New("code taken")
)

type FieldError struct {
	Fields map[string]string
}

func (e *FieldError) Error() string { return "invalid fields" }

func fieldError(m map[string]string) error {
	if len(m) == 0 {
		return nil
	}
	return &FieldError{Fields: m}
}
```

`internal/catalog/validate.go` 用 `strings.TrimSpace` 和 `utf8.RuneCountInString`。控制字符用 `unicode.Is(unicode.Cc, r)`。名称、编号、型号、别名、规格、数量说明不允许任何 Cc。备注和放置说明允许 `'\n'` 与 `'\r'`，其余 Cc 拒绝。空字符串表示可空字段应存 NULL。必填名称为空时返回「请填写名称」。编号必填为空时返回「请填写编号」。超长短句按规格表：名称过长、编号过长、型号过长、别名过长、规格过长、数量说明过长、备注过长、放置说明过长。

`internal/httpapi/decode.go`：

```go
var errInvalidBody = errors.New("invalid body")

func parseObject(body []byte) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil || obj == nil {
		return nil, errInvalidBody
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errInvalidBody
	}
	return obj, nil
}
```

整数解析：`json.Number` 的文本含 `.`、`e` 或 `E` 时返回 `errInvalidBody`。`Int64` 失败同样是类型错误。`version` 缺失由调用方记成字段错误，不在解码阶段产生。

`locations` 的值去掉空白后如果是 `null`，记下 `LocationsNull`。如果首字符不是 `[`，返回 `errInvalidBody`。数组元素去掉空白后首字符不是 `{`，返回 `errInvalidBody`。元素内 `location_id` 缺失或 JSON `null` 先记在解码结果里，由 Task 5 在字段规则阶段返回「位置格式不正确」；这一步只拒绝类型。`location_id` 带小数返回 `errInvalidBody`。

位置的 `parent_id: null` 记下 `ParentNull`，不返回 `errInvalidBody`。`name` 或 `type` 为 `null` 返回 `errInvalidBody`。`code` 和可空文字允许字符串或 `null`。

- [ ] **Step 4: 测试通过**

Run: `go test ./internal/catalog ./internal/httpapi -run 'TestValidate|TestDecode' -count=1`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/catalog internal/httpapi/decode.go internal/httpapi/decode_test.go
git commit -m "feat: 校验物品文本并区分 JSON 类型"
```

---

### Task 3: 位置的创建、读取和列表

**Files:**
- Create: `internal/catalog/tx.go`
- Create: `internal/catalog/location.go`
- Modify: `internal/httpapi/server.go`
- Create: `internal/httpapi/catalog_http.go`
- Modify: `internal/httpapi/catalog_test.go`

- [ ] **Step 1: 写失败测试**

在 `catalog_test.go` 增加 `cookie := login(t, h)`，复用 `server_test.go` 的 `testHandler` 和 `request`。登录用现有 `POST /api/v1/session`。

覆盖规格测试 3 和 4 的成功与父级部分，以及列表：

- 创建区域 `{"name":"厨房","type":"area"}` 返回 201，`version` 为 1，`code` 和 `parent_id` 为 null，`path` 只有自己。
- 区域再提交 `"code":"001"` 返回 400，`fields.code` 为「区域不使用编号」，数据库行数不变。
- 固定储物位不带父级返回 400，`fields.parent_id` 为「请选择父级」。
- 移动容器的父级是区域、区域的父级是移动容器，返回 400 `invalid_parent`，不创建。
- 区域下建区域、区域下建固定储物位 `001`、固定储物位下建移动容器 `014`、移动容器不带父级，都返回 201。
- 另一个位置再使用 `001` 返回 409 `code_taken`。`01` 可以创建。读取 `001` 时 `code` 仍是 `"001"`。
- `GET /api/v1/locations` 只含 `parent_id` 为空的行。`parent={区域 id}` 含直接子级，不含孙级。
- 创建 101 个名为 `区域-001` 到 `区域-101` 的根区域。`GET /api/v1/locations?offset=100` 的 `data` 含 `区域-101`，`total` 为 101。`flat=1&limit=100&offset=100` 和 `eligible_parent_for=area&limit=100&offset=100` 也都含 `区域-101`。
- `flat=1` 与 `parent` 同时出现返回 400，`fields` 里有对应参数。
- `parent=0` 返回 400，`fields.parent` 为「参数不正确」。
- 已登录的 `GET /api/v1/locations` 不带 Origin 也返回 200。
- `POST /api/v1/locations/{id}` 返回 404 `not_found`，不是 405。
- `eligible_parent_for=area&exclude={父区域}` 的结果不含该区域，也不含它下面的子区域。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/httpapi -run 'TestCreateLocation|TestLocationLists' -count=1`

Expected: FAIL，路由是 404。

- [ ] **Step 3: 实现位置读写**

`tx.go` 从 `db.Conn` 执行 `BEGIN IMMEDIATE`，失败时 `ROLLBACK`，成功时 `COMMIT`。列表用 `BEGIN` 后在同一连接上先 `COUNT(*)` 再查当页，然后 `COMMIT`。`data` 为空时编码成 `[]`，不要编码成 `null`。

`location.go` 的创建顺序：校验名称、类型和编号；固定储物位没有父级时返回 `parent_id=请选择父级`；在立即事务里确认父级存在且类型匹配；不匹配返回 `ErrParent`；插入时不写 `id`。时间用 `now.UTC().Format(time.RFC3339Nano)`。唯一索引冲突返回 `ErrCodeTaken`。

父级规则：

- `area`：无父级，或父级是 `area`。
- `fixed`：父级是 `area` 或 `fixed`。
- `movable`：无父级，或父级是 `fixed` 或 `movable`。

路径从当前行沿 `parent_id` 向上收集，反转后根在前。最后一项的 `id` 就是该位置。遇到重复 id 时返回普通错误，HTTP 层把它变成 500，避免坏数据死循环。

列表排序：

- 根和子级：`CASE type WHEN 'area' THEN 0 WHEN 'fixed' THEN 1 ELSE 2 END`，然后 `code IS NULL`，然后 `code, name, id`。
- `flat` 和合法父级：`ORDER BY id`。

合法父级：`area` 只取区域，`fixed` 取区域和固定储物位，`movable` 取固定储物位和移动容器。`exclude` 用递归 CTE 去掉该位置和全部下级。`exclude` 不存在返回 `ErrNotFound`。`parent` 不存在也返回 `ErrNotFound`。

`server.go` 的 `New` 增加精确注册，不要在模式里写 HTTP 方法：

```go
mux.HandleFunc("/api/v1/locations", h.locationsCollection)
mux.HandleFunc("/api/v1/locations/{id}", h.locationByID)
```

`locationsCollection` 只接受 GET 和 POST。`locationByID` 在本任务只接受 GET。id 不匹配 `^[1-9][0-9]*$` 时返回 404。

写请求调用顺序固定为：方法不对则 404；`originOK`；`currentUser`；`readLimited(w, r, 32768)`；`parseObject`；解码。类型错误返回 `invalid_body`。把现有 `readLimited` 改成接受字节上限。`login` 和 `logout` 仍传 4096。

`currentUser` 没有 Cookie 或 `auth.Lookup` 失败时写 401 `unauthenticated`，message「未登录」。

查询参数每个键只允许出现一次。`limit` 省略为 30，必须是 1 到 100 的无前导零正整数，否则 `fields.limit` 为「数量超出范围」。`offset` 省略为 0，允许 `0` 或无前导零正整数，否则 `fields.offset` 为「起点不正确」。`flat` 只能是 `1`。`eligible_parent_for` 只能是 `area`、`fixed`、`movable`。同时出现 `parent`、`flat`、`eligible_parent_for` 中的两个，返回「不支持的参数」。

- [ ] **Step 4: 测试通过**

Run: `go test ./internal/httpapi ./internal/catalog -count=1`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/catalog internal/httpapi
git commit -m "feat: 创建并列出位置"
```

---

### Task 4: 移动、循环和删除位置

**Files:**
- Modify: `internal/catalog/location.go`
- Modify: `internal/httpapi/catalog_http.go`
- Modify: `internal/httpapi/catalog_test.go`

- [ ] **Step 1: 写失败测试**

- 使用当前版本把位置父级改成它的下级，返回 409 `location_cycle`，`parent_id` 和 `version` 不变。
- 父级改成自己同样返回 `location_cycle`。
- 合法 PATCH 即使名称相同，`version` 也加 1，`created_at` 不变，`updated_at` 变化。
- PATCH 只有 `version` 时返回 400，`fields.request` 为「没有要修改的内容」。
- PATCH 出现 `type` 时返回 400，`fields.type` 为「类型不能修改」，即使值与当前类型相同。
- 过旧 `version` 加上会形成循环的父级，返回 409 `version_conflict`，不是 `location_cycle`。
- 删除仍有子位置的位置返回 409 `location_in_use`，行还在。
- 删除空位置返回 204。再 GET 返回 404。
- 删除最大 id 的空位置后新建，新 id 不同。用旧 id 和 `version=1` 去 PATCH 和 DELETE 都返回 404，新位置名称不变。
- 过旧版本 DELETE 返回 409，行还在。
- 无会话 PATCH 返回 401。错误 Origin 的 PATCH 返回 403，名称不变。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/httpapi -run 'TestMoveLocation|TestDeleteLocation|TestLocationVersion' -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现修改和删除**

PATCH 在立即事务里按 id 读取。没有行返回 `ErrNotFound`。然后做字段规则，再比较版本，最后检查父级和循环。版本过旧时直接返回 `ErrVersion`，不再查父级。

沿新父级向上走。走到自己返回 `ErrCycle`。父级不存在或类型不合法返回 `ErrParent`。`parent_id: null` 对区域和移动容器是去掉父级；对固定储物位返回「请选择父级」。区域的非空编号返回「区域不使用编号」。固定储物位和移动容器的 `code: null` 或空白返回「编号不能清空」。

更新语句带 `WHERE id = ? AND version = ?`。影响行数是 0 时再读一次：行没有了就 `ErrNotFound`，版本变了就 `ErrVersion`。成功时 `version = version + 1`。

删除前在同一事务确认没有子行、`item_locations` 没有引用。有引用返回 `ErrInUse`。版本检查在引用检查之前。

HTTP 把错误映射为：

| 错误 | 状态 | code | message |
| --- | --- | --- | --- |
| `ErrNotFound` | 404 | `not_found` | 未找到 |
| `*FieldError` | 400 | `invalid_fields` | 有字段不符合要求 |
| `ErrParent` | 400 | `invalid_parent` | 不能放在这个父级下 |
| `ErrVersion` | 409 | `version_conflict` | 记录已被修改 |
| `ErrCycle` | 409 | `location_cycle` | 不能移到自己的下级 |
| `ErrInUse` | 409 | `location_in_use` | 这个位置下面还有内容 |
| `ErrCodeTaken` | 409 | `code_taken` | 编号已被使用 |

DELETE 不读正文。`version` 查询参数不合法时，在查数据库之前返回 400 `fields.version`「版本不正确」。因此 id 不存在且 `version=abc` 返回 400。`version=1` 但 id 不存在返回 404。

- [ ] **Step 4: 测试通过**

Run: `go test ./internal/httpapi ./internal/catalog -count=1`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/location.go internal/httpapi/catalog_http.go internal/httpapi/catalog_test.go
git commit -m "feat: 移动并删除位置"
```

---

### Task 5: 物品和直接位置

**Files:**
- Create: `internal/catalog/item.go`
- Modify: `internal/httpapi/catalog_http.go`
- Modify: `internal/httpapi/server.go`
- Modify: `internal/httpapi/catalog_test.go`

- [ ] **Step 1: 写失败测试**

覆盖规格第 8 节里还没落地的物品条款，包括：

- 只提交 `{"name":"温湿度传感器"}` 返回 201，`locations` 为 `[]`，`version` 为 1。同名再创建一条也成功。
- 一个移动容器里两件物品。容器改挂到另一个固定储物位后，两件物品的 `location_id` 仍是容器。路径含新的直接父级 id，不含旧的直接父级 id，共同的更高层还在。路径最后一项的 id 等于 `location_id`。
- 一件物品两个位置，说明分别是「车和遥控器」和「备用轮胎」。读取时按 `location_id` 升序，两条说明都在。
- 只挂在子位置上的物品不出现在 `location={父级 id}`。父级的子位置列表能看到该子位置。
- `placement=unlocated` 只返回没有关联的物品。加上位置后离开该列表。`placement=any` 返回 400。`placement` 与 `location` 同时出现返回 400。
- PATCH 省略 `alias` 时原值保留。`alias: null` 后读取为 null。`locations: []` 变为待定位。省略 `locations` 时关联保留。
- 相同名称的合法 PATCH 使 `version` 从 1 变为 2。
- 删除物品后，物品和关联都不在，位置还在。
- 删除被物品引用的位置返回 `location_in_use`。
- 删除最大物品 id 后新建，旧 id 加 `version=1` 的 PATCH 和 DELETE 都是 404，新物品不变。
- 名称合法但版本过旧：PATCH 和 DELETE 都是 `version_conflict`，新名称保留，删除不发生。
- 名称空着且版本过旧：返回 400，`fields.name` 为「请填写名称」，库里的名称不变。
- 不存在的 id 配上 `{` 返回 `invalid_body`。不存在的 id 配上类型正确但名称为空的 JSON 返回 404。
- 已存在物品的 `locations` 为字符串、`[1]` 或 `[null]` 时返回 `invalid_body`，响应没有 `fields`。`locations: null` 返回 `invalid_fields`，短句「位置格式不正确」。
- 重复的 `location_id` 返回「同一位置只能关联一次」，不写入。小于 1 的 `location_id` 返回「所选位置不存在」。
- 无会话创建返回 401，没有新行。错误 Origin 创建返回 403，没有新行。
- 已登录且 Origin 正确时，超过 32768 字节的物品 POST 返回 413。Origin 正确但无会话的超长 POST 返回 401。Origin 不正确的超长 POST 返回 403。登录 POST 超过 4096 仍返回 413。
- `DELETE /api/v1/items/999999?version=abc` 返回 400，不是 404。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/httpapi -run TestItem -count=1`

Expected: FAIL。

- [ ] **Step 3: 实现物品**

注册：

```go
mux.HandleFunc("/api/v1/items", h.itemsCollection)
mux.HandleFunc("/api/v1/items/{id}", h.itemByID)
```

创建可以只有名称。`locations` 省略或 `[]` 都表示没有关联。`locations: null` 在确认类型之后、写入之前返回字段错误；创建没有既有 id，所以它在字段规则阶段返回，不先查一条不存在的物品。

替换关联在物品版本事务里完成：检查版本，删除旧 `item_locations`，插入新行，`version` 加 1。某个位置不存在则整次回滚。关联 `note` 省略或 null 存 NULL。列表和详情的 `locations` 按 `location_id` 升序，每条带完整路径。

物品列表排序 `name, id`。`placement=unlocated` 是 `NOT EXISTS` 关联。`location={id}` 位置不存在时 404，存在但没有物品时 `data` 为空、`total` 为 0。

`COUNT(*)` 和当页查询放在同一个 `BEGIN` 里。

删除物品只删 `items` 行，依赖 `ON DELETE CASCADE` 删除关联。

超长正文在会话检查之后才读。无会话时不读到 413。Origin 失败时也不读到 413。实现上先判断 Origin 和会话，再 `readLimited`。

- [ ] **Step 4: 测试通过**

Run: `go test ./... -count=1`

Expected: PASS。`cmd/youchu` 没有测试也可以。

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/item.go internal/httpapi
git commit -m "feat: 登记物品和直接位置"
```

---

### Task 6: 登录后的网页

**Files:**
- Modify: `web/src/api.ts`
- Modify: `web/src/App.tsx`
- Create: `web/src/pages.tsx`
- Modify: `web/src/styles.module.css`
- Modify: `README.md`

- [ ] **Step 1: 扩展 API**

`web/src/api.ts` 保留 `fetchMe`、`login`、`logout`。增加位置和物品类型，字段与规格 JSON 一致。`request` 统一带 `credentials: "same-origin"`。401 抛出 `code === "unauthenticated"` 的错误，由页面转到 `/login`。

```ts
export async function fetchAllLocations(params: URLSearchParams): Promise<Location[]> {
  const out: Location[] = [];
  let offset = 0;
  for (;;) {
    const page = await getPage<Location>("/api/v1/locations", params, 100, offset);
    out.push(...page.data);
    if (out.length >= page.total || page.data.length === 0) {
      return out;
    }
    offset += page.data.length;
  }
}
```

物品存放位置使用 `flat=1`。父级使用 `eligible_parent_for`，编辑时再加 `exclude`。两种选择器都走 `fetchAllLocations`，不能只取第一页。

- [ ] **Step 2: 页面**

`App.tsx` 路由：

- `/login` 保持现有登录页。
- `/` 物品列表。
- `/items/new` 与 `/items/:id` 物品表单。
- `/locations` 顶层位置。
- `/locations/new` 新增位置。
- `/locations/:id` 位置页。
- 其余进入 `/`。

登录后的页头显示用户名、「物品」、「位置」和「退出」。页面容器增加 `appPage`：`max-width: 40rem`。登录页继续用原来的 `page`。`appPage` 里的 `input`、`select`、`textarea` 和 `button` 设 `min-height: 2.75rem`。390 宽下不设会撑出横向滚动的固定宽度；路径用普通换行。

物品列表读 `offset` 和 `placement=unlocated`。有下一页、上一页、「只看待定位」和「全部物品」。空列表分别显示「还没有物品」和「没有待定位的物品」。路径用 ` / ` 连接，有编号时在名称后加一个空格和编号。没有位置显示「待定位」。

顶层位置页不传 `limit`，使用响应里的 `limit`（默认 30）计算下一页 `offset`。本页记录分成「区域」和「尚未放入的容器」，只渲染本页实际出现的组。`total` 大于 `offset + data.length` 时显示「下一页」。

位置页用 `children` 和 `items` 两个 offset。快速登记只提交名称和当前 `location_id`。成功后清空名称并刷新列表。失败时保留名称。子位置和直接物品都有上一页、下一页。空文案分别是「这里还没有下一级位置」和「这个位置里还没有物品」。

删除确认先显示「永久删除，无法恢复」，再显示「确认删除」和「取消」。第一次按「删除」不发请求。

物品表单用下面的方式保住草稿。位置表单对名称、编号和父级使用同一规则。

```tsx
const [formReady, setFormReady] = useState(false);
const [name, setName] = useState("");
const [note, setNote] = useState("");
const [locations, setLocations] = useState<ItemLocation[]>([]);
const [version, setVersion] = useState(0);
const [conflict, setConflict] = useState(false);

useEffect(() => {
  if (!item || formReady) {
    return;
  }
  setName(item.name);
  setNote(item.note ?? "");
  setLocations(item.locations);
  setVersion(item.version);
  setFormReady(true);
}, [item, formReady]);

function applyServer(next: Item) {
  setName(next.name);
  setNote(next.note ?? "");
  setLocations(next.locations);
  setVersion(next.version);
  setConflict(false);
}
```

`useQuery` 再次返回时，因为 `formReady` 已经是 true，不能再调用 `applyServer`。保存若得到 `version_conflict`，只执行 `setConflict(true)`。不修改 `name`、`note`、`locations` 或 `version`，也不自动重新提交。点击「加载最新内容」时手动 GET，成功后才调用 `applyServer`。GET 得到 404 时显示「未找到」并回到列表。删除冲突在 `applyServer` 之后收回确认。

新增位置在地址带 `parent` 时，父级固定，类型只给出规格允许的子类型。「重新选择父级」链到不带查询参数的 `/locations/new`。父级读不到时显示 message，不提交。

页面上不出现搜索框、分类、照片、归位或回收站。

- [ ] **Step 3: 构建**

Run: `cd web && npm run build`

Expected: `tsc --noEmit` 和 Vite 构建成功。不新增依赖，不改 `package.json` 的依赖版本。

- [ ] **Step 4: 更新 README**

把第一句改成：本地可以单账号登录，并登记物品和位置。分类、搜索、归位、回收站、照片、MCP 和部署还不在这里。

保留两个进程、`http://127.0.0.1:5173`、Origin、首次账号、会话和数据目录的说明。补一句：顶层位置超过 30 条时用下一页；选择存放位置或父级时会继续请求后续页。

- [ ] **Step 5: Commit**

```bash
git add web/src README.md
git commit -m "feat: 增加物品和位置页面"
```

---

### Task 7: 浏览器验证

**Files:**
- 不把 Playwright 或脚本放进仓库。脚本写在 `/tmp/youchu-items-check.py`，跑完删除。

规格第 9 节的桌面流程、390 宽流程都要做。另外补上本次复核要求的两项，不能少：

1. 超过 30 个顶层位置、超过 100 个候选位置时，分页和两个选择器都能到达最后一条。
2. 版本冲突时，备注和位置选择都还在，不只看名称。

- [ ] **Step 1: 准备账号和 101 个根区域**

用临时 `YOUCHU_DATA_DIR` 启动 Go 和 Vite。浏览器只打开 `http://127.0.0.1:5173`。

脚本登录后，用接口创建 `区域-001` 到 `区域-101`。这 101 个都是根区域，所以它们同时是顶层列表、`flat=1` 和 `eligible_parent_for=area` 的候选。

- [ ] **Step 2: 分页和两个选择器**

在 `/locations` 连续按「下一页」，直到看到 `区域-101`。第一页只有 30 条时不能已经看到它。

打开 `/items/new`。存放位置选择器里要有 `区域-101`。选择器发出的请求必须包含 `flat=1`，并且在 `total` 大于 100 时出现 `offset=100`。

打开 `/locations/new`，类型选固定储物位。父级选择器里要有 `区域-101`。请求必须包含 `eligible_parent_for=fixed` 或 `eligible_parent_for=area` 中规格允许的那一个；固定储物位的父级包含区域，所以 `eligible_parent_for=fixed` 的后续页里要有 `区域-101`。

- [ ] **Step 3: 规格第 9 节的桌面流程**

登录；创建区域「厨房」；其下创建「洗衣机柜最上层」编号 `001`；再创建「左下格」编号 `002`；在 `001` 下创建「传感器盒」编号 `014`；在盒子页连续添加「温湿度传感器」和「门磁传感器」；把盒子父级改到 `002`；路径文本是「厨房 / 左下格 002 / 传感器盒 014」；两件物品仍在盒子页；打开 `001` 看不到这两件物品；删除「门磁传感器」时能看到「永久删除，无法恢复」，确认后列表不再有它。

- [ ] **Step 4: 版本冲突保留备注和位置选择**

准备两个位置，例如 `001` 和 `002`。创建物品「水晶头钳」，先挂在 `001`，备注为空。

打开物品页，不要保存：

- 名称改成「未保存的名称」。
- 备注改成「未保存的备注」。
- 存放位置改成 `002`，或在 `001` 之外再选中 `002`。

用另一个已登录请求 PATCH 这件物品，把备注改成「其他会话的备注」，并把位置改回只有 `001`。

回到前一个页面按「保存」。然后同时断言三件事：

- 名称输入仍是「未保存的名称」。
- 备注输入仍是「未保存的备注」。
- 存放位置选择仍是用户刚选的 `002`，没有被其他会话的 `001` 换掉。

页面显示「记录已被修改」和「加载最新内容」。这个时刻不得自动把表单改成服务器内容。

点击「加载最新内容」后，备注变为「其他会话的备注」，位置变为 `001`。

- [ ] **Step 5: 390 宽**

视口 390×844 打开盒子页。路径换行可见，`document.documentElement.scrollWidth` 不大于视口宽度。再快速添加「水晶头钳」，列表中出现它。

- [ ] **Step 6: 收尾**

停掉 Go 和 Vite。删除 `/tmp/youchu-items-check.py`。不要把临时数据库留在仓库里。

Run: `go test ./... -count=1` 和 `cd web && npm run build`

Expected: 两者都成功。

- [ ] **Step 7: Commit**

只有 README 或源码在验证中又改了才提交。验证脚本不提交。

```bash
git add README.md web/src internal
git commit -m "test: 核对物品位置的分页和冲突草稿"
```

没有新的源码改动就不要做空提交。

---

## 自查

- 规格第 4 到 6 节的表、父级、版本、删除和错误顺序分别在 Task 1、3、4、5。
- `locations` 不是数组或元素不是对象走 `invalid_body`；`null`、缺少位置 id 和重复关联走 `invalid_fields`。这在 Task 2 和 Task 5。
- 101 条候选的接口断言在 Task 3。页面上的分页和两个选择器在 Task 7。
- 冲突时保留备注和位置选择在 Task 6 的 state 规则，并在 Task 7 用浏览器同时检查。名称也保留，但不能只检查名称。
- 不实现分类、搜索、待归位、回收站、照片、MCP、OAuth 和 Docker。
