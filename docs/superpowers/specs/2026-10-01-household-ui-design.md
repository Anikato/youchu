# 有处：家用界面与账号

日期：2026-10-01

状态：已按用户确认的方案写成。用户要求直接实现、中间不再停下来确认。本文是本阶段的实现依据。

依据：同仓库《家庭物品整理与查找需求.md》《技术方案草稿.md》，以及已实现的登录、物品和位置、分类和搜索、归位与回收站、照片规格。冲突时，会话、Origin、迁移器、SQLite 连接以第一阶段为准；物品、位置、分类、归位、回收站、照片的业务规则和接口契约以各已通过规格为准；本文件只覆盖外观、操作排布，以及改用户名和改密码。

## 1. 目标

操作者打开网页时，看到的是一份家里用的目录，而不是内部工具。登录、查找、登记、改账号都顺着日常使用排。已登录的人可以改用户名和密码。

本阶段结束时：

- 登录页有一句说明家里这是干什么的，表单在纸色底上的一块抬起面板里。
- 登录后页头：左上「有处」回物品列表；主入口是物品、位置、分类、待归位；用户名进入账号。回收站和退出只在账号页。
- 物品列表最上是查找，旁边是新增。列表行是封面加名称加位置。
- 物品页按「叫什么 → 放在哪 → 哪一类 → 更多说明 → 保存」，照片和待归位仍在保存之后，移到回收站在页尾。
- `/account` 可以改用户名、改密码、进回收站、退出。

## 2. 不做的事

不实现 MCP、OAuth、个人访问令牌、Docker、备份、扫码、HEIC 解码、深色模式、公开注册、找回密码、第二个账号、用户表版本号、变更事件表、`item_categories.source`、归位事项绑定位置。

不改物品、位置、分类、归位、回收站、照片的 JSON 契约、失败顺序、分页、占用检查、草稿保留规则。不新增环境变量。不新增 npm 依赖。不引入组件库。不把 Playwright 放进仓库。不启用 CGO。不新增迁移文件。

`YOUCHU_USERNAME` / `YOUCHU_PASSWORD` 仍只在用户表为空时建号；已有用户时环境变量仍不改名、不改密。

## 3. 视觉

家用纸质目录。暖纸色底、墨色字、粘土色主操作。标题用系统宋体，界面用系统黑体。不从网络加载字体，不使用 Inter、Roboto、Arial 作为指定字体。

新增 `web/src/theme.css`，由 `web/src/main.tsx` 引入。变量：

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
```

`html, body` 背景 `--paper`，文字 `--ink`，字体 `--font-ui`。`#root` 最小高度 100%。

`web/src/styles.module.css` 改用这些变量：

- 登录页最大宽度 `--login-max`，在视口里垂直方向用 `min-height: 100%` 加弹性布局居中；上下至少留 1.5rem，内容过高时页面可以滚动。面板背景 `--paper-raised`，圆角 `--radius`，细边框 `--line`，内边距 1.5rem。标题「有处」用 `--font-display`。说明句「家里的东西在哪儿」用 `--ink-muted`。
- 登录后页面最大宽度 `--page-max`，左右内边距 1rem，上下 1.25rem。
- 输入框、选择框、文本域：背景 `--paper-raised`，边框 `--line`，圆角 `--radius-sm`，最小高度 `--control-min`。焦点描边 `--clay`，宽度 2px。
- `.button` 为次要按钮：透明底、`--line` 边框、`--ink` 字。
- `.buttonPrimary` 为主按钮：底 `--clay`，字 `--paper-raised`，无描边。按住时底 `--clay-press`。登录、查找、新增、保存、账号里改用户名和改密码用这一类。
- 危险操作用现有确认流程，按钮仍是次要按钮加 `--danger` 字色，不做成实心红块。
- 列表行 `.itemLink`：封面 `--thumb` 见方，`object-fit: cover`，圆角 `--radius-sm`。名称在右，位置路径或「待定位」用 `--ink-muted` 写在名称下面。
- 页头 `position: sticky; top: 0`，背景 `--paper`，底边 `--line`，`z-index: 2`。
- 390 宽下任何已有页面都没有横向滚动。路径继续 `white-space: normal; overflow-wrap: anywhere`。

