# PixEz Sync Backend — 插画镜像缓存与加载

本方案设计在插画详情页新增“镜像”与“已镜像”加速加载功能。核心边界如下：

1. `/mirror/**` 是 Pixiv 官方接口的镜像对接命名空间，接口路径、请求语义与 Pixiv 官方 API 对齐，只在后端实现本地镜像命中、域名重写与图片流读取逻辑。
2. 任何不属于 Pixiv 官方请求语义的系统业务接口都不得放在 `/mirror/**` 下，例如“请求镜像”“检查是否存在镜像”等操作必须放在 `/api/pixez/**` 下。
3. “请求镜像”接口只接收插画 ID。该请求必须是异步入队请求：后端只把插画 ID 写入数据库队列并立即返回，不在请求链路内访问 Pixiv 服务器。
4. 后端拆分为两个 SQLite 表：`mirror_tasks` 作为通用镜像任务队列表，`mirror_illust` 作为插画元数据镜像表。任务开始处理时才请求 Pixiv 官方接口获取插画详情，再下载该插画的图片资源并写入本地缓存。
5. “检查是否存在镜像”必须查询本地 SQLite 的 `mirror_tasks` 统计字段。只要同一插画任务 `success_count > 0`，即可返回 `mirrored=true`；`mirror_illust` 只表示 Pixiv 详情元数据已落库，不单独代表图片镜像完成。

## User Review Required

> [!IMPORTANT]
> **接口命名空间边界**：
> `/mirror/**` 只能用于 Pixiv 官方接口镜像。它不是 PixEz Sync Backend 的业务操作接口空间。
> - 允许：`GET /mirror/v1/illust/detail?illust_id=...`，语义对齐 Pixiv `GET /v1/illust/detail`。
> - 允许：`GET /mirror/pximg/*path`，用于承载被重写后的 Pixiv 图片资源读取。
> - 禁止：`POST /mirror/v1/illust/detail` 用于“请求镜像”。
> - 禁止：`GET /mirror/v1/illust/detail/check` 用于“检查镜像”。

> [!IMPORTANT]
> **镜像缓存数据来源**：
> 客户端发起镜像时只提交 `illust_id`。请求到达后只入队，不阻塞等待下载完成。
> 后端 worker 消费任务时必须自行调用 Pixiv 官方接口获取该插画详情，不能要求客户端上传插画详情 JSON 或图片 URL 列表。

> [!IMPORTANT]
> **本地缓存与数据库职责**：
> SQLite 拆分为两个职责明确的表：
> - `mirror_tasks`：通用镜像任务表，记录插画镜像、小说镜像等异步任务、任务状态、请求图片 URL 总列表、成功数、失败数、失败下载地址、错误详情和重试信息。
> - `mirror_illust`：记录已可读取的插画元数据镜像，包括 Pixiv 原始详情 JSON、已成功图片文件清单和更新时间；即使图片下载全部失败，也保留详情记录以便追踪。
> 图片二进制文件仍存放在本地文件树 `{MirrorDir}/{illust_id}/` 下，便于后续迁移至 S3、MinIO 等对象存储。

---

## Proposed Changes

### Go 后端 (`server/`)

新增通用镜像任务队列、插画镜像记录数据库表、异步消费 worker、Pixiv 接口镜像读取与本地图片流服务。

#### [NEW] `mirror_tasks` 数据表

新增迁移文件，例如 `server/migrations/*_create_mirror_tasks.sql`，使用 SQLite 充当通用镜像任务队列。该表不仅服务插画镜像，也必须能承载后续小说镜像任务。任务使用独立 `id` 作为主键，资源对象由 `target_type` 与 `target_id` 表达。

建议字段：

| 字段名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `id` | `TEXT PRIMARY KEY` | 任务 ID，建议使用 UUID/ULID |
| `task_type` | `TEXT NOT NULL` | 任务类型，例如 `illust_mirror` / `novel_mirror` |
| `target_type` | `TEXT NOT NULL` | 目标资源类型，例如 `illust` / `novel` |
| `target_id` | `INTEGER NOT NULL` | Pixiv 目标资源 ID，例如插画 ID 或小说 ID |
| `status` | `TEXT NOT NULL` | `queued` / `processing` / `success` / `failed` |
| `request_payload_json` | `TEXT` | 入队时保存的最小请求上下文 JSON；插画镜像至少包含 `illust_id` |
| `request_urls_json` | `TEXT` | 从 Pixiv 详情提取出的 original 档图片 URL JSON 数组，不记录 master/square URL |
| `retry_urls_json` | `TEXT` | 下载失败的 original 档图片 URL JSON 数组，用于后续重试，不记录 master/square URL |
| `error_message` | `TEXT` | 最近一次失败详情，必须包含原始请求、响应状态、响应内容与错误摘要 |
| `total_count` | `INTEGER NOT NULL DEFAULT 0` | 本次任务请求下载的图片 URL 总数 |
| `success_count` | `INTEGER NOT NULL DEFAULT 0` | 本次任务成功下载的图片数量，`> 0` 即可视为镜像可用 |
| `failed_count` | `INTEGER NOT NULL DEFAULT 0` | 本次任务下载失败的图片数量 |
| `attempt_count` | `INTEGER NOT NULL DEFAULT 0` | 已处理次数 |
| `locked_at` | `DATETIME` | worker 领取任务时间，用于处理中任务恢复 |
| `started_at` | `DATETIME` | 开始处理时间 |
| `finished_at` | `DATETIME` | 完成时间 |
| `created_at` | `DATETIME` | 创建时间 |
| `updated_at` | `DATETIME` | 更新时间 |

