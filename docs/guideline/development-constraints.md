# Development Constraints

本项目的核心开发约束、数据模型规范、API 设计准则及变更准入标准。

## API 设计准则

### 1. 统一接口响应规范
所有系统 API（除镜像代理 `/mirror` 接口外）必须统一使用以下响应格式包裹：
```json
{
  "success": true,
  "message": "",
  "data": {}
}
```

*   **`success`**: 布尔值，标识操作是否成功。
*   **`message`**: 字符串，用于携带自定义操作成功消息或失败的异常提示。
*   **`data`**: 任意类型，包含实际的业务负载（对象、数组、哈希等）。

### 2. 后端 Response 包装工具
后端统一采用 `pixez-sync/response` 封装的响应函数进行输出：
*   **成功并携带数据**: `response.RespondSuccess(c, data)`
*   **成功携带额外字段**: `response.RespondSuccessWithExtras(c, data, extras)`
*   **成功携带自定义消息**: `response.RespondSuccessMessage(c, message)`
*   **普通错误**: `response.RespondFailure(c, message)`
*   **参数错误 (400 Bad Request)**: `response.RespondBadRequest(c, message)`
*   **未授权 (401 Unauthorized)**: `response.RespondUnauthorized(c, message)`
*   **越权 (403 Forbidden)**: `response.RespondForbidden(c, message)`
*   **特定状态码错误**: `response.RespondErrorWithStatus(c, code, message)`
