# 分类和搜索 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在已有物品和位置之上，建立单父级分类树，给物品挂多个分类，并在物品列表用关键词、分类、位置范围、待定位和未分类查找。

**Architecture:** `migrations/003_categories_search.sql` 增加 `categories` 和 `item_categories`。分类 CRUD 在 `internal/catalog/category.go`。物品关联和列表筛选仍在 `internal/catalog/item.go`，全部匹配按每个所选分类的分支分别 `EXISTS`，再按物品去重分页。`internal/httpapi` 先处理 Origin、会话、正文和查询形状，再调用 catalog。网页不新增依赖。

**Tech Stack:** 现有 Go 1.27.1、`modernc.org/sqlite` v1.60.1、React 19.3.0、Vite 8.3.1、TypeScript 7.0.2、React Router 8.4.0、TanStack Query 5.104.0。不新增 Go 模块或 npm 包。

规格：`docs/superpowers/specs/2026-09-30-categories-search-design.md`。

提交：本目录还不是 Git 仓库，也没有 `user.name`。每个提交步骤先检查 `git rev-parse --is-inside-work-tree` 和 `git config user.name`。仓库不存在就先 `git init`。用户名为空就停下来问用户，不要改 Git 配置，也不要编造作者。不要提交 `/data/`、`web/node_modules/` 或 `web/dist/`。

---

## 文件职责

- `migrations/003_categories_search.sql`：两张表和两条部分唯一索引。
- `internal/catalog/errors.go`：增加 `ErrNameTaken`。循环和占用仍用 `ErrCycle` / `ErrInUse`，由分类 HTTP 映射成 `category_cycle` / `category_in_use`。
- `internal/catalog/validate.go`：增加 `NormalizeKeyword`。分类名继续用 `NormalizeName`。
- `internal/catalog/category.go`：分类的创建、列表、读取、修改、删除、路径。
- `internal/catalog/item.go`：`categories` 关联、列表筛选 SQL。
- `internal/httpapi/decode.go`：物品正文的 `categories`；分类正文。
- `internal/httpapi/catalog_http.go`：分类路由；扩展物品查询参数和 JSON。
- `internal/httpapi/server.go`：注册 `/api/v1/categories`。
- `internal/httpapi/catalog_test.go`：规格第 8 节。
- `internal/httpapi/decode_test.go`：`categories` 数组类型。
- `web/src/api.ts`：分类请求；物品带 `categories`。
- `web/src/filters.ts`：物品列表地址栏条件的读写和依附参数清理。
- `web/src/pages.tsx`：页头、物品列表搜索筛选、物品表单分类、分类各页。
- `web/src/App.tsx`：分类路由。
- `web/src/styles.module.css`：筛选摘要和展开区。
- `README.md`：说明可以管理分类并搜索。

路由不要写成 Go 1.22 的 `"GET /path"` 方法模式。沿用 `HandleFunc("/api/v1/...")` 加 `switch r.Method`。

`writeCatalogError` 保持位置的 `location_cycle` / `location_in_use`。分类写路由用 `writeCategoryError`。

---

### Task 1: 迁移 003

**Files:**
- Create: `migrations/003_categories_search.sql`
- Modify: `internal/httpapi/catalog_test.go`

- [ ] **Step 1: 写失败测试**

在 `TestMigration002` 旁增加：

```go
func TestMigration003(t *testing.T) {
	db := migratedDB(t)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 3`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("version 3 rows = %d", n)
	}
	var sqlText string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'categories'`).Scan(&sqlText); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sqlText, "AUTOINCREMENT") {
		t.Fatalf("categories missing AUTOINCREMENT: %s", sqlText)
	}
	for _, name := range []string{"categories_root_name", "categories_sibling_name"} {
		var indexSQL string
		if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = ?`, name).Scan(&indexSQL); err != nil {
			t.Fatal(name, err)
		}
		if !strings.Contains(indexSQL, "UNIQUE") {
			t.Fatalf("%s not unique: %s", name, indexSQL)
		}
	}
	var pk string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'item_categories'`).Scan(&pk); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pk, "PRIMARY KEY") {
		t.Fatalf("item_categories pk: %s", pk)
	}
	files, err := loadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(context.Background(), db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 3`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("second apply rows = %d", n)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/httpapi -run TestMigration003 -count=1`

Expected: FAIL。版本 3 或表还不存在。

- [ ] **Step 3: 写入迁移**

`migrations/003_categories_search.sql`：

```sql
CREATE TABLE categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK (name <> ''),
    parent_id INTEGER REFERENCES categories(id),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE UNIQUE INDEX categories_root_name
    ON categories(name) WHERE parent_id IS NULL;
CREATE UNIQUE INDEX categories_sibling_name
    ON categories(parent_id, name) WHERE parent_id IS NOT NULL;
CREATE INDEX categories_parent_id ON categories(parent_id);

CREATE TABLE item_categories (
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    category_id INTEGER NOT NULL REFERENCES categories(id),
    PRIMARY KEY (item_id, category_id)
);

