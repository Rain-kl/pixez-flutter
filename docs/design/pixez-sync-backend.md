# PixEz Sync Backend 设计文档

本文档描述 PixEz Sync Backend 的系统设计、核心数据模型、API 设计以及部署方案。

## 1. 概述与产品范围

PixEz Sync Backend 是专门为 PixEz Flutter 客户端设计的一个轻量级伴生后端服务，主要用于：
1. 在多设备间同步 Pixiv 登录凭证（Tokens）与 7 张本地核心业务表数据。
2. 支持“通过后端保存的凭证一键登录”的能力。
3. 提供“插画镜像加速缓存与加载 (Illustration Mirroring & Proxy)”功能，通过在本地磁盘缓存插画详情与图片流，动态重写域名，从而实现客户端无需代理的加速体验。
4. 提供“小说镜像缓存”功能，通过通用镜像任务队列缓存小说详情与正文，客户端可在打开小说详情时自动入队。
5. 提供“收藏导出与失效追踪”功能，由服务端每日拉取 Pixiv 用户收藏，增量写入数据库并标记从 Pixiv 收藏夹中消失的插画与小说。

后端服务仅提供 API 接口，不提供独立的前端管理界面。

### 系统边界

```text
+-----------------------+                    +-------------------------+
|                       |  /api/pixez/*      |                         |
|  PixEz Flutter Client | <================> |  PixEz Sync Server      |
|                       |  Basic Auth        |  (Go / Gin / SQLite)    |
|                       |                    +------------+------------+
+-----------------------+                                 |
                                                          | GORM
                                                          v
                                                     +----+----+
                                                     | SQLite  |
                                                     |  (DB)   |
                                                     +---------+
```

---

## 2. 架构设计

服务采用 Go 语言编写，使用三层经典架构：
*   **路由/控制层 (Gin)**: 负责路由解析、Basic Auth 鉴权及请求校验。
*   **业务逻辑/数据访问层 (GORM)**: 负责对 SQLite 数据库进行读写。
*   **数据库迁移 (goose)**: 负责版本化的数据库 Schema 演进。

### 技术栈选型
*   **语言**: Go 1.25+
*   **Web 框架**: Gin
*   **ORM**: GORM
*   **数据库**: SQLite
*   **迁移工具**: goose
*   **部署**: Docker & docker-compose

---

## 3. 数据模型设计

服务只有一个核心业务表 `pixiv_users`。由于 Pixiv 用户的 `access_token` 可能会刷新，我们使用 Pixiv 用户 ID（画师 ID）作为唯一主键。

### 表结构: `pixiv_users`

| 字段名 | 类型 | 约束 | 说明 |
| :--- | :--- | :--- | :--- |
| `pixiv_user_id` | `TEXT` | PRIMARY KEY | Pixiv 用户的画师 ID |
| `name` | `TEXT` | NOT NULL | 用户昵称 |
| `account` | `TEXT` | NOT NULL | 用户账号/画师账号名 |
| `mail_address`| `TEXT` | | 注册邮箱 |
| `user_image` | `TEXT` | | 头像 URL |
| `access_token`| `TEXT` | NOT NULL | Pixiv 访问令牌 |
| `refresh_token`| `TEXT` | NOT NULL | Pixiv 刷新令牌 |
| `device_token`| `TEXT` | | 设备令牌 |
| `is_premium` | `INTEGER`| DEFAULT 0 | 是否 Pixiv 会员 (0/1) |
| `x_restrict` | `INTEGER`| DEFAULT 0 | 限制级别 |
| `is_mail_authorized` | `INTEGER`| DEFAULT 0 | 邮箱是否验证 (0/1) |
| `created_at` | `DATETIME`| | 创建时间 |
| `updated_at` | `DATETIME`| | 更新时间 |

### 同步业务表结构 (User Synced Data)

除 `pixiv_users` 外，以下 7 张业务表被纳入同步体系，每行数据通过 `pixiv_user_id` 与特定的用户账号进行关联：

1.  **`ban_comments` (对应本地 `banComment`)**
    *   `id` (INTEGER AUTOINCREMENT PRIMARY KEY)
    *   `pixiv_user_id` (TEXT, Index)
    *   `comment_id` (TEXT)
    *   `name` (TEXT)
