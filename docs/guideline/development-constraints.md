# Development Constraints

本项目的核心开发约束、数据模型规范、API 设计准则及变更准入标准。

## API 设计准则

### 1. 统一接口响应规范
PixezServer/Wavelet 当前系统 API（除镜像代理 `/mirror` 接口外）必须统一使用以下响应格式包裹：
```json
{
  "error_msg": "",
  "data": {}
}
```

*   **`error_msg`**: 空字符串表示成功，非空字符串表示错误提示。
*   **`data`**: 任意类型，包含实际的业务负载（对象、数组、哈希等）。
*   **例外**: `/mirror/**` 保持 Pixiv 官方响应形态或二进制文件输出，不使用系统 envelope。

### 2. 后端 Response 包装工具
PixezServer 后端统一采用 Wavelet `internal/util/response.go` 的响应结构：
*   **成功并携带数据**: `util.OK(data)`
*   **成功无数据**: `util.OKNil()`
*   **普通错误**: `util.Err(message)`
*   **认证失败**: Wavelet OAuth middleware 返回 `{ "error_msg": "...", "data": null }`，状态码为 401。

旧 `server/` 后端中的 `pixez-sync/response` 与 `{success,message,data}` 只属于 legacy 设计，不作为当前开发规范。
