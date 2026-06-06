# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- Flutter 客户端同步整合 (`lib/custom/` & Pages):
  - 插画详情接口返回 `limit_unknown_360.png` 占位图时，自动切换到同步后端镜像详情加载。
- Go 同步后端服务 (`server/`):
  - 新增收藏导出后台任务，默认每日拉取用户公开收藏全量分页结果，增量 upsert 到数据库并将本轮缺失的历史收藏标记为已移除。
  - 新增 `bookmark_export_runs` 与 `bookmark_illusts` 数据表，保存每轮导出状态、统计信息以及每条收藏插画的完整 Pixiv JSON。
  - 新增 `PIXEZ_BOOKMARK_EXPORT_INTERVAL_HOURS` 配置项，用于调整收藏导出后台任务执行间隔。
  - 新增 `PixivUtils` 服务封装，统一 Pixiv 官方接口请求头、Authorization、token 刷新、响应模型解析与图片下载请求。
