# 有处：照片

日期：2026-09-30

状态：复核通过。本文是第五阶段的实现依据。尚未开始实现。

依据：同仓库《家庭物品整理与查找需求.md》第三节、第七节、第九节，《技术方案草稿.md》第 1、3、7、8 节，以及已实现的《基础登录与存储》《物品和位置》《分类和搜索》《归位与回收站》规格。冲突时，登录、会话、迁移器和 SQLite 连接以第一阶段规格为准；位置页、直接位置列表 `location=`、物品和位置的字段长度与失败顺序以第二阶段规格为准；分类树、物品分类关联、关键词和物品列表上的筛选以第三阶段规格为准；归位事项、回收站、普通删除、占用检查和事项 404 分支以第四阶段规格为准；照片以本文为准。

本文只覆盖交付顺序中的第五段。

## 1. 目标

已登录的操作者可以：

- 给已经保存的物品补照片，也可以没有照片。
- 一件物品最多 20 张。新照片排在末尾。把某张设为第一张后，所有出现该物品名称的列表都用这一张做缩略图。
- 在物品页查看原图、删除某张、把某张设为第一张。
- 在物品列表、位置页、分类页、待归位、回收站的物品名旁边看到第一张的缩略图。
- 回收站里仍能认出照片；永久删除一件时，只清掉这件自己的图。

## 2. 不做的事

本阶段不实现 MCP、OAuth、个人访问令牌、Docker、备份、扫码、主动通知、变更事件表、`item_categories.source`、AI 归类、按图搜索、照片说明文字、独立照片库、公开无登录的图片地址、HEIC/HEIF 解码。

不把用户上传的字节原样保存。服务端能解码的 JPEG、PNG、WebP 转成 JPEG 再落盘。HEIC/HEIF 到了服务端就拒绝，并给出明确短句。

不在新建物品页上传。不一次请求传送多张（网页一次选出多张时，对每张各发一次请求）。不提供照片列表接口。不提供 `has_photo` 筛选。不扫磁盘上的孤儿文件。不新增环境变量。不新增 npm 依赖。不把 Playwright 放进仓库。不启用 CGO。

登录、会话、Origin、登录限流和已有迁移保持现有行为。现有 JSON 写接口的正文上限仍是 32768 字节。

## 3. 仓库

新增：

```
migrations/005_photos.sql
internal/photo/
```

`internal/photo` 负责识别格式、解码、按 EXIF 转正、缩放、写出 JPEG、读写 `originals/` 与 `thumbnails/`。`internal/catalog` 负责 `photos` 表、顺序、20 张上限，以及物品对象和归位事项对象上的照片字段。HTTP 路由仍由 `internal/httpapi` 注册，负责认证、Origin、正文大小、multipart、状态码；读图成功时返回 JPEG 字节，错误仍返回 JSON。网页仍在 `web/src`，页面可以拆文件，路由集中注册。

沿用现有迁移器，只增加 `005` 文件。嵌入版本因此是连续的 `001`、`002`、`003`、`004`、`005`。

启动在打开数据库并完成迁移之后、开始监听之前，创建：

```
{YOUCHU_DATA_DIR}/originals/
{YOUCHU_DATA_DIR}/thumbnails/
{YOUCHU_DATA_DIR}/tmp/
```

目录已存在则继续。创建失败则启动失败，不监听。不新增环境变量，数据根目录仍是 `YOUCHU_DATA_DIR`，默认 `./data`。

实现时更新 `README.md`：说明本地可以给物品加照片；开发入口仍是 `http://127.0.0.1:5173`，并保留 Origin 与首次账号的说明。第一句改为：本地可以单账号登录，登记物品和位置，管理分类，搜索筛选，登记待归位，把物品放进回收站，并给物品加照片。MCP 和部署还不在这里。

允许新增纯 Go 模块以解码 WebP、缩放和读取 JPEG 的 Orientation。`go.sum` 记录实际版本。`web/package-lock.json` 的依赖项不增加。

## 4. 表

`005_photos.sql` 创建照片表。时间列使用 UTC 的 RFC3339Nano 文本，与现有表一致。

```sql
CREATE TABLE photos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position >= 0),
    width INTEGER NOT NULL CHECK (width >= 1),
    height INTEGER NOT NULL CHECK (height >= 1),
    byte_size INTEGER NOT NULL CHECK (byte_size >= 1),
    version INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    UNIQUE (item_id, position)
);

CREATE INDEX photos_item_id ON photos(item_id);
```

