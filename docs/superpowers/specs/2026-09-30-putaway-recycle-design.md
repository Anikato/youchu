# 有处：归位与回收站

日期：2026-09-30

状态：复核通过。本文是第四阶段的实现依据。尚未开始实现。

依据：同仓库《家庭物品整理与查找需求.md》第八节、《技术方案草稿.md》第 1、3、5、8 节，以及已实现的《基础登录与存储》《物品和位置》《分类和搜索》规格。冲突时，登录、会话、迁移器和 SQLite 连接以第一阶段规格为准；位置页、直接位置列表 `location=`、物品和位置的字段长度与失败顺序以第二阶段规格为准；分类树、物品分类关联、关键词和物品列表上的筛选以第三阶段规格为准；归位事项、回收站、普通删除切换和引用检查以本文为准。

本文只覆盖交付顺序中的第四段。

## 1. 目标

已登录的操作者可以：

- 给一件物品建立多条彼此独立的归位事项：整件，或一条文字描述的配件；完成一条不影响其他条。
- 临时取出时保留物品上的存放位置；临时去向写在事项里。完成归位只关闭该事项。正式调整存放位置仍走物品编辑。
- 普通删除把物品放进回收站：位置、分类、归位事项都保留；默认从正常列表、搜索和各项 `total` 里隐藏。
- 从回收站恢复后，物品回到正常列表；未完成的归位事项回到待归位清单。
- 删除位置或分类时，占用检查包含回收站里的物品关联。
- 永久删除只从回收站发起，再次确认后删除该物品及其专属关联，不删除共享的位置和分类。

## 2. 不做的事

本阶段不实现照片、MCP、OAuth、个人访问令牌、Docker、备份、扫码、主动通知、变更事件表、`item_categories.source`、AI 归类和静态资源托管。

归位事项不记位置 id，也不记分类 id。不把完成归位和改存放位置合成一次操作。不自动清空回收站。不提供把已完成事项改回未完成。回收站中的物品不能修改、不能新建或完成或去掉归位事项。

位置和分类本身仍是物理删除，不进回收站。空位置、空分类的确认文案仍是「永久删除，无法恢复」。

本阶段之前已经用旧接口物理删除的物品（以及当时一并删掉的关联），不会因为增加回收站而重新出现。迁移只给还在 `items` 表里的行加上 `deleted_at` 列，现有行都是 `NULL`。没有从备份或日志里重建已删行的步骤。

登录、会话、Origin、登录限流和已有迁移保持现有行为。不新增环境变量，不新增 Go 模块依赖，不新增 npm 依赖。不把 Playwright 放进仓库。

## 3. 仓库

新增：

```
migrations/004_putaway_recycle.sql
```

归位事项、进回收站、恢复、永久删除和占用检查放在 `internal/catalog`。HTTP 路由仍由 `internal/httpapi` 注册，负责认证、Origin、正文大小、状态码和 JSON，不复制一套业务判断。网页仍在 `web/src`，页面可以拆文件，路由集中注册。

沿用现有迁移器，只增加 `004` 文件。嵌入版本因此是连续的 `001`、`002`、`003`、`004`。

实现时更新 `README.md`：说明本地可以登记待归位、把物品放进回收站并恢复或永久删除；开发入口仍是 `http://127.0.0.1:5173`，并保留 Origin 与首次账号的说明。

## 4. 表

`004_putaway_recycle.sql` 给 `items` 增加一列，并创建归位事项表。时间列使用 UTC 的 RFC3339Nano 文本，与现有表一致。

```sql
ALTER TABLE items ADD COLUMN deleted_at TEXT;

CREATE INDEX items_deleted_at ON items(deleted_at) WHERE deleted_at IS NOT NULL;

CREATE TABLE return_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    part_note TEXT CHECK (part_note IS NULL OR part_note <> ''),
    reason TEXT CHECK (reason IS NULL OR reason <> ''),
    destination_note TEXT CHECK (destination_note IS NULL OR destination_note <> ''),
    completed_at TEXT,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX return_tasks_item_id ON return_tasks(item_id);
CREATE INDEX return_tasks_open ON return_tasks(item_id) WHERE completed_at IS NULL;
```

`deleted_at` 为 `NULL` 表示仍在正常列表。进回收站时写入时间文本。应用只写 `NULL` 或时间文本，不写空字符串。