2.  **`ban_illusts` (对应本地 `banillustid`)**
    *   `id` (INTEGER AUTOINCREMENT PRIMARY KEY)
    *   `pixiv_user_id` (TEXT, Index)
    *   `illust_id` (TEXT)
    *   `name` (TEXT)
3.  **`ban_tags` (对应本地 `bantag`)**
    *   `id` (INTEGER AUTOINCREMENT PRIMARY KEY)
    *   `pixiv_user_id` (TEXT, Index)
    *   `name` (TEXT)
    *   `translate_name` (TEXT)
4.  **`ban_users` (对应本地 `banuserid`)**
    *   `id` (INTEGER AUTOINCREMENT PRIMARY KEY)
    *   `pixiv_user_id` (TEXT, Index)
    *   `user_id` (TEXT)
    *   `name` (TEXT)
5.  **`illust_histories` (对应本地 `illustpersist`)**
    *   `id` (INTEGER AUTOINCREMENT PRIMARY KEY)
    *   `pixiv_user_id` (TEXT, Index)
    *   `illust_id` (INTEGER)
    *   `user_id` (INTEGER)
    *   `picture_url` (TEXT)
    *   `title` (TEXT)
    *   `user_name` (TEXT)
    *   `time` (INTEGER)
6.  **`novel_histories` (对应本地 `Novelpersist`)**
    *   `id` (INTEGER AUTOINCREMENT PRIMARY KEY)
    *   `pixiv_user_id` (TEXT, Index)
    *   `novel_id` (INTEGER)
    *   `user_id` (INTEGER)
    *   `picture_url` (TEXT)
    *   `title` (TEXT)
    *   `user_name` (TEXT)
    *   `time` (INTEGER)
7.  **`tag_histories` (对应本地 `tag`)**
    *   `id` (INTEGER AUTOINCREMENT PRIMARY KEY)
    *   `pixiv_user_id` (TEXT, Index)
    *   `name` (TEXT)
    *   `translated_name` (TEXT)
    *   `type` (INTEGER)

### 镜像任务队列与插画缓存存储结构

插画镜像加速功能由 SQLite 与本地文件树共同承载：

*   **SQLite 通用镜像任务表**: `mirror_tasks` 表充当镜像任务队列。该表不仅服务插画镜像，也服务后续小说镜像。任务使用独立任务 ID 作为主键，目标资源通过 `target_type` 与 `target_id` 表达。客户端发起镜像请求时只写入基础字段并立即返回，不访问 Pixiv。后台 worker 领取任务后才请求 Pixiv、下载资源并更新任务状态，同时记录请求 URL 总列表、成功数、失败数和失败 URL 列表。
*   **SQLite 元数据镜像表**: `mirror_illust` 表记录已经取得的 Pixiv 插画详情元数据，包括 Pixiv 原始详情 JSON 与已成功下载的图片文件清单。即使部分或全部图片下载失败，只要 Pixiv 详情获取成功，也应保留该表记录，便于追踪和后续重试。
*   **本地图片文件**: 仅下载并缓存 Pixiv original 档图片，文件写入 `{MirrorDir}/{illust_id}/`，以 original URL 的原始文件名保存。前端请求 master 或 square 档图片时，后端会将文件名映射到同页 original 文件返回；若 original 文件缺失则返回 404。

检查某个插画是否已镜像完成时，必须查询 SQLite 的 `mirror_tasks.success_count`。只要同一插画镜像任务已有成功下载图片（`success_count > 0`），即可视为镜像可用；失败图片请求 `/mirror/pximg/*path` 时按本地文件缺失返回 404。`mirror_illust` 仅表示详情元数据可读取，不单独代表图片镜像完成。

小说镜像复用同一 `mirror_tasks` 队列，使用 `task_type=novel_mirror`、`target_type=novel`。小说没有图片资源下载，worker 处理时读取 Pixiv 小说详情与正文，并写入 `mirror_novel.detail_json` 与 `mirror_novel.text_json`。当正文与详情成功入库后，将对应任务的 `success_count` 置为 `1`，客户端即可通过镜像读取接口加载小说内容。Flutter 客户端可以在“数据同步设置”中控制是否自动镜像小说；该开关仅影响客户端打开小说详情时是否自动调用入队接口，不改变后端 API 的幂等队列语义。

