# 有处

本地可以单账号登录，在网页里改用户名和密码、签发接入令牌，登记物品和位置，管理分类，搜索筛选，登记待归位，把物品放进回收站，并给物品加照片。Safari 可以把站点加到主屏幕，以独立窗口打开。添加照片可以拍照或从相册选，新建物品时就能选图。添加物品可以不选位置，也可以当场新建根分类，之后再改成子分类。移动容器可以克隆，编号类型不动、序号加 1。位置可以选内置图标或自己上传的 SVG。MCP 可用个人访问令牌接入。生产用 Docker Compose，本机 Nginx 反代。

源码：https://github.com/Anikato/youchu。镜像：`l97312/youchu`。接手正式环境、MCP 和本机开发：`docs/接手说明.md`。

登录后点用户名进入账号，可改用户名和密码、签发接入令牌；回收站和退出也在账号页。明文令牌只在创建成功时出现一次，列表只有前缀，可以撤销。

照片可在新建物品时先选好并看到预览，保存物品后再上传；物品页也可拍照或从相册添加。列表默认最新在前，显示第一张缩略图和添加时间。新增物品保存后仍留在本页，记住上次位置和分类。添加或编辑物品时用「找位置」按名称或编号筛选，点一条就加上。手机上保存按钮贴在底栏上方。

顶层位置超过 30 条时用下一页；选择存放位置或父级时会继续请求后续页。分类选择器和物品列表筛选会按页取完整棵分类树。

普通删除进入回收站；永久删除只在回收站里再次确认。改用户名、改密码、签发令牌和永久删除只在网页上进行。

开发时开两个进程。浏览器只访问 Vite，`/api` 和 `/mcp` 由 Vite 转发到 Go。来源必须是 `http://127.0.0.1:5173`，不要改用 `localhost`。Cookie 会话不能当 `/mcp` 凭证。MCP 用 `Authorization: Bearer` 访问 `http://127.0.0.1:8080/mcp`；开发时也可用 `http://127.0.0.1:5173/mcp`。

```bash
YOUCHU_PUBLIC_ORIGIN=http://127.0.0.1:5173 \
YOUCHU_USERNAME=ada \
YOUCHU_PASSWORD=correct-horse \
go run ./cmd/youchu
```

另一个终端：

```bash
cd web
npm run dev
```

然后打开 `http://127.0.0.1:5173`。

第一次启动、用户表还是空的时候，程序用上面的用户名和密码创建唯一账号，密码以 argon2id 存储。账号已经存在时，环境变量不会改掉密码。创建成功后，从长期运行环境里去掉明文密码。

`YOUCHU_SESSION_TTL` 至少 1 秒，默认 14 天。`YOUCHU_LOGIN_LIMIT` 至少 1，默认每个地址每 60 秒 10 次。开发时 Go 看到的地址是 Vite，所以这些登录请求共用一个限流桶。开发默认不读 `X-Forwarded-For`。

数据文件默认在 `./data/youchu.db`。可以用 `YOUCHU_DATA_DIR` 换目录。

## 生产

把 `deploy/` 里的文件拷到目标机一个目录，把 `.env.example` 复制为 `.env`，填写 `YOUCHU_PUBLIC_ORIGIN`（HTTPS 来源，无路径、无末尾斜杠）和首次账号，然后：

```bash
mkdir data
docker compose up -d
```

Compose 把容器端口映射到 `127.0.0.1:${YOUCHU_HOST_PORT:-8080}`。TLS 留在本机 Nginx，证书不进容器。片段见 `deploy/nginx.conf.example`：反代到 `127.0.0.1` 与 `YOUCHU_HOST_PORT`，不要改写 `Origin`。示例里 `client_max_body_size` 是 `41m`，对应应用层照片上限 40 MiB。

备份：`docker compose stop`，打包整个目录（含 `data/`），再 `docker compose start`。运行中直接复制 WAL 里的数据库不保证一致。

推送 `main` 后 GitHub Actions 构建镜像。到仓库 Settings → Secrets and variables → Actions 增加 `DOCKERHUB_TOKEN`（用户名 `l97312`）。没有这个 Secret 时工作流仍会构建，但不登录、不推送。不要把 token 写进仓库、`.env` 或聊天。