`photos.id` 插入时不指定 `id`。已提交的删除不会把该 id 再分配给新行。回滚、未提交的插入不在本阶段测试里。

测试要删除当前最大 id 的照片后新建一张，新 id 与被删 id 不同。再用被删 id 和 `version=1` 去设第一张、去删除，都返回 404。同物品上其余照片保持原样。

同一物品的 `position` 是从 0 起的连续整数，没有空档。**0 是列表用的那一张**。增、删、设第一张都在同一事务里重写该物品全部照片的 `position`，避免 UNIQUE 冲突（可先写成互不冲突的临时值，再写成 0..n-1）。

`width`、`height`、`byte_size` 是**存盘原图**（转正并必要时缩小后的 JPEG）的像素和字节数，不是用户上传文件的。缩略图尺寸不存库。

库里不存格式、存储键或用户文件名。磁盘路径固定为：

- `{YOUCHU_DATA_DIR}/originals/{id}.jpg`
- `{YOUCHU_DATA_DIR}/thumbnails/{id}.jpg`

## 5. 文件与转码

文件和数据库不是同一事务。上传成功路径：

1. 第 6 节第 3 步用 `http.MaxBytesReader` 限制为 41943040 字节。`*http.MaxBytesError` 映射为 413 `body_too_large`，不得当成 `invalid_body`。`Content-Type` 经 `mime.ParseMediaType` 判定为 `multipart/form-data`（允许带 `boundary` 等参数）。这一步只解析 multipart，不解码，也不写入 `originals/` 或 `thumbnails/`。解析产生的 part 文件用完即删（`RemoveAll` 或等价），不留在 `os.TempDir`。
2. 确认物品存在且未进回收站之后，再解码并转码到 `{YOUCHU_DATA_DIR}/tmp/` 下**唯一文件名**的临时文件（与 `originals/` 同一文件系统，便于改名）。
3. `BEGIN IMMEDIATE`，再次确认物品仍在正常列表、当前张数小于 20，插入 `photos` 行（`position` 接到末尾，即当前张数），得到 `id`。
4. 把原图和缩略图临时文件都改名为最终 `{id}.jpg`。两份都改名成功才提交；只成功一份则删除已改名的文件、回滚行。
5. 任一步失败：回滚未提交的行，删除本次临时文件和已经改名的最终文件，不留照片行。物品 404 时丢掉未解码的上传字节，删除本次 tmp，不留下文件。

设第一张、删除一张照片、永久删除物品也都使用 `BEGIN IMMEDIATE`。

删除一张照片：核对版本后删行，把剩余照片按原相对顺序重写成 `position` 0..n-1，提交后再删除该 id 的两份文件。`os.IsNotExist` 视为已清理。其余删文件错误不把行恢复，也不写进 HTTP 响应。

永久删除走现有的 `catalog.PurgeItem` 和 `DELETE /api/v1/trash/{id}`：同一 `BEGIN IMMEDIATE` 内先读出该物品全部 `photos.id`，再 `DELETE FROM items`（CASCADE 去掉 `photos` 行），提交成功后再删这些 id 的两份文件。删文件失败不把物品恢复，响应仍是 204。本段不做启动时扫描孤儿文件。

进回收站和恢复不改照片行，也不改文件。

加、删、设第一张都不改物品的 `version` 和 `updated_at`。

### 5.1 识别与限制

不信任浏览器给的 `Content-Type`。用文件头判断：

| 判断 | 处理 |
| --- | --- |
| JPEG：以 `FF D8 FF` 开头 | 解码 |
| PNG：以 `89 50 4E 47 0D 0A 1A 0A` 开头 | 解码 |
| WebP：以 `RIFF` 开头，第 9–12 字节为 `WEBP` | 解码 |
| HEIC/HEIF：ISO BMFF，偏移 4 起为 `ftyp`，brand 或 compatible brand 含 `heic`、`heix`、`heif`、`heis`、`heim`、`hevc`、`hevx`、`mif1`、`msf1` | 拒绝，短句「暂不支持这种照片格式」 |
| 其它 | 拒绝，短句「不是可用的照片」 |

