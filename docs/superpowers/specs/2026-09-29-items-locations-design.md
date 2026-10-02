# 有处：物品和位置

日期：2026-09-29

状态：复核通过。已明确 `locations` 不是数组或元素不是对象时返回 `invalid_body`。本文是第二阶段的实现依据。尚未开始实现。

依据：同仓库《家庭物品整理与查找需求.md》、《技术方案草稿.md》第 3、5、8 节，以及已实现的《基础登录与存储》规格。冲突时，登录、会话、迁移器和 SQLite 连接以第一阶段规格为准；物品和位置以本文为准。

本文只覆盖交付顺序中的第二段。

## 1. 目标

已登录的操作者可以：

- 建立区域、固定储物位和移动容器，并给后两者编号。
- 只填名称登记物品；同名可以并存；位置可以稍后补。
- 把一件物品挂到多个直接位置，并分别写放置说明。
- 逐级查看某个位置里的直接子位置和直接物品。
- 移动容器只改它自己的父级；盒内物品仍指向该容器，再次读取时路径跟着变。
- 修改上述记录；删掉登错的物品；删掉没有子位置、也没有物品的位置。

## 2. 不做的事

本阶段不实现分类、关键词搜索、待归位、回收站、软删除、照片、MCP、OAuth、个人访问令牌、Docker、备份、扫码、通知、变更事件表和静态资源托管。

不增加 `deleted_at`。总体需求里的回收站留在后面的阶段。本阶段的物品删除已在范围确认时单独定为物理删除：登错的记录直接从库里去掉，界面不能恢复。空位置的删除同样不可恢复。确认删除前，界面必须显示「永久删除，无法恢复」。回收站阶段再把物品删除改成可恢复。

不记录数量的数值，不提供出入库。数量说明只是可选文字。

不把完整路径写进表。搬动位置时不改写下级行，也不改写物品关联行。

登录、会话、Origin、登录限流和第一阶段迁移保持现有行为。不新增环境变量，不新增 Go 模块依赖，不新增 npm 依赖。

## 3. 仓库

新增：

```
internal/catalog/
migrations/002_items_locations.sql
```

网页仍在 `web/src`。页面可以拆成多个文件，路由集中注册。HTTP 路由仍由 `internal/httpapi` 注册。

业务事务、父子规则、版本判断和字段校验放在 `internal/catalog`。`internal/httpapi` 负责认证、Origin、正文大小、状态码和 JSON，不复制一套业务判断。

沿用现有迁移器，只增加 `002` 文件。嵌入版本因此是连续的 `001`、`002`。

实现时更新 `README.md`：说明本地可以登记物品和位置，开发入口仍是 `http://127.0.0.1:5173`，并保留 Origin 与首次账号的说明。

## 4. 表

`002_items_locations.sql` 创建下面三张表。时间列使用 UTC 的 RFC3339Nano 文本，与现有用户表的写法一致。

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

`locations.id` 和 `items.id` 使用 `INTEGER PRIMARY KEY AUTOINCREMENT`。插入时不指定 `id`。已提交的删除不会把该 id 再分配给新行；新行的 id 大于这张表曾经提交过的最大 id。回滚、未提交的插入不在本阶段测试里。

测试要分别对物品和位置做一次：删除该表当前最大 id 的记录后新建一条，新 id 与被删 id 不同。再用被删 id 和 `version=1` 去修改、去删除，都返回 404。新记录的名称和 `version` 保持刚创建时的值。

可选文字清空后存 `NULL`，不存空字符串。名称按原文保存，不去掉大小写，不合并同名。编号按原文保存，比较区分大小写，保留前导零。`001` 与 `01` 是两个编号。

字符数按 Unicode 码点计算。SQLite 的 `length(TEXT)` 返回的也是码点数，不是字节数；字节长度要用 `octet_length`。应用层仍检查字符数和控制字符，以便返回下文的中文短句。最大长度不写成 SQL `CHECK`。