`return_tasks.id` 使用 `INTEGER PRIMARY KEY AUTOINCREMENT`。插入时不指定 `id`。已提交的删除不会把该 id 再分配给新行。回滚、未提交的插入不在本阶段测试里。

测试要删除当前最大 id 的归位事项后新建一条，新 id 与被删 id 不同。再用被删 id 和 `version=1` 去完成、去删除，都返回 404。同物品上其余事项保持原样。

可选文字清空后存 `NULL`，不存空字符串。写入前去掉首尾空白；去掉之后为空，按「未提供值」处理。

字符数按 Unicode 码点计算，规则与现有文字字段相同。最大长度不写成 SQL `CHECK`。

| 字段 | 码点 | 换行 |
| --- | --- | --- |
| 配件说明 `part_note`、原因 `reason` | 最多 200 | 不允许 |
| 临时去向 `destination_note` | 最多 2000 | 允许 U+000A 和 U+000D |

`part_note` 为 `NULL` 表示整件。`reason` 和 `destination_note` 都可以为 `NULL`。同一物品允许多条事项，包括多条整件，不做唯一约束。

事项没有位置 id、没有分类 id、没有物品版本快照。

## 5. 规则

### 5.1 归位与存放位置

物品上的 `item_locations` 是正式存放位置。建立归位事项、完成归位、去掉未完成事项，都不改 `item_locations`、`item_categories` 或物品的文字字段。

临时去向只写在事项的 `destination_note`。正式改存放位置仍是物品的 `PATCH`，与事项彼此独立：有未完成事项时也可以改位置；改位置也不自动完成事项。

完成一条事项只给该行写入 `completed_at`，并把该事项的 `version` 加 1。其他事项不变。物品的 `version` 和 `updated_at` 都不变。

去掉一条未完成事项是删除该行。已完成的事项不能用这个删除接口去掉，随物品永久删除一起去掉。去掉事项同样不改物品的 `version` 和 `updated_at`。

### 5.2 普通删除、恢复、永久删除

`DELETE /api/v1/items/{id}?version=` 从本阶段起改为进入回收站：写入 `deleted_at`，物品 `version` 加 1，更新 `updated_at`。`item_locations`、`item_categories`、`return_tasks` 都保留。响应仍是 204 空正文。

旧行为是物理删除该行及其关联。切换之后：

- 正常的 `GET /api/v1/items`、`GET /api/v1/items/{id}`、位置页的 `location=`、分类页的物品列表、关键词和筛选的 `total`，都只包含 `deleted_at IS NULL` 的物品。
- 进回收站的物品用 `GET /api/v1/trash` 和 `GET /api/v1/trash/{id}` 读取。
- 要把物品从库里拿掉，只能对已经在回收站中的记录调用 `DELETE /api/v1/trash/{id}?version=`。

恢复清除 `deleted_at`，物品 `version` 加 1。事项行原样保留：未完成的重新出现在待归位清单；已完成的仍是已完成。

永久删除在同一事务里删除该物品行。`item_locations`、`item_categories`、`return_tasks` 随外键 `ON DELETE CASCADE` 去掉。位置行和分类行仍在。

本阶段之前已经物理删除、此刻不在 `items` 表里的记录，接口不能恢复。

### 5.3 回收站只读

对已进入回收站的物品，下列请求视为找不到该物品，返回 404 `not_found`：

- `GET` / `PATCH` / `DELETE /api/v1/items/{id}`
- `POST /api/v1/items/{id}/return-tasks`
- 该物品下归位事项的读取、完成、删除（`/api/v1/return-tasks/{id}` 及其完成接口）

回收站详情只读。恢复和永久删除走 `/api/v1/trash` 下的接口。

对仍在正常列表的物品，`GET` / `DELETE /api/v1/trash/{id}` 和恢复接口返回 404。

### 5.4 位置和分类的占用

删除位置前，事务内确认它没有子位置，且 `item_locations` 没有引用它，**含** `deleted_at` 非空的物品。有任一种引用时返回 409 `location_in_use`，message 仍是「这个位置下面还有内容」。

删除分类前，事务内确认它没有子分类，且 `item_categories` 没有引用**该分类自己的 id**，**含**回收站物品。有任一种引用时返回 409 `category_in_use`，message 仍是「这个分类下面还有内容」。挂在子分类上的物品仍不单独阻止删除父分类。

