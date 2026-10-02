# 有处：分类和搜索

日期：2026-09-30

状态：复核通过。本文是第三阶段的实现依据。尚未开始实现。

依据：同仓库《家庭物品整理与查找需求.md》第六节、《技术方案草稿.md》第 1、3、4、5、8 节，以及已实现的《基础登录与存储》《物品和位置》规格。冲突时，登录、会话、迁移器和 SQLite 连接以第一阶段规格为准；位置页、直接位置列表 `location=`、物品和位置的字段长度与失败顺序以第二阶段规格为准；分类树、物品分类关联、关键词和物品列表上的筛选以本文为准。

本文只覆盖交付顺序中的第三段。

## 1. 目标

已登录的操作者可以：

- 建立单父级分类树；同一父级下名称不重复；根分类也受这条约束。
- 给一件物品挂多个分类；分类只用来缩小查找范围，不决定存放位置。
- 在物品列表用关键词搜索名称、别名、型号、备注。
- 按分类、位置范围、待定位、未分类缩小结果；条件同时出现时全部满足。
- 分类筛选和位置范围默认包含下级，也可以只要当前这一层。
- 多分类按分支做任意匹配或全部匹配；结果按物品去重后分页。

## 2. 不做的事

本阶段不实现待归位、回收站、软删除、照片、MCP、OAuth、个人访问令牌、Docker、备份、扫码、通知、变更事件表和静态资源托管。

不增加 `deleted_at`。不增加 `item_categories.source`，不实现 AI 归类、归类批次和撤销。以后接入 AI 时，本阶段已经写下的分类关联一律视为人工关联，再单独记录 AI 新增的关联。本文不设计那张来源列和撤销接口。

不把完整路径写进表。搬动分类时不改写下级行，也不改写物品关联行。

位置页继续只列出直接子位置和直接物品，不在位置页上增加搜索或「含下级」。

登录、会话、Origin、登录限流和已有迁移保持现有行为。不新增环境变量，不新增 Go 模块依赖，不新增 npm 依赖。

## 3. 仓库

新增：

```
migrations/003_categories_search.sql
```

分类的事务、父子规则、版本判断、字段校验和查找条件放在 `internal/catalog`。HTTP 路由仍由 `internal/httpapi` 注册，负责认证、Origin、正文大小、状态码和 JSON，不复制一套业务判断。网页仍在 `web/src`，页面可以拆文件，路由集中注册。

沿用现有迁移器，只增加 `003` 文件。嵌入版本因此是连续的 `001`、`002`、`003`。

实现时更新 `README.md`：说明本地可以管理分类，并在物品列表搜索和筛选；开发入口仍是 `http://127.0.0.1:5173`，并保留 Origin 与首次账号的说明。

## 4. 表

`003_categories_search.sql` 创建下面两张表。时间列使用 UTC 的 RFC3339Nano 文本，与现有表一致。

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

`categories.id` 使用 `INTEGER PRIMARY KEY AUTOINCREMENT`。插入时不指定 `id`。已提交的删除不会把该 id 再分配给新行。回滚、未提交的插入不在本阶段测试里。

测试要删除当前最大 id 的分类后新建一条，新 id 与被删 id 不同。再用被删 id 和 `version=1` 去修改、去删除，都返回 404。新记录的名称和 `version` 保持刚创建时的值。

可选文字清空后存 `NULL`，不存空字符串。名称按原文保存，不去掉内部大小写，不合并不同父级下的同名。写入前去掉首尾空白；去掉之后为空，按「未提供值」处理。

同级重名比较的是去掉首尾空白之后写入库的字符串，区分大小写。根分类的 `parent_id` 是 `NULL`。SQLite 里多条 `NULL` 在普通 `UNIQUE(parent_id, name)` 下可以并存，所以必须使用上面两条部分唯一索引：一条覆盖 `parent_id IS NULL` 的名称，一条覆盖同一非空父级下的名称。应用层按同样规则判断并返回 `name_taken`；这两条索引保证并发写入也不会插入第二条。

字符数按 Unicode 码点计算，规则与位置名相同：1–80 个码点，不允许换行，除此外不能包含 Unicode 类别为 Cc 的控制字符。最大长度不写成 SQL `CHECK`。

分类没有类型、没有编号、关联上没有放置说明。`item_categories` 没有来源列。

## 5. 分类规则

分类是单父级树。父级必须已经存在。新父级不能是自己，也不能是自己的下级。下级指沿 `parent_id` 向上能走到该分类的那些分类。