动效只允许 150ms 以内的颜色和边框过渡。不做入场编排、骨架屏动画、新的动效库。

控件最小高度仍是 2.75rem。界面文字使用中文。

## 4. 页头与路由

已登录外壳：

| 元素 | 行为 |
| --- | --- |
| 「有处」 | 链到 `/`，`aria-label="有处"` |
| 物品 | `/`；路径为 `/`、`/items/new`、`/items/:id` 时 `aria-current="page"` |
| 位置 | `/locations`；`/locations` 及其子路径时 `aria-current="page"` |
| 分类 | `/categories`；`/categories` 及其子路径时 `aria-current="page"` |
| 待归位 | `/returns`；该路径时 `aria-current="page"` |
| 当前用户名 | 链到 `/account`；在 `/account` 时 `aria-current="page"` |

页头没有回收站，没有退出。`nav` 的可访问名称是「主要」。

未登录访问受保护页进入 `/login`。已登录访问 `/login` 进入 `/`。任一业务接口 401 仍进入 `/login`。

新增路由 `/account`，放在 `RequireAuth` 内。未知路径仍回 `/`。

## 5. 登录页

路径仍是 `/login`。字段仍是用户名、密码、登录。密码 `type="password"`。`autocomplete` 仍是 `username` 与 `current-password`。失败显示接口 `message`。进行中禁用登录按钮，文案仍是「登录」。

标题「有处」。标题下固定一句「家里的东西在哪儿」。没有注册入口，没有「忘记密码」。

## 6. 账号页

路径 `/account`。标题「账号」。四块，从上到下：

### 6.1 用户名

当前用户名输入框，`autocomplete="username"`。按钮「保存用户名」。成功后页头用户名立刻换成新值，并显示「用户名已保存」。字段错误显示在输入框下。

### 6.2 密码

三个输入框：当前密码（`autocomplete="current-password"`）、新密码、再输入新密码（后两个 `autocomplete="new-password"`），都是 `type="password"`。按钮「修改密码」。

两次新密码不一致：不发请求，提示「两次输入的新密码不一致」。成功后三个框清空，显示「密码已修改」，当前会话继续有效。字段错误显示在对应框下。

### 6.3 回收站

文字链「回收站」，目标 `/trash`。

### 6.4 退出

按钮「退出」。行为与现在页头退出相同：`DELETE /api/v1/session`，清掉 `me` 缓存，进入 `/login`。

没有「注销账号」。

## 7. HTTP：改用户名和改密码

`GET /api/v1/me` 保持现有行为：不检查 Origin，不因查询字符串失败，成功仍是 `{"username":"..."}`，无会话 401 `unauthenticated`。

`/api/v1/me` 增加 `PATCH`。其它方法仍 404 `not_found`。

新增 `POST /api/v1/me/password`。其它方法 404。

写请求正文上限 4096 字节。JSON UTF-8。`invalid_fields` 的外壳与物品写接口相同：`code` 为 `invalid_fields`，`message` 为「有字段不符合要求」，`fields` 为键到中文短句。

失败顺序：Origin → 会话 → 正文大小与查询字符串 → JSON 形状 → 字段。路径上没有资源 id。不要先解析用户名规则再检查 Origin。

| 顺序 | 条件 | 状态 | code |
| --- | --- | --- | --- |
| 1 | Origin 缺失或与 `YOUCHU_PUBLIC_ORIGIN` 不等 | 403 | `origin_rejected` |
| 2 | 无有效会话 | 401 | `unauthenticated` |
| 3 | 正文超过 4096 字节 | 413 | `body_too_large` |
| 4 | URL 带查询字符串 | 400 | `invalid_fields`，各查询键「不支持的参数」 |
| 5 | JSON 不是对象，或必填键缺失、为 `null`、不是字符串 | 400 | `invalid_body`「请求格式不正确」 |

`GET /api/v1/me` 仍不走这张表。

### 7.1 `PATCH /api/v1/me`

正文：

```json
{ "username": "ada" }
```

只认 `username`。其它键忽略。

