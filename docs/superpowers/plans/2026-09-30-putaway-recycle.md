# 归位与回收站 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给物品建立独立归位事项，普通删除进入回收站并可恢复，永久删除只从回收站发起；位置和分类的占用检查包含回收站中的物品。

**Architecture:** `migrations/004_putaway_recycle.sql` 给 `items` 加 `deleted_at`，并建 `return_tasks`（不记位置 id）。`DELETE /api/v1/items/{id}` 改为写入 `deleted_at`。恢复和永久删除走 `/api/v1/trash`。归位事项独立 CRUD。`internal/httpapi` 仍先处理 Origin、会话、正文和查询形状，再调用 `internal/catalog`。网页不新增依赖。

**Tech Stack:** 现有 Go 1.27.1、`modernc.org/sqlite` v1.60.1、React 19.3.0、Vite 8.3.1、TypeScript 7.0.2、React Router 8.4.0、TanStack Query 5.104.0。不新增 Go 模块或 npm 包。

规格：`docs/superpowers/specs/2026-09-30-putaway-recycle-design.md`。

提交：本目录还不是 Git 仓库，也没有 `user.name`。每个提交步骤先检查 `git rev-parse --is-inside-work-tree` 和 `git config user.name`。仓库不存在就先 `git init`。用户名为空就停下来问用户，不要改 Git 配置，也不要编造作者。不要提交 `/data/`、`web/node_modules/` 或 `web/dist/`。

---

## 文件职责

- `migrations/004_putaway_recycle.sql`：`deleted_at` 列、`return_tasks` 表和索引。
- `internal/catalog/errors.go`：增加 `ErrAlreadyCompleted`。
- `internal/catalog/validate.go`：`NormalizePartNote`、`NormalizeReason`、`NormalizeDestinationNote`。
- `internal/catalog/item.go`：进回收站、列表排除已删、详情把已删当 404、加载 `return_tasks`。
- `internal/catalog/return_task.go`：事项的创建、列表、读取、完成、去掉。
- `internal/catalog/location.go` / `category.go`：`DirectItemCount`（含回收站关联）。占用 SQL 已计 `item_locations` / `item_categories` 全表，软删除后不必改判断句。
- `internal/httpapi/decode.go`：事项正文、恢复/完成的 `version`。
- `internal/httpapi/catalog_http.go`：trash 与 return-tasks 路由、JSON 增加 `deleted_at`、`return_tasks`、`direct_item_count`。
- `internal/httpapi/server.go`：注册新路径。不要写成 `"GET /path"` 方法模式。
- `internal/httpapi/catalog_test.go`：规格第 8 节；改写 `TestItemDelete`、`TestItemDeleteBody`、`TestItemIDReuse`。
- `web/src/api.ts`：事项和回收站请求。
- `web/src/filters.ts`：增加 `clampPageOffset`。
- `web/src/App.tsx`：`/returns`、`/trash`、`/trash/:id`。
- `web/src/pages.tsx`：导航、物品页事项、待归位、回收站、占用提示、404 分支、`DeleteConfirm` 文案。
- `web/src/styles.module.css`：事项列表和确认区沿用现有 class，缺的再加。
- `README.md`、`docs/工作记录.md`。

`writeCatalogError` 保持位置的 `location_cycle` / `location_in_use`。增加 `already_completed` → 409「这条事项已经完成」。分类仍走 `writeCategoryError`。

事项操作成功后不要调用现有的 `applyServer`（它会用物品详情覆盖草稿）。

---

### Task 1: 迁移 004

**Files:**
- Create: `migrations/004_putaway_recycle.sql`
- Modify: `internal/httpapi/catalog_test.go`

- [ ] **Step 1: 写失败测试**

在 `TestMigration003` 旁增加：