根分类的 `parent_id` 为 `NULL`。任意已存在的分类都可以作为父级，只要不形成循环。

同一父级下不能有两条去掉首尾空白后完全相同的名称，根级也是同一级。下面三种写入只要会与一条**其他**记录冲突，就返回 409 `name_taken`，并且不写入：

- 创建。
- 改名，仍留在当前父级。
- 改 `parent_id`（含移到根），名称与目标父级下已有记录相同。

把自己的名称改成现在的名称，或提交的父级仍是当前父级，不算与自己冲突。只要 PATCH 合法，`version` 仍加 1。

创建、修改、删除都使用 `BEGIN IMMEDIATE`。字段规则没有通过时不写入，也不因为版本过旧返回 409。版本通过之后，才在同一事务里校验父级、沿父链走到根、以及同级名称。任一校验失败则整次请求不写入。对外可观察的失败顺序以第 6 节开头的清单为准。

删除分类前，事务内确认它没有子分类，且 `item_categories` 没有引用**该分类自己的 id**。有任一种引用时不删除，物品行和物品的其他分类关联保持原样。挂在子分类上的物品不单独阻止删除父分类；父分类只要还有子分类就不能删。删除物品只级联删除该物品的 `item_categories` 行，分类仍在。

成功的修改把 `version` 加 1，并更新 `updated_at`。`created_at` 不变。新建时 `version` 为 1。字段只要出现在合法的 PATCH 里就写入，即使新值与旧值相同，`version` 也加 1。

搬动分类只改它自己的 `parent_id`。子分类的 `parent_id` 仍指向它。物品的 `category_id` 不变。之后读取时 `path` 按新的父链现算。

## 6. HTTP

响应沿用 `application/json; charset=utf-8`。错误至少包含 `code` 和 `message`。`message` 给人读，判断以 `code` 为准。未知 JSON 字段忽略。

已知的分类写路由，以及本阶段改到的物品写路由，按下面的顺序停在第一个失败上。方法不对的请求不进入这个顺序，直接返回 404。

1. Origin。不符合或缺少时返回 403 `origin_rejected`，message 为「来源不被接受」，并且不写入。
2. 会话。没有有效会话时返回 401 `unauthenticated`，message 为「未登录」，并且不写入。Origin 和会话都失败时返回 403。
3. 请求大小，以及 JSON 能否解析、类型是否符合下文。`POST` 和 `PATCH` 正文超过 32768 字节返回 413 `body_too_large`，message 为「请求正文过大」。JSON 类型错误返回 400 `invalid_body`，不带 `fields`。`POST` 和 `PATCH` 带了查询参数时，也在这一步返回 400 `invalid_fields`。`DELETE` 不读取正文；它的 `version` 查询参数不合法，或出现 `version` 以外的查询参数时，在这一步返回 400 `invalid_fields`。
4. 记录是否存在。这一步只用于修改和删除。路径 id 要匹配 `^[1-9][0-9]*$`，并且库里要有这一行。否则返回 404 `not_found`，message 为「未找到」。创建没有既有 id，跳过这一步，也跳过第 6 步。
5. 字段规则。返回 400 `invalid_fields`。
6. 版本。不一致时返回 409 `version_conflict`，不写入。
7. 关系与唯一性约束：父级、循环、同级重名、分类仍被引用、分类 id 是否存在。返回下文对应的 400 或 409。

因此，不存在的 id 配上坏 JSON 返回 `invalid_body`，不返回 404。不存在的 id 配上类型正确但字段不合法的 JSON 返回 404。字段不合法且版本也过旧时返回 400。版本过旧时不再返回 `invalid_parent`、`category_cycle`、`category_in_use` 或 `name_taken`。

路径 id 在 Origin、会话和正文（或 DELETE 的 `version`）之后才解析。`PATCH /api/v1/categories/abc` 在错误 Origin 下返回 403，不返回 404。

`GET` 先核对会话，没有会话时返回 401，不核对 Origin。会话有效后，查询参数不合法返回 400，然后再判断查询参数或路径里指向的记录是否存在。`GET /api/v1/categories/{id}` 和现有的物品、位置详情一样：会话有效后若 `RawQuery` 不是空字符串，返回 400 `invalid_fields`，键为查询参数名，短句为「不支持的参数」，不读取该行。

分类的创建、修改、删除，以及物品上替换 `categories`，都使用 `BEGIN IMMEDIATE`。

登录接口的正文上限仍是 4096 字节。分类和物品的 `POST` 与 `PATCH` 上限是 32768 字节。`DELETE` 的版本只放在查询参数里。

