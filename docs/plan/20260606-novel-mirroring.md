# PixEz Sync Backend — 小说镜像缓存

本方案在小说详情页新增"镜像"功能，与插画镜像流程完全对称。核心边界如下：

1. 小说内容（正文文本）**直接保存到数据库**（`mirror_novel` 表的 `text_json` 字段），而不写入本地文件，因为小说没有图片资源需要下载。
2. `mirror_tasks` 表作为通用任务队列，`task_type=novel_mirror`，`target_type=novel`，与插画镜像共用同一表（设计时已预留）。
3. 新增 `mirror_novel` 表，存储小说的 Pixiv 原始详情 JSON 与正文文本 JSON，供后续读取接口使用。
4. 新增后端接口 `POST /api/pixez/novels/:novel_id/mirror` 与 `GET /api/pixez/novels/:novel_id/mirror`，语义与插画接口完全对称。
5. 新增 `/mirror/v1/novel/detail` 读取缓存接口，返回 Pixiv 官方 `/v2/novel/detail` 形态 JSON，并重写图片域名。
6. Flutter 前端在小说详情页的"更多"菜单中，新增 `NovelMirrorListTile` 组件，与 `MirrorListTile` 结构对称，但无需更新 Store（小说无需从镜像加载图片流）。
7. Flutter 前端在打开小说详情页时，可以根据“设置 -> 数据同步 -> 自动镜像小说”配置自动调用入队接口。该功能默认开启，但仍受云端同步总开关与服务器地址配置约束。

---

## Proposed Changes

### Go 后端 (`server/`)

#### [NEW] `migrations/20260606000008_create_mirror_novel.sql`

新增 `mirror_novel` 表，存储小说元数据镜像：

| 字段名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `novel_id` | `INTEGER PRIMARY KEY` | Pixiv 小说 ID |
| `detail_json` | `TEXT NOT NULL` | Pixiv `/v2/novel/detail` 原始响应 JSON |
| `text_json` | `TEXT NOT NULL` | Pixiv `/webview/v2/novel` 提取的正文文本 JSON |
| `created_at` | `DATETIME` | 创建时间 |
| `updated_at` | `DATETIME` | 更新时间 |

#### [MODIFY] `model/mirror.go`

新增常量与 `MirrorNovel` 结构体：
- 常量 `MirrorTaskTypeNovel = "novel_mirror"`、`MirrorTargetNovel = "novel"`
- `MirrorNovel` struct，TableName = `mirror_novel`

#### [MODIFY] `service/pixiv_utils.go`

新增：
- `PixivNovelDetail` 与 `PixivNovelWebContent` 结构体（对应 Pixiv `/v2/novel/detail` 与 `/webview/v2/novel`）
- `GetNovelDetail(user, novelID)` 方法
- `GetNovelText(user, novelID)` 方法

#### [MODIFY] `service/mirror_worker.go`

在 `ProcessOne` 的 switch 中新增 `model.MirrorTaskTypeNovel` 分支，调用新增的 `processNovelTask` 方法：

1. 获取 Pixiv 用户 token
2. 调用 `GetNovelDetail` 获取小说详情
3. 调用 `GetNovelText` 获取小说正文
4. 将 `detail_json` 和 `text_json` 写入（或更新）`mirror_novel` 表
5. 更新 `mirror_tasks`：`total_count=1, success_count=1, failed_count=0`
6. 失败时更新 `error_message`，`success_count=0`

小说任务不涉及图片下载，`success_count=1` 即代表镜像可用。

#### [MODIFY] `handler/mirror.go`

新增：
- `MirrorNovel(c *gin.Context)`：接收 `novel_id`，入队 `novel_mirror` 任务，与 `MirrorIllust` 逻辑完全对称
- `CheckNovelMirror(c *gin.Context)`：查询 `mirror_tasks` 状态，与 `CheckIllustMirror` 完全对称
- `GetMirroredNovelDetail(c *gin.Context)`：从 `mirror_novel.detail_json` 读取缓存，重写 pximg 图片域名，挂载在 `/mirror/v1/novel/detail`