CREATE INDEX item_categories_category_id ON item_categories(category_id);
```

不要写 `UNIQUE(parent_id, name)` 作为根分类约束。`migratedDB` 已经走嵌入 FS，加上 `003` 文件后会自动执行。

- [ ] **Step 4: 测试通过**

Run: `go test ./internal/httpapi -run 'TestMigration00' -count=1`

Expected: PASS。`TestMigration002` 仍通过。

- [ ] **Step 5: Commit**

```bash
git add migrations/003_categories_search.sql internal/httpapi/catalog_test.go
git commit -m "feat: 增加分类表迁移"
```

若没有仓库或 `user.name`，按文件头说明停下。

---

### Task 2: 分类 CRUD

**Files:**
- Modify: `internal/catalog/errors.go`
- Create: `internal/catalog/category.go`
- Modify: `internal/httpapi/decode.go`
- Modify: `internal/httpapi/catalog_http.go`
- Modify: `internal/httpapi/server.go`
- Modify: `internal/httpapi/catalog_test.go`

- [ ] **Step 1: 写失败测试**

在 `catalog_test.go` 增加分类辅助类型和函数，形状对齐 `locBody`，路径项只有 `id`、`name`：

```go
type catPathNode struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type catBody struct {
	ID        int64         `json:"id"`
	Name      string        `json:"name"`
	ParentID  *int64        `json:"parent_id"`
	Version   int64         `json:"version"`
	CreatedAt string        `json:"created_at"`
	UpdatedAt string        `json:"updated_at"`
	Path      []catPathNode `json:"path"`
}
```

`decodeCat` 在 `path` 为 null、最后一项 id 不等于分类 id、或 path 项出现 `type`/`code` 键时失败。用 `json.RawMessage` 确认路径对象没有这两个键。

覆盖规格第 8 节 3、4、10、12、13、14、15、16（分类部分）：

`TestCategoryNameTaken`：

- `POST {"name":"电子配件"}` 201，`parent_id` 为 null，`path` 一项。
- 再 `POST {"name":"  电子配件  "}` 409 `name_taken`，`SELECT COUNT(*) FROM categories` 仍为 1。
- 两个根下各建「传感器」都 201。
- 把其中一个 `PATCH {"version":1,"parent_id":<另一个的父级>}` 409 `name_taken`，`parent_id` 不变。
- 同级改成已有名称 409 `name_taken`。
- `PATCH {"version":1,"name":"传感器"}` 对自己当前名称 200，`version` 为 2。

`TestCategoryCycle`：根 A 下建 B，再把 A 的父级改成 B，409 `category_cycle`，A 的 `parent_id` 仍为 null，`version` 仍为 1。`POST {"name":"x","parent_id":999999}` 400 `invalid_parent`。

`TestCategoryDelete`：有子分类时 DELETE 409 `category_in_use`。空分类 `?version=` 当前版本 204。删除后 GET 404。

`TestCategoryAuth`：无会话 POST 401，行数为 0。`Origin: http://evil.example` POST 403，行数为 0。`PATCH /api/v1/categories/abc` 错误 Origin 403。已登录 Origin 正确时，超过 32768 字节的 POST 413。

`TestCategoryDetailQuery`：已登录 `GET /api/v1/categories/{id}?foo=1` 400，`fields.foo`「不支持的参数」。无会话同一 URL 401。

`TestCategoryIDReuse`：删除当前最大 id 后新建，新 id 不同。旧 id 加 `version=1` 的 PATCH、DELETE 都 404。

`TestCategoryVersion`：名称合法 `version` 过旧 → 409 `version_conflict`。过旧且新父级是自己的下级 → 仍 `version_conflict`。过旧且新名称会与同级冲突 → 仍 `version_conflict`。创建 `version` 为 1；合法 PATCH 相同名称后为 2。

`TestCategoryLists`：两个根按 `name`、`id`。`parent={id}` 只返回直接子级。`flat=1` 按 id 升序含全部。`eligible_parent=1&exclude={有下级的 id}` 不含自己和下级。`parent` 与 `flat=1` 同时出现 400，两键都是「不支持的参数」。`exclude` 不带 `eligible_parent` 400。

辅助请求函数命名 `postCat` / `getCat` / `patchCat` / `deleteCat`，Origin 默认 `webOrigin`。`mustCreateCat` 对 201 解码，供本任务和 Task 4 使用。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/httpapi -run TestCategory -count=1`

Expected: FAIL。路由还不存在。

- [ ] **Step 3: 实现分类**

`internal/catalog/errors.go` 增加：

```go
ErrNameTaken = errors.New("name taken")
```

`internal/catalog/category.go` 类型：

```go
type CategoryInput struct {
	Name           OptionalText
	Parent         OptionalID
	VersionPresent bool
	Version        int64
}

type CategoryPathNode struct {
	ID   int64
	Name string
}

type Category struct {
	ID        int64
	Name      string
	ParentID  *int64
	Version   int64
	CreatedAt string
	UpdatedAt string
	Path      []CategoryPathNode
}

