# PixEz Local Database Schema Reference

本文档详细描述了 PixEz Flutter 客户端中使用的所有本地 SQLite 数据库、数据表及其用途。

PixEz 并没有将所有数据集中在单个数据库文件中，而是按照业务模块将它们分散存在多个 `.db` 数据库文件中（默认存储在系统的数据库目录下）。

---

## PixezServer/Wavelet 数据库

PixezServer 后端使用 Wavelet 数据库体系，支持 PostgreSQL 与 SQLite。表结构通过 goose 双方言迁移维护：

- `PixezServer/internal/db/migrator/goose/postgres/`
- `PixezServer/internal/db/migrator/goose/sqlite/`

PixEz 迁移相关表：

| 表名 | 用途 |
| :--- | :--- |
| `pixiv_users` | Flutter 同步上来的 Pixiv 账号凭证与基础信息。 |
| `ban_comments` | 按 Pixiv 用户隔离的屏蔽评论同步表。 |
| `ban_illusts` | 按 Pixiv 用户隔离的屏蔽插画同步表。 |
| `ban_tags` | 按 Pixiv 用户隔离的屏蔽标签同步表。 |
| `ban_users` | 按 Pixiv 用户隔离的屏蔽用户同步表。 |
| `illust_histories` | 按 Pixiv 用户隔离的插画浏览历史同步表。 |
| `novel_histories` | 按 Pixiv 用户隔离的小说浏览历史同步表。 |
| `tag_histories` | 按 Pixiv 用户隔离的搜索标签历史同步表。 |
| `mirror_illust` | 插画镜像 read-model，保存 Pixiv 详情 JSON、任务状态、图片 Upload 映射和统计计数。 |
| `mirror_novel` | 小说镜像 read-model，保存 Pixiv 详情 JSON、正文 JSON、任务状态和统计计数。 |
| `bookmark_export_runs` | 插画/小说收藏导出运行记录。 |
| `bookmark_illusts` | 插画收藏导出 read-model，保留 removed 状态和镜像状态。 |
| `bookmark_novels` | 小说收藏导出 read-model，保留 removed 状态和镜像状态。 |
| `uploads` | Wavelet 通用文件表，PixEz 插画镜像文件通过该表登记本地/S3 存储。 |
| `task_executions` | Wavelet 异步任务执行历史、日志、重试和失败原因。 |

PixEz 当前不创建新的 `mirror_tasks` 队列表。旧 `server/pixez-sync.db` 中的 `mirror_tasks` 仅在 legacy 导入时作为历史状态参考。

`mirror_illust.image_files_json` 数组项格式：

```json
{
  "pixiv_url": "https://i.pximg.net/img-original/...",
  "page": 0,
  "upload_id": "123456789",
  "file_name": "123_p0.jpg",
  "hash": "...",
  "mime": "image/jpeg",
  "size": 12345,
  "storage_key": "uploads/2026/06/10/123456789.jpg"
}
```

---

## 数据库与表概览

目前系统中共有 **12** 个数据库文件，每个文件包含一个主要的业务表，具体对照如下：

