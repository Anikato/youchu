# 有处：位置自传 SVG 图标库

日期：2026-10-03

状态：已实现。

依据：已实现的外观/位置树/图标、物品和位置、照片、MCP、部署规格。冲突时，会话、Origin、迁移器、SQLite 连接以第一阶段为准；位置业务规则以物品和位置规格为准；内置短名以 `2026-10-02-theme-tree-icons-design.md` 为准；本文件覆盖自传 SVG 库与 `custom_icon_id`。

## 1. 目标

操作者可以上传自己的 SVG，作为位置列表上的小符号。上传入口有两处：账号页「图标」，以及新建/编辑位置的图标选择器。两处写入同一份库。新建或编辑位置时，内置 20 个短名和库里的自传图标都能选。

本阶段结束时：

- 迁移 `008` 建 `location_icons`，并给 `locations` 增加可空 `custom_icon_id`。
- `GET/POST /api/v1/location-icons`，`GET/PATCH/DELETE /api/v1/location-icons/{id}`。
- 位置 JSON 始终含 `"icon"` 与 `"custom_icon_id"`。路径节点同样。两者不能同时非空。
- 账号页「图标」可上传、改名、删除。位置表单可上传并立即选中。
- 只收 SVG。消毒后存文本，不落新文件目录。

## 2. 不做的事

不收 PNG、ICO、照片。不给物品或分类自传图标。不做位置实拍。不改外观存储。不加环境变量。不新增 npm。不把 Playwright 放进仓库。不启用 CGO。不实现扫码、OAuth、HEIC。

## 3. 表

`008_location_icon_library.sql`：

```sql
CREATE TABLE location_icons (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    svg TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

ALTER TABLE locations ADD COLUMN custom_icon_id INTEGER REFERENCES location_icons(id);

CREATE INDEX locations_custom_icon_id ON locations(custom_icon_id);
```

嵌入迁移变为连续的 `001`–`008`。已有位置 `custom_icon_id` 为 `NULL`。应用层保证 `icon` 与 `custom_icon_id` 不同时非空。

上限 40 个。超过时 POST 返回 `400 invalid_fields`，`fields.svg` 为「图标已满」。

名称走 `NormalizeName`（必填、最长 80、去两端空白、不要控制字符）。SVG 原文最长 16384 字节；超过为「图标过大」。

## 4. SVG

只收 SVG。根元素必须是 `svg`，且带 `viewBox` 或同时带 `width` 与 `height`。

允许的元素：`svg`、`g`、`path`、`circle`、`ellipse`、`rect`、`line`、`polyline`、`polygon`。其它元素（含 `script`、`foreignObject`、`image`、`use`、`style`、`text`）整份拒绝，`fields.svg`「图标不正确」。

允许的属性：`viewBox`、`width`、`height`、`d`、`cx`、`cy`、`r`、`rx`、`ry`、`x`、`y`、`x1`、`y1`、`x2`、`y2`、`points`、`transform`、`fill`、`stroke`、`stroke-width`、`stroke-linecap`、`stroke-linejoin`、`fill-rule`、`stroke-miterlimit`、`opacity`、`fill-opacity`、`stroke-opacity`。根上可保留 `xmlns="http://www.w3.org/2000/svg"`。其它属性丢掉。`href`、事件属性、外链命名空间导致整份拒绝。

`fill` / `stroke` 若不是空、`none` 或 `currentColor`，改成 `currentColor`，使列表跟强调色走。

库里只存消毒后的文本。读出后界面按这份文本画，不再二次解释为短名。

## 5. JSON 与接口

图标对象：

```json
{
  "id": 1,
  "name": "药箱",
  "svg": "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 24 24\">...</svg>",
  "version": 1,
  "created_at": "...",
  "updated_at": "..."
}
```

`GET /api/v1/location-icons`：`{"data":[...]}`，按 `id` 升序。禁止查询串，否则 `400 invalid_fields`。无分页。

`POST /api/v1/location-icons`：正文 `name`、`svg`。201 返回对象。

`GET /api/v1/location-icons/{id}`：200。禁止查询串。

`PATCH /api/v1/location-icons/{id}`：`version` 必填；`name`、`svg` 至少一项。id 不变，已选用的位置一起变。

`DELETE /api/v1/location-icons/{id}?version=`：无位置引用则 204。有位置的 `custom_icon_id` 指向它则 `409 icon_in_use`，「有位置正在使用」。

写接口错误顺序：Origin → 会话 → 正文/字段 → 路径 id。无会话 GET 仍 401。

位置对象与路径节点增加 `"custom_icon_id": null` 或正整数，键必须出现。与 `"icon"` 不能同时非空。创建/PATCH 位置可含 `custom_icon_id`。指向不存在的库条目：`400 invalid_fields`，`fields.custom_icon_id`「图标不存在」，发生在查位置路径 id 之前。同时给非空 `icon` 与非空 `custom_icon_id`：两个字段都是「不能同时选用内置和自传图标」。PATCH 只改 `icon` 为短名时清掉 `custom_icon_id`；只改 `custom_icon_id` 为正整数时清掉 `icon`；`icon: null` 且未带 `custom_icon_id` 时两者都空（类型默认）。

MCP 位置 JSON 与 HTTP 相同。新增 `youchu_list_location_icons`（read）、`youchu_create_location_icon`（write）、`youchu_update_location_icon`（write）、`youchu_delete_location_icon`（write）。`youchu_create_location` / `youchu_update_location` 接受 `icon` 与 `custom_icon_id`。

## 6. 网页

账号页「外观」之下、「用户名」之上：标题「图标」。列表每条：预览、名称、删除。可改名。可上传 `.svg`（`accept` 为 `image/svg+xml,.svg`）。读成文本后 POST。390 宽无横滑。

位置表单图标选择器增加「我的」分组。可在此上传；成功后进库并选中。选默认则 `icon` 与 `custom_icon_id` 皆空。选内置则只写短名。选自传则只写 `custom_icon_id`。

列表行、物品路径：有 `custom_icon_id` 时画库里的 SVG（1.5rem / 路径 1.1rem），否则仍用内置短名或类型默认。原生 `select` 仍不画 SVG。

## 7. 测试

Go：

- 迁移 008 后有 `location_icons` 表和 `locations.custom_icon_id`。
- POST 合法 SVG 201，GET 列表含消毒后文本；`<script>`、无 `svg` 根、空正文 → `svg`「图标不正确」；超 16384 字节 →「图标过大」。
- 第 41 个 →「图标已满」。
- 位置 POST `custom_icon_id` 成功，GET 该键为 id、`icon` 为 null；路径节点一致。物品路径节点同样。
- 同时写短名和自传 id → 两个字段错误。
- 不存在的 `custom_icon_id` →「图标不存在」。
- 在用则 DELETE 409 `icon_in_use`；不用则可删。
- 无会话 401；错误 Origin 的 POST 403。

前端：`cd web && npm run build` 通过。lockfile 包数量不增加。

浏览器（`/tmp`，不进仓库）：账号页上传一个简单 SVG，列表能看到；位置新建选自传图标，位置列表行能看到该 SVG；该图标在用时账号页删除失败。390 宽账号页 `scrollWidth===390`。

## 8. README 与工作记录

README 可写一句：位置可以选内置图标或自己上传的 SVG。工作记录写明本段完成、迁移 `001`–`008`。不要把 OAuth 或扫码写成已完成。

## 9. 不变量

- 没有新 npm、没有 Playwright 进库、没有新环境变量、没有新数据子目录。
- 内置短名表不在本段增删。
- 编号规则仍是操作者填写的字符串。