解码前先 `DecodeConfig`。`width` 或 `height` 小于 1，或 `int64(width)*int64(height)` 大于 **50000000**，拒绝，短句「照片像素过多」，不进行完整解码。完整解码后再用实际像素检查一次，仍超则同样拒绝并清理。

上传请求正文上限 **41943040** 字节（`40 * 1024 * 1024`）。超过返回 413 `body_too_large`，message 为「请求正文过大」。

### 5.2 转正、缩放、编码

JPEG 若带 EXIF Orientation 2 到 8，先按该值转正像素，再按长边限制缩小，再从**已经转正并缩小后的存盘原图**生成缩略图。Orientation 1 或缺 EXIF 不旋转。PNG 和 WebP 本段不读方向元数据。

带透明通道的图（例如 PNG）先铺到白底上，再编码 JPEG。

存盘原图：长边大于 4096 则等比缩小，长边为 4096；已经不超过则不放大。缩略图：长边为 `min(400, 原图长边)`，等比，不放大。两者都用 JPEG 质量 **85**。存盘原图和缩略图都不得再写入 Orientation 2–8（`image/jpeg` 的 `Encode` 即满足）。浏览器按像素显示，不再二次旋转。

`image.Decode` 对动图只取能解出的第一帧。

## 6. HTTP

错误响应沿用 `application/json; charset=utf-8`。错误至少包含 `code` 和 `message`。`message` 给人读，判断以 `code` 为准。未知 JSON 字段忽略。读图成功时 `Content-Type` 为 `image/jpeg`（无 charset），正文是 JPEG 字节，不是 JSON。

本阶段的写路由（上传照片、设第一张、删除照片）以及现有写路由，按下面的顺序停在第一个失败上。方法不对的请求不进入这个顺序，直接返回 404。

1. Origin。不符合或缺少时返回 403 `origin_rejected`，message 为「来源不被接受」，并且不写入。
2. 会话。没有有效会话时返回 401 `unauthenticated`，message 为「未登录」，并且不写入。Origin 和会话都失败时返回 403。
3. 请求大小，以及正文能否解析、类型是否符合下文。`POST /api/v1/photos/{id}/first` 仍按现有 JSON 写接口：超过 32768 字节返回 413 `body_too_large`。`POST /api/v1/items/{id}/photos` 用 `MaxBytesReader(41943040)`；超过返回 413 `body_too_large`，不是 `invalid_body`。JSON 类型错误、或上传请求经 `mime.ParseMediaType` 后主类型不是 `multipart/form-data`、或 multipart 无法解析，返回 400 `invalid_body`，不带 `fields`。`POST` 带了查询参数时，也在这一步返回 400 `invalid_fields`。`DELETE` 不读取正文；它的 `version` 查询参数不合法，或出现 `version` 以外的查询参数时，在这一步返回 400 `invalid_fields`。Origin 或会话失败时停在第 1 或第 2 步，即使正文超过 41943040 也不返回 413。
4. 记录是否存在。路径 id 要匹配 `^[1-9][0-9]*$`，并且库里要有**本节对该接口所要求的那一行**。否则返回 404 `not_found`，message 为「未找到」。`POST /api/v1/items/{id}/photos` 跳过的是照片自己的 id 和版本，**不跳过**物品 id：必须先确认该物品存在且 `deleted_at` 为 `NULL`，再进入第 5 步。因此无法解码的文件配上不存在或已进回收站的物品 id，返回 404，不返回 400。`POST /api/v1/photos/{id}/first` 与 `DELETE /api/v1/photos/{id}` 要求照片行存在，且所属物品 `deleted_at` 为 `NULL`；物品已进回收站时这两条都是 404。
5. 字段规则。返回 400 `invalid_fields`。
6. 版本。不一致时返回 409 `version_conflict`，不写入。上传没有版本，跳过这一步。
7. 关系约束。本阶段增加：一件物品已有 20 张照片。返回 409 `photo_limit`。

因此，不存在的物品 id 配上非 multipart 的上传返回 `invalid_body`，不返回 404。不存在的物品 id 配上合法 multipart 但文件无法解码，返回 404。文件不合法且（若有版本）版本也过旧时返回 400。

路径 id 在 Origin、会话和正文（或 DELETE 的 `version`）之后才解析。`POST /api/v1/items/abc/photos` 在错误 Origin 下返回 403，不返回 404。设第一张、删除照片同样如此。