归位事项不引用位置或分类，不单独占用它们。`writeCatalogError` 的位置错误码不变；分类仍走 `writeCategoryError`。

成功的修改把对应记录的 `version` 加 1，并更新 `updated_at`。`created_at` 不变。新建时 `version` 为 1。进回收站和恢复都算物品的一次成功修改。

创建、修改、进回收站、恢复、永久删除、完成归位、去掉事项都使用 `BEGIN IMMEDIATE`。字段规则没有通过时不写入，也不因为版本过旧返回 409。版本通过之后，才在同一事务里做占用或「已经完成」等判断。对外可观察的失败顺序以第 6 节开头的清单为准。

## 6. HTTP

响应沿用 `application/json; charset=utf-8`。错误至少包含 `code` 和 `message`。`message` 给人读，判断以 `code` 为准。未知 JSON 字段忽略。

本阶段的写路由（物品进回收站、恢复、永久删除、归位事项的创建 / 完成 / 删除，以及现有的位置、分类、物品修改）按下面的顺序停在第一个失败上。方法不对的请求不进入这个顺序，直接返回 404。

1. Origin。不符合或缺少时返回 403 `origin_rejected`，message 为「来源不被接受」，并且不写入。
2. 会话。没有有效会话时返回 401 `unauthenticated`，message 为「未登录」，并且不写入。Origin 和会话都失败时返回 403。
3. 请求大小，以及 JSON 能否解析、类型是否符合下文。`POST` 和 `PATCH` 正文超过 32768 字节返回 413 `body_too_large`，message 为「请求正文过大」。JSON 类型错误返回 400 `invalid_body`，不带 `fields`。`POST` 和 `PATCH` 带了查询参数时，也在这一步返回 400 `invalid_fields`。`DELETE` 不读取正文；它的 `version` 查询参数不合法，或出现 `version` 以外的查询参数时，在这一步返回 400 `invalid_fields`。
4. 记录是否存在。路径 id 要匹配 `^[1-9][0-9]*$`，并且库里要有**本节对该接口所要求的那一行**。否则返回 404 `not_found`，message 为「未找到」。集合上的创建（`POST /api/v1/items`、`POST /api/v1/locations`、`POST /api/v1/categories`）没有既有 id，跳过这一步，也跳过第 6 步。`POST /api/v1/items/{id}/return-tasks` 跳过的是事项自己的 id 和版本，**不跳过**物品 id：必须先确认该物品存在且 `deleted_at` 为 `NULL`，再进入第 5 步。因此过长的 `part_note` 配上不存在或已进回收站的物品 id，返回 404，不返回 400。
5. 字段规则。返回 400 `invalid_fields`。
6. 版本。不一致时返回 409 `version_conflict`，不写入。创建事项没有版本，跳过这一步。
7. 关系约束。位置和分类的写路由仍按前两阶段返回 `invalid_parent`、`location_cycle`、`code_taken`、`name_taken`、`category_cycle`、`location_in_use`、`category_in_use`。本阶段增加：事项已经完成。返回下文对应的 400 或 409。

因此，不存在的 id 配上坏 JSON 返回 `invalid_body`，不返回 404。不存在的 id 配上类型正确但字段不合法的 JSON 返回 404。字段不合法且版本也过旧时返回 400。版本过旧时不再返回 `invalid_parent`、`location_cycle`、`location_in_use`、`code_taken`、`name_taken`、`category_cycle`、`category_in_use` 或 `already_completed`。

路径 id 在 Origin、会话和正文（或 DELETE 的 `version`）之后才解析。`DELETE /api/v1/items/abc` 在错误 Origin 下返回 403，不返回 404。恢复、永久删除、完成归位同样如此。

`GET` 先核对会话，没有会话时返回 401，不核对 Origin。会话有效后，查询参数不合法返回 400，然后再判断记录是否存在。`GET /api/v1/items/{id}`、`GET /api/v1/trash/{id}`、`GET /api/v1/return-tasks/{id}` 与现有详情一样：会话有效后若 `RawQuery` 不是空字符串，返回 400 `invalid_fields`，键为查询参数名，短句为「不支持的参数」，不读取该行。

登录接口的正文上限仍是 4096 字节。本阶段 `POST` 与 `PATCH` 上限是 32768 字节。`DELETE` 的版本只放在查询参数里。恢复和完成归位的版本放在 JSON 正文里。

