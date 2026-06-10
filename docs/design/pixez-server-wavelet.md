# PixEzServer/Wavelet 迁移设计

本文档描述 PixEz Sync 业务迁移到 `PixezServer/` 后的当前设计。旧 `server/` 的独立 Gin/SQLite/Basic Auth 后端只保留为 legacy 参考，当前实现以 Wavelet 的认证、路由、迁移、任务和文件存储能力为准。

## 目标与边界

迁移目标：

- 保留 Flutter custom 层已调用的 PixEz 业务路径，例如 `/api/pixez/**` 与 `/mirror/**`。
- `/api/pixez/**` 统一使用 Wavelet 响应包裹：

```json
{ "error_msg": "", "data": {} }
```

- `/mirror/**` 保持 Pixiv 官方响应形态或二进制文件输出，不套系统 envelope。
- 认证统一使用 Wavelet 登录态，支持 `Authorization: Bearer <access_token>` 与 `X-Access-Token`。
- 不恢复旧 `mirror_tasks` 轮询队列；任务调度、日志、重试和执行历史交给 Wavelet Asynq 与 `task_executions`。
- 图片文件统一登记到 Wavelet `uploads`，底层走本地存储或 S3 兼容存储。

当前仍按单租户伴生服务设计，不新增 `owner_user_id` 或跨租户隔离。

## 模块拓扑

```text
PixEz Flutter custom layer
  |
  |  Authorization: Bearer <access_token>
  v
PixezServer/Wavelet Gin router
  |-- /api/pixez/**        -> internal/apps/pixez
  |-- /mirror/**           -> internal/apps/pixez mirror read handlers
  |-- /api/v1/admin/tasks  -> Wavelet Admin task APIs
  |
  v
internal/service/pixez
  |-- Pixiv App API client and token refresh
  |-- bookmark export semantics
  |-- mirror processing and Upload mapping
  |-- legacy server import
  |
  v
GORM models + goose migrations
  |-- PixEz sync/read-model tables
  |-- uploads
  |-- task_executions
  |
  v
Asynq Redis worker + Upload local/S3 storage
```

路由只在 `PixezServer/internal/router/router.go` 注册，业务 handler 位于 `PixezServer/internal/apps/pixez/`，跨 handler 的 Pixiv 请求与状态写入逻辑位于 `PixezServer/internal/service/pixez/`。

## 数据模型

PixEz 账号与 7 张同步数据表继续沿用旧业务语义：

- `pixiv_users`
- `ban_comments`
- `ban_illusts`
- `ban_tags`
- `ban_users`
- `illust_histories`
- `novel_histories`
- `tag_histories`

本阶段新增 read-model 与导出表：

- `mirror_illust`：插画镜像状态、Pixiv 详情 JSON、图片 Upload 映射、请求 URL、失败 URL、统计计数。
- `mirror_novel`：小说镜像状态、Pixiv 详情 JSON、正文 JSON、请求 URL、失败 URL、统计计数。
- `bookmark_export_runs`：单次收藏导出运行记录。
- `bookmark_illusts`：插画收藏导出 read-model，包含 removed 状态与 mirror 状态。
- `bookmark_novels`：小说收藏导出 read-model，包含 removed 状态与 mirror 状态。

迁移文件位于：

- `PixezServer/internal/db/migrator/goose/sqlite/202606100001_pixez_mirror_bookmark.sql`
- `PixezServer/internal/db/migrator/goose/postgres/202606100001_pixez_mirror_bookmark.sql`

PostgreSQL 与 SQLite 使用同版本同名 goose 迁移。关系通过显式索引和唯一约束表达，不新增物理外键。

## 异步任务

PixEz 后台任务通过 Wavelet Admin 任务体系下发，Worker 统一进入 `task.ProcessTask`，执行记录写入 `task_executions`。

已注册 Asynq 类型：

- `pixez:mirror_illust`
- `pixez:mirror_novel`
- `pixez:export_bookmark_illusts`
- `pixez:export_bookmark_novels`
- `pixez:auto_enqueue_bookmark_mirrors`
- `pixez:import_legacy_server`

`pixez:auto_enqueue_bookmark_mirrors` 当前作为 Admin 手动任务使用，不新增 cron 配置。后续若需要周期调度，应按系统设置与 scheduler 规则追加配置。

业务状态与执行状态分离：

- PixEz 客户端状态保存在 `mirror_illust` / `mirror_novel`，值为 `queued`、`processing`、`success`、`failed`。
- Wavelet 任务执行状态保存在 `task_executions`，值为 `pending`、`running`、`succeeded`、`failed`。

## 镜像读取

插画镜像流程：