type CategoryFilter struct {
	Kind       ListKind
	ParentID   int64
	ExcludeID  int64
	HasExclude bool
	Limit      int
	Offset     int
}

type CategoryList struct {
	Categories []Category
	Total      int
	Limit      int
	Offset     int
}
```

复用 `ListRoots` / `ListChildren` / `ListFlat` / `ListEligible`。`ListEligible` 对分类就是全部分类，再按 `exclude` 去掉自己和下级。

函数：`CreateCategory`、`UpdateCategory`、`DeleteCategory`、`GetCategory`、`ListCategories`。全部写路径 `withImmediate`，读路径 `withRead`。

名称用 `NormalizeName`。`parent_id` 省略或 null 为根。父级不存在或 `< 1` 返回 `ErrParent`。沿 `parent_id` 向上走到自己返回 `ErrCycle`。同级重名（根用 `parent_id IS NULL`）返回 `ErrNameTaken`；判断时排除当前 id。INSERT/UPDATE 若仍碰到 `UNIQUE constraint failed`，也映射成 `ErrNameTaken`。

PATCH 必须 `VersionPresent` 且至少出现 `Name` 或 `Parent`。版本比较在父级、循环、重名之前。版本过旧只返回 `ErrVersion`。成功 `version = version + 1`。`WHERE id = ? AND version = ?` 影响 0 行时再读：没有行 `ErrNotFound`，否则 `ErrVersion`。

删除：先读出版本，不匹配 `ErrVersion`；再 `SELECT COUNT(*) FROM categories WHERE parent_id = ?` 和 `SELECT COUNT(*) FROM item_categories WHERE category_id = ?`，任一大于 0 则 `ErrInUse`。

路径从自己沿 `parent_id` 走到根，再反转。每项只有 id、name。最后一项 id 等于该分类。

列表排序：根和子级 `name, id`；`flat` 和 `eligible` 为 `id`。`parent` 所指分类不存在 404。`exclude` 所指不存在 404。`COUNT(*)` 和当页在同一只读事务。

`decode.go` 增加 `categoryWrite`（`Name`、`ParentPresent`、`ParentNull`、`ParentID`、`VersionPresent`、`Version`），解码规则与位置的 `name` / `parent_id` / `version` 相同。

`server.go`：

```go
mux.HandleFunc("/api/v1/categories", h.categoriesCollection)
mux.HandleFunc("/api/v1/categories/{id}", h.categoryByID)
```

HTTP 处理函数抄位置：Origin → 会话 → `readLimited` 32768 → POST/PATCH 拒绝查询串 → decode → 然后才 `parseDecimalID`。GET 详情：会话 → `RawQuery != ""` 则 `writeFields(queryRejected(r))` → 再解析 id。DELETE 先 `deleteVersion` 再解析 id。

`parseCategoryQuery` 允许 `limit`、`offset`、`parent`、`flat`、`eligible_parent`、`exclude`。`flat` 和 `eligible_parent` 只能是 `1`。`parent` / `flat` / `eligible_parent` 两两互斥时，出现的键都标「不支持的参数」。没有 `eligible_parent` 却有 `exclude` 时 `exclude` 为「不支持的参数」。

`writeCategoryError`：

| 错误 | 状态 | code | message |
| --- | --- | --- | --- |
| `*FieldError` | 400 | `invalid_fields` | 有字段不符合要求 |
| `ErrParent` | 400 | `invalid_parent` | 不能放在这个父级下 |
| `ErrVersion` | 409 | `version_conflict` | 记录已被修改 |
| `ErrCycle` | 409 | `category_cycle` | 不能移到自己的下级 |
| `ErrInUse` | 409 | `category_in_use` | 这个分类下面还有内容 |
| `ErrNameTaken` | 409 | `name_taken` | 同级已有相同名称 |
| `ErrNotFound` | 404 | `not_found` | 未找到 |

其余 500 空正文。不要改 `writeCatalogError` 里位置的 code。

分类 JSON：`parent_id` 用指针，根为 `null`。`path` 元素只有 `id`、`name`。

- [ ] **Step 4: 测试通过**

Run: `go test ./internal/httpapi ./internal/catalog -count=1`

Expected: PASS。已有物品和位置测试仍通过。

- [ ] **Step 5: Commit**

```bash
git add internal/catalog internal/httpapi
git commit -m "feat: 分类树的创建修改和删除"
```

---

### Task 3: 物品上的分类关联

**Files:**
- Modify: `internal/catalog/item.go`
- Modify: `internal/httpapi/decode.go`
- Modify: `internal/httpapi/decode_test.go`
- Modify: `internal/httpapi/catalog_http.go`
- Modify: `internal/httpapi/catalog_test.go`

- [ ] **Step 1: 写失败测试**

`decode_test.go` 增加与 `TestDecodeItemLocations` 平行的用例：

```go
func TestDecodeItemCategories(t *testing.T) {
	cases := []struct {
		body    string
		typeErr bool
		nullSet bool
		set     bool
	}{
		{`{"name":"钳","categories":"x"}`, true, false, false},
		{`{"name":"钳","categories":[1]}`, true, false, false},
		{`{"name":"钳","categories":[null]}`, true, false, false},
		{`{"name":"钳","categories":null}`, false, true, true},
		{`{"name":"钳"}`, false, false, false},
		{`{"name":"钳","categories":[]}`, false, false, true},
	}
	for _, tc := range cases {
		got, err := decodeItemWrite([]byte(tc.body))
		if tc.typeErr {
			if !errors.Is(err, errInvalidBody) {
				t.Fatalf("%s err=%v", tc.body, err)
			}
			continue
		}
		if err != nil || got.CategoriesNull != tc.nullSet || got.CategoriesSet != tc.set {
			t.Fatalf("%s got=%+v err=%v", tc.body, got, err)
		}
	}
}
```

`catalog_test.go`：

- 改 `itemBody` 增加 `Categories []itemCatBody`。`itemCatBody` 含 `category_id` 和 `path`（`catPathNode`）。
- `decodeItem` 在 `categories` 为 JSON null、Go nil、或某条 path 最后一项 id 不等于 `category_id` 时失败。已有 `locations` 检查保留。
- `TestItemCreate`：只提交名称时 `categories` 为 `[]`；再提交 `categories:[]` 仍为 `[]`；`SELECT COUNT(*) FROM item_categories` 为 0。
- `TestItemCategoriesReplace`：创建时带一个合法 `category_id`，读取按 `category_id` 升序且 path 正确。PATCH 省略 `categories` 时原关联保留。`categories:[]` 后为空数组。`categories` 为字符串或 `[null]` 返回 `invalid_body` 且响应没有 `fields`。`categories:null` 返回 `invalid_fields`，短句「分类格式不正确」。重复 id「同一分类只能关联一次」。`category_id` 小于 1 或不存在「所选分类不存在」。替换即使集合相同，物品 `version` 也加 1。
- `TestItemDelete` 现有用例补一句：删除物品后 `item_categories` 行数为 0，分类行仍在。
- `TestCategoryDelete` 补：物品直接关联该分类时 DELETE 409 `category_in_use`，物品还在。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/httpapi -run 'TestDecodeItemCategories|TestItemCreate|TestItemCategoriesReplace' -count=1`