| 字段 | 字符数 | 换行 |
| --- | --- | --- |
| 位置名、物品名 | 1–80 | 不允许 |
| 编号 | 1–32，且任何一个字符都不能是空白 | 不允许 |
| 型号 | 最多 120 | 不允许 |
| 别名、规格、数量说明 | 最多 200 | 不允许 |
| 备注、放置说明 | 最多 2000 | 允许 U+000A 和 U+000D |

除备注和放置说明里的换行外，上述文字不能包含 Unicode 类别为 Cc 的控制字符。写入前去掉首尾空白。去掉之后为空，按「未提供值」处理：必填字段报错，可空字段存 `NULL`。

物品可以关联区域、固定储物位或移动容器。关联只表示直接存放，不表示位于该位置的下级里面。

## 5. 位置规则

类型创建后不能改。

| 类型 | 界面文字 | 父级 | 编号 |
| --- | --- | --- | --- |
| `area` | 区域 | 可以没有，也可以是另一个区域 | 必须为空 |
| `fixed` | 固定储物位 | 必须是区域或固定储物位 | 必填，与移动容器共用全屋唯一编号 |
| `movable` | 移动容器 | 可以没有；要有则是固定储物位或移动容器 | 必填，与固定储物位共用全屋唯一编号 |

父级必须已经存在。新父级不能是自己，也不能是自己的下级。下级指沿 `parent_id` 向上能走到该位置的那些位置。

修改父级、创建和删除都使用 `BEGIN IMMEDIATE`。字段规则没有通过时不写入，也不因为版本过旧返回 409。版本通过之后，才在同一事务里校验父级并沿父链走到根。任一校验失败则整次请求不写入。对外可观察的失败顺序以第 6 节开头的清单为准。

删除位置前，事务内确认它没有子位置，且 `item_locations` 没有引用它。有任一种引用时不删除。删除物品只删除该物品及其关联，不删除位置。

成功的修改把 `version` 加 1，并更新 `updated_at`。`created_at` 不变。新建时 `version` 为 1。字段只要出现在合法的 PATCH 里就写入，即使新值与旧值相同，`version` 也加 1。

版本不同则不写入。id 是否存在、以及它和坏 JSON 谁先返回，以第 6 节开头的清单为准。

## 6. HTTP

响应沿用 `application/json; charset=utf-8`。错误至少包含 `code` 和 `message`。`message` 给人读，判断以 `code` 为准。未知 JSON 字段忽略。

已知的物品和位置写路由按下面的顺序停在第一个失败上。方法不对的请求不进入这个顺序，直接返回 404。

1. Origin。不符合或缺少时返回 403 `origin_rejected`，message 为「来源不被接受」，并且不写入。
2. 会话。没有有效会话时返回 401 `unauthenticated`，message 为「未登录」，并且不写入。Origin 和会话都失败时返回 403。
3. 请求大小，以及 JSON 能否解析、类型是否符合下文。`POST` 和 `PATCH` 正文超过 32768 字节返回 413 `body_too_large`，message 为「请求正文过大」。JSON 类型错误返回 400 `invalid_body`，不带 `fields`。`POST` 和 `PATCH` 带了查询参数时，也在这一步返回 400 `invalid_fields`。`DELETE` 不读取正文；它的 `version` 查询参数不合法，或出现 `version` 以外的查询参数时，在这一步返回 400 `invalid_fields`。
4. 记录是否存在。这一步只用于修改和删除。路径 id 要匹配 `^[1-9][0-9]*$`，并且库里要有这一行。否则返回 404 `not_found`，message 为「未找到」。创建没有既有 id，跳过这一步，也跳过第 6 步。
5. 字段规则。返回 400 `invalid_fields`。
6. 版本。不一致时返回 409 `version_conflict`，不写入。
7. 关系与唯一性约束：父级、循环、位置仍被引用、编号重复。返回下文对应的 400 或 409。