建议新增 `mirror_tasks` 表：

| 字段名 | 类型 | 约束 | 说明 |
| :--- | :--- | :--- | :--- |
| `id` | `TEXT` | PRIMARY KEY | 任务 ID，建议使用 UUID/ULID |
| `task_type` | `TEXT` | NOT NULL | 任务类型，例如 `illust_mirror` / `novel_mirror` |
| `target_type` | `TEXT` | NOT NULL | 目标资源类型，例如 `illust` / `novel` |
| `target_id` | `INTEGER` | NOT NULL | Pixiv 目标资源 ID，例如插画 ID 或小说 ID |
| `status` | `TEXT` | NOT NULL | `queued` / `processing` / `success` / `failed` |
| `request_payload_json` | `TEXT` | | 入队时保存的最小请求上下文 JSON |
| `request_urls_json` | `TEXT` | | worker 从 Pixiv 详情中提取出的 original 档图片 URL JSON 数组，不记录 master/square URL |
| `retry_urls_json` | `TEXT` | | 下载失败的 original 档图片 URL JSON 数组，用于后续重试，不记录 master/square URL |
| `error_message` | `TEXT` | | 最近一次失败详情，必须包含原始请求、响应状态、响应内容与错误摘要 |
| `total_count` | `INTEGER` | NOT NULL DEFAULT 0 | 本次任务请求下载的图片 URL 总数 |
| `success_count` | `INTEGER` | NOT NULL DEFAULT 0 | 本次任务成功下载的图片数量，`> 0` 即可视为镜像可用 |
| `failed_count` | `INTEGER` | NOT NULL DEFAULT 0 | 本次任务下载失败的图片数量 |
| `attempt_count` | `INTEGER` | NOT NULL DEFAULT 0 | 已处理次数 |
| `locked_at` | `DATETIME` | | worker 领取任务时间，用于处理中任务恢复 |
| `started_at` | `DATETIME` | | 开始处理时间 |
| `finished_at` | `DATETIME` | | 完成时间 |
| `created_at` | `DATETIME` | | 创建时间 |
| `updated_at` | `DATETIME` | | 更新时间 |

建议增加唯一约束：`UNIQUE(task_type, target_type, target_id)`，防止同一资源的同类镜像任务重复入队，同时允许不同任务类型或不同资源类型复用同一通用任务表。

建议新增 `mirror_illust` 表：

| 字段名 | 类型 | 约束 | 说明 |
| :--- | :--- | :--- | :--- |
| `illust_id` | `INTEGER` | PRIMARY KEY | Pixiv 插画 ID |
| `detail_json` | `TEXT` | NOT NULL | Pixiv 官方 `v1/illust/detail` 原始响应 JSON |
| `image_files_json` | `TEXT` | NOT NULL | 已成功下载图片文件清单 JSON |
| `created_at` | `DATETIME` | | 创建时间 |
| `updated_at` | `DATETIME` | | 更新时间 |

### 收藏导出与失效追踪存储结构

收藏导出功能由服务端后台 worker 完成，目标是解决 Pixiv 在作品失效、作者删除或不可见时可能直接从用户收藏夹移除且不通知用户的问题。后端每日对已保存 Pixiv 凭证的用户执行一次收藏导出，覆盖公开与非公开范围，并分别处理插画与小说：

1. 插画使用 Pixiv 官方 `GET /v1/user/bookmarks/illust?user_id={pixiv_user_id}&restrict={restrict}` 作为首个请求；小说使用 `GET /v1/user/bookmarks/novel?user_id={pixiv_user_id}&restrict={restrict}` 作为首个请求。
2. 按响应中的 `next_url` 逐页拉取，直到 `next_url` 为空。
3. 每个收藏作品解析为服务端 Pixiv 响应模型，并将完整作品 JSON 写入数据库。
4. 写入策略必须是增量 upsert，禁止“先删后查”。这样即使某个作品已从 Pixiv 收藏页消失，历史记录仍保留。
5. 若本轮返回的作品 ID 已存在且未标记失效，直接跳过数据库更新，不刷新 JSON、标题、统计数等字段，避免全量导出产生大量无意义更新。
6. 若本轮返回的收藏插画图片 URL 包含 `limit_unknown_360` 占位图，将该插画标记为 `removed=true`，并且不把它视为本轮有效出现。
7. 若本轮返回的收藏小说封面 URL 包含 `limit_unknown_100` 占位图，将该小说标记为 `removed=true`，并且不把它视为本轮有效出现。
8. 单次完整导出成功结束后，将同一用户同一 `restrict` 下“本轮未出现但历史存在且尚未移除”的记录标记为 `removed=true`，并写入 `removed_at`。
9. 若导出中途失败，不执行缺失标记，避免分页中断导致误判。

