# 有处：外观、位置树与图标

日期：2026-10-02

状态：已实现。默认暗色、账号页可切浅色；三种强调色；位置树缩进；每个位置可选图标。

依据：已实现的家用界面、物品和位置、分类、照片、MCP、部署规格。冲突时，会话、Origin、迁移器、SQLite 连接以第一阶段为准；位置业务规则以物品和位置规格为准；本文件覆盖外观存储、强调色、列表层级样式，以及位置 `icon`。

## 1. 目标

打开网页默认是暗色。账号页可以切浅色，并在三种强调色里选一种。位置和分类的列表、选择器能看出父子缩进。每个位置可以选一个家用图标；没选时按类型用默认图标。物品列表的位置路径带这个图标。

本阶段结束时：

- `html` 带 `data-theme`（`dark` 或 `light`）和 `data-accent`（`moss`、`clay`、`ink`）。
- 未选过时主题是 `dark`、强调色是 `moss`（现有绿）。
- 选择记在本机 `localStorage`，键 `youchu-theme`、`youchu-accent`。不进用户接口，不加环境变量。
- 账号页有「外观」：深色/浅色，苔绿/陶土/墨蓝。
- 位置 JSON 有 `icon`（`null` 或允许清单里的短名）。路径节点同样带 `icon`。
- 位置列表、位置页子位置、分类列表、分类页子分类、父级选择器：按 `path` 深度缩进。
- 位置行：图标、名称、编号徽章、直接件数。

## 2. 不做的事

不跟系统外观。不做自定义色值、字体大小滑条、按房间换肤。不给物品单独选图标。不给分类选图标。不做位置主题色。不上传图标文件。不新增 npm 依赖和图标库。不把 Playwright 放进仓库。不启用 CGO。不加环境变量。不改 Origin、Cookie、MCP 工具集合。不实现扫码、打印标签、OAuth、HEIC。

外观不进服务端、不同步到别的浏览器。

## 3. 外观

`web/src/theme.css` 继续用 CSS 变量。`:root` 为浅色苔绿（现有变量保留为 `light` + `moss` 的值）。`html[data-theme="dark"]` 覆盖纸色、墨色、线色：

| 变量 | 浅色（现有） | 暗色 |
| --- | --- | --- |
| `--paper` | `#f3f6f4` | `#161a18` |
| `--paper-raised` | `#ffffff` | `#212824` |
| `--ink` | `#20362e` | `#e8eee9` |
| `--ink-muted` | `#64766c` | `#9aaba1` |
| `--line` | `#dce5df` | `#3a4740` |
| `--danger` | `#9f1239` | `#f0a0b0` |

强调色只改 `--clay` 和 `--clay-press`，以及 `:focus-visible` 描边：

| `data-accent` | 中文 | 浅色 `--clay` / `--clay-press` | 暗色 `--clay` / `--clay-press` |
| --- | --- | --- | --- |
| `moss` | 苔绿 | `#27654c` / `#194c37` | `#3d9b74` / `#2d7a5a` |
| `clay` | 陶土 | `#8f4d36` / `#743d2b` | `#c47a4a` / `#a35f38` |
| `ink` | 墨蓝 | `#3d5a73` / `#2c4256` | `#7ea0bd` / `#5c7f9c` |

`html, body` 背景仍是 `--paper`，字 `--ink`。页头、底栏、输入、主按钮继续引用这些变量，使暗色和强调色全局生效。

`web/index.html` 的 `<head>` 里放一段同步小脚本（在加载 React 之前执行）：读 `localStorage`，非法或缺省时主题 `dark`、强调色 `moss`，写到 `document.documentElement` 的 `data-theme`、`data-accent`。避免先闪浅色。生产嵌入的 `index.html` 随 Vite 构建带上这段脚本。

账号页「外观」在改用户名之上：

- 标题「外观」
- 「亮度」：深色、浅色（单选）
- 「强调色」：苔绿、陶土、墨蓝（单选）

点选立即改 `html` 属性和 `localStorage`，不用保存按钮，不请求 `/api/v1/me`。当前值与 `html` 上的属性一致。

390 宽账号页仍 `document.documentElement.scrollWidth === 390`。

## 4. 位置图标

迁移 `007_location_icon.sql`：

```sql
ALTER TABLE locations ADD COLUMN icon TEXT CHECK (icon IS NULL OR icon <> '');
```

嵌入迁移变为连续的 `001`–`007`。已有行 `icon` 为 `NULL`。

允许的 `icon` 值只有下面这些短名（小写字母）。写入其它值：`400 invalid_fields`，`fields.icon` 为「图标不正确」。空字符串与未知名相同，按不正确处理。JSON `null` 表示清除，存 `NULL`，界面回落到类型默认。省略该键：创建时当 `NULL`；PATCH 当不改。