因此，不存在的 id 配上坏 JSON 返回 `invalid_body`，不返回 404。不存在的 id 配上类型正确但字段不合法的 JSON 返回 404。字段不合法且版本也过旧时返回 400。版本过旧时不再返回 `invalid_parent`、`location_cycle`、`location_in_use` 或 `code_taken`。

`GET` 先核对会话，没有会话时返回 401，不核对 Origin。会话有效后，查询参数不合法返回 400，然后再判断 `parent`、`location` 或 `exclude` 所指的记录是否存在。

物品和位置的创建、修改、删除都使用 `BEGIN IMMEDIATE`。

登录接口的正文上限仍是 4096 字节。物品和位置的 `POST` 与 `PATCH` 上限是 32768 字节。`DELETE` 的版本只放在查询参数里。

未知路径返回 404 `not_found`，message 为「未找到」。

`POST` 和 `PATCH` 不接受查询参数。`GET` 只接受本节列出的查询参数，每个参数最多出现一次。`DELETE` 只接受一个 `version`。不符合时返回 400 `invalid_fields`。

内部错误仍返回 500 和空正文，不把 SQL 原文给客户端。应用层能够识别的校验失败和约束失败，映射成下面的 code，不返回 500。

### 6.1 位置

| 方法与路径 | 成功 | 说明 |
| --- | --- | --- |
| `GET /api/v1/locations` | 200，分页对象 | 见下方四种列表 |
| `POST /api/v1/locations` | 201，位置对象 | 创建 |
| `GET /api/v1/locations/{id}` | 200，位置对象 | 单个位置 |
| `PATCH /api/v1/locations/{id}` | 200，位置对象 | 修改名称、编号或父级 |
| `DELETE /api/v1/locations/{id}?version=` | 204，空正文 | 删除空位置 |

位置对象：

```json
{
  "id": 1,
  "name": "厨房",
  "type": "area",
  "code": null,
  "parent_id": null,
  "version": 1,
  "created_at": "2026-09-29T00:00:00Z",
  "updated_at": "2026-09-29T00:00:00Z",
  "path": [
    { "id": 1, "name": "厨房", "type": "area", "code": null }
  ]
}
```

`path` 从根到自己，每项只有 `id`、`name`、`type`、`code`。最后一项就是这个位置，其 `id` 等于位置对象的 `id`。物品关联里的 `path` 最后一项的 `id` 等于该关联的 `location_id`。读取时由 `parent_id` 计算。

分页对象：

```json
{ "data": [], "total": 0, "limit": 30, "offset": 0 }
```

`data` 的元素就是上面的位置对象，每条都带 `path`。`total` 是筛选后的总条数，和当页 `data` 在同一个事务里读取。`limit` 省略时为 30，允许 1 到 100。`offset` 省略时为 0，允许大于等于 0 的整数。两者都是不带正号、不带前导零的十进制。`offset` 超过总数时返回空 `data` 和真实 `total`。`parent=0` 不是「没有父级」，而是 400。

`GET /api/v1/locations` 的四种列表互斥：

| 查询 | 结果 | 排序 |
| --- | --- | --- |
| 无 `parent`、`flat`、`eligible_parent_for` | `parent_id` 为空的位置 | 类型、编号、名称、`id` |
| `parent={id}` | 该位置的直接子级。父级不存在则 404 | 同上 |
| `flat=1` | 全部位置，供物品选择存放位置 | `id` 升序 |
| `eligible_parent_for=area\|fixed\|movable` | 允许作为该类型父级的位置 | `id` 升序 |

类型排序为区域、固定储物位、移动容器。编号为空的排在有编号的后面，编号本身按二进制排序。

`eligible_parent_for=area` 返回全部区域。`fixed` 返回全部区域和固定储物位。`movable` 返回全部固定储物位和移动容器。可选 `exclude={id}` 会去掉这个位置和它的全部下级；`exclude` 所指位置不存在时 404。没有 `eligible_parent_for` 时不能带 `exclude`。

`flat` 的值只能是 `1`。

创建正文：

```json
{ "name": "传感器盒", "type": "movable", "code": "014", "parent_id": 2 }
```