1. `POST /api/pixez/illusts/:illust_id/mirror` 幂等下发 Asynq 任务。
2. Worker 读取最近同步的 Pixiv 用户凭证，请求 Pixiv `v1/illust/detail`。
3. 从详情中收集 original 图片 URL。
4. 下载成功的图片写入 Wavelet Upload 存储，并创建 `uploads` 记录。
5. `mirror_illust.image_files_json` 保存 `{pixiv_url,page,upload_id,file_name,hash,mime,size,storage_key}` 数组。
6. `/mirror/v1/illust/detail` 返回原始 Pixiv 详情 JSON，并把 `i.pximg.net` / `s.pximg.net` 改写为当前 `/mirror/pximg`。
7. `/mirror/pximg/*path` 根据 original/master/square 文件名映射查找 Upload 记录并流式输出。

小说镜像流程：

1. `POST /api/pixez/novels/:novel_id/mirror` 幂等下发 Asynq 任务。
2. Worker 请求 Pixiv `v2/novel/detail` 与 `webview/v2/novel`。
3. `mirror_novel.detail_json` 保存详情原始 JSON，`mirror_novel.text_json` 保存正文 JSON。
4. `/mirror/v1/novel/detail` 与 `/mirror/webview/v2/novel` 返回 Pixiv 形态 JSON，不套 envelope。

## 收藏导出

收藏导出按 Pixiv 用户和 `restrict=public|private` 分页执行：

- 插画使用 `/v1/user/bookmarks/illust`。
- 小说使用 `/v1/user/bookmarks/novel`。
- 正常作品增量 upsert。
- 未变化的 active 记录只刷新本轮运行 ID 与最后出现时间，不重写完整 JSON。
- placeholder 插画 `limit_unknown_360` 与 placeholder 小说 `limit_unknown_100` 立即标记 removed。
- 只有分页完整成功后，才把本轮缺失的历史 active 记录标记 removed。
- 分页中途失败时不做缺失标记，避免误删语义。

客户端查询已移除插画使用：

```text
GET /api/pixez/users/:pixiv_user_id/bookmarks/illust/removed
```

该接口返回 Wavelet envelope，`data` 内保持 Pixiv 列表形态：`illusts` 与 `next_url`。

## Legacy 导入

旧后端导入通过 Admin 任务 `pixez:import_legacy_server` 执行，默认 payload：

```json
{
  "sqlite_path": "server/pixez-sync.db",
  "mirror_dir": "server/data/mirror",
  "dry_run": false
}
```

导入规则：

- `pixiv_users` 和 7 张同步表导入到 PixezServer 当前数据库。
- `bookmark_illusts`、`bookmark_novels` 使用唯一键 upsert，支持重复运行。
- `mirror_illust.image_files_json` 中存在的旧文件会计算 hash/mime/size 并登记为 Wavelet Upload。
- 文件缺失只记录 missing 计数，不伪造成功 Upload。
- 旧 `mirror_tasks` 只读取历史状态、URL 和计数，用于填充 read-model，不作为新队列。

## 公共接口

`/api/pixez/**`：

- `GET /api/pixez/ping`
- `GET /api/pixez/users`
- `GET /api/pixez/users/:pixiv_user_id`
- `PUT /api/pixez/users/:pixiv_user_id`
- `DELETE /api/pixez/users/:pixiv_user_id`
- `GET /api/pixez/users/:pixiv_user_id/sync-data`
- `POST /api/pixez/users/:pixiv_user_id/sync-data`
- `GET /api/pixez/users/:pixiv_user_id/sync-data/hashes`
- `GET /api/pixez/users/:pixiv_user_id/bookmarks/illust/removed`
- `POST /api/pixez/illusts/:illust_id/mirror`
- `GET /api/pixez/illusts/:illust_id/mirror`
- `POST /api/pixez/illusts/mirror/batch`
- `POST /api/pixez/novels/:novel_id/mirror`
- `GET /api/pixez/novels/:novel_id/mirror`
- `POST /api/pixez/novels/mirror/batch`
- `GET /api/pixez/mirror/illusts`
- `GET /api/pixez/mirror/novels`
- `DELETE /api/pixez/mirror/illusts/:illust_id`
- `DELETE /api/pixez/mirror/novels/:novel_id`
- `POST /api/pixez/mirror/batch-delete`

`/mirror/**`：

- `GET /mirror/v1/illust/detail`
- `GET /mirror/pximg/*path`
- `GET /mirror/v1/novel/detail`
- `GET /mirror/webview/v2/novel`

`/mirror/**` 也需要 Wavelet AccessToken 鉴权，但错误响应保持简单 JSON 或 HTTP 状态。
