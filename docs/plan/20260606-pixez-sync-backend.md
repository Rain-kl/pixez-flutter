# PixEz Sync Backend — Token 同步与远程登录

## 概述

本方案分为两阶段实现：
1. **第一阶段 (已完成)**：实现基本的 Token 同步与后端登录一键拉取。
2. **第二阶段 (新规划)**：将屏蔽表（评论、插画、标签、画师）与历史表（浏览、插画、小说、搜索标签）共计 8 张表纳入同步体系，归属于特定 Pixiv 用户 ID。在切换用户（或从后端登录）时，执行“先删后插”逻辑，自动备份当前用户数据并恢复新用户的数据。

---

## User Review Required

> [!IMPORTANT]
> **Go 后端项目路径**：计划在 `pixez-flutter` 仓库根目录下创建 `server/` 子目录存放 Go 后端代码。

> [!IMPORTANT]
> **安全考量**：后端仅通过环境变量指定的用户名/密码进行 Basic Authentication。
> 前端保存后端地址与密码到 SharedPreferences（非加密存储）。

> [!WARNING]
> **Token 敏感性**：同步到后端的 `access_token` 和 `refresh_token` 是 Pixiv 账号的完整凭据。
> 后端需要在部署时确保网络安全（建议 HTTPS + 局域网），数据库文件妥善保管。

---

## Proposed Changes

### Go 后端 (`server/`)

新增用户个性化业务数据的存储与同步 API。

#### [NEW] [migrations/20260606000002_create_user_data_tables.sql](file:///Users/ryan/DEV/Flutter/pixez-flutter/server/migrations/20260606000002_create_user_data_tables.sql)
创建以下 8 张业务数据表：
*   `ban_comments`
*   `ban_illusts`
*   `ban_tags`
*   `ban_users`
*   `glance_illusts`
*   `illust_histories`
*   `novel_histories`
*   `tag_histories`
每张表均包含 `pixiv_user_id` 列并对其创建索引。

#### [NEW] [model/sync_data.go](file:///Users/ryan/DEV/Flutter/pixez-flutter/server/model/sync_data.go)
定义与这 8 张表对应的 GORM 结构体以及接口的请求/响应 Bulk Payload 结构体。

#### [NEW] [handler/sync_data.go](file:///Users/ryan/DEV/Flutter/pixez-flutter/server/handler/sync_data.go)
实现两个主要端点：
*   `GetUserData(c *gin.Context)`: 查询此 `pixiv_user_id` 下所有 8 张表的数据并返回。
*   `PostUserData(c *gin.Context)`: 通过事务在 GORM 中，先 `DELETE` 对应 `pixiv_user_id` 的所有旧记录，然后 `INSERT` 接收到的所有新记录（先删后插）。

#### [MODIFY] [main.go](file:///Users/ryan/DEV/Flutter/pixez-flutter/server/main.go)
注册新的端点路由：
*   `GET` `/api/pixez/users/:pixiv_user_id/sync-data`
*   `POST` `/api/pixez/users/:pixiv_user_id/sync-data`

---

### Flutter 前端改造

#### [MODIFY] [sync_service.dart](file:///Users/ryan/DEV/Flutter/pixez-flutter/lib/custom/services/sync_service.dart)
增加以下两个网络及本地数据库交互方法：
*   `uploadUserData(String userId)`: 读取本地 8 张表（除 `task1.db`, `NovelViewerPersist.db`, `kvpair.db`, `account.db` 外）的所有记录，并以 JSON 格式上报到 `/api/pixez/users/$userId/sync-data`。
*   `downloadAndRestoreUserData(String userId)`: 从服务器下载最新的备份数据，并对本地 8 个 Provider 执行 `deleteAll()` 后逐条插入，恢复新用户数据（先删后插）。

#### [MODIFY] [account_select_page.dart](file:///Users/ryan/DEV/Flutter/pixez-flutter/lib/page/account/select/account_select_page.dart)
在 `onTap` 切换账户逻辑中：
1.  切换前：如果当前已有登录账号，异步调用 `SyncService.uploadUserData(currentUserId)` 上报当前用户的本地状态。
2.  执行 `accountStore.select(index)` 切换至新账户。
3.  切换后：如果新账号已成功选中，异步调用 `SyncService.downloadAndRestoreUserData(newUserId)` 清空本地并从云端同步新账号数据。

#### [MODIFY] [sync_login_page.dart](file:///Users/ryan/DEV/Flutter/pixez-flutter/lib/custom/pages/sync_login_page.dart)
在后端一键登录成功后，立刻调用 `SyncService.downloadAndRestoreUserData(newUserId)` 恢复新用户的云端数据。

#### [MODIFY] [sync_settings_page.dart](file:///Users/ryan/DEV/Flutter/pixez-flutter/lib/custom/pages/sync_settings_page.dart)
在“同步工具”卡片中，新增两个操作按钮以满足手动管控需求：
*   **手动备份本地数据**：调用 `SyncService.uploadUserData` 强制将本地现有数据保存到云端。
*   **从云端恢复本地数据**：调用 `SyncService.downloadAndRestoreUserData` 强制从云端覆盖到本地。

---

## Verification Plan

### Automated Tests
*   在后端添加对 `/api/pixez/users/:pixiv_user_id/sync-data` 接口的单元与集成测试。
*   运行 `go test ./...` 确保新数据表与同步端点正常。

### Manual Verification
1.  **用户数据覆盖同步验证**：
    *   在账户 A 下添加若干屏蔽标签和屏蔽画师。
    *   在“同步工具”中点击“手动备份本地数据”，或直接执行用户切换。
    *   检查后端 SQLite 数据库中，对应账户 A 的屏蔽标签、画师表是否录入相应数据。
2.  **切换恢复验证**：
    *   切换到账户 B，本地数据应被清空并拉取账户 B 的云端数据。
    *   再次切换回账户 A，验证账户 A 的屏蔽标签与画师是否成功通过“先删后插”恢复。

3.  **Flutter 编译验证**：
    ```bash
    flutter analyze
    ```
