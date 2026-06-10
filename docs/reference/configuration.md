# Configuration Reference

本项目的配置项、环境变量参考手册。

## PixezServer/Wavelet

当前 PixEz Sync 后端位于 `PixezServer/`，复用 Wavelet 配置体系。Flutter custom 层不再填写 Basic Auth 用户名/密码，而是在 PixezServer Web 管理端创建 AccessToken 后填入同步设置页。

### 客户端同步配置

| 配置项 | 说明 |
| :--- | :--- |
| 同步服务器地址 | PixezServer 对外地址，例如 `https://example.com` 或 `http://127.0.0.1:8000`。 |
| AccessToken | Wavelet 用户 AccessToken。请求头使用 `Authorization: Bearer <access_token>`。 |
| 自动同步开关 | 控制 Flutter custom 同步服务是否启用。 |
| 小说自动镜像开关 | 控制打开小说详情时是否自动调用 `/api/pixez/novels/:novel_id/mirror`。 |

### PixezServer 常用配置

配置文件示例见 `PixezServer/config.example.yaml`，也可通过环境变量覆盖。

| 配置/环境变量 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `app.addr` / `APP_ADDR` | `:8000` | PixezServer HTTP 监听地址。 |
| `app.api_prefix` / `APP_API_PREFIX` | `/api` | Wavelet API 前缀；PixEz 业务路径挂载为 `/api/pixez/**`。 |
| `app.session_secret` | 无，部署必填 | Wavelet session 加密密钥。 |
| `database.enabled` / `DB_ENABLED` | `true` | 是否启用 PostgreSQL；关闭时使用 SQLite。 |
| `database.sqlite_path` / `SQLITE_PATH` | `wavelet.db` | SQLite 数据库路径。 |
| `DB_HOST` / `DB_PORT` / `DB_USERNAME` / `DB_PASSWORD` / `DB_NAME` | 见示例配置 | PostgreSQL 连接配置。 |
| `redis.addrs` / `REDIS_ADDR` | `127.0.0.1:6379` | Redis 地址，Asynq 任务队列依赖 Redis。 |
| `redis.key_prefix` / `REDIS_KEY_PREFIX` | `wavelet:` | Redis key 前缀。 |
| `worker.concurrency` | `20` | Asynq worker 并发数。 |
| `worker.queues` | `default` 等 | Worker 队列与优先级。PixEz 任务当前使用 `default` 队列。 |
| `s3.enabled` / `S3_ENABLED` | `false` | 是否启用 S3 兼容对象存储。 |
| `S3_ENDPOINT` / `S3_REGION` / `S3_BUCKET` / `S3_ACCESS_KEY_ID` / `S3_SECRET_ACCESS_KEY` | 见示例配置 | S3/R2/MinIO 等对象存储参数。 |
| `s3.local_cache.enabled` | `false` | S3 读取本地缓存开关，影响 `/mirror/pximg/*path` 流式读取。 |

### PixEz 后台任务

以下任务通过 Wavelet Admin 任务页下发，不需要单独环境变量：

- `pixez:mirror_illust`
- `pixez:mirror_novel`
- `pixez:export_bookmark_illusts`
- `pixez:export_bookmark_novels`
- `pixez:auto_enqueue_bookmark_mirrors`
- `pixez:import_legacy_server`

Legacy 导入任务默认 payload：

```json
{
  "sqlite_path": "server/pixez-sync.db",
  "mirror_dir": "server/data/mirror",
  "dry_run": false
}
```

### Legacy server 变量

以下变量只适用于旧 `server/` 后端，不被 PixezServer/Wavelet 当前实现读取：

| Legacy 环境变量 | 说明 |
| :--- | :--- |
| `PIXEZ_AUTH_USER` | 旧后端 HTTP Basic Authentication 用户名。 |
| `PIXEZ_AUTH_PASS` | 旧后端 HTTP Basic Authentication 密码。 |
| `PIXEZ_DB_PATH` | 旧后端 SQLite 数据库路径。 |
| `PIXEZ_LISTEN_ADDR` | 旧后端监听地址。 |
| `PIXEZ_MIRROR_DIR` | 旧后端插画镜像文件缓存根目录。 |
| `PIXEZ_MIRROR_DOWNLOAD_CONCURRENCY` | 旧后端镜像下载并发数。 |
| `PIXEZ_BOOKMARK_EXPORT_INTERVAL_HOURS` | 旧后端收藏导出轮询间隔。 |