辅助函数 `enqueueNovelMirrorTask`、`isNovelMirrored`、`parseNovelIDParam`（与插画版完全对称）。

#### [MODIFY] `main.go`

在 `/api/pixez` 路由组中新增：
```go
api.POST("/novels/:novel_id/mirror", handler.MirrorNovel)
api.GET("/novels/:novel_id/mirror", handler.CheckNovelMirror)
```

在 `/mirror` 路由组中新增：
```go
mirror.GET("/v1/novel/detail", handler.GetMirroredNovelDetail)
```

---

### Flutter 前端改造

#### [MODIFY] `lib/custom/services/sync_api.dart`

新增三个方法（与插画版完全对称）：
- `getNovelMirrorStatus(int id)`: `GET /api/pixez/novels/{id}/mirror`
- `isNovelMirrored(int id)`: 读取 `data.mirrored`
- `mirrorNovel(int id)`: `POST /api/pixez/novels/{id}/mirror`

#### [MODIFY] `lib/custom/services/sync_service.dart`

新增对应的代理方法。

#### [MODIFY] `lib/custom/services/sync_config.dart`

新增 `autoMirrorNovels` 配置，使用 SharedPreferences 保存，默认值为 `true`。该配置只控制客户端是否在小说详情页打开时自动入队，不影响手动镜像入口。

#### [NEW] `lib/custom/services/novel_auto_mirror_service.dart`

创建小说自动镜像服务：
- 检查 `SyncConfig.enabled`、`SyncConfig.serverUrl` 与 `SyncConfig.autoMirrorNovels`
- 在同一 App 会话内对已请求的小说 ID 做内存去重
- 异步调用 `SyncService.mirrorNovel(id)`，失败时释放本次内存去重记录，允许后续再次尝试

#### [NEW] `lib/custom/widgets/novel_mirror_list_tile.dart`

创建 `NovelMirrorListTile` StatefulWidget，结构与 `MirrorListTile` 对称：
- 只接受 `novel_id`（int），不需要 Store 引用（小说镜像只入队，不做 Store 刷新）
- `initState` 时异步检查镜像状态
- 点击已镜像时：仅显示 toast「已镜像（内容已保存到后端）」
- 点击未镜像时：调用 `mirrorNovel`，入队成功后提示「镜像任务已加入队列」，并轮询状态

#### [MODIFY] `lib/page/novel/viewer/novel_viewer.dart`

在 `_showMessage` 方法构建的 `Column` 中，在「分享」ListTile 之后添加：
```dart
NovelMirrorListTile(id: widget.id),
```

在 `initState` 中调用：
```dart
NovelAutoMirrorService.enqueueIfEnabled(widget.id);
```

这是原小说详情页的唯一自动镜像接入口。

#### [MODIFY] `lib/custom/pages/sync_settings_page.dart`

在“启用云端同步”配置区域新增“自动镜像小说”开关，文案为“打开小说详情时自动加入镜像队列”。保存设置时同步写入 `SyncConfig.autoMirrorNovels`。

---

## Verification Plan

### Manual Verification

1. 点击小说详情「更多」→「镜像」，确认 `mirror_tasks` 中生成 `task_type=novel_mirror` 的 `queued` 记录
2. Worker 消费后，`mirror_novel` 表中存在 `novel_id` 对应记录，`detail_json` 和 `text_json` 均不为空
3. `GET /api/pixez/novels/{id}/mirror` 返回 `mirrored: true`，`success_count=1`
4. `/mirror/v1/novel/detail` 返回 Pixiv 形态 JSON，图片 URL 已重写为 `/mirror/pximg/...`
5. 在“设置 -> 数据同步”中关闭“自动镜像小说”后，打开小说详情不应新增 `novel_mirror` 任务
6. 默认配置下，打开小说详情应自动调用 `POST /api/pixez/novels/{id}/mirror`，重复打开同一小说不应在同一客户端会话内重复发起入队请求