`type` 必填。区域不能带非空 `code`；`code` 省略或 `null` 都可以。固定储物位和移动容器必须带编号。固定储物位必须带 `parent_id`。区域和移动容器的 `parent_id` 可以省略或为 `null`。

修改正文必须带整数 `version`，并且至少再出现一个可修改字段：`name`、`code` 或 `parent_id`。未出现的字段保持原值。`parent_id: null` 表示去掉父级，只有区域和移动容器可以。固定储物位和移动容器的 `code: null` 拒绝。区域的 `code` 只能省略或为 `null`。正文里出现 `type` 就拒绝，即使值与当前类型相同。

解析必须区分「字段未出现」和 JSON `null`。不能把两者都当成 Go 的 nil 指针。

### 6.2 物品

| 方法与路径 | 成功 | 说明 |
| --- | --- | --- |
| `GET /api/v1/items` | 200，分页对象 | 见下方筛选 |
| `POST /api/v1/items` | 201，物品对象 | 创建。可以只有名称 |
| `GET /api/v1/items/{id}` | 200，物品对象 | 详情 |
| `PATCH /api/v1/items/{id}` | 200，物品对象 | 修改文字或整体替换位置关联 |
| `DELETE /api/v1/items/{id}?version=` | 204，空正文 | 物理删除物品和它的关联 |

物品对象：

```json
{
  "id": 1,
  "name": "遥控车",
  "alias": null,
  "model": null,
  "spec": null,
  "quantity_note": null,
  "note": null,
  "version": 1,
  "created_at": "2026-09-29T00:00:00Z",
  "updated_at": "2026-09-29T00:00:00Z",
  "locations": [
    {
      "location_id": 4,
      "note": "车和遥控器",
      "path": [
        { "id": 1, "name": "客厅", "type": "area", "code": null },
        { "id": 3, "name": "储物柜", "type": "fixed", "code": "002" },
        { "id": 4, "name": "玩具盒", "type": "movable", "code": "014" }
      ]
    }
  ]
}
```

分页参数、信封和 `total` 的读法与位置列表相同。列表里 `data` 的元素与物品对象相同。`locations` 按 `location_id` 升序。没有位置时为空数组，这就是待定位。关联元素必须有 `location_id`。`note` 省略或为 `null` 时存 `NULL`。`location_id` 小于 1 视为位置不存在。

列表排序为 `name`、`id`。中文按 Unicode 码点，不按拼音。筛选互斥：

| 查询 | 结果 |
| --- | --- |
| 无 `placement` 和 `location` | 全部物品 |
| `placement=unlocated` | 没有任何位置关联的物品 |
| `location={id}` | 直接关联该位置的物品。位置不存在则 404 |

`placement` 不接受 `any`。它和 `location` 同时出现则 400。

创建时除 `name` 外都可以不写。`locations` 省略或 `[]` 都表示没有位置。`locations: null` 拒绝。

修改必须带 `version`，并且至少再出现一个可修改字段：`name`、`alias`、`model`、`spec`、`quantity_note`、`note`、`locations`。可空文字传 `null` 或去掉空白后为空，就存 `NULL`。`locations` 不出现则关联不变；出现数组就在同一事务里用这组关联替换全部旧关联，包括 `[]`。数组元素是 `{ "location_id": 4, "note": "车和遥控器" }`，`note` 可省略。按数组顺序检查，同一个位置只能出现一次，位置必须存在。

替换关联时也把物品的 `version` 加 1，并更新 `updated_at`。

### 6.3 状态码