未知路径返回 404 `not_found`，message 为「未找到」。不使用 Go 1.22 的 `"GET /path"` 方法前缀，方法不对时也是 404。

`POST` 和 `PATCH` 不接受查询参数。`GET` 只接受本节列出的查询参数，每个参数最多出现一次。`DELETE` 只接受一个 `version`。不符合时返回 400 `invalid_fields`。

内部错误仍返回 500 和空正文，不把 SQL 原文给客户端。

分页对象仍是 `{ "data": [], "total": 0, "limit": 30, "offset": 0 }`。`limit` 省略时为 30，允许 1 到 100。`offset` 省略时为 0，允许大于等于 0 的整数。两者都是不带正号、不带前导零的十进制。`offset` 超过总数时返回空 `data` 和真实 `total`。`total` 与当页 `data` 在同一个事务里读取。

路由注册示例（方法在处理器内分支）：

```
/api/v1/items/{id}/return-tasks
/api/v1/return-tasks
/api/v1/return-tasks/{id}
/api/v1/return-tasks/{id}/complete
/api/v1/trash
/api/v1/trash/{id}
/api/v1/trash/{id}/restore
```

### 6.1 物品对象

物品对象在第三阶段字段之外增加 `deleted_at` 和 `return_tasks`。列表里 `data` 的元素仍是完整物品对象。

```json
{
  "id": 1,
  "name": "遥控车",
  "alias": null,
  "model": null,
  "spec": null,
  "quantity_note": null,
  "note": null,
  "version": 2,
  "created_at": "2026-09-30T00:00:00Z",
  "updated_at": "2026-09-30T00:00:00Z",
  "deleted_at": null,
  "locations": [],
  "categories": [],
  "return_tasks": [
    {
      "id": 8,
      "item_id": 1,
      "item_name": "遥控车",
      "part_note": "充电器",
      "reason": "借出",
      "destination_note": "放在老王家",
      "completed_at": null,
      "version": 1,
      "created_at": "2026-09-30T00:00:00Z",
      "updated_at": "2026-09-30T00:00:00Z"
    }
  ]
}
```

正常列表和 `GET /api/v1/items/{id}` 里 `deleted_at` 为 `null`。回收站对象里 `deleted_at` 为进回收站时的时间文本。

`return_tasks` 没有事项时为空数组，不出现 `null`。排序：未完成在前（`completed_at` 为空），已完成在后；同一组内按 `id` 升序。

`GET /api/v1/items` 的查询参数仍以第三阶段为准。本阶段固定加上 `deleted_at IS NULL`。出现 `deleted`、`trashed`、`include_deleted` 等不认识的参数，返回 400，短句「不支持的参数」。

`GET /api/v1/items/{id}`：行不存在，或 `deleted_at` 非空，都返回 404。

`POST /api/v1/items` 新建的 `deleted_at` 为 `null`，`return_tasks` 为 `[]`。

`PATCH /api/v1/items/{id}` 仍只改第三阶段允许的字段。正文出现 `deleted_at` 或 `return_tasks` 时忽略（未知字段）。目标已进回收站则 404。

### 6.2 普通删除改为进回收站

| 方法与路径 | 成功 | 说明 |
| --- | --- | --- |
| `DELETE /api/v1/items/{id}?version=` | 204，空正文 | 进入回收站 |

路径所指行必须存在且 `deleted_at` 为 `NULL`。否则 404。版本不一致则 409，物品仍在正常列表。

成功后：

- `GET /api/v1/items/{id}` 返回 404。
- `GET /api/v1/trash/{id}` 返回 200，带非空 `deleted_at`，以及原来的位置、分类、归位事项。
- 正常物品列表、搜索、`placement=unlocated`、`location=`、`in_location`、`category`、`uncategorized` 都不包含它，对应 `total` 减 1（若它原先命中）。
- 位置行、分类行、关联行、事项行都还在。

第二阶段规格里「物理删除物品和它的关联」被本段取代。位置的 `DELETE /api/v1/locations/{id}` 和分类的 `DELETE /api/v1/categories/{id}` 仍是物理删除空记录。

仓库里按旧契约写的测试必须改写，不能再为了让它们通过而继续物理删除：