Expected: FAIL。`categories` 还没进 JSON。

- [ ] **Step 3: 实现关联**

`itemWrite` / `ItemInput` 增加 `CategoriesSet`、`CategoriesNull`、`Categories []itemLink`（只用 `category_id`，没有 note）。`decodeItemWrite` 在 `locations` 之后用同一套 `decodeLocations` 逻辑读 `categories`，元素字段名是 `category_id`。抽 `decodeIDLinks(raw, idKey string)`，`locations` 走 `location_id` 并可带 `note`，`categories` 走 `category_id` 并忽略未知字段。

`Item` 增加 `Categories []ItemCategory`：

```go
type ItemCategory struct {
	CategoryID int64
	Path       []CategoryPathNode
}
```

`loadItem` 在位置关联之后加载分类：`SELECT category_id FROM item_categories WHERE item_id = ? ORDER BY category_id`，再 `loadCategoryPath`。没有行时切片为 `[]`，不是 nil。迁移前的物品同样返回 `[]`。

`itemChange` 增加 `in.CategoriesSet`。

`prepareCreate` / `prepareUpdate` 增加 `categoryFields`，规则平行 `linkFields`：null →「分类格式不正确」；缺 id → 同一句；重复 →「同一分类只能关联一次」；`< 1` 或不存在 →「所选分类不存在」。只保留按数组顺序发现的第一条分类错误。

创建：`categories` 省略或空数组都不插入关联行。修改：省略则不碰 `item_categories`；出现数组则 `DELETE FROM item_categories WHERE item_id = ?` 再插入，即使新旧集合相同也替换，并已在物品 UPDATE 里把 `version` 加 1。

`toItemJSON` 输出 `categories`，空时为 `[]`。`itemInput` 拷贝分类字段。

一次 PATCH 可以同时带 `locations` 和 `categories`。

- [ ] **Step 4: 测试通过**

Run: `go test ./... -count=1`