| 状态 | code | message | 何时 |
| --- | --- | --- | --- |
| 400 | `invalid_body` | 请求格式不正确 | 正文不是 JSON 对象，或 JSON 类型不对，例如版本写成字符串 |
| 400 | `invalid_fields` | 有字段不符合要求 | 字段不合法。另含 `fields` |
| 400 | `invalid_parent` | 不能放在这个父级下 | 父级不存在，或类型不允许这种父子关系 |
| 401 | `unauthenticated` | 未登录 | 没有有效会话 |
| 403 | `origin_rejected` | 来源不被接受 | 写请求来源不对或缺少 Origin |
| 404 | `not_found` | 未找到 | 路径、方法或 id 不存在 |
| 409 | `version_conflict` | 记录已被修改 | 版本与库里的不同。不写入 |
| 409 | `code_taken` | 编号已被使用 | 非空编号与已有编号相同 |
| 409 | `location_cycle` | 不能移到自己的下级 | 父级是自己或自己的下级 |
| 409 | `location_in_use` | 这个位置下面还有内容 | 删除时仍有子位置或物品关联 |
| 413 | `body_too_large` | 请求正文过大 | `POST` 或 `PATCH` 正文超过 32768 字节 |

失败顺序以第 6 节开头的清单为准。

正文的 JSON 类型：

- `name` 和 `type` 若出现，必须是字符串。`null` 或其他类型返回 `invalid_body`。
- `code`、`alias`、`model`、`spec`、`quantity_note`、`note`，以及关联里的 `note`，若出现，必须是字符串或 `null`。其他类型返回 `invalid_body`。
- `version` 若出现，必须是不带小数的数字。字符串、布尔、数组、对象、`null`、`1.5`，以及 `encoding/json` 无法放入 Go 整数的 `1.0`，都返回 `invalid_body`。缺失，或整数小于 1，返回 `invalid_fields`，短句是「版本不正确」。
- `parent_id` 若出现，必须是不带小数的数字或 `null`。`null` 表示去掉父级，不是类型错误。其他非法类型返回 `invalid_body`。整数小于 1 视为父级不存在，返回 `invalid_parent`。
- `locations` 若出现，JSON 类型必须是数组。字符串、数字、布尔、对象，以及数组里不是对象的元素（含 `null`、数字和字符串），整份返回 `invalid_body`，不带 `fields`。
- `locations: null` 返回 `invalid_fields`，`fields.locations` 为「位置格式不正确」。元素缺少 `location_id`，或 `location_id` 为 `null`，也用这一句。`location_id` 不是不带小数的数字时返回 `invalid_body`。整数小于 1 时，`fields.locations` 为「所选位置不存在」。重复关联仍是 `invalid_fields`，短句是「同一位置只能关联一次」。

类型错误整份返回 `invalid_body`，不带 `fields`。

查询参数不是 JSON。不认识的参数、重复参数、`flat` 不是 `1`、`eligible_parent_for` 不是三种类型、`placement` 不是 `unlocated`，都返回 `invalid_fields`，键为参数名，短句是「不支持的参数」。`limit` 不是 1 到 100 的无前导零正整数时，`fields.limit` 为「数量超出范围」。`offset` 不是 `0` 或无前导零正整数时，`fields.offset` 为「起点不正确」。`version` 不是无前导零正整数时，`fields.version` 为「版本不正确」。`parent`、`location` 和 `exclude` 不是无前导零正整数时，对应键的短句为「参数不正确」。

`invalid_fields` 一次带上本次能够确定的全部字段。`fields` 的键和短句：