`GET` 先核对会话，没有会话时返回 401，不核对 Origin。会话有效后，查询参数不合法返回 400，然后再判断记录是否存在。`GET /api/v1/photos/{id}/thumbnail` 与 `GET /api/v1/photos/{id}/original` 与现有详情一样：会话有效后若 `RawQuery` 不是空字符串，返回 400 `invalid_fields`，键为查询参数名，短句为「不支持的参数」，不读取该行、不读文件。

读图时照片行存在即可，**不要求**物品仍在正常列表。物品在回收站时缩略图和原图仍返回 200。行不存在返回 404。行在而磁盘文件不在，返回 500 和空正文。

读图成功时加上 `Cache-Control: private, no-store`。不设置 `Content-Disposition`。不实现 Range（整份 200）。前端不得给读图 URL 加查询参数。

未知路径返回 404 `not_found`，message 为「未找到」。不使用 Go 1.22 的 `"GET /path"` 方法前缀，方法不对时也是 404。

4xx 错误返回 JSON。内部错误仍返回 500 和空正文（包括行在而文件不在），不把 SQL 原文或本地路径给客户端，也不是 `image/jpeg`。

路由注册示例（方法在处理器内分支）：

```
/api/v1/items/{id}/photos
/api/v1/photos/{id}
/api/v1/photos/{id}/first
/api/v1/photos/{id}/thumbnail
/api/v1/photos/{id}/original
```

### 6.1 照片对象与物品、事项上的字段

照片对象：

```json
{
  "id": 3,
  "item_id": 1,
  "position": 0,
  "width": 3024,
  "height": 4032,
  "byte_size": 412003,
  "version": 1,
  "created_at": "2026-09-30T00:00:00Z",
  "updated_at": "2026-09-30T00:00:00Z"
}
```

不返回 URL。客户端用 id 拼 `/api/v1/photos/{id}/thumbnail` 和 `/api/v1/photos/{id}/original`。

物品对象增加 `photos` 数组，按 `position`、`id` 排序。没有照片时为 `[]`，不能是 `null`，键必须出现。列表、详情、回收站详情都是这一个形状。列表缩略图用 `photos[0]`。

`POST /api/v1/items` 新建的 `photos` 为 `[]`。`PATCH /api/v1/items/{id}` 正文出现 `photos` 时忽略（未知字段），照片集合不变。

归位事项对象增加 `cover_photo`：没有照片时为 `null`；有照片时为 `{"id": 3}`，id 是该物品 `position = 0` 的那张。事项列表、事项详情、物品对象里嵌套的 `return_tasks` 都带这个键。

### 6.2 上传

`POST /api/v1/items/{id}/photos` 成功 201，正文是新照片对象。

`Content-Type` 必须是 `multipart/form-data`（带 boundary）。字段名是 `file`。同名多个 part 时用第一个。其它表单字段忽略。不保存用户文件名。

缺 `file`、或该 part 为空：物品存在时返回 400，`fields.file` 为「没有照片文件」。格式与像素短句见第 5.1 节，键都是 `file`。

新行 `version` 为 1，`position` 为写入前该物品的张数（第一张是 0）。已有 20 张再传，409 `photo_limit`，message 为「一件物品最多 20 张照片」，不写入、不留文件。张数在同一 `BEGIN IMMEDIATE` 事务里读取。

### 6.3 设为第一张

`POST /api/v1/photos/{id}/first` 成功 200，正文是该照片对象（此时 `position` 为 0）。

正文：

```json
{ "version": 1 }
```

`version` 规则与完成归位相同。该照片放到最前，其余保持相对顺序，全体 `position` 重写为 0..n-1。只有这一张的 `version` 加 1，并更新它的 `updated_at`。已经是第一张也返回 200，仍加版本。不改其余照片的 `version`。不改物品版本。

### 6.4 删除一张

`DELETE /api/v1/photos/{id}?version=` 成功 204，空正文。

`version` 必填，类型规则与物品 DELETE 相同。成功后该行不在，两份文件按第 5 节删除，剩余照片 `position` 从 0 连续。不改剩余照片的 `version`。不改物品版本。

### 6.5 状态码与字段短句

沿用前四阶段的状态码。本阶段增加：

| 状态 | code | message | 何时 |
| --- | --- | --- | --- |
| 409 | `photo_limit` | 一件物品最多 20 张照片 | 该物品已有 20 张时再上传 |