Expected: PASS。旧的物品测试因为 `decodeItem` 要求 `categories` 为数组，实现必须给所有物品响应带上 `[]`。

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/item.go internal/httpapi
git commit -m "feat: 物品关联多个分类"
```

---

### Task 4: 关键词和筛选

**Files:**
- Modify: `internal/catalog/validate.go`
- Modify: `internal/catalog/validate_test.go`
- Modify: `internal/catalog/item.go`
- Modify: `internal/httpapi/catalog_http.go`
- Modify: `internal/httpapi/catalog_test.go`

- [ ] **Step 1: 写失败测试**

`validate_test.go`：`NormalizeKeyword("  usb  ")` 得到 `"usb"`；含 `\n` 返回「关键词不能包含控制字符」；201 个码点返回「关键词过长」；全空白得到 `""`。

`TestItemSearchMatch`（规格 8.5）：

```go
func TestItemSearchMatch(t *testing.T) {
	_, h, _ := testHandler(t, false)
	cookie := login(t, h)
	elec := mustCreateCat(t, h, cookie, `{"name":"电子配件"}`)
	sensor := mustCreateCat(t, h, cookie, fmt.Sprintf(`{"name":"传感器","parent_id":%d}`, elec.ID))
	onlyChild := mustItem(t, h, cookie, fmt.Sprintf(
		`{"name":"温湿度传感器","categories":[{"category_id":%d}]}`, sensor.ID))
	onlyParent := mustItem(t, h, cookie, fmt.Sprintf(
		`{"name":"配件盒","categories":[{"category_id":%d}]}`, elec.ID))

	all := mustItemPage(t, getItem(h, cookie, fmt.Sprintf(
		"/api/v1/items?category=%d,%d&category_match=all", elec.ID, sensor.ID)))
	if all.Total != 1 || len(all.Data) != 1 || all.Data[0].ID != onlyChild.ID {
		t.Fatalf("all descendants=%+v", all)
	}
	direct := mustItemPage(t, getItem(h, cookie, fmt.Sprintf(
		"/api/v1/items?category=%d,%d&category_match=all&category_descendants=0", elec.ID, sensor.ID)))
	if direct.Total != 0 || len(direct.Data) != 0 {
		t.Fatalf("all direct=%+v", direct)
	}
	anyPage := mustItemPage(t, getItem(h, cookie, fmt.Sprintf(
		"/api/v1/items?category=%d,%d", elec.ID, sensor.ID)))
	if anyPage.Total != 2 {
		t.Fatalf("any=%+v", anyPage)
	}
	parentOnly := mustItemPage(t, getItem(h, cookie, fmt.Sprintf("/api/v1/items?category=%d", elec.ID)))
	if parentOnly.Total != 2 {
		t.Fatalf("parent branch=%+v", parentOnly)
	}
	_ = onlyParent
}
```

`mustCreateCat` 与 `mustCreate` 相同，走 `/api/v1/categories`。

`TestItemSearchKeyword`：名称 `100%`、`a_b`、`USB-Cable`；备注含「中文线索」、规格含「中文线索」的另一件。`q=100%` 只命中前者；`q=a_b` 只命中 `a_b`；`q=100` 命中 `100%`；`q=usb` 命中 `USB-Cable`；`q=中文线索` 只命中备注那件。

`TestItemSearchLocationRange`：物品在子位置上。`location={父级}` 不含它；`in_location={父级}` 含它；`in_location={父级}&in_location_descendants=0` 不含它。再加能命中的 `q`，与 `in_location` 同时生效。`in_location=999999` 404。

`TestItemSearchQueryErrors`：下列都 400，互斥组合的键都在 `fields` 里：

- `placement=unlocated&in_location=1`
- `location=1&in_location=1`
- `uncategorized=1&category=1`
- `category_match=all&category=1`
- `in_location_descendants=0`（没有 `in_location`）
- `category=1&category=2`
- `category=3,3`

`TestItemUncategorized`：无分类物品出现在 `uncategorized=1`；挂上分类后离开。`placement=unlocated&q=温湿度` 可以同时使用。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/httpapi -run 'TestItemSearch|TestItemUncategorized' -count=1`

Expected: FAIL。查询参数还不认识。

- [ ] **Step 3: 实现查找**

`NormalizeKeyword`：先按原文扫 Unicode Cc，命中则「关键词不能包含控制字符」；再 `TrimSpace`；码点超过 200 则「关键词过长」；全空白返回 `""`。不要打开 SQLite `case_sensitive_like`。

```go
func likePattern(q string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return `%` + r.Replace(q) + `%`
}
```

`ItemFilter` 增加：

```go
type ItemFilter struct {
	Unlocated             bool
	HasLocation           bool
	LocationID            int64
	HasInLocation         bool
	InLocationID          int64
	InLocationDescendants bool
	Keyword               string
	Uncategorized         bool
	CategoryIDs           []int64
	CategoryMatchAll      bool
	CategoryDescendants   bool
	Limit                 int
	Offset                int
}
```

`InLocationDescendants` 和 `CategoryDescendants` 在对应条件存在且参数省略时为 `true`。

`parseItemQuery` 允许的键：`limit`、`offset`、`placement`、`location`、`q`、`in_location`、`in_location_descendants`、`category`、`category_match`、`category_descendants`、`uncategorized`。每个最多一次。

形状阶段一次收齐 `fields`，不要先 404：

- `placement` 与 `location`、`placement` 与 `in_location`、`location` 与 `in_location`、`uncategorized` 与 `category`：涉及的键都是「不支持的参数」。
- `in_location_descendants` 没有合法 `in_location`；`category_descendants` 没有合法 `category`；`category_match` 出现时 `category` 不是两个及以上合法 id；`uncategorized` 与 `category_match` 或 `category_descendants` 同时出现：该键「不支持的参数」。
- `uncategorized` 不是 `1`；`*_descendants` 不是 `0` 或 `1`；`category_match` 不是 `any` 或 `all`；`placement` 不是 `unlocated`：不支持的参数。
- `in_location` / `location` id 格式同现有 `location`。
- `category` 按逗号拆，无空格；空段、空格、重复、超过 20 个、某段不是无前导零正整数：`fields.category`「参数不正确」。
- `q`：调用 `NormalizeKeyword`，错误短句用返回值；空白视为没有关键词。