| 键 | 短句 | 条件 |
| --- | --- | --- |
| `name` | 请填写名称 | 缺失或去掉空白后为空 |
| `name` | 名称过长 | 超过 80 |
| `name` | 名称不能包含控制字符 | 含不允许的控制字符 |
| `code` | 请填写编号 | 固定储物位或移动容器缺少编号 |
| `code` | 编号过长 | 超过 32 |
| `code` | 编号不能包含空白 | 含空白 |
| `code` | 编号不能包含控制字符 | 含控制字符 |
| `code` | 区域不使用编号 | 区域带了非空编号 |
| `code` | 编号不能清空 | 固定储物位或移动容器把编号清成空 |
| `type` | 类型不正确 | 创建时缺失或不是三种类型 |
| `type` | 类型不能修改 | PATCH 出现 `type` |
| `parent_id` | 请选择父级 | 固定储物位没有父级 |
| `alias` | 别名过长 / 别名不能包含控制字符 | 超过 200，或含控制字符 |
| `model` | 型号过长 / 型号不能包含控制字符 | 超过 120，或含控制字符 |
| `spec` | 规格过长 / 规格不能包含控制字符 | 超过 200，或含控制字符 |
| `quantity_note` | 数量说明过长 / 数量说明不能包含控制字符 | 超过 200，或含控制字符 |
| `note` | 备注过长 / 备注不能包含控制字符 | 超过 2000，或含换行以外的控制字符 |
| `version` | 版本不正确 | 缺失，或 JSON 整数小于 1；查询参数里则是缺失、带前导零、带正号或小于 1 |
| `locations` | 位置格式不正确 | 值为 `null`，或元素缺少 `location_id`。不是数组，或元素不是 JSON 对象时返回 `invalid_body`，不用本行 |
| `locations` | 同一位置只能关联一次 | 数组里位置重复 |
| `locations` | 所选位置不存在 | 某个 `location_id` 不存在 |
| `locations` | 放置说明过长 / 放置说明不能包含控制字符 | 某条说明超过 2000，或含换行以外的控制字符 |
| `request` | 没有要修改的内容 | PATCH 除版本外没有可修改字段 |
| 参数名 | 不支持的参数 | 不认识的查询参数，或同一参数重复出现。数值不合法时用上文的「数量超出范围」「起点不正确」「版本不正确」「参数不正确」，不用这句 |

同一键有多个短句时，用上表对应的那一句。`locations` 只保留按数组顺序发现的第一条关联错误；其他字段的错误仍同时返回。`offset` 合法但超过总数时不报错。

## 7. 网页

界面文字使用中文。登录页保持现有宽度和字段。登录后的页面最大宽度 40rem，单列。按钮和输入框最小高度 2.75rem。沿用现有 CSS Modules、字体和颜色，不引入组件库。390 宽下页面不出现横向滚动，路径可以换行。

已登录访问 `/login` 进入 `/`。未登录访问下面任一页进入 `/login`。任一业务接口返回 401 时进入 `/login`。

页头显示用户名、通向 `/` 的「物品」、通向 `/locations` 的「位置」，以及「退出」。

| 路径 | 页面 |
| --- | --- |
| `/` | 物品列表 |
| `/items/new` | 新增物品 |
| `/items/:id` | 查看和修改物品 |
| `/locations` | 顶层位置 |
| `/locations/new` | 新增位置 |
| `/locations/:id` | 查看位置、快速登记、修改位置 |

物品列表的标题是「物品」，有通向 `/items/new` 的「新增物品」。每行显示名称；没有位置时显示「待定位」；有位置时，每个位置的路径用 ` / ` 连接，段内是名称，有编号时再加一个空格和编号。点击进入该物品。查询参数 `placement=unlocated` 时只看待定位，并提供回到全部物品的「全部物品」。未筛选时提供「只看待定位」。`offset` 表示页起点。还有下一页时显示「下一页」，`offset` 大于 0 时显示「上一页」。空列表在未筛选时显示「还没有物品」，筛选时显示「没有待定位的物品」。

新增物品的标题是「新增物品」。字段为名称、别名、型号、规格、数量说明、备注。可以添加若干存放位置和对应的放置说明。地址带合法的 `location` 时，先读取该位置并放进初始关联；读取失败则显示接口的 message，仍允许只保存名称。保存成功后进入该物品页。

物品页的标题是物品名称。可以修改上述文字字段和全部位置关联，再按「保存」。第一次按「删除」不发请求。确认区域显示「永久删除，无法恢复」，然后才是「确认删除」和「取消」。按「取消」撤回确认。删除成功后回到 `/`。

新增物品页和物品页的存放位置选择器都请求 `GET /api/v1/locations?flat=1`，`limit=100`，从 `offset=0` 起按 `total` 连续取完。不能只使用第一页。

