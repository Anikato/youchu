# 有处：主屏幕、拍照与动效

日期：2026-10-03

状态：已按用户确认写成（三件一起做：独立窗口、拍照/新建可拍、几处 GSAP）。待实现。

依据：已实现的照片、家用界面、外观、部署规格。冲突时，会话、Origin、照片转码与 20 张上限以照片规格为准；本文件覆盖主屏幕声明、拍照入口、新建物品时选图、以及网页动效。

## 1. 目标

操作者用 Safari 把 https://yc.hiny.cn 加到主屏幕后，以独立窗口打开。添加照片时可以直接开后置相机，也可以从相册选。新建物品时就能选图，保存物品后再按现有接口一张张上传。列表和换页有短动效。

本阶段结束时：

- `/manifest.json`、180 的 `apple-touch-icon.png`、192 与 512 的 PNG 图标。
- `index.html` 含 apple-mobile-web-app 与 theme-color。
- 物品页与新建物品页有「拍照」「相册」两个入口。
- 新建物品保存成功后，把待传文件按现有 `POST /api/v1/items/{id}/photos` 上传，最多 20 张。
- 网页用 GSAP：换页进入、登录面板、物品/位置列表入场。尊重 `prefers-reduced-motion`。

## 2. 不做的事

不做 Service Worker、离线目录、推送、App Store。不改照片转码、不解码 HEIC。不把照片接口改成创建物品时带图。不加滚动驱动、ScrollSmoother、全站时间轴。不改 Origin、会话、环境变量。Playwright 不进仓库。

## 3. 主屏幕

`web/public/manifest.json`：

```json
{
  "name": "有处",
  "short_name": "有处",
  "description": "家里的东西在哪儿",
  "start_url": "/",
  "scope": "/",
  "display": "standalone",
  "lang": "zh-CN",
  "background_color": "#161a18",
  "theme_color": "#161a18",
  "icons": [
    { "src": "/icon-192.png", "sizes": "192x192", "type": "image/png" },
    { "src": "/icon-512.png", "sizes": "512x512", "type": "image/png" }
  ]
}
```

`index.html`：`apple-mobile-web-app-capable`、`mobile-web-app-capable`、`apple-mobile-web-app-title` 为「有处」，`apple-touch-icon` 指向 `/apple-touch-icon.png`，`theme-color` 默认 `#161a18`。切外观时用现有 `setTheme` 把 `theme-color` 改成当前 `--paper`（深 `#161a18`，浅 `#f3f6f4`）。

这些文件走现有 SPA 静态服务，无需登录。不新增路由。Nginx 仍反代到应用，不改 MIME。

图标用现有 `logo-yc.svg` 画在深色方底上，不另做一套品牌。

## 4. 拍照

物品页「添加照片」拆成两个 `input`：

- 「拍照」：`accept="image/jpeg,image/png,image/webp"`，`capture="environment"`，单张。
- 「相册」：同样 `accept`，可 `multiple`，无 `capture`。

现有一次一张请求、20 张上限、HEIC 拒绝短句不变。

新建物品页在保存按钮上方同样两个入口。选中的文件只留在本页内存，显示数量。保存时先 `POST` 物品；成功后再按序上传，最多 20 张。物品已建成、某张图失败：仍进入该物品页，把失败短句放进页面通知。上传期间保存按钮不可用。

## 5. 动效

新增 npm：`gsap` 与 `@gsap/react`。用 `useGSAP`，换页和卸载时 revert。属性只用 `autoAlpha`、`y`。时长约 0.35s，`power2.out`。`prefers-reduced-motion: reduce` 时时长为 0。

三处：

- 登录面板进入。
- 登录后主内容区随路径进入。
- 物品列表与位置列表的行 stagger 0.03s，只在该列表第一次画出时。

390 宽无横滑。

## 6. 测试

Go：假文件系统里 `GET /manifest.json` 与 `GET /apple-touch-icon.png` 无需会话即 200。

前端：`cd web && npm run build` 通过。

浏览器（`/tmp`）：首页 HTML 含 `rel="manifest"` 与 `apple-mobile-web-app-capable`；`/manifest.json` 的 `display` 为 `standalone`；新建物品页有「拍照」且该控件带 `capture="environment"`。

## 7. README 与工作记录

README 可写一句：Safari 可加到主屏幕；添加照片可拍照。工作记录写明本段完成。不要把离线、HEIC 解码或 OAuth 写成已完成。