形状合法后 `ListItems` 在同一只读事务里按顺序核对存在性：`location` → `in_location` → `category` 从左到右每个 id。缺哪个返回 `ErrNotFound`。

`itemListSQL` 把已出现的条件用 AND 连接。关键词：

```sql
(name LIKE ? ESCAPE '\' OR alias LIKE ? ESCAPE '\' OR model LIKE ? ESCAPE '\' OR note LIKE ? ESCAPE '\')
```

四个绑定都是同一个 `likePattern(q)`。`NULL` 字段不命中。不搜索 `spec`、`quantity_note`。

位置范围含下级：

```sql
EXISTS (
  SELECT 1 FROM item_locations il
  WHERE il.item_id = items.id AND il.location_id IN (
    WITH RECURSIVE tree(id) AS (
      SELECT id FROM locations WHERE id = ?
      UNION ALL
      SELECT locations.id FROM locations JOIN tree ON locations.parent_id = tree.id
    )
    SELECT id FROM tree
  )
)
```

分类对**每个**所选 id 单独一段 `EXISTS`（含下级时用分类的同样 CTE，仅当前时 `ic.category_id = ?`）。任意匹配用 `OR` 连接这些 `EXISTS`，全部匹配用 `AND`。不要写成 `COUNT(DISTINCT item_categories.category_id) = N`，也不要用「命中所选 id 的个数」。

外层 `SELECT` 仍按物品行，`ORDER BY name, id LIMIT ? OFFSET ?`。`total` 是同一 `WHERE` 的 `COUNT(*)`。一件物品命中多个分支时只出现一次。

`GET /api/v1/items` 继续先会话，再 `parseItemQuery`，再 `ListItems`。

- [ ] **Step 4: 测试通过**

Run: `go test ./... -count=1`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/catalog internal/httpapi
git commit -m "feat: 按关键词分类和位置范围查找物品"
```

---

### Task 5: 分类页面和物品表单

**Files:**
- Modify: `web/src/api.ts`
- Modify: `web/src/App.tsx`
- Modify: `web/src/pages.tsx`
- Modify: `web/src/styles.module.css`

- [ ] **Step 1: API 类型**

```ts
export type CategoryPathNode = { id: number; name: string };

export type Category = {
  id: number;
  name: string;
  parent_id: number | null;
  version: number;
  created_at: string;
  updated_at: string;
  path: CategoryPathNode[];
};

export type ItemCategory = {
  category_id: number;
  path: CategoryPathNode[];
};
```

`Item` 增加 `categories: ItemCategory[]`。`ItemCreate` / `ItemUpdate` 增加可选 `categories?: { category_id: number }[]`。`normalizeItem` 把缺失的 `categories` 收成 `[]`。

增加 `listCategories`、`getCategory`、`createCategory`、`updateCategory`、`deleteCategory`、`fetchAllCategories`。`fetchAllCategories` 与 `fetchAllLocations` 相同：`limit=100`，从 `offset=0` 按 `total` 取完。

- [ ] **Step 2: 路由和页头**

`App.tsx` 增加：

- `/categories` → `CategoryListPage`
- `/categories/new` → `CategoryCreatePage`
- `/categories/:id` → `CategoryPage`

页头在「位置」后面加 `<Link to="/categories">分类</Link>`。

- [ ] **Step 3: 分类各页**

分类列表：`offset`，默认每页 30，空文案「还没有分类」，「新增分类」。

新增分类：名称；「不设置父级」；父级选择器 `fetchAllCategories(new URLSearchParams({ eligible_parent: "1" }))`。地址 `parent` 合法则固定父级，提供「重新选择父级」到 `/categories/new`。`parent` 读失败显示 message，不提交。

分类页：面包屑用 `path`，上级是链接。子分类查询参数 `children`，物品 `items`。范围开关：省略 `category_descendants` 表示含下级；仅当前写 `category_descendants=0`；不要写 `=1`。切换开关时 `params.delete("items")`，保留 `children`。子分类翻页只改 `children`，保留 `items` 和当时的范围开关。物品请求 `listItems`，带 `category={id}`，仅当前再带 `category_descendants=0`，`offset` 取 `items`。空文案：「这里还没有下一级分类」「这个分类下还没有物品」。「在此分类下新增物品」链到 `/items/new?category={id}`。

版本冲突用与位置页相同的 `formReady`：`useEffect` 只在 `formReady === false` 时把服务器记录填进名称和父级；保存得到 `version_conflict` 只 `setConflict(true)`，不改草稿、不改 `version`、不自动重试。点「加载最新内容」再 GET，成功后才填表。删除冲突在填表后收回确认。

删除按钮仅当子分类 `total === 0` 且 `listItems` 在 `category_descendants=0` 下 `total === 0`。确认文案「永久删除，无法恢复」。删除成功回父级，没有父级回 `/categories`。

编辑名称和父级。父级选择器 `eligible_parent=1&exclude={id}`，按页取完。版本冲突：未保存的名称和父级保持原样，显示「记录已被修改」和「加载最新内容」，逻辑与位置页相同（`formReady` 挡住 refetch）。

- [ ] **Step 4: 物品表单分类**

在存放位置下方增加分类多选，没有逐条说明。选择器 `fetchAllCategories(new URLSearchParams({ flat: "1" }))`。新增页同时支持 `location` 与 `category` 预填；`key` 用两者拼起来。分类预填失败显示 message，仍允许只保存名称。

创建：草稿没有分类时请求体不要带 `categories`（或带 `[]`）。编辑：把加载时的 `category_id` 列表记下；提交时若集合未变，PATCH 省略 `categories`；有改动才带数组。

版本冲突时分类选择保持原样，与位置选择同一套 `formReady` / `applyServer`。`applyServer` 要写入 `categories`。

- [ ] **Step 5: 构建**

Run: `cd web && npm run build`

Expected: `tsc --noEmit` 和 Vite 成功。不改 `package.json` 依赖。

- [ ] **Step 6: Commit**

```bash
git add web/src
git commit -m "feat: 分类页面和物品表单挂分类"
```

---

### Task 6: 物品列表搜索和筛选

**Files:**
- Create: `web/src/filters.ts`
- Modify: `web/src/pages.tsx`
- Modify: `web/src/styles.module.css`

- [ ] **Step 1: 地址栏辅助**

`web/src/filters.ts`：

```ts
export type ItemListState = {
  q: string;
  unlocated: boolean;
  inLocation: string;
  locationSelf: boolean;
  categoryIds: string[];
  matchAll: boolean;
  categorySelf: boolean;
  uncategorized: boolean;
  offset: string;
};