未知路径返回 404 `not_found`，message 为「未找到」。

`POST` 和 `PATCH` 不接受查询参数。`GET` 只接受本节列出的查询参数，每个参数最多出现一次。`DELETE` 只接受一个 `version`。不符合时返回 400 `invalid_fields`。

内部错误仍返回 500 和空正文，不把 SQL 原文给客户端。应用层能够识别的校验失败和约束失败，映射成下面的 code，不返回 500。

分页对象仍是 `{ "data": [], "total": 0, "limit": 30, "offset": 0 }`。`limit` 省略时为 30，允许 1 到 100。`offset` 省略时为 0，允许大于等于 0 的整数。两者都是不带正号、不带前导零的十进制。`offset` 超过总数时返回空 `data` 和真实 `total`。`total` 与当页 `data` 在同一个事务里读取。`parent=0` 不是「没有父级」，而是 400。

### 6.1 分类

| 方法与路径 | 成功 | 说明 |
| --- | --- | --- |
| `GET /api/v1/categories` | 200，分页对象 | 见下方四种列表 |
| `POST /api/v1/categories` | 201，分类对象 | 创建 |
| `GET /api/v1/categories/{id}` | 200，分类对象 | 单个分类 |
| `PATCH /api/v1/categories/{id}` | 200，分类对象 | 修改名称或父级 |
| `DELETE /api/v1/categories/{id}?version=` | 204，空正文 | 删除空分类 |

分类对象：

```json
{
  "id": 5,
  "name": "传感器",
  "parent_id": 1,
  "version": 1,
  "created_at": "2026-09-30T00:00:00Z",
  "updated_at": "2026-09-30T00:00:00Z",
  "path": [
    { "id": 1, "name": "电子配件" },
    { "id": 5, "name": "传感器" }
  ]
}
```

`path` 从根到自己，每项只有 `id`、`name`。最后一项就是这个分类，其 `id` 等于分类对象的 `id`。读取时由 `parent_id` 计算。

`GET /api/v1/categories` 的四种列表互斥：

| 查询 | 结果 | 排序 |
| --- | --- | --- |
| 无 `parent`、`flat`、`eligible_parent` | `parent_id` 为空的分类 | `name`、`id` |
| `parent={id}` | 该分类的直接子级。父级不存在则 404 | 同上 |
| `flat=1` | 全部分类，供物品勾选 | `id` 升序 |
| `eligible_parent=1` | 允许作为父级的分类 | `id` 升序 |

中文按 Unicode 码点，不按拼音。`flat` 和 `eligible_parent` 的值只能是 `1`。`eligible_parent=1` 返回全部分类。可选 `exclude={id}` 会去掉这个分类和它的全部下级；`exclude` 所指分类不存在时 404。没有 `eligible_parent` 时不能带 `exclude`。`parent`、`flat`、`eligible_parent` 两两同时出现，返回 400，涉及的键都是「不支持的参数」。

创建正文：

```json
{ "name": "传感器", "parent_id": 1 }
```

`name` 必填。`parent_id` 省略或为 `null` 表示根。

修改正文必须带整数 `version`，并且至少再出现一个可修改字段：`name` 或 `parent_id`。未出现的字段保持原值。`parent_id: null` 表示移到根。解析必须区分「字段未出现」和 JSON `null`。

### 6.2 物品对象上的分类

物品对象在第二阶段字段之外增加 `categories`。列表里 `data` 的元素仍是完整物品对象。

```json
"categories": [
  {
    "category_id": 5,
    "path": [
      { "id": 1, "name": "电子配件" },
      { "id": 5, "name": "传感器" }
    ]
  }
]
```

`categories` 按 `category_id` 升序。没有关联行时为空数组，迁移前已有的物品也是 `[]`。`path` 的读法与分类对象相同，最后一项的 `id` 等于该关联的 `category_id`。

创建时 `categories` 省略或 `[]` 都表示没有分类，响应里为 `[]`。`categories: null` 拒绝。「不出现则关联不变」只适用于 `PATCH`：`PATCH` 省略 `categories` 时原关联保留；出现数组就在同一事务里用这组关联替换全部旧关联，包括 `[]`。替换关联也把物品的 `version` 加 1，并更新 `updated_at`。

可修改字段在第二阶段名单上增加 `categories`。至少出现其中一个可修改字段（加上 `version`）才算有修改内容。