`invalid_fields` 一次带上本次能够确定的全部字段。本阶段增加的键和短句：

| 键 | 短句 | 条件 |
| --- | --- | --- |
| `file` | 没有照片文件 | 缺少名为 `file` 的 part，或 part 长度为 0 |
| `file` | 暂不支持这种照片格式 | 第 5.1 节的 HEIC/HEIF |
| `file` | 不是可用的照片 | 不是 JPEG/PNG/WebP，或解码失败 |
| `file` | 照片像素过多 | 宽×高大于 50000000 |
| `version` | 版本不正确 | 缺失，或整数小于 1 |

设第一张的正文不是对象时，即使路径 id 不存在也返回 `invalid_body`。`DELETE /api/v1/photos/{不存在的 id}` 的 `version` 不是正整数时返回 400，不返回 404。

## 7. 页面

中文界面。页面最大宽度约 40rem。控件最小高度 2.75rem。继续用 CSS Modules，不引入新的组件库。路径过长时在容器内换行，规则与第二阶段 `.path` 相同。390 宽下导航、列表、照片区都没有横向滚动。

不新增网页路径。读图走现有站点的 `/api`（开发时由 Vite 代理），`<img>` 与页面同源，会带上会话 Cookie。

### 7.1 物品页

`/items/new` 没有照片区。保存成功进入 `/items/:id` 之后再添加。

物品页在保存/删除表单下面、待归位事项上面增加「照片」。

- 按 `position` 显示缩略图。`position = 0` 的标明「列表用」。
- 有照片时，默认在本页显示第一张原图；点其它缩略图切换原图。原图 `max-width: 100%`，不离开物品页。删除当前正在看的那张之后，若还有照片则显示当时 `position = 0` 的原图；一张不剩则不显示原图。上传成功不切换当前正在看的那张；从 0 张变成有图时显示新的第一张。
- 不是第一张的，有「设为第一张」，点了就提交 `POST .../first`，不再确认。
- 「删除这张照片」第一次不发请求。确认区域显示「删除这张照片」，然后是「确认删除」和「取消」。不要显示「将移到回收站」或「永久删除，无法恢复」。
- 「添加照片」是 `input type="file"`，`accept="image/jpeg,image/png,image/webp"`，`multiple`，不设 `capture`。可用 `<label>` 包住以保持最小高度。一次选出的多张按选择顺序各发一次 `POST`。一张失败只提示该张（优先 `fields.file`，否则接口 `message`），其余继续。已有 20 张后不提供添加，并显示「一件物品最多 20 张照片」。
- 缩略图 `alt=""`，原图 `alt=""`。
- 上传、设第一张、删除成功后，只更新照片区，以及物品列表、位置页、分类页、回收站列表缓存里该物品的 `photos`，和待归位清单里对应的 `cover_photo`。**不**用物品详情覆盖未保存的名称、备注、位置、分类和新事项草稿。刷新照片区时，物品对象里嵌套的 `return_tasks[].cover_photo` 一并改成服务器上的值。

设第一张或删除照片返回 404：先再读一次所属物品。

- 物品仍在：只刷新照片区，提示「该照片已不存在」。物品表单和未提交的新增事项草稿保持原样，不回到列表。
- 物品也返回 404：显示「未找到」并回到 `/`。
- 核实请求失败（网络或其他错误，且不是物品 404）：留在当前页，允许重试。

上传返回 404：同样先再读一次物品。物品也 404 则显示「未找到」并回到 `/`；物品仍在则只刷新照片区，提示接口 `message`，草稿不动。核实失败则留在本页。

`version_conflict`：显示接口 `message`，只刷新照片区，草稿不动。

`photo_limit`：显示「一件物品最多 20 张照片」，已成功的上传留在照片区。

两个会话都还在正常列表里改同一件物品，保存得到 `version_conflict` 时：未保存的名称、备注、位置、分类、删除确认、未提交的新增事项草稿，以及照片区当前内容，都保持原样，显示「记录已被修改」和「加载最新内容」。点「加载最新内容」后照片区改为服务器上的 `photos`；未保存的文字和选择在加载最新时仍按第四阶段规则被服务器内容替换（与现有「加载最新内容」一致）。

物品保存返回 404：仍显示「未找到」并回到 `/`。

### 7.2 列表缩略图