- `TestItemDelete`、`TestItemDeleteBody`：`DELETE /api/v1/items/{id}` 之后 `items` 行仍在，`deleted_at` 非空；`item_locations`、`item_categories` 行仍在。
- 第三阶段「删除物品后，该物品的分类关联消失」改为：普通删除后关联仍在，物品不出现在分类页的物品列表里；永久删除后关联才消失，分类行仍在。
- 「删最大 id 再新建、旧 id 不复用」只适用于 `DELETE /api/v1/trash/{id}`。不能再用 `DELETE /api/v1/items/{id}` 当物理删除。

### 6.3 回收站

| 方法与路径 | 成功 | 说明 |
| --- | --- | --- |
| `GET /api/v1/trash` | 200，分页对象 | 仅 `deleted_at` 非空的物品 |
| `GET /api/v1/trash/{id}` | 200，物品对象 | 已在回收站中的一件 |
| `POST /api/v1/trash/{id}/restore` | 200，物品对象 | 恢复到正常列表 |
| `DELETE /api/v1/trash/{id}?version=` | 204，空正文 | 永久删除 |

`GET /api/v1/trash` 只接受 `limit`、`offset`。排序为 `name`、`id`。`data` 的元素是带非空 `deleted_at` 的物品对象。不提供关键词和筛选。

`GET /api/v1/trash/{id}`：行不存在，或 `deleted_at` 为空，都返回 404。

恢复正文：

```json
{ "version": 2 }
```

`version` 必填，类型规则与物品 PATCH 相同。成功后 `deleted_at` 为 `null`，`version` 加 1，响应是正常物品对象。之后 `GET /api/v1/items/{id}` 返回 200，`GET /api/v1/trash/{id}` 返回 404。

永久删除只作用于 `deleted_at` 非空的行。成功后 `GET /api/v1/items/{id}` 和 `GET /api/v1/trash/{id}` 都是 404。再用该 id 去 PATCH、去恢复、去进回收站，都是 404。该物品的关联和事项都不在了。它曾经占用的位置和分类仍在。

测试要：永久删除物品表当前最大 id 后新建，新 id 与被删 id 不同。用被删 id 和 `version=1` 去 PATCH、去 `DELETE /items`、去 `DELETE /trash`，都返回 404，新物品不变。

### 6.4 归位事项

事项对象：

```json
{
  "id": 8,
  "item_id": 1,
  "item_name": "遥控车",
  "part_note": null,
  "reason": "借出",
  "destination_note": "放在老王家",
  "completed_at": null,
  "version": 1,
  "created_at": "2026-09-30T00:00:00Z",
  "updated_at": "2026-09-30T00:00:00Z"
}
```

`item_name` 是读取时物品的当前名称，不另存一份。

| 方法与路径 | 成功 | 说明 |
| --- | --- | --- |
| `GET /api/v1/return-tasks` | 200，分页对象 | 未完成且物品未进回收站 |
| `POST /api/v1/items/{id}/return-tasks` | 201，事项对象 | 新建 |
| `GET /api/v1/return-tasks/{id}` | 200，事项对象 | 单条，含已完成；物品在回收站则 404 |
| `POST /api/v1/return-tasks/{id}/complete` | 200，事项对象 | 完成这一条 |
| `DELETE /api/v1/return-tasks/{id}?version=` | 204，空正文 | 去掉未完成的一条 |

`GET /api/v1/return-tasks` 只接受 `limit`、`offset`。命中条件：`completed_at IS NULL`，并且所属物品 `deleted_at IS NULL`。排序为 `created_at`、`id`（先建立的在前）。`data` 的元素是事项对象。

创建正文，字段均可省略：

```json
{
  "part_note": "充电器",
  "reason": "借出",
  "destination_note": "放在老王家"
}
```

省略、`null`、或去掉空白后为空，该字段存 `NULL`。`part_note` 为 `NULL` 就是整件。物品不存在或已进回收站则 404。创建不带 `version`。新建 `completed_at` 为 `null`，`version` 为 1。不改物品的 `version` 和 `updated_at`。

完成正文：

```json
{ "version": 1 }
```

成功后 `completed_at` 为当前时间文本，事项 `version` 加 1。物品位置、分类、物品 `version` 和 `updated_at` 都不变。其他事项不变。已经完成则 409 `already_completed`，message 为「这条事项已经完成」，`completed_at` 保持第一次完成时的值。物品在回收站则 404。

去掉事项：仅当 `completed_at` 为空且物品未进回收站。已完成则 409 `already_completed`，事项仍在。物品在回收站则 404。不改物品的 `version` 和 `updated_at`。