| `fields` 键 | 短句 | 何时 |
| --- | --- | --- |
| `username` | 用户名不能为空 | 去掉首尾空白后长度为 0 |
| `username` | 用户名过长 | 去掉首尾空白后超过 64 个 Unicode 码点 |
| `username` | 用户名不能包含空白 | 去掉首尾空白后仍含空白字符 |

成功 200，正文 `{"username":"..."}`，值为规范化后的用户名。写入 `users.username` 和 `users.updated_at`（UTC RFC3339Nano）。现有会话全部保留。用新用户名可以登录，用旧用户名得到与现在相同的 `invalid_credentials`。

新值与当前值相同：仍 200，正文为当前用户名；可以不写库。

全库仍只有 `id = 1` 这一行用户。不要为此增加「名称已被使用」分支。

### 7.2 `POST /api/v1/me/password`

正文：

```json
{ "current_password": "...", "new_password": "..." }
```

密码不去掉空白。只认这两个键。

| `fields` 键 | 短句 | 何时 |
| --- | --- | --- |
| `current_password` | 请输入当前密码 | 值为空字符串 |
| `new_password` | 密码至少 8 个字符 | 长度 0 到 7 |
| `new_password` | 密码过长 | 超过 128 个 Unicode 码点 |
| `new_password` | 新密码不能与当前密码相同 | 新密码长度合法，且与 `current_password` 逐字相同 |
| `current_password` | 当前密码不正确 | 当前密码非空，但与库中哈希不匹配 |

同一请求里可同时返回 `current_password` 与 `new_password` 的格式错误。当前密码不正确只在格式都通过之后检查，此时不要再带其它 `fields` 键。

成功 204，空正文。argon2id 参数与建号时相同。更新 `password_hash` 和 `updated_at`。在同一事务里删除**当前 Cookie 以外**的会话。当前 Cookie 继续有效。其它设备上的会话再访问 `GET /api/v1/me` 得到 401。

不把明文密码写入日志。不计入登录限流。不改 Cookie。

实现放在 `internal/auth`：改名、改密、按令牌删除其它会话。HTTP 仍在 `internal/httpapi`。`HandleFunc("/api/v1/me")` 与 `HandleFunc("/api/v1/me/password")` 加 `switch r.Method`。不要写成 Go 1.22 的 `"PATCH /path"` 方法模式。

## 8. 物品列表

标题仍是「物品」。最上是查找表单：

- `type="search"`
- `aria-label` 与 `placeholder` 都是「找家里的东西」，不再另放可见 `<label>`
- 回车或「查找」才提交；组字过程中不发请求
- 提交规则仍按分类规格：去掉首尾空白写入 `q`，空白则去掉 `q`

同一行右侧是主按钮「新增」，链到 `/items/new`。文案用「新增」，不用「新增物品」，链接目标不变。

「只看待定位」/「全部物品」仍在，作为次要按钮，放在查找行下面。筛选仍在 `details` 里，摘要文案「筛选」。筛选条件和地址栏规则不变。

列表行：有封面则显示 `4.5rem` 缩略图；名称；名称下一行是位置路径或「待定位」。点整行仍进入该物品。空状态句子不变。分页不变。

## 9. 物品表单

新增页标题仍是「新增物品」。物品页标题仍是当前名称草稿。

字段顺序：

1. 名称
2. 存放位置（现有选择器）
3. 分类（现有选择器）
4. 「更多说明」：别名、型号、规格、数量说明、备注
5. 主按钮「保存」

「更多说明」用 `details`。新增页默认收起。编辑页：这五个字段任一有非空内容则默认展开，否则收起。展开状态只影响显示，提交仍带这五个字段的当前值。

照片区、待归位事项仍只在已保存的物品页、且在保存按钮之后。移到回收站的确认在保存所在 `<form>` 之外、页尾。确认文案、版本冲突、事项 404、照片 404 的草稿保留规则不变。

新建页仍然没有照片。

## 10. 其它页面

位置、分类、待归位、回收站沿用新变量和按钮类别。各页「新增位置」「新增分类」用主按钮，文案可改为「新增」，链接目标不变。列表分组标题、分页、确认删除、占用检查、回收站只读照片都不改。