| 短名 | 界面说明 | 适用 |
| --- | --- | --- |
| `home` | 房子 | 默认：区域 |
| `kitchen` | 厨房 | 区域 |
| `living` | 客厅 | 区域 |
| `bedroom` | 卧室 | 区域 |
| `bathroom` | 卫生间 | 区域 |
| `balcony` | 阳台 | 区域 |
| `garage` | 储藏/车位 | 区域 |
| `cabinet` | 柜子 | 默认：固定储物位 |
| `drawer` | 抽屉 | 柜格 |
| `shelf` | 层板 | 柜格 |
| `fridge` | 冰箱 | 柜格 |
| `washer` | 洗衣机 | 柜格 |
| `wardrobe` | 衣柜 | 柜格 |
| `bookcase` | 书架 | 柜格 |
| `box` | 盒子 | 默认：移动容器 |
| `crate` | 箱子 | 盒子 |
| `basket` | 篮子 | 盒子 |
| `toolbox` | 工具箱 | 盒子 |
| `bin` | 桶 | 盒子 |
| `safe` | 保险箱 | 盒子或柜格 |

类型不限制能选哪几个；上表「适用」只指导默认和选择器分组文案。任意类型都可以选表中任一短名。

默认（`icon` 为 `null` 时仅界面使用，不写回数据库）：

- `area` → `home`
- `fixed` → `cabinet`
- `movable` → `box`

图标是内联 SVG，按短名画在 `web/src` 一个模块里，颜色用 `currentColor`。不引入图标 npm 包。

### JSON

位置对象增加 `"icon": null` 或 `"icon": "kitchen"`。键必须出现。路径节点同样增加 `icon`（`null` 或短名）。MCP 位置 JSON 与 HTTP 相同。

创建、更新位置的正文可含 `icon`。校验顺序仍是 Origin → 会话 → 正文/字段 → 路径 id。`icon` 不正确属于字段错误，发生在查路径 id 之前（与其它字段一起）。

## 5. 树形样式

不新增树形接口。深度用已有 `path.length`（根为 1，子级更大）。

位置顶层列表、位置页「下一级位置」、分类顶层列表、分类页「下一级分类」的每一行：

- 左侧内边距按 `path.length - 1` 增加（每级 1rem，最大按 6 级封顶，以免极深把 390 宽撑出横向滚动）。
- 子级（深度 ≥ 2）左侧一条竖向浅线（`var(--line)`）。
- 位置行内容顺序：图标（1.5rem）、名称、有编号则徽章（等宽、小号、`--line` 边框）、`direct_item_count > 0` 时「n 件」（`--ink-muted`）。件数为 0 时不写「0 件」。
- 分类行：无图标；名称；有直接物品时「n 件」。

父级 `<select>`、物品上的位置选择、分类选择：选项文案前面加与深度对应的全角空格或 `· `，使下拉里能看出层级。原生 `select` 里不画 SVG。

位置页、分类页的面包屑：上级已是链接，当前级不是链接。保持现有行为。物品列表位置路径：每个直接位置显示该位置的图标（`null` 则类型默认）再写路径文字。

390 宽下上述列表 `scrollWidth === 390`。路径和名称继续 `overflow-wrap: anywhere`。

## 6. 测试

Go：

- 迁移 007 后 `locations` 有 `icon` 列。
- 新位置 GET `icon` 为 `null`。
- POST `{..., "icon":"kitchen"}` 成功，GET 为 `"kitchen"`。
- POST/PATCH `"icon":"nope"` 或 `""` → `400 invalid_fields`，`fields.icon`「图标不正确」。
- PATCH `"icon": null` 清成 `null`。
- GET 物品的 `locations[].path[].icon` 与该位置一致。
- 无会话仍 401；错误顺序与现有位置写入一致。

前端构建：`cd web && npm run build` 通过。lockfile 包数量不增加。

浏览器（`/tmp`，不进仓库）：账号页切浅色后 `html[data-theme=light]`，刷新仍浅色；强调色陶土后主按钮颜色变化；再切深色。位置新建选图标厨房，列表行能看到对应 SVG。390 宽位置列表无横向滚动。

## 7. README 与工作记录

README 不强调外观。工作记录写明本段完成、迁移 `001`–`007`、外观只在本机、默认暗色。不要把 OAuth 或扫码写成已完成。

## 8. 不变量

- 没有新 npm、没有 Playwright 进库、没有新环境变量。
- 外观不进服务端。
- 允许的 icon 短名以本文表格为准，增删短名必须改规格。
- 编号规则（`K101`、`B101`）仍是操作者填写的字符串，程序不解析。