export function readItemList(params: URLSearchParams): ItemListState {
  const categoryRaw = params.get("category") ?? "";
  const categoryIds = categoryRaw === "" ? [] : categoryRaw.split(",");
  return {
    q: params.get("q") ?? "",
    unlocated: params.get("placement") === "unlocated",
    inLocation: params.get("in_location") ?? "",
    locationSelf: params.get("in_location_descendants") === "0",
    categoryIds,
    matchAll: params.get("category_match") === "all",
    categorySelf: params.get("category_descendants") === "0",
    uncategorized: params.get("uncategorized") === "1",
    offset: params.get("offset") ?? "",
  };
}

export function writeItemList(state: ItemListState): URLSearchParams {
  const next: ItemListState = { ...state };
  if (next.unlocated) {
    next.inLocation = "";
    next.locationSelf = false;
  }
  if (next.inLocation === "") {
    next.locationSelf = false;
  } else {
    next.unlocated = false;
  }
  if (next.uncategorized) {
    next.categoryIds = [];
    next.matchAll = false;
    next.categorySelf = false;
  }
  if (next.categoryIds.length === 0) {
    next.matchAll = false;
    next.categorySelf = false;
  } else {
    next.uncategorized = false;
  }
  if (next.categoryIds.length < 2) {
    next.matchAll = false;
  }
  const params = new URLSearchParams();
  const q = next.q.trim();
  if (q !== "") params.set("q", q);
  if (next.unlocated) params.set("placement", "unlocated");
  if (next.inLocation !== "") {
    params.set("in_location", next.inLocation);
    if (next.locationSelf) params.set("in_location_descendants", "0");
  }
  if (next.uncategorized) params.set("uncategorized", "1");
  if (next.categoryIds.length > 0) {
    params.set("category", next.categoryIds.join(","));
    if (next.matchAll) params.set("category_match", "all");
    if (next.categorySelf) params.set("category_descendants", "0");
  }
  if (next.offset !== "" && next.offset !== "0") params.set("offset", next.offset);
  return params;
}
```

列表改关键词或筛选时调用 `writeItemList({ ...state, offset: "" })`。翻页只改 `offset`。

「只看待定位」：`unlocated: true`，清掉 `inLocation` / `locationSelf`，清 `offset`，保留 `q` 和分类。已经是待定位时「全部物品」只清 `unlocated` 和 `offset`。清空筛选：`writeItemList` 的全空状态，回到 `/`。

分类从两个减到一个时 `matchAll` 被 `writeItemList` 丢掉。清空分类或改未分类时丢掉 `category` / `category_match` / `category_descendants`。清空位置或改待定位时丢掉 `in_location` / `in_location_descendants`。

- [ ] **Step 2: 物品列表 UI**

`ItemListPage`：

- 顶部常驻 `<input type="search">` 和「查找」。`onSubmit` 才写入 `q`。组字过程不请求。输入框空白提交时从地址去掉 `q`。
- `<details>`「筛选」，默认不展开。内含：位置范围单选（`fetchAllLocations({flat:1})` 取完）、仅当前位置、分类多选（`fetchAllCategories({flat:1})` 取完）、仅当前分类、任意/全部匹配、待定位、未分类。
- 「仅当前位置」只在已选 `inLocation` 时渲染。「仅当前分类」只在 `categoryIds.length > 0` 时渲染。任意/全部只在 `categoryIds.length >= 2` 时渲染。
- 有任一条件时，筛选区外显示摘要：关键词正文；位置路径（含下级或仅当前）；分类路径（多个时标明任意或全部，以及含下级或仅当前）；未分类；待定位。并提供「清空筛选」。
- `queryKey` 必须包含整串 search，不能只看 `offset` 和 `placement`。
- 空列表：无任何条件且 `total===0` →「还没有物品」；仅 `placement=unlocated` →「没有待定位的物品」；其余有条件 →「没有符合条件的物品」。
- 列表行仍只显示名称和位置路径。
- 发给 API 的查询与地址栏一致，不要写 `location=`。

390 宽下筛选和摘要换行。`overflow-wrap: anywhere` 已在 `.appPage` 和 `.path` 上，摘要不要 `white-space: nowrap`。

- [ ] **Step 3: 构建**

Run: `cd web && npm run build`

Expected: 成功。

- [ ] **Step 4: Commit**

```bash
git add web/src
git commit -m "feat: 物品列表关键词和筛选"
```

---

### Task 7: README 和浏览器验收

**Files:**
- Modify: `README.md`
- Modify: `docs/工作记录.md`
- 不把 Playwright 放进仓库。临时脚本放 `/tmp`，跑完删除。

- [ ] **Step 1: README**

第一句改为：本地可以单账号登录，登记物品和位置，管理分类，并在物品列表搜索和筛选。归位、回收站、照片、MCP 和部署还不在这里。

保留两个进程、`http://127.0.0.1:5173`、Origin、首次账号、会话和数据目录。补一句：分类选择器和物品列表筛选会按页取完整棵分类树。

