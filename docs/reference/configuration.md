# Configuration Reference

本项目的配置项、环境变量参考手册。

## PixEz Sync Backend

| 环境变量 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `PIXEZ_AUTH_USER` | 无，必填 | 同步后端 HTTP Basic Authentication 用户名。 |
| `PIXEZ_AUTH_PASS` | 无，必填 | 同步后端 HTTP Basic Authentication 密码。 |
| `PIXEZ_DB_PATH` | `./pixez-sync.db` | 同步后端 SQLite 数据库路径。 |
| `PIXEZ_LISTEN_ADDR` | `:8080` | 同步后端监听地址。 |
| `PIXEZ_MIRROR_DIR` | `./data/mirror` | 插画镜像图片文件缓存根目录。 |
| `PIXEZ_MIRROR_DOWNLOAD_CONCURRENCY` | `5` | 每个插画镜像任务下载图片时的最大并发数。非法值或非正整数会回退到默认值。 |
| `PIXEZ_BOOKMARK_EXPORT_INTERVAL_HOURS` | `24` | 收藏导出后台任务执行间隔，单位小时。非法值或非正整数会回退到默认值。 |