`GET /api/v1/return-tasks/{id}` 对未完成和已完成都返回 200，只要所属物品未进回收站。

没有修改事项字段的 PATCH。写错了就去掉未完成的那条再新建。

### 6.5 位置和分类对象上的占用计数

位置对象和分类对象增加整数 `direct_item_count`：直接关联该 id 的物品条数，**含**回收站中的物品。列表里每条也带这个字段。没有直接关联时值为 `0`，键仍出现，不能省略。不含下级。位置计的是 `item_locations` 行数；分类计的是 `item_categories` 行数。

位置页、分类页上的物品列表仍请求现有的 `GET /api/v1/items`，因此只显示未进回收站的物品。删除按钮不能只看这个列表的 `total`，必须看 `direct_item_count`（以及子级是否为空）。

`location_in_use` / `category_in_use` 的判断与 `direct_item_count` 一致：回收站里仍挂着的关联也算占用。

### 6.6 状态码与字段短句

沿用前三阶段的状态码。本阶段增加：

| 状态 | code | message | 何时 |
| --- | --- | --- | --- |
| 409 | `already_completed` | 这条事项已经完成 | 对已完成事项再完成，或用删除接口去掉已完成事项 |

`location_in_use` 和 `category_in_use` 的 code、message 不变；命中条件改为含回收站物品。

正文 JSON 类型：

- `part_note`、`reason`、`destination_note` 若出现，必须是字符串或 `null`。其他类型返回 `invalid_body`。
- 恢复和完成归位的 `version` 与物品 PATCH 的 `version` 相同。

`invalid_fields` 一次带上本次能够确定的全部字段。本阶段增加的键和短句：

| 键 | 短句 | 条件 |
| --- | --- | --- |
| `part_note` | 配件说明过长 / 配件说明不能包含控制字符 | 超过 200，或含控制字符 |
| `reason` | 原因过长 / 原因不能包含控制字符 | 超过 200，或含控制字符 |
| `destination_note` | 临时去向过长 / 临时去向不能包含控制字符 | 超过 2000，或含换行以外的控制字符 |
| `version` | 版本不正确 | 缺失，或整数小于 1 |
| `request` | 没有要修改的内容 | 现有 PATCH 规则不变 |

## 7. 页面

中文界面。页面最大宽度约 40rem。控件最小高度 2.75rem。继续用 CSS Modules，不引入新的组件库。路径过长时在容器内换行，规则与第二阶段 `.path` 相同。390 宽下导航和列表都没有横向滚动。

登录后导航：物品、位置、分类、待归位、回收站，然后是退出。

| 路径 | 页面 |
| --- | --- |
| `/returns` | 待归位清单 |
| `/trash` | 回收站列表 |
| `/trash/:id` | 回收站中的一件，只读 |

物品、位置、分类的现有路径不变。地址 `/items/:id` 读到 404（含已进回收站）时，仍显示「未找到」并回到物品列表。

### 7.1 物品页

仍可修改文字、位置、分类。有未完成归位事项时也可以保存新的存放位置。

「删除」第一次不发请求。确认区域显示「将移到回收站」，然后是「确认删除」和「取消」。成功后回到 `/`。不要显示「永久删除，无法恢复」。

新增物品页没有删除，也没有归位事项。保存成功进入物品页之后再建立事项。

物品页在分类下方增加「待归位事项」。列出该物品全部事项：未完成在前。未完成的显示配件说明（空则显示「整件」）、原因、临时去向、建立时间，以及「完成归位」和「去掉」。已完成的显示「已归位」和完成时间，没有这两个按钮。

「新增归位事项」：配件说明、原因、临时去向，均可空。配件说明的说明文字是「整件则留空」。临时去向用多行输入。提交后留在本页，列表出现新事项。

「完成归位」直接提交完成接口，不改存放位置，不另开改位置的表单。

「去掉」第一次不发请求。确认区域显示「去掉这条提醒」，然后是「确认去掉」和「取消」。

新增、完成、去掉事项成功后，只更新事项列表（或该事项对应的那一行），**不**用物品详情覆盖未保存的名称、备注、位置、分类和新事项草稿。