客户端可通过 `GET /api/pixez/users/{pixiv_user_id}/bookmarks/illust/removed` 查询已标记失效的收藏插画。接口返回标准系统响应包裹，`data` 内保持 Pixiv 列表兼容结构：

```json
{
  "illusts": [],
  "next_url": "/api/pixez/users/123/bookmarks/illust/removed?offset=30&limit=30"
}
```

该接口默认合并公开与非公开收藏中的失效记录，支持可选 `restrict=public|private`、`offset`、`limit` 参数；`limit` 默认 30，最大 100。

收藏导出定时任务提供以下 API 端点：

| 方法 | 路径 | 说明 |
| :--- | :--- | :--- |
| `GET` | `/api/pixez/scheduled-tasks/bookmark-export` | 查询收藏导出定时任务状态 |
| `POST` | `/api/pixez/scheduled-tasks/bookmark-export/run` | 立即异步触发一次收藏导出任务；若已有任务运行中返回 `409` |
| `GET` | `/api/pixez/scheduled-tasks/novel-bookmark-export` | 查询小说收藏导出定时任务状态 |
| `POST` | `/api/pixez/scheduled-tasks/novel-bookmark-export/run` | 立即异步触发一次小说收藏导出任务；若已有任务运行中返回 `409` |

所有请求 Pixiv 官方接口的代码统一封装在服务层 `PixivUtils` 中。该工具类统一构造 Pixiv App 请求头、处理 Authorization、刷新 token、解析响应模型，并使用以下 User-Agent：

```text
PixivAndroidApp/5.0.166 (Android 16; PKX110)
```

建议新增 `bookmark_export_runs` 表：

| 字段名 | 类型 | 约束 | 说明 |
| :--- | :--- | :--- | :--- |
| `id` | `TEXT` | PRIMARY KEY | 单次导出运行 ID |
| `pixiv_user_id` | `TEXT` | NOT NULL, INDEX | 被导出的 Pixiv 用户 ID |
| `restrict` | `TEXT` | NOT NULL, INDEX | 收藏范围，当前为 `public` / `private` |
| `status` | `TEXT` | NOT NULL, INDEX | `running` / `success` / `failed` |
| `total_count` | `INTEGER` | NOT NULL DEFAULT 0 | 本轮成功解析的收藏插画数量 |
| `new_count` | `INTEGER` | NOT NULL DEFAULT 0 | 本轮新插入数量 |
| `updated_count` | `INTEGER` | NOT NULL DEFAULT 0 | 本轮更新数量 |
| `removed_count` | `INTEGER` | NOT NULL DEFAULT 0 | 本轮标记为已移除的数量 |
| `error_message` | `TEXT` | | 导出失败原因 |
| `started_at` | `DATETIME` | NOT NULL | 开始时间 |
| `finished_at` | `DATETIME` | | 完成时间 |
| `next_url` | `TEXT` | | 最近一次响应中的下一页 URL |
| `last_request_url` | `TEXT` | | 最近一次请求 URL |
| `created_at` | `DATETIME` | | 创建时间 |
| `updated_at` | `DATETIME` | | 更新时间 |

建议新增 `bookmark_illusts` 表：