```go
func TestMigration004(t *testing.T) {
	db := migratedDB(t)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 4`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("version 4 rows = %d", n)
	}
	var sqlText string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = 'return_tasks'`).Scan(&sqlText); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sqlText, "AUTOINCREMENT") {
		t.Fatalf("return_tasks missing AUTOINCREMENT: %s", sqlText)
	}
	if strings.Contains(sqlText, "location_id") {
		t.Fatalf("return_tasks must not store location_id: %s", sqlText)
	}
	var deletedAt string
	if err := db.QueryRow(`SELECT COALESCE(deleted_at, 'NULL') FROM pragma_table_info('items') WHERE name = 'deleted_at'`).Scan(&deletedAt); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"items_deleted_at", "return_tasks_item_id", "return_tasks_open"} {
		var indexSQL string
		if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE name = ?`, name).Scan(&indexSQL); err != nil {
			t.Fatal(name, err)
		}
		if name == "items_deleted_at" && !strings.Contains(indexSQL, "deleted_at IS NOT NULL") {
			t.Fatalf("%s not partial: %s", name, indexSQL)
		}
	}
	files, err := loadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Apply(context.Background(), db, files, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 4`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("second apply rows = %d", n)
	}
}
```

`pragma_table_info` 若在该驱动下不好用，改为 `SELECT deleted_at FROM items` 在空表上不报「no such column」即可。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/httpapi -run TestMigration004 -count=1`

Expected: FAIL。版本 4 或表还不存在。

- [ ] **Step 3: 写入迁移**

`migrations/004_putaway_recycle.sql` 按规格第 4 节全文写入：`ALTER TABLE items ADD COLUMN deleted_at TEXT;`、部分索引 `items_deleted_at`、`return_tasks` 表、`return_tasks_item_id`、`return_tasks_open`。没有 `location_id`、没有 `source`。

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/httpapi -run TestMigration004 -count=1`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add migrations/004_putaway_recycle.sql internal/httpapi/catalog_test.go
git commit -m "feat: 增加归位事项表和物品 deleted_at"
```

先检查仓库和 `user.name`。没有身份就停。

---

### Task 2: 普通删除改为进回收站

**Files:**
- Modify: `internal/catalog/item.go`
- Modify: `internal/httpapi/catalog_http.go`
- Modify: `internal/httpapi/server.go`
- Modify: `internal/httpapi/decode.go`
- Modify: `internal/httpapi/catalog_test.go`

- [ ] **Step 1: 改写并新增失败测试**

把 `TestItemDelete` 改成规格 6.2：`DELETE /items/{id}` 之后

- `SELECT COUNT(*) FROM items WHERE id = ?` 为 1
- `deleted_at` 非空
- `item_locations`、`item_categories` 仍为 1
- `GET /api/v1/items/{id}` 404
- `GET /api/v1/trash/{id}` 200，带原来的位置和分类
- 再 `DELETE /api/v1/locations/{id}` 仍 409 `location_in_use`（回收站物品仍占用）

`TestItemDeleteBody`：忽略正文，行仍在且 `deleted_at` 非空。

`TestItemIDReuse`：`DELETE /items` 后新建 id 不同；对旧 id 的 PATCH 和 `DELETE /items` 仍 404；旧 id 在 `GET /trash/{id}` 为 200。另增 `TestItemPurgeIDReuse`：先 `DELETE /items` 再 `DELETE /trash/{id}?version=`（用进回收站后的 version，即 2），然后新建，新 id 不同；对旧 id 的 PATCH、`DELETE /items`、`DELETE /trash` 都 404。

新增 `TestItemTrash` 覆盖规格第 8 节 4、5、7、12 中与回收站相关的部分（恢复、已在回收站再 `DELETE /items` 为 404、对活物品 `POST /trash/{id}/restore` 为 404、`GET /items?deleted=1` 为 400、过旧 version 进回收站 409 且仍在正常列表、进回收站→恢复→再进回收站后用第一次的 version 恢复为 409）。

详情 GET：`GET /api/v1/trash/{id}?foo=1` 已登录 400，无会话 401。

`POST /api/v1/trash/abc/restore` 错误 Origin 返回 403。恢复正文不是对象时，即使 id 不存在也是 `invalid_body`。`DELETE /api/v1/trash/999999?version=abc` 返回 400，不是 404。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/httpapi -run 'TestItemDelete$|TestItemDeleteBody|TestItemIDReuse|TestItemTrash|TestItemPurgeIDReuse' -count=1`

Expected: FAIL。`GET /api/v1/trash` 还不存在，或 `DELETE /items` 仍物理删除。

- [ ] **Step 3: 实现软删除和 trash 路由**

`Item` 增加 `DeletedAt *string` 和 `ReturnTasks []ReturnTask`（本任务里事项恒为 `[]`，Task 3 再加载）。

`itemCols` 加上 `deleted_at`。`scanItem` 扫入 `sql.NullString`。

`GetItem` / `UpdateItem`：加载后若 `deleted_at` 非空，返回 `ErrNotFound`。

`ListItems` 的 `itemListSQL` 固定加上 `deleted_at IS NULL`。

`DeleteItem`：存在且 `deleted_at` 为空才继续；已删或没有行 → `ErrNotFound`；version 不对 → `ErrVersion`；然后 `UPDATE items SET deleted_at = ?, version = version + 1, updated_at = ? WHERE id = ? AND version = ?`。

新增 `ListTrash`（`deleted_at IS NOT NULL`，排序 `name, id`）、`GetTrashItem`（不存在或 `deleted_at` 为空 → `ErrNotFound`）、`RestoreItem`（清除 `deleted_at`，version+1）、`PurgeItem`（`DELETE FROM items`，仅当已在回收站）。

HTTP：

```
mux.HandleFunc("/api/v1/trash", h.trashCollection)
mux.HandleFunc("/api/v1/trash/{id}", h.trashByID)
mux.HandleFunc("/api/v1/trash/{id}/restore", h.restoreTrash)
```

方法在处理器内 `switch`。`itemJSON` 增加 `DeletedAt *string \`json:"deleted_at"\`` 和 `ReturnTasks []returnTaskJSON \`json:"return_tasks"\``，没有事项时编码 `[]`。

恢复正文用 `decodeVersionWrite`：必须是对象，必须有整数 `version`，规则与物品 PATCH 的 version 相同。`POST` 带查询参数 → 400。

`GET /api/v1/items` 出现 `deleted` / `trashed` / `include_deleted` → 400「不支持的参数」（加进 `parseItemQuery` 的未知键分支即可，它们本来就会落到不认识的参数）。

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/httpapi -count=1`

Expected: PASS。`TestItemSearchMatch` 等列表测试仍只看到未删除物品。

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/item.go internal/httpapi/catalog_http.go internal/httpapi/server.go internal/httpapi/decode.go internal/httpapi/catalog_test.go
git commit -m "feat: 物品普通删除进入回收站"
```

---

### Task 3: 归位事项 API

**Files:**
- Create: `internal/catalog/return_task.go`
- Modify: `internal/catalog/errors.go`
- Modify: `internal/catalog/validate.go`
- Modify: `internal/catalog/item.go`（`loadItem` 加载事项）
- Modify: `internal/httpapi/decode.go`
- Modify: `internal/httpapi/catalog_http.go`
- Modify: `internal/httpapi/server.go`
- Modify: `internal/httpapi/catalog_test.go`
- Modify: `internal/catalog/validate_test.go`（可选，字段长度）

- [ ] **Step 1: 写失败测试 `TestReturnTasks`**

覆盖规格第 8 节 2、3、8、9、11、13：

1. 一件物品两条事项：省略 `part_note` 为整件；另一条 `part_note` 为「充电器」。完成整件后另一条仍未完成；物品 `locations`、`version`、`updated_at` 不变。`GET /api/v1/return-tasks` 只含未完成那条。已完成那条 `GET /api/v1/return-tasks/{id}` 仍 200。
2. 物品在位置 A，建事项并写临时去向，改挂到位置 B：事项仍在；空的 A 可删，B 409。
3. 物品进回收站后，创建/完成/去掉事项都 404，数据不变。`GET /return-tasks/{id}` 也 404。恢复后未完成事项回到 `GET /return-tasks`。
4. 再完成已完成事项 → 409 `already_completed`，`completed_at` 不变。去掉已完成同样 409。过旧 version 完成只返回 `version_conflict`。
5. 无会话创建 401；坏 Origin 403；`POST /api/v1/items/{不存在的 id}/return-tasks` 配上超过 200 码点的 `part_note` 返回 404（先存在性后字段）。正文超过 32768 返回 413。`GET /api/v1/return-tasks/{id}?foo=1` 已登录 400，无会话 401。
6. 去掉当前最大事项 id 后新建，新 id 不同；被删 id 完成、删除都 404。
7. 新建事项 version=1，完成一次后为 2。去掉未完成事项后物品 version / updated_at 不变。

`POST /api/v1/items/{id}/return-tasks` 跳过的是事项自己的 id，不跳过物品 id。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/httpapi -run TestReturnTasks -count=1`

Expected: FAIL。路由不存在。

- [ ] **Step 3: 实现事项**

`errors.go` 增加 `ErrAlreadyCompleted`。

```go
func NormalizePartNote(s string) (string, error) {
	return normalizeText(s, 200, "", "配件说明过长", "配件说明不能包含控制字符", false, false, false)
}
func NormalizeReason(s string) (string, error) {
	return normalizeText(s, 200, "", "原因过长", "原因不能包含控制字符", false, false, false)
}
func NormalizeDestinationNote(s string) (string, error) {
	return normalizeText(s, 2000, "", "临时去向过长", "临时去向不能包含控制字符", false, true, false)
}
```

`return_task.go`：`CreateReturnTask`（物品必须存在且未删）、`ListOpenReturnTasks`（`completed_at IS NULL` 且物品 `deleted_at IS NULL`，排序 `created_at, id`）、`GetReturnTask`（物品已删 → `ErrNotFound`）、`CompleteReturnTask`、`DeleteReturnTask`（已完成 → `ErrAlreadyCompleted`）。全部 `BEGIN IMMEDIATE`。完成/去掉/创建都不改物品 `version` / `updated_at`。

`loadItem` 再加载该物品全部事项：未完成在前，同组 `id` 升序。`item_name` 用物品当前名称。

HTTP：

```
mux.HandleFunc("/api/v1/items/{id}/return-tasks", h.itemReturnTasks)
mux.HandleFunc("/api/v1/return-tasks", h.returnTasksCollection)
mux.HandleFunc("/api/v1/return-tasks/{id}", h.returnTaskByID)
mux.HandleFunc("/api/v1/return-tasks/{id}/complete", h.completeReturnTask)
```

`writeCatalogError` 增加 `ErrAlreadyCompleted` → 409 `already_completed`「这条事项已经完成」。

创建正文三个可选字段，省略/`null`/空白 → NULL。完成正文与恢复相同，走 `decodeVersionWrite`。

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./... -count=1`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/catalog internal/httpapi
git commit -m "feat: 独立归位事项接口"
```

---

### Task 4: 占用计数含回收站

**Files:**
- Modify: `internal/catalog/location.go`
- Modify: `internal/catalog/category.go`
- Modify: `internal/httpapi/catalog_http.go`（`locationJSON` / `categoryJSON` 增加 `direct_item_count`）
- Modify: `internal/httpapi/catalog_test.go`

- [ ] **Step 1: 写失败测试**

`TestTrashOccupiesLocationAndCategory`：物品关联位置和分类后进回收站。`GET` 该位置、该分类的 `direct_item_count` 为 1。删除位置 409 `location_in_use`，删除分类 409 `category_in_use`。永久删除物品后，空位置和空分类 `direct_item_count` 为 0（键仍在），删除返回 204。

空的新位置/新分类 JSON 必须带 `"direct_item_count": 0`。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/httpapi -run TestTrashOccupiesLocationAndCategory -count=1`

Expected: FAIL。JSON 还没有该字段。

- [ ] **Step 3: 实现计数**

读取位置/分类时：

```sql
SELECT COUNT(*) FROM item_locations WHERE location_id = ?
SELECT COUNT(*) FROM item_categories WHERE category_id = ?
```

不要加 `deleted_at IS NULL`。列表每条都带这个整数。`toLocationJSON` / `toCategoryJSON` 写入 `DirectItemCount int \`json:"direct_item_count"\``。

删除位置/分类的 SQL 不必改：它们已经数全表关联。本任务补的是 JSON 和测试。

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./... -count=1`

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/location.go internal/catalog/category.go internal/httpapi/catalog_http.go internal/httpapi/catalog_test.go
git commit -m "feat: 位置和分类占用计数包含回收站"
```

---

### Task 5: 待归位页面和物品页事项

**Files:**
- Modify: `web/src/api.ts`
- Modify: `web/src/filters.ts`
- Modify: `web/src/App.tsx`
- Modify: `web/src/pages.tsx`
- Modify: `web/src/styles.module.css`

- [ ] **Step 1: 类型和请求**

`Item` 增加 `deleted_at: string | null` 和 `return_tasks: ReturnTask[]`。`normalizeItem` 缺省事项为 `[]`。

```ts
export type ReturnTask = {
  id: number;
  item_id: number;
  item_name: string;
  part_note: string | null;
  reason: string | null;
  destination_note: string | null;
  completed_at: string | null;
  version: number;
  created_at: string;
  updated_at: string;
};

export type ReturnTaskCreate = {
  part_note?: string | null;
  reason?: string | null;
  destination_note?: string | null;
};
```

实现 `listReturnTasks`、`createReturnTask`、`completeReturnTask`、`deleteReturnTask`、`listTrash`、`getTrashItem`、`restoreTrashItem`、`purgeTrashItem`。`Location` / `Category` 增加 `direct_item_count: number`。

`filters.ts` 增加：

```ts
export function clampPageOffset(total: number, offset: number, limit = 30): number {
  if (total <= 0 || offset <= 0) return 0;
  if (offset < total) return offset;
  return Math.floor((total - 1) / limit) * limit;
}
```

完成或去掉之后，若当前页空了但 `total > 0`，用 `clampPageOffset` 改 `offset` 再取一页。回收站恢复/永久删除同样用这个函数。不要出现「本页空白、前面还有记录」。

- [ ] **Step 2: 构建确认类型能通过**

Run: `cd web && npx tsc --noEmit`

Expected: 现有页面还没用新字段时也应通过；若 `Item` 缺字段则先在 `normalizeItem` 补默认。

- [ ] **Step 3: 导航、物品页、待归位清单**

`App.tsx` 增加 `/returns` → `ReturnListPage`。`/trash` 和 `/trash/:id` 在 Task 6 注册。

登录后导航：物品、位置、分类、待归位、回收站、退出。

`DeleteConfirm` 增加 `warning`（默认「永久删除，无法恢复」）和可选 `confirmLabel`（默认「确认删除」）。物品页传入 `warning="将移到回收站"`。

物品页在分类下方增加事项区：

- 未完成：整件或配件说明、原因、临时去向、建立时间、「完成归位」、「去掉」
- 已完成：显示「已归位」和完成时间
- 新增表单：配件说明（说明「整件则留空」）、原因、临时去向用 `<textarea>`
- 「去掉」确认文案「去掉这条提醒」，按钮「确认去掉」

成功回调：

```ts
onSuccess: (task) => {
  queryClient.setQueryData(["item", id], (cur: Item | undefined) =>
    cur ? { ...cur, return_tasks: mergeTask(cur.return_tasks, task) } : cur,
  );
}
```

去掉成功则从数组里删掉该 id。不要 `applyServer`，不要 `setDraft(draftFromItem(...))`。

事项 404 处理（规格 7.1）：

```ts
async function handleTaskNotFound() {
  try {
    const latest = await getItem(id);
    queryClient.setQueryData(["item", id], (cur: Item | undefined) =>
      cur ? { ...cur, return_tasks: latest.return_tasks } : cur,
    );
    setTaskNotice("该事项已不存在");
  } catch (error) {
    if (isNotFound(error)) {
      navigate("/", { replace: true, state: { notice: "未找到" } });
      return;
    }
    setTaskVerifyError(messageOf(error, "无法确认物品"));
  }
}
```

物品保存 404 仍走现有「未找到」回 `/`。核实失败时留在本页，保留草稿，允许再点完成/去掉。

`ReturnListPage`：`GET /api/v1/return-tasks`，空文案「没有待归位事项」。每行链到物品页。完成/去掉规则相同。404 时用事项上的 `item_id` 调 `getItem`：物品在则提示「该事项已不存在」并重拉清单；物品 404 则提示「未找到」并重拉清单；核实失败则留在本页。

翻页 `offset`，每页 30。完成/去掉后用响应或再 GET 的 `total` 调用 `clampPageOffset`。

- [ ] **Step 4: 构建**

Run: `cd web && npm run build`

Expected: 成功。

- [ ] **Step 5: Commit**

```bash
git add web/src
git commit -m "feat: 待归位清单和物品页事项"
```

---

### Task 6: 回收站页面和占用提示

**Files:**
- Modify: `web/src/pages.tsx`
- Modify: `web/src/App.tsx`（若 Task 5 未挂 `/trash`）
- Modify: `web/src/styles.module.css`

- [ ] **Step 1: 回收站列表和详情**

`/trash`：标题「回收站」，空文案「回收站是空的」。每行名称和 `deleted_at`，链到 `/trash/:id`。翻页 30，删除或恢复后 `clampPageOffset`。

`/trash/:id`：只读名称、备注、位置路径、分类路径、归位事项。「恢复」成功后 `navigate(/items/${id})`。「永久删除」用默认 `DeleteConfirm` 文案「永久删除，无法恢复」。成功回 `/trash`。版本冲突保留只读内容和删除确认，提供「加载最新内容」；加载 404 则回 `/trash` 并提示「未找到」。

活物品打开 `/items/:id` 得到 404（含已进回收站）仍显示「未找到」回列表。

- [ ] **Step 2: 位置页和分类页占用**

`canDelete` 改为：子级 `total === 0` **且** `record.direct_item_count === 0`。不要再用 `listItems` 的 live `total` 决定能不能删。

活物品列表仍走 `GET /items`。当前范围没有活物品时文案保持「这个位置里还没有物品」/「这个分类下还没有物品」。若此时 `direct_item_count > 0`，额外显示「回收站里还有物品占用这个位置」或「回收站里还有物品占用这个分类」，并且不显示删除。

- [ ] **Step 3: 构建**

Run: `cd web && npm run build`

Expected: 成功。

- [ ] **Step 4: Commit**

```bash
git add web/src
git commit -m "feat: 回收站页面和占用提示"
```

---

### Task 7: README、工作记录和浏览器验收

**Files:**
- Modify: `README.md`
- Modify: `docs/工作记录.md`

- [ ] **Step 1: README**

第一句改为：本地可以单账号登录，登记物品和位置，管理分类，搜索筛选，登记待归位，并把物品放进回收站。照片、MCP 和部署还不在这里。

保留两个进程、`http://127.0.0.1:5173`、Origin、首次账号。补一句：普通删除进入回收站；永久删除只在回收站里再次确认。

- [ ] **Step 2: 工作记录**

写明归位与回收站已按本计划实现，规格和计划路径，下一段是照片。不要把照片、MCP、OAuth、Docker 写成已完成。嵌入迁移连续到 `004`。

- [ ] **Step 3: Go 与构建**

Run: `go test ./... -count=1`

Expected: PASS。

Run: `cd web && npm run build`

Expected: 成功。`go.sum` 与 `web/package-lock.json` 不新增依赖项。

- [ ] **Step 4: 浏览器按规格第 9 节走一遍，并加上计划收尾**

桌面宽度：

1. 登录。登记「遥控车」放到某个位置。新增两条归位事项（整件「借出」，配件「充电器」去向「老王家」）。完成整件后清单只剩充电器。把遥控车改到另一位置，充电器事项仍在；原位置若已空则可以删除。
2. 普通删除看到「将移到回收站」，确认后物品列表、搜索、待归位都没有它；回收站看得到，位置和分类还在。恢复后物品和未完成事项都回来。再放进回收站，看到「永久删除，无法恢复」，确认后回收站也没有，空位置可删。
3. 另一件物品只进回收站。所在位置和所挂分类没有删除按钮，并看到占用提示。
4. 物品页改名称未保存，另一会话先放进回收站。回来保存显示「未找到」。两个会话都还在正常列表时保存冲突，出现「加载最新内容」。
5. 两个会话打开同一件物品。未保存名称、备注、位置、分类，并写好一条未提交的新增事项草稿。页面 A 去掉已有事项后，页面 B 再完成它：提示「该事项已不存在」，上述草稿都还在，不回到列表。
6. 草稿保护：物品页未保存名称、备注、位置、分类时，分别新增、完成、去掉一条事项，这四项输入都不变。
7. 待归位清单造出至少 31 条未完成事项。翻到 `offset=30` 后完成或去掉本页最后一条（若本页只剩一条则完成它）：回到有记录的一页，不要停在空白页而前面其实还有事项。回收站用 31 件进站物品做同样的恢复或永久删除翻页回退。

390 宽：导航能点到待归位和回收站；文字换行；`scrollWidth` 等于视口宽度。

确认仓库没有照片、MCP、OAuth、Docker，没有 `item_categories.source`，`return_tasks` 没有位置 id 列。

Playwright 仍放在 `/tmp`，用完删除。不要写进仓库。

- [ ] **Step 5: Commit**

```bash
git add README.md docs/工作记录.md
git commit -m "docs: 说明归位与回收站已经可用"
```

---

## 规格对照

| 规格 | 任务 |
| --- | --- |
| §4 表、`deleted_at`、事项不记位置 id | Task 1 |
| §5.2–5.3、§6.2–6.3 普通删除切换、trash API、旧 `TestItemDelete` 改写 | Task 2 |
| §5.1、§6.4 归位事项独立完成、不改物品位置和 version | Task 3 |
| §5.4、§6.5 `direct_item_count`、占用含回收站 | Task 4 |
| §7.1–7.2 物品页事项、待归位、事项 404 分支、草稿保护 | Task 5 |
| §7.3–7.4 回收站页、占用提示、删除文案 | Task 6 |
| 完成/去掉最后一页最后一条后回退有效页；回收站同理 | Task 5–6、验收 Task 7 |
| 未保存名称/备注/位置/分类时新增、完成、去掉事项草稿不变 | Task 5、验收 Task 7 |
| 两会话去掉同一事项，另一页保留物品草稿 | Task 5、验收 Task 7 |
| §8 Go 测试 | Task 1–4 |
| §9 浏览器验收 | Task 7 |
| 不做照片、MCP、不新增依赖 | 全程 |