两个会话都还在正常列表里改同一件物品，保存得到 `version_conflict` 时：未保存的名称、备注、位置、分类、删除确认、未提交的新增事项草稿，都保持原样，显示「记录已被修改」和「加载最新内容」。删除冲突在加载最新内容后收回确认。加载后若接口已是 404（另一会话已放进回收站），显示「未找到」并回到 `/`。事项的完成或去掉遇到 `version_conflict` 或 `already_completed` 时，显示接口 `message`，该条事项按最新内容刷新。

物品保存返回 404：显示「未找到」并回到 `/`。

事项的新增、完成或去掉返回 404：先再读一次所属物品（物品页用当前路径的物品 id；待归位清单用该事项的 `item_id`）。

- 物品仍在：只刷新事项区域，提示「该事项已不存在」。物品表单和未提交的新增事项草稿保持原样，不回到列表。
- 物品也返回 404：物品页显示「未找到」并回到 `/`；待归位清单显示「未找到」并重新读取清单。
- 核实请求失败（网络或其他错误，且不是物品 404）：留在当前页，允许重试。

### 7.2 待归位清单

标题是「待归位」。空列表显示「没有待归位事项」。不在本页新建事项。

每行：物品名称（链到该物品页）、整件或配件说明、原因、临时去向。「完成归位」和「去掉」的规则与物品页相同。翻页用 `offset`，每页 30。

这里只出现未完成、且物品未进回收站的事项。完成一条后该行离开清单；同一物品的其他未完成事项仍在。

### 7.3 回收站

列表标题是「回收站」。空列表显示「回收站是空的」。每行显示名称和进入回收站的时间，点击进入 `/trash/:id`。翻页用 `offset`，每页 30。不提供搜索和筛选。

详情只读：名称、备注、位置路径、分类路径、归位事项（含已完成）。提供「恢复」。第一次按「永久删除」不发请求。确认区域显示「永久删除，无法恢复」，然后是「确认删除」和「取消」。

恢复成功后进入 `/items/:id`。永久删除成功后回到 `/trash`。

版本冲突时，只读内容和删除确认保持原样，显示「记录已被修改」和「加载最新内容」。删除冲突在加载最新内容后收回确认。若加载后记录已经不在回收站，显示「未找到」并回到 `/trash`。

### 7.4 位置页和分类页

物品列表仍只显示未进回收站的。当前范围没有这类物品时，文案仍是「这个位置里还没有物品」或「这个分类下还没有物品」。若此时 `direct_item_count` 大于 0，额外显示「回收站里还有物品占用这个位置」或「回收站里还有物品占用这个分类」，并且不显示删除。

删除按钮仅当没有子级、且 `direct_item_count` 为 0 时出现。确认文案仍是「永久删除，无法恢复」。

### 7.5 物品列表

搜索和筛选的行为仍按第三阶段。结果不含回收站。不在物品列表增加「只看回收站」；回收站走 `/trash`。

## 8. 测试

Go 测试使用临时数据库和 `httptest`，通过 HTTP 验证对外行为，不模拟 SQLite。至少覆盖：