| 数据库文件名 | 数据表名 | 用途说明 | 定义文件 |
| :--- | :--- | :--- | :--- |
| `account.db` | `account` | 存储已登录的 Pixiv 用户账号凭证与基础信息 | `lib/models/account.dart` |
| `banncommentid.db` | `banComment` | 存储被屏蔽/禁言的评论 ID 列表 | `lib/models/ban_comment_persist.dart` |
| `banillustid.db` | `banillustid` | 存储被屏蔽的插画 ID 列表 | `lib/models/ban_illust_id.dart` |
| `bantag.db` | `bantag` | 存储被屏蔽的标签列表（包含正则匹配规则） | `lib/models/ban_tag.dart` |
| `banuserid.db` | `banuserid` | 存储被屏蔽的画师/用户 ID 列表 | `lib/models/ban_user_id.dart` |
| `glanceillustpersist.db` | `glanceillustpersist` | 存储浏览过的插画历史记录（用于快速翻阅和临时缓存） | `lib/models/glance_illust_persist.dart` |
| `illustpersist.db` | `illustpersist` | 存储插画的历史记录或收藏记录 | `lib/models/illust_persist.dart` |
| `kvpair.db` | `kvpair` | 通用 Key-Value 缓存，支持配置过期时间（缓存 API 响应等）| `lib/models/key_value_pair.dart` |
| `Novelpersist.db` | `Novelpersist` | 存储小说的历史阅读记录 | `lib/models/novel_persist.dart` |
| `NovelViewerPersist.db` | `NovelViewerPersist` | 存储小说阅读进度（页内滚动偏移量 offset） | `lib/models/novel_viewer_persist.dart` |
| `tag.db` | `tag` | 存储搜索过的标签历史记录或常用标签及翻译 | `lib/models/tags.dart` |
| `task1.db` | `task` | 存储后台下载任务的进度、下载状态及文件路径 | `lib/models/task_persist.dart` |

---

## 数据表结构详情

### 1. `account` 表
*   **用途**：管理本地多账号登录凭证。
*   **表结构**：
    *   `id` (INTEGER, Primary Key, Autoincrement): 账户自增 ID。
    *   `access_token` (TEXT, Not Null): 访问令牌。
    *   `refresh_token` (TEXT, Not Null): 刷新令牌（用于刷新 access_token）。
    *   `device_token` (TEXT, Not Null): 设备推送令牌。
    *   `user_id` (TEXT, Not Null): Pixiv 用户唯一 ID。
    *   `user_image` (TEXT, Not Null): 用户头像 URL。
    *   `name` (TEXT, Not Null): 用户昵称。
    *   `password` (TEXT, Not Null): 密码记录（已不再使用，存为 "no more"）。
    *   `account` (TEXT, Not Null): 账号名称。
    *   `mail_address` (TEXT, Not Null): 注册邮箱。
    *   `is_premium` (INTEGER, Not Null): 是否是 Pixiv 会员 (0/1)。
    *   `x_restrict` (INTEGER, Not Null): 限制级别（如 R-18 限制）。
    *   `is_mail_authorized` (INTEGER, Not Null): 邮箱是否已验证 (0/1)。

### 2. `banComment` 表
*   **用途**：禁言屏蔽评论。
*   **表结构**：
    *   `id` (INTEGER, Primary Key, Autoincrement): 自增 ID。
    *   `comment_id` (TEXT, Not Null): 屏蔽的评论 ID。
    *   `name` (TEXT, Not Null): 发表该评论的用户昵称。

### 3. `banillustid` 表
*   **用途**：屏蔽特定插画。
*   **表结构**：
    *   `id` (INTEGER, Primary Key, Autoincrement): 自增 ID。
    *   `illust_id` (TEXT, Not Null): 屏蔽的插画唯一 ID。
    *   `name` (TEXT, Not Null): 插画标题。

### 4. `bantag` 表
*   **用途**：基于标签或正则表达式屏蔽插画。
*   **表结构**：
    *   `id` (INTEGER, Primary Key, Autoincrement): 自增 ID。
    *   `name` (TEXT, Not Null): 被屏蔽标签的原名（支持 `r'regex'` 格式正则过滤）。
    *   `translate_name` (TEXT, Not Null): 标签的翻译名。

### 5. `banuserid` 表
*   **用途**：屏蔽画师。
*   **表结构**：
    *   `id` (INTEGER, Primary Key, Autoincrement): 自增 ID。
    *   `user_id` (TEXT, Not Null): 屏蔽的画师 ID。
    *   `name` (TEXT, Not Null): 画师昵称。