位置列表使用查询参数 `offset`，每页条数用接口默认的 30。本页记录按类型分成「区域」和「尚未放入的容器」，只渲染本页里实际出现的组，组内保持接口顺序。`total` 大于 `offset` 加本页条数时显示「下一页」，`offset` 大于 0 时显示「上一页」。整份列表为空时显示「还没有位置」。有通向 `/locations/new` 的「新增位置」。

新增位置的标题是「新增位置」。先选类型。固定储物位和移动容器显示编号。区域和移动容器可以选择「不设置父级」。父级选择器使用 `eligible_parent_for`，`limit=100`，从 `offset=0` 起按 `total` 连续取完，不能停在第一页。地址带 `parent` 时，父级固定为该位置，类型只提供它允许的子类型：区域下面是区域或固定储物位，固定储物位下面是固定储物位或移动容器，移动容器下面只有移动容器。另有「重新选择父级」，链到不带 `parent` 的新增页。保存成功后进入新位置页。

位置页的标题是名称，有编号时加空格和编号。面包屑使用 `path`，上级是链接，当前不是链接，之间用 ` / `。下面是直接子位置和直接物品。子位置的页起点用查询参数 `children`，物品用 `items`，各自有上一页和下一页。没有子位置时显示「这里还没有下一级位置」。没有直接物品时显示「这个位置里还没有物品」。

位置页上的快速登记只有名称和「添加物品」。成功后留在本页，清空名称，保留当前路径和查询参数，并刷新物品列表。失败时保留已输入的名称，并显示错误。

新增位置时，如果地址里的 `parent` 读不到，显示接口的 message，不提交创建。

位置页可以修改名称、编号和父级。父级选择器带上 `exclude` 为当前 id，同样按页取完。区域不显示编号。区域和移动容器可以改成「不设置父级」。子位置总数和直接物品总数都为 0 时才显示删除。确认区域同样显示「永久删除，无法恢复」。删除成功后回到父级；没有父级则回到 `/locations`。

保存或删除得到 `version_conflict` 时，输入框、未保存的位置选择和删除确认都保持原样，并显示「记录已被修改」和「加载最新内容」。点击该按钮后才读取并填回最新记录；点击前不改草稿。不把新版本写进原草稿，也不自动再次提交。加载时若记录已经不存在，显示「未找到」并回到对应列表。删除冲突在加载最新内容后收回确认，用户要重新从「删除」开始。其他错误显示接口返回的 `message`；有 `fields` 时，在对应控件旁显示短句。读取中显示「正在加载」。读取失败显示 message 和「重试」。提交进行中禁用对应按钮。

页面上不出现搜索框、分类、照片、归位或回收站。

## 8. 测试

Go 测试使用临时数据库和 `httptest`，通过 HTTP 验证对外行为，不模拟 SQLite。至少覆盖：