1. 已有 `001`、`002`、`003` 的库启动后执行 `004`。再次启动不重复执行。`items.deleted_at` 列存在；已有物品的 `deleted_at` 为 `NULL`。部分索引 `items_deleted_at` 存在。`return_tasks` 表和 `return_tasks_item_id`、`return_tasks_open` 存在。
2. 一件物品建两条归位事项：一条整件（`part_note` 省略），一条 `part_note` 为「充电器」。完成整件那条后，充电器那条仍未完成；物品的 `locations`、`version`、`updated_at` 都不变。待归位列表只含未完成那条。已完成那条 `GET /api/v1/return-tasks/{id}` 仍是 200。
3. 物品放在位置 A。建立归位事项并填写临时去向。把物品改挂到位置 B。事项仍在，`destination_note` 不变；位置 A 若已无关联且无子级则可以删除；位置 B 不能删。
4. `DELETE /api/v1/items/{id}?version=` 返回 204 后：`GET /items/{id}` 为 404；`GET /trash/{id}` 为 200，位置、分类、未完成事项都在；`GET /items` 和带 `q` 的搜索都不含它；`GET /return-tasks` 不含它的未完成事项。恢复后上述正常列表重新包含它，待归位清单重新出现未完成事项，已完成事项仍是已完成。
5. 物品进回收站后，`PATCH /items/{id}`、`POST .../return-tasks`、完成事项、去掉事项都返回 404，数据不变。`DELETE /items/{id}` 对已在回收站的 id 返回 404。`POST /trash/{id}/restore` 对仍在正常列表的 id 返回 404。`GET /api/v1/items?deleted=1` 返回 400 `invalid_fields`。
6. 回收站中的物品仍关联位置和分类时，删除该位置返回 409 `location_in_use`，删除该分类返回 409 `category_in_use`，行都还在。永久删除该物品后，空位置和空分类可以删除，返回 204。
7. `DELETE /api/v1/trash/{id}?version=` 之后物品行、关联、事项都不在；位置和分类仍在。对同一 id 再恢复或再永久删除返回 404。
8. 完成已完成的事项返回 409 `already_completed`，`completed_at` 不变。去掉已完成事项同样 `already_completed`。过旧 `version` 完成事项只返回 `version_conflict`。
9. 无会话创建事项返回 401，且没有新行。错误 Origin 的创建返回 403，且没有新行。`POST /api/v1/trash/abc/restore` 在错误 Origin 下返回 403。已登录且 Origin 正确时，创建事项正文超过 32768 字节返回 413。`POST /api/v1/items/{不存在的 id}/return-tasks` 配上过长的 `part_note` 返回 404。恢复正文不是对象时，即使路径 id 不存在也返回 `invalid_body`。`DELETE /api/v1/trash/{不存在的 id}` 的 `version` 不是正整数时返回 400，不返回 404。
10. `GET /api/v1/trash/{id}?foo=1` 与 `GET /api/v1/return-tasks/{id}?foo=1` 在已登录时返回 400 `invalid_fields`；无会话时返回 401。
11. 把物品表当前最大 id 放进回收站，再 `DELETE /api/v1/trash/{id}?version=` 永久删除后新建，新 id 与被删 id 不同。用被删 id 和 `version=1` 去 PATCH、去 `DELETE /items`、去 `DELETE /trash`，都返回 404，新物品不变。对归位事项做同样的「去掉最大 id 再新建」：被删 id 完成、删除都是 404。
12. 进回收站时名称合法但 `version` 过旧，返回 409 `version_conflict`，物品仍在正常列表。先放进回收站再恢复再放进回收站之后，用第一次进回收站时的 `version` 去恢复或永久删除，返回 409 `version_conflict`，物品仍在回收站。
13. 创建后事项 `version` 为 1。完成一次后为 2。进回收站和恢复都把物品 `version` 加 1。去掉一条未完成事项后，物品 `version` 和 `updated_at` 不变。空位置和空分类的 `direct_item_count` 为 `0` 且键存在。

网页不把浏览器自动化套件放进仓库。实现时在浏览器里实际完成第 9 节的步骤。

## 9. 验收

同时满足以下条件才算本阶段完成：

- 第 8 节的 Go 测试通过。
- 桌面宽度下实际完成：登录；登记「遥控车」，放在某个位置；新增两条归位事项（整件「借出」，配件「充电器」去向「老王家」）；完成整件后清单里只剩充电器；把遥控车改到另一个位置，充电器事项仍在，原位置若已空则可以删除。普通删除遥控车时看到「将移到回收站」，确认后物品列表和待归位清单都没有它，搜索也搜不到；打开回收站看得到，位置和分类还在。恢复后物品和未完成事项都回来。再放进回收站，在回收站里看到「永久删除，无法恢复」，确认后回收站也没有它，空位置可以删。
- 另一件物品只进回收站、不永久删除。它所在的位置和所挂分类都没有删除按钮，并看到占用提示。
- 物品页改了名称但还没保存时，另一个已登录会话先把它放进回收站。回到前一个页面保存，接口返回 404，页面显示「未找到」。两个会话都还在正常列表时发生保存冲突，仍出现「记录已被修改」和「加载最新内容」。
- 两个会话打开同一件物品。页面 A 去掉某条事项后，页面 B 再完成它：提示「该事项已不存在」，未保存的名称、备注、位置、分类和新事项草稿仍在，不回到列表。
- 390 宽下打开待归位和回收站：导航可点到两项；路径和文字换行；`scrollWidth` 等于视口宽度。
- 仓库中没有照片、MCP、OAuth、Docker，也没有 `item_categories.source`。归位事项表没有位置 id 列。
- `go.sum` 与 `web/package-lock.json` 仍记录实际使用的版本。本阶段不新增依赖时，这两个文件的依赖项不增加。