建议增加唯一约束：`UNIQUE(task_type, target_type, target_id)`，防止同一资源的同类镜像任务重复入队。

该表只表示队列与处理状态，不作为 `/mirror/v1/illust/detail` 的详情读取来源。插画镜像成功后的可读取元数据必须写入 `mirror_illust`；后续小说镜像应写入对应的小说元数据镜像表，而不是把业务结果塞进 `mirror_tasks`。

#### [NEW] `mirror_illust` 数据表

新增迁移文件，例如 `server/migrations/*_create_mirror_illust.sql`，记录已经获取、可以被 `/mirror/**` 读取的插画详情元数据。图片可以部分失败；失败图片由 `/mirror/pximg/*path` 按文件缺失返回 404。

建议字段：

| 字段名 | 类型 | 说明 |
| :--- | :--- | :--- |
| `illust_id` | `INTEGER PRIMARY KEY` | Pixiv 插画 ID |
| `detail_json` | `TEXT NOT NULL` | Pixiv 官方 `v1/illust/detail` 原始响应 JSON |
| `image_files_json` | `TEXT NOT NULL` | 已成功下载图片文件清单 JSON |
| `created_at` | `DATETIME` | 创建时间 |
| `updated_at` | `DATETIME` | 更新时间 |

检查镜像时不能只看该表中 `illust_id` 对应记录是否存在，必须以 `mirror_tasks.success_count > 0` 作为镜像可用判定。

#### [MODIFY] [config/config.go](file:///Users/ryan/DEV/Flutter/pixez-flutter/server/config/config.go)

新增配置项：

| 配置项 | 环境变量 | 默认值 | 说明 |
| :--- | :--- | :--- | :--- |
| `MirrorDir` | `PIXEZ_MIRROR_DIR` | `./data/mirror` | 镜像图片本地缓存根目录 |
| `MirrorDownloadConcurrency` | `PIXEZ_MIRROR_DOWNLOAD_CONCURRENCY` | `5` | 每个镜像任务下载图片时的最大并发数 |

`PIXEZ_MIRROR_DOWNLOAD_CONCURRENCY` 必须限制为正整数，非法或未设置时使用默认值 `5`。

#### [NEW] `service/mirror_worker.go`

实现数据库队列消费逻辑：

1. 服务启动时启动后台 worker。
2. worker 从 `mirror_tasks` 中领取 `queued` 任务，并将状态更新为 `processing`。
3. worker 根据 `task_type` 分发处理逻辑。当前计划只实现 `illust_mirror`；后续 `novel_mirror` 复用同一任务表。
4. 插画镜像任务领取后才请求 Pixiv 官方 `GET /v1/illust/detail?illust_id=...`。
5. 解析响应中的 original 图片 URL，包括 `meta_single_page.original_image_url` 与 `meta_pages[].image_urls.original`；不下载 `square_medium`、`medium`、`large` 等 master 或 square 档图片。
6. 使用 `PIXEZ_MIRROR_DOWNLOAD_CONCURRENCY` 控制图片下载并发，默认最多并发 5 个下载。
7. Pixiv 详情获取成功后，即写入或更新 `mirror_illust`，并将全部请求图片 URL 记录到 `mirror_tasks.request_urls_json`。
8. 图片下载完成后，将下载总数、成功数、失败数和失败 URL 列表写入 `mirror_tasks`。只要 `success_count > 0`，任务即可更新为 `success`；失败图片后续通过 `/mirror/pximg/*path` 返回 404。
9. 如 Pixiv 详情请求失败，或图片全部下载失败，将 `mirror_tasks.status` 更新为 `failed`，但 Pixiv 详情获取成功时仍应保留 `mirror_illust`：
   - 下载失败的原始 URL 写入 `mirror_tasks.retry_urls_json` JSON 数组。
   - `mirror_tasks.error_message` 必须保存原始请求、响应状态、响应内容和错误摘要，便于定位问题。