待归位清单、回收站列表的物品行与物品列表同一套封面加名称。

## 11. 测试

Go 测试使用临时数据目录和 `httptest`，通过 HTTP 验证，不模拟 SQLite。至少覆盖：

1. 已登录 `PATCH /api/v1/me` `{"username":"kevin"}` 返回 200 `{"username":"kevin"}`。随后 `GET /api/v1/me` 为 kevin。用 kevin 可以登录，用 ada 得到 `invalid_credentials`。原会话仍有效。
2. `username` 为 `"  "`、含中间空格、超过 64 码点，分别得到上表短句。值为 `null` 或缺失得到 `invalid_body`。与当前值相同返回 200，用户名不变。
3. 两个会话都登录 ada。用会话 A 把密码改成 8 个字符以上的新值，204。会话 A 的 `GET /api/v1/me` 仍 200。会话 B 的 `GET /api/v1/me` 为 401。用旧密码登录 `invalid_credentials`，用新密码 200。
4. 当前密码错误：400，`fields.current_password` 为「当前密码不正确」，`password_hash` 不变，其它会话仍在。新密码与当前相同：400「新密码不能与当前密码相同」，哈希不变。新密码 7 个字符：「密码至少 8 个字符」。
5. `PATCH /api/v1/me` 与 `POST /api/v1/me/password`：错误 Origin 即使带会话也是 403，用户名和哈希不变；无会话且 Origin 正确是 401；带查询字符串且已登录是 400 `invalid_fields`；超过 4096 字节是 413。错误 Origin 加超限正文是 403 不是 413。无会话加超限正文是 401 不是 413。
6. `PUT /api/v1/me`、`POST /api/v1/me`、`GET /api/v1/me/password` 返回 404 `not_found`。
7. 现有登录、限流、Cookie、`GET /api/v1/me` 测试仍然通过。

网页不把浏览器自动化套件放进仓库。实现时在浏览器里实际完成第 12 节。

## 12. 验收

同时满足：

- 第 11 节的 Go 测试通过。
- `cd web && npm run build` 通过。`web/package-lock.json` 的依赖项不增加。没有新的迁移文件。嵌入迁移仍是 `001` 到 `005`。
- 桌面宽度，`http://127.0.0.1:5173`：
  1. 登录页能看到「有处」和「家里的东西在哪儿」。用 `ada` / `correct-horse` 进入物品列表。页头能看到物品、位置、分类、待归位和用户名，看不到「回收站」和「退出」。
  2. 点用户名进入账号。把用户名改成 `kevin`，页头立刻显示 kevin。把密码改成 `new-horse-1`，看到「密码已修改」，仍停在账号页。退出后用 ada 进不去，用 kevin 和新密码进入。
  3. 物品列表查找框写着「找家里的东西」。新增一件只填名称的物品。物品页名称在最上，位置和分类在「更多说明」外面，「更多说明」默认收起；打开后能看到别名等到备注。保存后出现照片区和待归位。页尾能把物品移到回收站。账号页的「回收站」能看到它。
  4. 390×844：登录页、物品列表、物品页、账号页的 `document.documentElement.scrollWidth` 均为 390；页头主入口可点。
- 仓库中没有 MCP、OAuth、Docker，没有 HEIC 解码，没有 `item_categories.source`。

## 13. 给实现的边界

- 目录还不是 Git 仓库。实现结束不要 `git init`，也不要提交。
- 本机 Git 若没有 `user.name` / `user.email`，不要发明作者信息，也不要改 Git 配置。
- 浏览器验收打开 `http://127.0.0.1:5173`，不要用 `localhost`。
- Playwright 若使用，放在仓库外（例如 `/tmp`）。
- 实现时更新 `README.md`：说明可以在网页里改用户名和密码；开发入口仍是 `http://127.0.0.1:5173`。第一句改为：本地可以单账号登录，在网页里改用户名和密码，登记物品和位置，管理分类，搜索筛选，登记待归位，把物品放进回收站，并给物品加照片。MCP 和部署还不在这里。