1. 已有 `001` 的库启动后执行 `002`。再次启动不重复执行。三张表和唯一编号索引存在。
2. 只提交名称可以创建物品。另一条同名物品也可以创建。两条的 `locations` 都为空数组。
3. 编号 `001` 在响应里仍是 `001`。另一位置再使用 `001` 返回 409 `code_taken`。`01` 可以同时存在。区域提交编号 `001` 返回 400，且 `fields.code` 为「区域不使用编号」。
4. 创建固定储物位时不带父级，返回 400 `invalid_fields`，`fields.parent_id` 为「请选择父级」。移动容器的父级是区域、区域的父级是移动容器，都返回 400 `invalid_parent`，且不创建位置。区域挂到区域、固定储物位挂到区域、移动容器挂到固定储物位、移动容器没有父级，都成功。
5. 使用当前版本，把位置的父级改成它的下级，返回 409 `location_cycle`。原来的 `parent_id` 和 `version` 不变。
6. 一个移动容器里创建两件物品。容器改挂到另一个固定储物位后，两件物品的 `location_id` 仍是该容器。每条路径都包含新的直接父级 id，不包含旧的直接父级 id；两者共同的更高层仍留在路径里。
7. 一件物品关联两个位置，放置说明分别为「车和遥控器」和「备用轮胎」。读取时两条说明都在。
8. 只挂在子位置上的物品，不出现在父级的 `location={父级 id}` 结果里。父级能看到这个子位置。
9. `placement=unlocated` 只返回没有关联的物品。加上位置后，该物品离开这个列表。
10. 删除物品后，该物品和它的关联都不再存在，位置仍在。删除空位置返回 204。删除仍有子位置或物品的位置返回 409 `location_in_use`，位置仍在。
11. 名称合法但 `version` 过旧时，修改返回 409 `version_conflict`，库里保持较新的名称。用过旧 `version` 删除也不发生。
12. PATCH 省略 `alias` 时原别名保留。`alias: null` 后读取为 `null`。`locations: []` 后物品变为待定位。省略 `locations` 时原关联保留。
13. 无会话创建物品返回 401，且没有新行。错误 Origin 的创建请求返回 403，且没有新行。
14. 已登录且 Origin 正确时，物品 `POST` 正文超过 32768 字节返回 413。登录接口仍按 4096 字节拒绝。
15. `placement` 与 `location` 同时出现、`flat=1` 与 `parent` 同时出现、重复查询参数，都返回 400。
16. 创建后的 `version` 为 1。合法 PATCH 即使提交了相同名称，`version` 也变为 2。
17. 删除物品表当前最大 id 后新建，新 id 与被删 id 不同。用被删 id 和 `version=1` 去 PATCH、去 DELETE，都返回 404，新物品不变。对一个空的位置重复这组步骤，新位置也不被旧请求改动。
18. 不存在的物品 id 配上无法解析的 JSON，返回 400 `invalid_body`。已存在的物品若 `locations` 是字符串，或数组元素不是对象，也返回 `invalid_body` 且不带 `fields`。已存在的物品提交 `locations: null` 返回 400 `invalid_fields`，`fields.locations` 为「位置格式不正确」。同一个不存在的 id 配上类型正确但名称为空的 JSON，返回 404。已存在的物品在名称空着且 `version` 过旧时返回 400，`fields.name` 为「请填写名称」，库里的名称不变。已存在的位置在 `version` 过旧、且新父级会形成循环时，返回 409 `version_conflict`，`parent_id` 不变。Origin 正确但无会话，且正文超过 32768 字节时返回 401。Origin 不正确且正文超过 32768 字节时返回 403。这两种情况都不写新行。`DELETE` 的 `version` 不是正整数、且 id 不存在时返回 400，不返回 404。

网页不把浏览器自动化套件放进仓库。实现时在浏览器里实际完成第 9 节的步骤。

## 9. 验收

同时满足以下条件才算本阶段完成：

- 第 8 节的 Go 测试通过。
- 桌面宽度下实际完成：登录；创建区域「厨房」；在其下创建固定储物位「洗衣机柜最上层」，编号 `001`；再创建固定储物位「左下格」，编号 `002`；在 `001` 下创建移动容器「传感器盒」，编号 `014`；在盒子页连续添加「温湿度传感器」和「门磁传感器」；把盒子的父级改到 `002`；盒子路径显示为「厨房 / 左下格 002 / 传感器盒 014」；两件物品仍列在盒子中；打开 `001` 看不到这两件物品；删除「门磁传感器」时能看到「永久删除，无法恢复」，确认后列表中不再有它。
- 物品页改了名称但还没保存时，另一个已登录会话把该物品改成别的名称。回到前一个页面保存后，输入框仍是未保存的名称，并出现「加载最新内容」。点击后表单变为后一个会话保存的名称。
- 390 宽下打开这个盒子页：路径换行可见，页面没有横向滚动；再快速添加「水晶头钳」，列表中出现它。
- 仓库中没有分类、搜索、待归位、回收站、照片、MCP、OAuth 和 Docker 实现。
- `go.sum` 与 `web/package-lock.json` 仍记录实际使用的版本。本阶段不新增依赖时，这两个文件的依赖项不增加。
