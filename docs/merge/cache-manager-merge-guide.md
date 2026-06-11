# 图片缓存分流代理合并与冲突解决指南 (2026-06-11)

## 改动背景
Android 端使用 `rhttp` (`rustls-platform-verifier 0.6.2`) 加载 2025 年 8 月后签发、且只有 CRL 没有 OCSP 地址的 Let's Encrypt 证书时，会触发证书吊销检查的误判缺陷（错误返回 `InvalidCertificate(Revoked)`）。这导致所有镜像图片（如 `/mirror/pximg`）加载失败。

## 为什么这么改
为了解决该证书校验缺陷而不全局关闭证书安全校验，我们将图片缓存管理器重构为按 URL 动态分流的代理：
* 镜像图片（`/mirror/pximg`）走 Flutter 默认缓存下载器（使用标准 Dart `HttpClient` 和系统 TLS 栈，避免 `rhttp` 证书验证缺陷）。
* 原始 Pixiv 图片继续使用 `rhttp` 网络模式下载。
* 全局共享一个代理实例 `PixivCacheManager.instance`，确保详情页、头像、缩放页和下载逻辑等所有组件的缓存行为一致。

## 改动点
1. **[NEW] [pixiv_cache_manager.dart](file:///Users/ryan/DEV/Flutter/pixez-flutter/lib/custom/services/pixiv_cache_manager.dart)**:
   新增一个集中管理的分流代理类，实现 `BaseCacheManager`，根据 URL 动态分发给默认 `HttpClient` 缓存或 `rhttp` 缓存。
2. **[DELETE] `lib/custom/services/mirror_image_cache_manager.dart`**:
   删除了旧的镜像缓存管理器。
3. **[MODIFY] [pixiv_image.dart (Material)](file:///Users/ryan/DEV/Flutter/pixez-flutter/lib/component/pixiv_image.dart)**:
   * 导入 `pixiv_cache_manager.dart`，替代旧文件。
   * 全局变量 `pixivCacheManager` 初始值改为 `PixivCacheManager.instance`。
   * `generatePixivCache` 中的缓存实例重新配置逻辑改为指向 `PixivCacheManager.instance`。
4. **[MODIFY] [pixiv_image.dart (Fluent)](file:///Users/ryan/DEV/Flutter/pixez-flutter/lib/fluent/component/pixiv_image.dart)**:
   * 进行与 Material 版本一致的修改（导入、全局变量和初始化配置）。