| 字段名 | 类型 | 约束 | 说明 |
| :--- | :--- | :--- | :--- |
| `id` | `INTEGER` | PRIMARY KEY AUTOINCREMENT | 本地行 ID |
| `pixiv_user_id` | `TEXT` | NOT NULL | 收藏所属 Pixiv 用户 ID |
| `restrict` | `TEXT` | NOT NULL | 收藏范围 |
| `illust_id` | `INTEGER` | NOT NULL | Pixiv 插画 ID |
| `title` | `TEXT` | | 标题冗余字段，便于查询 |
| `type` | `TEXT` | | `illust` / `manga` / `ugoira` |
| `user_id` | `INTEGER` | | 作者 Pixiv 用户 ID |
| `user_name` | `TEXT` | | 作者名称 |
| `page_count` | `INTEGER` | | 页数 |
| `width` / `height` | `INTEGER` | | 尺寸 |
| `sanity_level` | `INTEGER` | | Pixiv 内容级别 |
| `x_restrict` | `INTEGER` | | R-18 等限制标识 |
| `total_view` | `INTEGER` | | 浏览数 |
| `total_bookmarks` | `INTEGER` | | 收藏数 |
| `visible` | `INTEGER` | NOT NULL DEFAULT 0 | Pixiv 响应中的可见性 |
| `is_muted` | `INTEGER` | NOT NULL DEFAULT 0 | 是否被屏蔽 |
| `illust_ai_type` | `INTEGER` | | AI 类型 |
| `illust_json` | `TEXT` | NOT NULL | Pixiv 返回的完整插画 JSON，保留全部字段 |
| `last_export_run_id` | `TEXT` | NOT NULL, INDEX | 最近一次看到该插画的导出运行 ID |
| `last_seen_at` | `DATETIME` | NOT NULL | 最近一次在收藏页出现的时间 |
| `removed` | `INTEGER` | NOT NULL DEFAULT 0, INDEX | 是否已从收藏导出结果中消失 |
| `removed_at` | `DATETIME` | | 首次标记移除时间 |
| `created_at` | `DATETIME` | | 创建时间 |
| `updated_at` | `DATETIME` | | 更新时间 |

`bookmark_illusts` 必须具备唯一约束 `UNIQUE(pixiv_user_id, restrict, illust_id)`，以支持跨多次导出的稳定 upsert。

---

## 4. API 接口设计

所有 PixEz Sync Backend 业务接口都在 `/api/pixez/*` 命名空间下，且必须通过 HTTP Basic Authentication 鉴权。用户名和密码由服务启动时的环境变量 `PIXEZ_AUTH_USER` 和 `PIXEZ_AUTH_PASS` 指定。

`/mirror/**` 是 Pixiv 官方接口的镜像对接命名空间，只允许承载与 Pixiv 官方请求语义一致的镜像接口。例如 `GET /mirror/v1/illust/detail` 对齐 Pixiv `GET /v1/illust/detail`。任何“请求镜像”“检查镜像”等 PixEz Sync Backend 自身业务操作都不得注册到 `/mirror/**` 下。

服务启动后同时提供 Swagger/OpenAPI 文档：

*   **Swagger UI**: `GET /swagger/index.html`
*   **OpenAPI JSON**: `GET /swagger/doc.json`

Swagger 文档由 `swaggo/swag` 根据 Go 注释生成，生成产物位于 `server/docs/`。修改接口注释或对外模型后，应在 `server/` 目录执行 `make swagger` 更新文档。Swagger UI 不额外挂鉴权中间件，但业务接口本身仍按各自路由要求使用 HTTP Basic Authentication。

### 接口列表

| 请求方法 | 接口路径 | 说明 |
| :--- | :--- | :--- |
| `GET` | `/api/pixez/ping` | 健康检查 & 鉴权验证 |
| `GET` | `/api/pixez/users` | 获取所有已同步的 Pixiv 用户列表（只包含基本信息，不返回敏感 tokens） |
| `GET` | `/api/pixez/users/:pixiv_user_id` | 获取特定 Pixiv 用户的完整凭证信息（包含 tokens） |
| `PUT` | `/api/pixez/users/:pixiv_user_id` | 增量/全量上传或更新某个 Pixiv 用户的 Token 信息（Upsert） |
| `DELETE` | `/api/pixez/users/:pixiv_user_id` | 从后端删除某个 Pixiv 用户的凭证记录（连带删除同步的数据） |
| `GET` | `/api/pixez/users/:pixiv_user_id/sync-data` | 下载此用户下的所有备份表数据 |
| `POST` | `/api/pixez/users/:pixiv_user_id/sync-data` | 覆盖上报此用户下的所有备份表数据（先删后插） |
| `POST` | `/api/pixez/illusts/:illust_id/mirror` | 将插画镜像任务写入 SQLite 队列；入参只有插画 ID，不阻塞等待下载 |
| `GET` | `/api/pixez/illusts/:illust_id/mirror` | 查询 SQLite 元数据镜像记录和任务状态，检测该插画是否已完成镜像缓存 |
| `GET` | `/mirror/v1/illust/detail` | Pixiv 官方插画详情接口的镜像读取版本，返回 Pixiv 响应形态并动态改写图片 URL |
| `GET` | `/mirror/pximg/*path` | Pixiv 图片资源镜像，从本地磁盘查找并读取插画图片输出二进制流 |