### 6. `glanceillustpersist` 表
*   **用途**：记录历史浏览的插画数据，支持多种视图历史。
*   **表结构**：
    *   `id` (INTEGER, Primary Key, Autoincrement): 自增 ID。
    *   `illust_id` (INTEGER, Not Null): 插画 ID。
    *   `user_id` (INTEGER, Not Null): 作者 ID。
    *   `picture_url` (TEXT, Not Null): 插画缩略图 URL。
    *   `type` (TEXT, Not Null): 记录类型。
    *   `title` (TEXT): 插画标题。
    *   `large_url` (TEXT): 大图 URL。
    *   `original_url` (TEXT): 原图 URL。
    *   `user_name` (TEXT): 作者名称。
    *   `time` (INTEGER, Not Null): 浏览时间戳。

### 7. `illustpersist` 表
*   **用途**：记录常态化的插画查看历史或本地标记。
*   **表结构**：
    *   `id` (INTEGER, Primary Key, Autoincrement): 自增 ID。
    *   `illust_id` (INTEGER, Not Null): 插画 ID。
    *   `user_id` (INTEGER, Not Null): 作者 ID。
    *   `picture_url` (TEXT, Not Null): 图片 URL。
    *   `title` (TEXT): 标题。
    *   `user_name` (TEXT): 作者昵称。
    *   `time` (INTEGER, Not Null): 时间戳。

### 8. `kvpair` 表
*   **用途**：通用的键值对缓存数据库，多用于缓存网络 API 的 json 响应，节省流量。
*   **表结构**：
    *   `_id` (INTEGER, Primary Key, Autoincrement): 自增 ID。
    *   `key` (TEXT, Not Null): 缓存 key (唯一)。
    *   `value` (TEXT, Not Null): 缓存的内容 (通常为 JSON 字符串)。
    *   `expire_time` (INTEGER, Not Null): 过期截止时间戳。
    *   `date_time` (INTEGER, Not Null): 缓存写入时间戳。

### 9. `Novelpersist` 表
*   **用途**：小说历史阅读记录。
*   **表结构**：
    *   `id` (INTEGER, Primary Key, Autoincrement): 自增 ID。
    *   `novel_id` (INTEGER, Not Null): 小说 ID。
    *   `user_id` (INTEGER, Not Null): 作者 ID。
    *   `picture_url` (TEXT, Not Null): 小说封面 URL。
    *   `title` (TEXT, Not Null): 小说标题。
    *   `user_name` (TEXT, Not Null): 作者昵称。
    *   `time` (INTEGER, Not Null): 阅读时间戳。

### 10. `NovelViewerPersist` 表
*   **用途**：保存小说的具体阅读页进度，使得下次打开能接续阅读。
*   **表结构**：
    *   `id` (INTEGER, Primary Key, Autoincrement): 自增 ID。
    *   `novel_id` (INTEGER, Not Null): 小说 ID。
    *   `offset` (REAL, Not Null): 页面滚动的 offset 偏移数值。

### 11. `tag` 表
*   **用途**：历史搜索标签。
*   **表结构**：
    *   `_id` (INTEGER, Primary Key, Autoincrement): 自增 ID。
    *   `name` (TEXT, Not Null): 标签名。
    *   `translated_name` (TEXT, Not Null): 标签的翻译名称。
    *   `type` (INTEGER): 标签的类别/模式 (0 表示插画标签，1 表示小说标签)。

### 12. `task` 表
*   **用途**：管理后台文件/插画下载任务。
*   **表结构**：
    *   `id` (INTEGER, Primary Key, Autoincrement): 自增 ID。
    *   `title` (TEXT, Not Null): 下载作品标题。
    *   `user_name` (TEXT, Not Null): 作者昵称。
    *   `url` (TEXT, Not Null): 下载文件的源 URL。
    *   `sanity_level` (INTEGER): 健全度评级。
    *   `illust_id` (INTEGER, Not Null): 插画 ID。
    *   `user_id` (INTEGER, Not Null): 作者 ID。
    *   `status` (INTEGER, Not Null): 下载状态（如等待中、下载中、已完成、失败等）。
    *   `file_name` (TEXT, Not Null): 保存的文件名。
    *   `medium` (TEXT): 缩略图路径/URL。