10. 对 `processing` 状态但 `locked_at` 超时的任务，需要后续实现恢复或重新入队策略，避免进程退出后任务永久卡住。

#### [NEW] `handler/pixez_mirror.go`

实现 PixEz Sync Backend 业务操作接口，全部挂载在 `/api/pixez/**` 下，并遵守统一响应格式。

* `POST /api/pixez/illusts/:illust_id/mirror`
  1. 仅从路径参数读取 `illust_id`，不接收客户端上传的详情 JSON、图片 URL 列表或 Pixiv 用户 ID。
  2. 将任务写入 `mirror_tasks`。新任务生成独立 `id`，设置 `task_type=illust_mirror`、`target_type=illust`、`target_id={illust_id}`、`status=queued`。
  3. 如果同一 `(task_type, target_type, target_id)` 任务已存在，直接返回当前任务 ID 与状态，不重复创建任务；如果已失败，后续可扩展为重新入队。
  4. 该请求不访问 Pixiv 服务器，不下载图片，不阻塞等待任务完成。
  5. 返回统一响应，例如：
     ```json
     {
       "success": true,
       "message": "",
       "data": {
         "task_id": "01J...",
         "illust_id": 123,
         "status": "queued",
         "mirrored": false
       }
     }
     ```

* `GET /api/pixez/illusts/:illust_id/mirror`
  1. 查询 `mirror_tasks` 中 `task_type=illust_mirror AND target_type=illust AND target_id={illust_id}` 的任务。
  2. 如果任务 `success_count > 0`，返回 `mirrored: true`；否则返回 `mirrored: false`。
  3. 同时返回当前任务 ID、任务状态、下载总数、成功数、失败数、请求 URL 列表与失败 URL 列表。
  4. 不访问 Pixiv 官方接口，不扫描图片目录作为主要判断依据。

#### [NEW] `handler/mirror.go`

实现 Pixiv 官方接口形态的镜像读取接口，只挂载在 `/mirror/**` 下。

* `GET /mirror/v1/illust/detail?illust_id=...`
  1. 语义对齐 Pixiv 官方 `GET /v1/illust/detail`。
  2. 只从 `mirror_illust.detail_json` 读取本地缓存详情，不访问 Pixiv 服务器，也不读取任务表作为详情来源。
  3. 将 JSON 内所有 `https://i.pximg.net` / `https://s.pximg.net` 图片域名动态改写为当前同步服务器的 `/mirror/pximg/*` 路径。
  4. 返回结构应尽量保持 Pixiv 官方响应格式，不使用 PixEz Sync Backend 的 `success/message/data` 包装。

* `GET /mirror/pximg/*path`
  1. 承载被重写后的 Pixiv 图片 URL。
  2. 根据请求路径定位本地镜像文件并输出二进制图片流。若请求文件名带 `_master...` 或 `_square...` 后缀，后端会映射到同页 original 文件名（例如 `123_p0_master1200.jpg` 映射为 `123_p0.jpg`）。
  3. 该接口服务于 Pixiv 图片资源镜像，不承担检查、创建或管理镜像记录的职责。

#### [MODIFY] [main.go](file:///Users/ryan/DEV/Flutter/pixez-flutter/server/main.go)

注册业务接口与 Pixiv 镜像接口：

```go
api := r.Group("/api/pixez", middleware.BasicAuth(cfg.AuthUser, cfg.AuthPass))
{
	api.POST("/illusts/:illust_id/mirror", handler.MirrorIllust)
	api.GET("/illusts/:illust_id/mirror", handler.CheckIllustMirror)
}

mirror := r.Group("/mirror", middleware.BasicAuth(cfg.AuthUser, cfg.AuthPass))
{
	mirror.GET("/v1/illust/detail", handler.GetMirroredIllustDetail)
	mirror.GET("/pximg/*path", handler.ServeMirroredImage)
}
```

---

### Flutter 前端改造

在插画详情“更多”菜单中新增操作逻辑，并实现镜像状态获取与拉取渲染。

#### [MODIFY] [sync_service.dart](file:///Users/ryan/DEV/Flutter/pixez-flutter/lib/custom/services/sync_service.dart)

增加以下三个与镜像相关的客户端 API：

* `isIllustMirrored(int id)`: 请求 `GET /api/pixez/illusts/{id}/mirror`，读取统一响应内的 `data.mirrored`。
* `mirrorIllust(int id)`: 请求 `POST /api/pixez/illusts/{id}/mirror`，请求体为空或不携带业务字段。该请求只负责入队，立即返回任务状态，不等待镜像完成。
* `getMirroredIllustDetail(int id)`: 请求 `GET /mirror/v1/illust/detail?illust_id={id}`，获取 Pixiv 官方响应形态的本地镜像详情。