数组元素是 `{ "category_id": 5 }`。按数组顺序检查，同一个分类只能出现一次，分类必须存在。`category_id` 小于 1 视为分类不存在。元素上的未知字段忽略。

`categories` 的 JSON 类型规则与 `locations` 平行：

- 若出现，JSON 类型必须是数组。字符串、数字、布尔、对象，以及数组里不是对象的元素（含 `null`、数字和字符串），整份返回 `invalid_body`，不带 `fields`。
- `categories: null` 返回 `invalid_fields`，`fields.categories` 为「分类格式不正确」。元素缺少 `category_id`，或 `category_id` 为 `null`，也用这一句。
- `category_id` 不是不带小数的数字时返回 `invalid_body`。
- 整数小于 1，或该 id 在库里不存在时，`fields.categories` 为「所选分类不存在」。
- 重复关联是 `invalid_fields`，短句是「同一分类只能关联一次」。

`locations` 仍按第二阶段规格处理。一次 PATCH 可以同时出现 `locations` 和 `categories`。`categories` 只保留按数组顺序发现的第一条关联错误；其他字段的错误仍同时返回。

### 6.3 `GET /api/v1/items` 查询参数

第二阶段的 `limit`、`offset`、`placement=unlocated`、`location={id}` 保留。`location={id}` **只**表示直接关联该位置的物品，供位置页使用；位置不存在则 404。本阶段增加下面的参数。每个参数最多出现一次。

| 参数 | 含义 |
| --- | --- |
| `q` | 关键词。去掉首尾空白后为空，视为没有关键词条件。最长 200 个码点 |
| `in_location={id}` | 位置范围。命中至少有一个存放位置落在该范围内的物品。位置不存在则 404 |
| `in_location_descendants` | 只与 `in_location` 一起出现。`1` 含该位置及其下级（该参数省略时也按 `1`）。`0` 仅当前这一层 |
| `category` | 一个或多个分类 id，逗号分隔、无空格，例如 `3` 或 `3,7`。至少一个、最多 20 个，不能重复。所列 id 有一个不存在则 404 |
| `category_match` | 只在 `category` 含两个或更多 id 时出现。`any` 任意匹配；`all` 全部匹配。省略时按 `any` |
| `category_descendants` | 只与 `category` 一起出现。`1` 每个所选分类都含下级（省略时也按 `1`）。`0` 每个所选分类都只含自己 |
| `uncategorized=1` | 没有任何分类关联的物品 |

`location` 没有含下级开关，语义仍是直接关联。物品列表的位置筛选使用 `in_location`。

`in_location_descendants=0` 的命中集合与 `location={同一 id}` 相同，两者仍是不同参数：前者给物品列表，后者给位置页。同时出现返回 400。

查询参数形状全部检查完，有错误则一次返回本次能确定的全部 `fields`，不判断记录是否存在。形状合法后再按这个顺序核对存在性：`location` → `in_location` → `category` 里从左到右每个 id。缺哪一个就返回 404 `not_found`，message「未找到」。

互斥组合返回 400 `invalid_fields`，涉及的键都带「不支持的参数」：

| 组合 |
| --- |
| `placement` 与 `location` |
| `placement` 与 `in_location` |
| `location` 与 `in_location` |
| `uncategorized` 与 `category` |

下列情况该参数名的短句也是「不支持的参数」：

- `in_location_descendants` 没有伴随合法的 `in_location`
- `category_descendants` 没有伴随合法的 `category`
- `category_match` 出现时，`category` 不是两个或更多合法 id
- `uncategorized` 与 `category_match` 或 `category_descendants` 同时出现
- `uncategorized` 不是 `1`
- `in_location_descendants` 或 `category_descendants` 不是 `0` 或 `1`
- `category_match` 不是 `any` 或 `all`
- `placement` 不是 `unlocated`
- 不认识的参数，或同一参数出现两次

`in_location` 单个 id 的格式与 `location` 相同：无前导零正整数，否则「参数不正确」。`category` 整段按逗号拆开后，每一段都必须是无前导零正整数；出现空段、空格、重复 id、超过 20 个 id，`fields.category` 为「参数不正确」。`q` 去掉首尾空白后超过 200 个码点为「关键词过长」；含不允许的控制字符为「关键词不能包含控制字符」。`q` 允许内部空白，整段当作一个子串，不拆成多个词。

### 6.4 查找命中规则

有多个条件时，物品必须**同时满足**已经出现的关键词、位置条件和分类条件。位置条件指 `placement`、`location`、`in_location` 三者之中合法的那一个。分类条件指 `uncategorized` 或 `category` 之中合法的那一个。