- [ ] **Step 2: 工作记录**

写明分类和搜索已按本计划实现，规格路径和计划路径，下一段是归位与回收站。不要把未做的功能写成已完成。

- [ ] **Step 3: Go 与构建**

Run: `go test ./... -count=1`

Expected: PASS。

Run: `cd web && npm run build`

Expected: 成功。`go.sum` 与 `web/package-lock.json` 不新增依赖项。

- [ ] **Step 4: 浏览器按规格第 9 节走一遍**

桌面宽度：

1. 登录。创建根分类「电子配件」「智能家居」；在「电子配件」下创建「传感器」。
2. 登记「温湿度传感器」，只挂「传感器」。
3. 物品列表搜「温湿度」看得到。筛选「电子配件」（含下级）看得到。同时选「电子配件」和「传感器」、全部匹配、含下级，仍看得到且只有一条。改成仅当前分类后，「电子配件」看不到它，「传感器」看得到。
4. 「未分类」在挂上分类后不再列出它。清空筛选后地址栏没有 `category_match` 或 `*_descendants`。
5. 在「传感器」下再挂满至少 31 件物品。打开「电子配件」分类页，含下级翻到物品下一页（`items=30`），再切「仅当前分类」。地址栏没有 `items`；「电子配件」没有直接关联时显示「这个分类下还没有物品」；若切换前有 `children`，切换后仍在。
6. 再建一个根分类也叫「电子配件」，得到同级已有相同名称。有子分类或直接物品关联时不能删除。删除空分类前看到「永久删除，无法恢复」。
7. 物品页改了分类未保存，另一会话先保存。回来保存后未保存的分类选择仍在，出现「加载最新内容」。

390 宽：搜索框可见；筛选默认收起；展开可选分类和位置；无横向滚动。

预先建「分类-001」到「分类-101」，根分类列表和两个分类选择器（物品表单、新增分类父级）都能翻到「分类-101」。

确认仓库没有待归位、回收站、照片、MCP、OAuth、Docker，也没有 `item_categories.source`。

- [ ] **Step 5: Commit**

```bash
git add README.md docs/工作记录.md
git commit -m "docs: 说明分类和搜索已经可用"
```

---

## 规格对照

| 规格 | 任务 |
| --- | --- |
| §4 表、根分类两条部分唯一索引 | Task 1 |
| §5 分类规则、删除、搬动只改自己 | Task 2 |
| §6.1 分类 HTTP、失败顺序、详情拒查询串 | Task 2 |
| §6.2 创建省略 `categories` 得 `[]`；PATCH 省略不变 | Task 3 |
| §6.3–6.4 查询参数、AND、按分支全部匹配、去重 | Task 4 |
| §6.5 `name_taken` / `category_cycle` / `category_in_use` | Task 2 |
| §7.1 搜索框、筛选区、依附参数清理、空文案 | Task 6 |
| §7.2 物品表单分类、预填、冲突保留分类 | Task 5 |
| §7.3 分类页、`items` 在切换范围时清掉 | Task 5 |
| §8 测试 | Task 1–4 |
| §9 浏览器验收、101 个根分类 | Task 7 |
| 不做 AI 来源列、不新增依赖 | 全程 |