### 接口定义详情

#### 1. 健康检查 `GET /api/pixez/ping`
*   **请求头**: `Authorization: Basic <credentials>`
*   **响应 (200 OK)**:
    ```json
    {
      "success": true,
      "message": "",
      "data": {
        "status": "ok"
      }
    }
    ```

#### 2. 获取用户列表 `GET /api/pixez/users`
*   **请求头**: `Authorization: Basic <credentials>`
*   **响应 (200 OK)**:
    ```json
    {
      "success": true,
      "message": "",
      "data": [
        {
          "pixiv_user_id": "12345678",
          "name": "PixivUser",
          "account": "user_account",
          "mail_address": "user@example.com",
          "user_image": "https://pximg.net/...",
          "is_premium": 0,
          "x_restrict": 0,
          "is_mail_authorized": 1,
          "created_at": "2026-06-06T16:00:00Z",
          "updated_at": "2026-06-06T16:15:00Z"
        }
      ]
    }
    ```

#### 3. 获取特定用户凭证 `GET /api/pixez/users/:pixiv_user_id`
*   **请求头**: `Authorization: Basic <credentials>`
*   **响应 (200 OK)**:
    ```json
    {
      "success": true,
      "message": "",
      "data": {
        "pixiv_user_id": "12345678",
        "name": "PixivUser",
        "account": "user_account",
        "mail_address": "user@example.com",
        "user_image": "https://pximg.net/...",
        "access_token": "pixiv_access_token_value",
        "refresh_token": "pixiv_refresh_token_value",
        "device_token": "pixiv_device_token_value",
        "is_premium": 0,
        "x_restrict": 0,
        "is_mail_authorized": 1,
        "created_at": "2026-06-06T16:00:00Z",
        "updated_at": "2026-06-06T16:15:00Z"
      }
    }
    ```

#### 4. 上报/更新用户凭证 `PUT /api/pixez/users/:pixiv_user_id`
*   **请求头**: `Authorization: Basic <credentials>`, `Content-Type: application/json`
*   **请求体**:
    ```json
    {
      "name": "PixivUser",
      "account": "user_account",
      "mail_address": "user@example.com",
      "user_image": "https://pximg.net/...",
      "access_token": "pixiv_access_token_value",
      "refresh_token": "pixiv_refresh_token_value",
      "device_token": "pixiv_device_token_value",
      "is_premium": 0,
      "x_restrict": 0,
      "is_mail_authorized": 1
    }
    ```
*   **响应 (200 OK)**:
    ```json
    {
      "success": true,
      "message": "updated successfully"
    }
    ```

#### 5. 删除用户凭证 `DELETE /api/pixez/users/:pixiv_user_id`
*   **请求头**: `Authorization: Basic <credentials>`
*   **响应 (200 OK)**:
    ```json
    {
      "success": true,
      "message": "deleted successfully"
    }
    ```

#### 6. 请求镜像插画 `POST /api/pixez/illusts/:illust_id/mirror`
*   **请求头**: `Authorization: Basic <credentials>`
*   **请求参数**: 仅路径参数 `illust_id`。客户端不上传插画详情 JSON、图片 URL 列表或 Pixiv 用户 ID。
*   **后端行为**: 只将任务写入 SQLite 的 `mirror_tasks` 队列并立即返回。新任务生成独立 `id`，设置 `task_type=illust_mirror`、`target_type=illust`、`target_id={illust_id}`、`status=queued`。该接口不访问 Pixiv 服务器，不下载图片，不阻塞等待镜像完成。
*   **响应 (200 OK)**:
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