凡是现在用物品名称做链接的地方，若有第一张照片，在名称旁边显示缩略图：`src` 为 `/api/v1/photos/{id}/thumbnail`，`loading="lazy"`，`alt=""`。没有照片就不放 `<img>`。图在链接内部，不另开链接。缩略图约 3.5rem 见方，`object-fit: cover`。链接的可访问名称仍以物品名称为主（图无文字）。

包括：

- 物品列表 `/`
- 位置页上的物品
- 分类页上的物品
- 待归位清单里指向物品的名称（用 `cover_photo.id`；`cover_photo` 为 `null` 则无图）
- 回收站列表

链接目标不变。

### 7.3 回收站详情

`/trash/:id` 在文字字段和待归位事项之间显示只读照片区：缩略图和原图与物品页相同，没有添加、设第一张、删除照片。

### 7.4 其它页面

位置页、分类页、待归位、回收站列表的现有文案、分页、确认删除规则不变。不增加「只看有照片」。

## 8. 测试

Go 测试使用临时 `YOUCHU_DATA_DIR`（库文件、`originals/`、`thumbnails/`、`tmp/` 都在里面）和 `httptest`，通过 HTTP 验证对外行为，不模拟 SQLite。走上传或读图的测试在发请求前必须已经有这三个目录（与启动行为相同，可调用同一创建函数）。测试用的 JPEG/PNG/WebP 在测试里用标准库生成，不要把真实相机照片放进仓库。HEIC/HEIF 用最小的 `ftyp` 字节夹具即可。像素过多的测试可用只改了尺寸头、不必填满像素的 JPEG 夹具。至少覆盖：

1. 已有 `001`、`002`、`003`、`004` 的库启动后执行 `005`。再次启动不重复执行。`photos` 表、`UNIQUE (item_id, position)`、`photos_item_id` 存在。启动后 `originals/`、`thumbnails/`、`tmp/` 存在。
2. 给一件物品上传 JPEG、PNG、WebP 各一张，都是 201。物品 `photos` 长度为 3，顺序与上传一致，`photos` 不是 `null`。物品 `version` 和 `updated_at` 与上传前相同。磁盘上有三对 `{id}.jpg`。该物品一条归位事项在没有照片时 `cover_photo` 为 `null`，上传第一张之后为 `{"id": <第一张 id>}`。
3. 把第二张设为第一张：该张 `position` 为 0，`version` 加 1，物品版本不变。`GET /api/v1/items` 里该物品 `photos[0].id` 是这张。已经是第一张再设一次仍 200，`version` 再加 1。
4. 删除当前第一张：204；该 id 的两份文件不在；剩余 `position` 从 0 连续；物品封面变成原来的下一张。
5. HEIC/HEIF 夹具返回 400，`fields.file` 为「暂不支持这种照片格式」，无新行、无新文件。缺 `file` 且物品存在：400「没有照片文件」。随机字节：「不是可用的照片」。`DecodeConfig` 宽×高大于 50000000：「照片像素过多」，不留下完整解码后的大文件。
6. 连续上传到 20 张后第 21 张返回 409 `photo_limit`，已有 20 张和 20 对文件不变。
7. 长边大于 4096 的 JPEG 上传成功后，对象 `width` 与 `height` 的较大值等于 4096。带 EXIF Orientation 6 的 JPEG：先转正再缩小；存盘后宽高相对未转正像素对调；`originals/{id}.jpg` 与缩略图字节里都不再带 Orientation 2–8；缩略图也能读到 200。上传成功后对象 `byte_size` 等于 `originals/{id}.jpg` 的字节数。
8. `DELETE /api/v1/items/{id}?version=` 进回收站后：`GET .../thumbnail` 与 `.../original` 仍 200；上传、设第一张、删除照片都 404，行和文件还在。恢复后 `GET /api/v1/items/{id}` 的 `photos` 仍在。`DELETE /api/v1/trash/{id}?version=` 之后照片行和这两份文件都不在；另一件物品的文件还在。
9. 无会话上传返回 401，且没有新行、没有新文件。无会话读缩略图返回 401。错误 Origin 上传返回 403，且没有新行。`POST /api/v1/items/abc/photos` 在错误 Origin 下返回 403。已登录且 Origin 正确时，上传超过 41943040 字节返回 413。上传 `Content-Type: application/json` 返回 `invalid_body`。`POST /api/v1/items/{不存在的 id}/photos` 配上无法解码的文件返回 404。设第一张正文不是对象时，即使照片 id 不存在也返回 `invalid_body`。`DELETE /api/v1/photos/{不存在的 id}` 的 `version` 不是正整数时返回 400，不返回 404。
10. `GET /api/v1/photos/{id}/thumbnail?foo=1` 与 `GET /api/v1/photos/{id}/original?foo=1` 在已登录时返回 400 `invalid_fields`；无会话时返回 401。上传带查询参数返回 400。
11. 删掉照片表当前最大 id 再上传，新 id 与被删 id 不同。用被删 id 和 `version=1` 去设第一张、去删除，都返回 404，新照片不变。
12. 新建物品 `photos` 为 `[]`。PATCH 物品带 `"photos": []` 之后，已有照片仍在。PNG 透明像素编码后的 JPEG 能被 `image/jpeg` 解码。读缩略图和原图成功时 `Content-Type` 为 `image/jpeg`。
13. 现有物品详情、物品列表、回收站详情、归位事项的 HTTP 测试必须带上新键。断言看**响应原文**：新建物品含 `"photos":[]`；没有照片的事项含 `"cover_photo":null`。不能只靠结构体零值区分省略和 `null`。这是本阶段要改的断言，不是行为回退。
14. 设第一张或删除照片时 `version` 过旧：409 `version_conflict`，`position`、其余照片的 `version`、两份文件都不变。
15. 错误 Origin 加超过 41943040 字节的上传返回 403，无新行无新文件；无会话加同样超限的上传返回 401。两者都不是 413。
16. 照片行在、对应 `originals` 或 `thumbnails` 文件不在：读图 500、空正文、`Content-Type` 不是 `image/jpeg`。
17. 设第一张后，`GET /api/v1/return-tasks` 与物品嵌套 `return_tasks[].cover_photo` 的 `id` 都是新的 `position = 0`。删掉封面后变为下一张；删光后响应原文为 `"cover_photo":null`。
18. 现有 HTTP 测试助手必须设置临时 `Config.DataDir` 并创建 `originals/`、`thumbnails/`、`tmp/`。上传和读图不得写到仓库工作目录。