以上接口不再接收或发送 `pixivUserId`，镜像业务的入参只保留插画 ID。

#### [MODIFY] [illust_lighting_page.dart](file:///Users/ryan/DEV/Flutter/pixez-flutter/lib/page/picture/illust_lighting_page.dart)
#### [MODIFY] [illust_row_page.dart](file:///Users/ryan/DEV/Flutter/pixez-flutter/lib/page/picture/illust_row_page.dart)

在 `buildShowModalBottomSheet` 弹出的底部操作表单中：

1. 使用 `StatefulBuilder` 封装内容，使状态可以局部刷新。
2. 打开时异步检查当前插画的镜像状态，在加载中时，镜像按钮提示为“镜像 (检测中...)”。
3. 若未镜像：
   - 按钮标题为“镜像”，点击后发起 `SyncService.mirrorIllust(id)`。
   - 入队成功后提示“镜像任务已加入队列”，本地按钮文字更新为“镜像中”或“处理中”。
   - 前端可通过 `SyncService.isIllustMirrored(id)` 轮询任务状态，直到 `success` 后再显示“已镜像”。
4. 若已镜像：
   - 按钮标题为“已镜像”（附带勾选图标），点击后弹出加载提示，调用 `SyncService.getMirroredIllustDetail(id)` 重新拉取同步服务器的 Pixiv 形态 JSON。
   - 解析返回 JSON 并通过更新 MobX store 的值完成页面重新渲染。此时，因为 JSON 中的图片 URL 已被重写为同步服务器地址，客户端后续图片请求会打到同步服务器。

---

## Verification Plan

### Go 后端集成测试

* 在后端测试中添加 `/api/pixez/illusts/:illust_id/mirror` 的创建镜像与检查镜像测试。
* 添加镜像入队测试，确认 `POST /api/pixez/illusts/:illust_id/mirror` 不访问 Pixiv、不下载图片，只写入 `queued` 任务并立即返回。
* 添加 worker 消费测试，确认 `mirror_tasks` 任务状态从 `queued` 更新为 `processing`，Pixiv 详情获取成功后在 `mirror_illust` 写入元数据镜像，并记录请求总数、成功数、失败数。
* 添加失败测试，确认失败图片 URL 写入 `mirror_tasks.retry_urls_json`，并且 `error_message` 包含原始请求、响应状态、响应内容和错误摘要；Pixiv 详情获取成功时即使图片失败也保留 `mirror_illust`。
* 添加通用任务表约束测试，确认 `mirror_tasks.id` 为主键，且 `(task_type, target_type, target_id)` 唯一约束可以防止同一插画镜像任务重复入队，同时不阻止未来小说镜像任务使用 `task_type=novel_mirror`、`target_type=novel`。
* 添加并发配置测试，确认图片下载并发默认值为 5，且可由 `PIXEZ_MIRROR_DOWNLOAD_CONCURRENCY` 覆盖。
* 添加 `/mirror/v1/illust/detail` 的读取测试，确认返回 Pixiv 官方响应形态且不包含 `success/message/data` 包装。
* 添加 `/mirror/pximg/*path` 图片流测试，确认本地图片可被正确读取。
* 确认“请求镜像”与“检查镜像”不再注册在 `/mirror/**` 下。

### Manual Verification

1. **未镜像状态校验**：
   - 打开任意未镜像过的插画，点击“更多”，选项中应显示“镜像”。
   - 后端 `GET /api/pixez/illusts/{id}/mirror` 返回 `data.mirrored=false`。
2. **点击镜像操作校验**：
   - 点击“镜像”按钮，前端只提交插画 ID。
   - 接口立即返回，SQLite `mirror_tasks` 中生成 `queued` 记录，此时不应发生 Pixiv 请求。
   - 后台 worker 消费后将任务置为 `processing`，随后请求 Pixiv 官方接口获取插画详情，并只下载 original 图片到 `./data/mirror/{illust_id}/`。
   - 详情获取成功后 SQLite `mirror_illust` 中写入详情记录；若至少一张图片下载成功，`mirror_tasks` 中对应任务状态更新为 `success`，否则更新为 `failed`。
3. **已镜像直接加载校验**：
   - 后端 `GET /api/pixez/illusts/{id}/mirror` 返回 `data.mirrored=true`。
   - 点击“已镜像”后，前端请求 `GET /mirror/v1/illust/detail?illust_id={id}`。
   - 页面应使用来自后端本地的数据刷新，图片展示正常。