关键词：在 `name`、`alias`、`model`、`note` 四个字段上做子串匹配，任一字段命中即可。不搜索规格和数量说明。使用参数化 `LIKE`，用户输入里的 `\`、`%`、`_` 先转义，再作为普通字符。依赖 SQLite 默认对 ASCII 字母大小写不敏感；不要打开 `case_sensitive_like`。中文按原文匹配。字段为 `NULL` 时该字段不命中。

位置范围：`in_location_descendants` 为 `1` 或省略时，范围是该位置自己以及它的全部下级；为 `0` 时只有它自己。物品的任意一条 `item_locations` 落在这个集合里即满足位置范围。同一物品因多个存放位置落入同一范围时只计一次。

分类按**分支**判断，不要用「命中的 `item_categories` 条数」或「命中的所选 id 个数等于所选个数」来实现全部匹配。

对每个所选分类 id 单独计算一个分支集合：

- `category_descendants` 为 `1` 或省略：该分类自己，以及沿 `parent_id` 向上能走到它的全部分类（它的下级）。
- 为 `0`：只有该分类自己。

物品命中一个所选分支，当且仅当它至少有一条 `item_categories` 的 `category_id` 落在该分支集合里。

- 任意匹配（`category_match` 省略或 `any`）：至少命中一个所选分支。
- 全部匹配（`all`）：每一个所选分支都命中。挂在子分类上算命中祖先的分支；不要求物品再直接关联那个祖先。

固定例子：根分类「电子配件」，其子分类「传感器」。物品只关联「传感器」。

| 查询 | 该物品 |
| --- | --- |
| `category={电子配件},{传感器}`（默认含下级、任意匹配） | 命中 |
| 同上并 `category_match=all` | 命中。传感器既落在电子配件的分支里，也落在传感器的分支里 |
| 同上并 `category_match=all` 且 `category_descendants=0` | 不命中。它没有直接关联电子配件 |
| 只关联电子配件，选这两个分类，含下级，`all` | 不命中。电子配件不落在传感器的分支里 |

`uncategorized=1` 命中没有任何 `item_categories` 行的物品。

匹配之后按物品 id 去重，再按 `name`、`id` 排序，再应用 `limit` 与 `offset`。`total` 是去重后的条数。一件物品命中多个分类分支或多个位置时，`data` 里只出现一次，对象仍带它的全部 `locations` 和 `categories`。

### 6.5 状态码与字段短句

沿用第二阶段的状态码。本阶段增加：

| 状态 | code | message | 何时 |
| --- | --- | --- | --- |
| 409 | `name_taken` | 同级已有相同名称 | 创建、改名或移动后会与同级另一条记录重名 |
| 409 | `category_cycle` | 不能移到自己的下级 | 新父级是自己或自己的下级 |
| 409 | `category_in_use` | 这个分类下面还有内容 | 删除时仍有子分类，或仍有物品直接关联该分类 |

`invalid_parent` 用于父级不存在，或 `parent_id` 整数小于 1。分类没有类型上的父子限制。

正文 JSON 类型：分类的 `name`、`parent_id`、`version` 与位置对应字段相同。`categories` 见 6.2。

`invalid_fields` 一次带上本次能够确定的全部字段。本阶段增加的键和短句：

| 键 | 短句 | 条件 |
| --- | --- | --- |
| `name` | 请填写名称 / 名称过长 / 名称不能包含控制字符 | 与位置名相同 |
| `categories` | 分类格式不正确 | 值为 `null`，或元素缺少 `category_id`。不是数组或元素不是对象时走 `invalid_body` |
| `categories` | 同一分类只能关联一次 | 数组里分类重复 |
| `categories` | 所选分类不存在 | 某个 `category_id` 小于 1 或不在库里 |
| `q` | 关键词过长 | 去掉首尾空白后超过 200 个码点 |
| `q` | 关键词不能包含控制字符 | 含 Unicode 类别 Cc 的字符 |
| `request` | 没有要修改的内容 | PATCH 除版本外没有可修改字段 |
| `in_location` / `category` 等 | 参数不正确 | id 格式不合法，或 `category` 的逗号列表不合法 |
| 参数名 | 不支持的参数 | 不认识、重复、互斥、或枚举值不在允许集合里 |

`offset` 合法但超过总数时不报错。

## 7. 网页

界面文字使用中文。登录页保持现有宽度和字段。登录后的页面最大宽度 40rem，单列。按钮和输入框最小高度 2.75rem。沿用现有 CSS Modules、字体和颜色，不引入组件库。390 宽下页面不出现横向滚动，路径可以换行。

已登录访问 `/login` 进入 `/`。未登录访问下面任一页进入 `/login`。任一业务接口返回 401 时进入 `/login`。

页头显示用户名、通向 `/` 的「物品」、通向 `/locations` 的「位置」、通向 `/categories` 的「分类」，以及「退出」。

| 路径 | 页面 |
| --- | --- |
| `/` | 物品列表：搜索与筛选 |
| `/items/new` | 新增物品，可挂分类 |
| `/items/:id` | 查看和修改物品，含分类 |
| `/locations` 及位置各页 | 仍按第二阶段规格 |
| `/categories` | 根分类 |
| `/categories/new` | 新增分类 |
| `/categories/:id` | 查看分类、改名、改父级、子分类、该分支下的物品 |

### 7.1 物品列表

标题是「物品」，有通向 `/items/new` 的「新增物品」。每行显示名称和位置路径，规则与第二阶段相同。点击进入该物品。

顶部常驻搜索框和「查找」。中文输入以回车或「查找」提交；组字过程中不发请求。提交时把去掉首尾空白后的关键词写入 `q`；空白则从地址里去掉 `q`。

分类、位置范围、待定位、未分类放在可展开的「筛选」里，默认收起。已生效的关键词和筛选在筛选区外显示摘要，并提供「清空筛选」。摘要包括：关键词正文；位置路径，并标明含下级或仅当前；分类路径，多个时标明任意匹配或全部匹配，以及含下级或仅当前；未分类；待定位。

地址栏只使用第 6.3 节的参数。物品列表**不写** `location=`。位置筛选只写 `in_location`。含下级时省略对应的 `*_descendants`；仅当前时写 `=0`。任意匹配时省略 `category_match`。改关键词或任一筛选时去掉 `offset`，回到第一页。上一页、下一页只改 `offset`，其余条件保持。

界面在改条件时必须同时清掉会变成非法的依附参数，不能只把控件藏起来而把旧参数留在地址栏：

| 操作 | 从地址里去掉 | 写入 |
| --- | --- | --- |
| 分类从两个减到一个 | `category_match` | 仍保留 `category`（一个 id）和现有的 `category_descendants` |
| 分类全部去掉 | `category`、`category_match`、`category_descendants` | 无 |
| 改为未分类 | `category`、`category_match`、`category_descendants` | `uncategorized=1` |
| 在未分类下再选分类 | `uncategorized` | `category` |
| 去掉位置范围 | `in_location`、`in_location_descendants` | 无 |
| 改为待定位 | `in_location`、`in_location_descendants` | `placement=unlocated` |
| 在待定位下再选位置范围 | `placement` | `in_location` |
| 清空筛选 | `q`、`placement`、`in_location`、`in_location_descendants`、`category`、`category_match`、`category_descendants`、`uncategorized`、`offset` | 无，回到 `/` |

「仅当前位置」只在已选位置范围时出现，默认含下级。「仅当前分类」只在已选至少一个分类时出现，默认含下级。任意匹配 / 全部匹配只在选了两个及以上分类时出现，默认任意匹配。

未处于待定位时提供「只看待定位」：写入 `placement=unlocated`，去掉 `in_location`、`in_location_descendants` 和 `offset`，保留 `q` 和分类相关参数。已经是待定位时提供「全部物品」：去掉 `placement` 和 `offset`，保留其余仍合法的参数。

空列表：地址里没有任何筛选且库中无物品时显示「还没有物品」；仅有 `placement=unlocated`、没有其他条件且没有此类物品时显示「没有待定位的物品」；其余有条件而没有命中时显示「没有符合条件的物品」。

位置范围选择器请求 `GET /api/v1/locations?flat=1`，分类选择器请求 `GET /api/v1/categories?flat=1`，都是 `limit=100`，从 `offset=0` 起按 `total` 连续取完。

### 7.2 物品表单

新增和编辑在存放位置下方增加分类，可添加多条，每条显示完整路径，没有逐条说明。选择器请求 `GET /api/v1/categories?flat=1`，同样按页取完。

地址带合法 `category` 时，先读取该分类并放入初始关联；读取失败则显示接口的 `message`，仍允许只保存名称。`location` 与 `category` 预填可以同时存在。保存成功后进入该物品页。

创建请求在操作者没有选分类时不要带 `categories`，或带 `[]`；两种在服务端都得到空关联。编辑时未改分类则 PATCH 不出现 `categories`。

版本冲突时，未保存的名称、备注、位置选择、分类选择和删除确认都保持原样，显示「记录已被修改」和「加载最新内容」。点击该按钮后才读取并填回最新记录。删除冲突在加载最新内容后收回确认。其他错误显示接口 `message`；有 `fields` 时在对应控件旁显示短句。读取中显示「正在加载」。读取失败显示 message 和「重试」。提交进行中禁用对应按钮。

### 7.3 分类各页

分类列表使用查询参数 `offset`，每页条数用接口默认的 30。本页按接口顺序展示根分类。`total` 大于 `offset` 加本页条数时显示「下一页」，`offset` 大于 0 时显示「上一页」。整份列表为空时显示「还没有分类」。有通向 `/categories/new` 的「新增分类」。

新增分类的标题是「新增分类」。填写名称。可以选择「不设置父级」。父级选择器使用 `eligible_parent=1`，`limit=100`，从 `offset=0` 起按 `total` 连续取完。地址带 `parent` 时，父级固定为该分类，并提供「重新选择父级」，链到不带 `parent` 的新增页。保存成功后进入新分类页。地址里的 `parent` 读不到则显示接口的 `message`，不提交创建。

分类页的标题是名称。面包屑使用 `path`，上级是链接，当前不是链接，之间用 ` / `。下面是直接子分类和该范围内的物品。子分类的页起点用查询参数 `children`，物品用 `items`。范围开关写入 `category_descendants`：省略表示含下级，`0` 表示仅当前分类。地址栏不写 `category_descendants=1`。切换到仅当前分类时写入 `category_descendants=0`；切回含下级时去掉 `category_descendants`。无论往哪边切，都去掉 `items`，保留 `children`。子分类翻页只改 `children`，保留 `items` 与当时的范围开关。

物品列表请求 `GET /api/v1/items?category={当前 id}`，需要仅当前时再加 `category_descendants=0`；`offset` 取自页面的 `items`。没有子分类时显示「这里还没有下一级分类」。当前范围没有物品时显示「这个分类下还没有物品」。提供「在此分类下新增物品」，链到 `/items/new?category={id}`。

子分类 `total` 为 0、且 `GET /api/v1/items?category={id}&category_descendants=0` 的 `total` 为 0 时才显示删除。确认区域显示「永久删除，无法恢复」。删除成功后回到父级；没有父级则回到 `/categories`。

分类页可以修改名称和父级。父级选择器带上 `exclude` 为当前 id，同样按页取完。可以改成「不设置父级」。版本冲突的处理与物品页相同，未保存的名称和父级选择保持原样。

位置页行为保持第二阶段规格：只列出直接物品，不增加搜索和含下级。

## 8. 测试

Go 测试使用临时数据库和 `httptest`，通过 HTTP 验证对外行为，不模拟 SQLite。至少覆盖：

1. 已有 `001` 和 `002` 的库启动后执行 `003`。再次启动不重复执行。两张表、两条名称唯一索引和关联表主键存在。
2. 只提交名称创建物品，响应里 `categories` 为 `[]`。再提交 `categories: []`，仍为 `[]`。`POST` 省略 `categories` 与带 `[]` 都不写关联行。
3. 两个根分类使用同一名称（含「电子配件」和前后带空白、去掉后相同的名称）时，第二条返回 409 `name_taken`，库里仍是一条。不同根下各建一个「传感器」都成功。把其中一个改挂到另一个「传感器」的父级下，返回 409 `name_taken`，`parent_id` 不变。改成与同级另一条相同的名称，同样 `name_taken`。PATCH 提交自己当前的名称，返回 200，`version` 加 1。
4. 把分类的父级改成它的下级，返回 409 `category_cycle`。原来的 `parent_id` 和 `version` 不变。`parent_id` 指向不存在的分类时返回 400 `invalid_parent`，不创建、不改写。
5. 根「电子配件」下建「传感器」。物品只关联「传感器」。`GET /api/v1/items?category={电子配件}` 含该物品。`category={电子配件},{传感器}&category_match=all` 也含该物品，`total` 为 1，`data` 里该物品只出现一次。加上 `category_descendants=0` 后该物品不出现。另建只关联「电子配件」的物品：含下级的 `all` 查询不包含它；任意匹配包含它。
6. 物品名称含 `100%` 和 `a_b`。`q=100%` 只命中前者，`q=a_b` 只命中后者，`q=100` 命中前者。名称为 `USB-Cable` 时 `q=usb` 命中。备注里的中文子串可以命中；规格里的同一子串不能单凭规格命中。
7. 物品放在子位置上。`location={父级}` 不含它。`in_location={父级}` 含它。`in_location={父级}&in_location_descendants=0` 不含它。该物品同时命中关键词时，与位置范围同时生效。`in_location` 指向不存在的 id 时返回 404。
8. `placement=unlocated` 与 `in_location` 同时出现、`location` 与 `in_location` 同时出现、`uncategorized=1` 与 `category=` 同时出现、`category_match=all` 只配一个分类 id、`in_location_descendants=0` 不带 `in_location`、`category=1&category=2` 重复参数、`category=3,3` 重复 id，都返回 400，且互斥组合里涉及的键都在 `fields` 中。
9. 未分类物品出现在 `uncategorized=1`，挂上分类后离开该列表。待定位与关键词可以同时使用。
10. 删除仍有子分类或仍有物品直接关联的分类，返回 409 `category_in_use`，分类和物品都还在。删除空分类返回 204。删除物品后，该物品的分类关联消失，分类仍在。
11. `PATCH` 省略 `categories` 时原分类保留。`categories: []` 后读取为空数组。`categories` 为字符串或元素不是对象时返回 `invalid_body`，不带 `fields`。`categories: null` 返回 `invalid_fields`。
12. 无会话创建分类返回 401，且没有新行。错误 Origin 的创建返回 403，且没有新行。`PATCH /api/v1/categories/abc` 在错误 Origin 下返回 403。已登录且 Origin 正确时，分类 `POST` 超过 32768 字节返回 413。
13. `GET /api/v1/categories/{id}?foo=1` 在已登录时返回 400 `invalid_fields`；无会话时返回 401。
14. 删除分类表当前最大 id 后新建，新 id 与被删 id 不同。用被删 id 和 `version=1` 去 PATCH、去 DELETE，都返回 404，新分类不变。
15. 名称合法但 `version` 过旧时返回 409 `version_conflict`。过旧版本且新父级会形成循环时，仍只返回 `version_conflict`，`parent_id` 不变。过旧版本且新名称会与同级冲突时，也只返回 `version_conflict`。
16. 创建后的 `version` 为 1。合法 PATCH 即使提交了相同名称，`version` 也变为 2。替换 `categories` 即使集合相同，物品 `version` 也加 1。

网页不把浏览器自动化套件放进仓库。实现时在浏览器里实际完成第 9 节的步骤。

## 9. 验收

同时满足以下条件才算本阶段完成：

- 第 8 节的 Go 测试通过。
- 桌面宽度下实际完成：登录；创建根分类「电子配件」和「智能家居」；在「电子配件」下创建「传感器」；登记「温湿度传感器」，只挂「传感器」，位置随意；物品列表搜「温湿度」能看到它；筛选「电子配件」（含下级）能看到它；同时选「电子配件」和「传感器」、全部匹配、含下级，仍能看到它，列表里只有一条；改成仅当前分类后，筛选「电子配件」看不到它，打开「传感器」看得到；「未分类」在它挂上分类后不再列出它；清空筛选后地址栏没有残留的 `category_match` 或 `*_descendants`。
- 在「传感器」下再挂满至少 31 件物品。打开「电子配件」分类页，含下级时能看到这些物品；把物品翻到下一页（地址含 `items=30`），再切换「仅当前分类」。地址栏不再有 `items`，物品区按仅当前重新从第一页读（「电子配件」没有直接关联时显示「这个分类下还没有物品」）；切换前若带有 `children`，切换后仍在。
- 再建一个根分类也叫「电子配件」，界面或接口给出同级已有相同名称，库里仍是一条。有子分类或仍有物品直接关联时不能删除。删除空分类前能看到「永久删除，无法恢复」。
- 物品页改了分类但还没保存时，另一个已登录会话先保存。回到前一个页面保存后，未保存的分类选择仍在，并出现「加载最新内容」。
- 390 宽下打开物品列表：搜索框可见；筛选默认收起；展开后可以选分类和位置；路径和摘要换行，页面没有横向滚动。根分类列表和两个分类选择器都能翻到「分类-101」（预先建好「分类-001」到「分类-101」共 101 个根分类）。
- 仓库中没有待归位、回收站、照片、MCP、OAuth、Docker，也没有 `item_categories.source`。
- `go.sum` 与 `web/package-lock.json` 仍记录实际使用的版本。本阶段不新增依赖时，这两个文件的依赖项不增加。