网页不把浏览器自动化套件放进仓库。实现时在浏览器里实际完成第 9 节的步骤。

## 9. 验收

同时满足以下条件才算本阶段完成：

- 第 8 节的 Go 测试通过。
- `cd web && npm run build` 通过。
- 桌面宽度下实际完成：登录；只填名称登记「遥控车」，物品列表该行没有缩略图；打开物品页加上两张 JPEG，第二张排在后面；默认能看到第一张原图。物品列表、它所在的位置页、所挂分类页都能看到第一张缩略图。把第二张设为第一张后，这三处缩略图都换成原来的第二张。建一条待归位，清单物品名旁边也是这张图。
- 普通删除遥控车后，回收站列表和回收站详情都能看到这张图；详情里没有添加、设第一张、删除照片。恢复后照片还在。再进回收站永久删除后，回收站没有它；再请求原来的缩略图地址得到 404。
- 物品页改了名称但还没保存时加上一张照片：名称草稿仍在，照片区出现新图。两个会话打开同一件物品；页面 A 删掉某张后，页面 B 再删它或对它设第一张：提示「该照片已不存在」，未保存的名称、备注、位置、分类和新事项草稿仍在，不回到列表。
- 一次选择两张都能加上。传到第 20 张后不能再加，并看到「一件物品最多 20 张照片」。
- 390 宽下打开带缩略图的物品列表、物品页照片区、待归位和回收站：导航可点；`scrollWidth` 等于视口宽度。
- 仓库中没有 MCP、OAuth、Docker，也没有 `item_categories.source`，没有 HEIC 解码实现。归位事项表没有位置 id 列。
- `go.sum` 记录本段实际使用的图片库版本。`web/package-lock.json` 的依赖项不增加。

## 10. 给实现的边界

- 目录还不是 Git 仓库。实现结束不要 `git init`，也不要提交。
- 本机 Git 若没有 `user.name` / `user.email`，不要发明作者信息，也不要改 Git 配置。
- 浏览器验收打开 `http://127.0.0.1:5173`，不要用 `localhost`。
- Playwright 若使用，放在仓库外（例如 `/tmp`）。