#### 7. 检查镜像插画 `GET /api/pixez/illusts/:illust_id/mirror`
*   **请求头**: `Authorization: Basic <credentials>`
*   **后端行为**: 查询 `mirror_tasks` 中 `task_type=illust_mirror AND target_type=illust AND target_id={illust_id}` 的任务，返回当前任务 ID、任务状态、请求总数、成功数、失败数、请求 URL 列表与失败 URL 列表。`mirrored` 由 `success_count > 0` 决定。该接口不访问 Pixiv 官方接口，不扫描图片目录作为主要判断依据。
*   **响应 (200 OK)**:
    ```json
    {
      "success": true,
      "message": "",
      "data": {
        "task_id": "01J...",
        "illust_id": 123,
        "status": "processing",
        "mirrored": false,
        "total_count": 40,
        "success_count": 0,
        "failed_count": 0,
        "request_urls_json": "",
        "retry_urls_json": ""
      }
    }
    ```
*   **成功镜像响应示例 (200 OK)**:
    ```json
    {
      "success": true,
      "message": "",
      "data": {
        "task_id": "01J...",
        "illust_id": 123,
        "status": "success",
        "mirrored": true,
        "total_count": 40,
        "success_count": 28,
        "failed_count": 12,
        "request_urls_json": "[\"https://i.pximg.net/...\"]",
        "retry_urls_json": "[\"https://i.pximg.net/...\"]"
      }
    }
    ```

#### 8. 获取镜像插画详情 `GET /mirror/v1/illust/detail`
*   **请求头**: `Authorization: Basic <credentials>`
*   **请求参数**: `illust_id`
*   **接口语义**: 对齐 Pixiv 官方 `GET /v1/illust/detail`。该接口只读取已缓存镜像，不负责创建镜像或检查镜像状态。
*   **后端行为**: 只从 `mirror_illust.detail_json` 读取本地缓存详情。即使本地未命中，也不得在该接口内发起 Pixiv 官方请求；通用任务表 `mirror_tasks` 不作为详情读取来源。
*   **响应 (200 OK)**: 返回 Pixiv 官方响应形态。响应不使用 PixEz Sync Backend 的 `success/message/data` 包装，但会将详情 JSON 内图片 URL 改写为当前同步服务器的 `/mirror/pximg/*` 地址。

#### 9. 获取镜像图片 `GET /mirror/pximg/*path`
*   **请求头**: `Authorization: Basic <credentials>`
*   **接口语义**: 对齐 Pixiv 图片资源访问。该接口根据被重写后的图片路径读取本地文件并返回二进制图片流。若请求文件名带 `_master...` 或 `_square...` 后缀，后端会映射到同页 original 文件名（例如 `123_p0_master1200.jpg` 映射为 `123_p0.jpg`）；本地 original 文件不存在时返回 404。

---

## 5. 部署与配置设计

### 环境变量

| 变量名 | 是否必填 | 默认值 | 说明 |
| :--- | :--- | :--- | :--- |
| `PIXEZ_AUTH_USER` | ✅ 是 | — | Basic Auth 用户名 |
| `PIXEZ_AUTH_PASS` | ✅ 是 | — | Basic Auth 密码 |
| `PIXEZ_DB_PATH` | ❌ 否 | `/app/data/pixez-sync.db` | SQLite 数据库文件的绝对或相对路径 |
| `PIXEZ_LISTEN_ADDR`| ❌ 否 | `:8080` | 服务的监听端口和地址 |
| `PIXEZ_MIRROR_DIR` | ❌ 否 | `./data/mirror` | 镜像插画及图片本地缓存的根目录 |
| `PIXEZ_MIRROR_DOWNLOAD_CONCURRENCY` | ❌ 否 | `5` | 每个镜像任务下载图片时的最大并发数，非法值回退到默认值 |
| `PIXEZ_BOOKMARK_EXPORT_INTERVAL_HOURS` | ❌ 否 | `24` | 收藏导出后台任务执行间隔，单位小时，非法值回退到默认值 |

### Docker 支持
后端会提供 `Dockerfile` 和 `docker-compose.yml`，以容器化方式简化部署。
在容器化部署时，SQLite 数据库文件会被存储在挂载卷中（例如映射到宿主机的 `./data` 目录下），以确保数据持久化。
