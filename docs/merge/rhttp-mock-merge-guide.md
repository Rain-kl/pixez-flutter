# rhttp 依赖覆盖模拟（Dependency Overrides Mock）合并与冲突解决指南 (2026-06-25)

## 1. 改动背景
为了完全跳过 Rust `rhttp` 客户端的本地编译负担，并彻底解决 Android 系统上由底层 Rustls 引起的证书吊销校验缺陷（`InvalidCertificate(Revoked)`），项目采用 `dependency_overrides` 机制，在 `lib/custom/mocks/` 下注入了本地 Mock 实现，将所有底层网络调用无缝降级并桥接为 Dio 官方原生的标准 `IOHttpClientAdapter`。

---

## 2. 合并冲突预防原理
本设计通过 `dependency_overrides`（依赖覆盖）从 **编译期/包解析期** 直接截获依赖指向。
- **业务代码零侵入**：原项目所有文件（如 `main.dart`、`api_client.dart` 等）中依然保留原版的 `import 'package:rhttp/rhttp.dart';` 导入，没有做任何物理改动。
- **冲突最小化**：后续拉取上游（`master`）最新代码进行合并时，业务代码文件的 Git 冲突概率为 0%。

---

## 3. 合并与冲突解决步骤

如果在后续合并上游代码时遇到变化，请遵循以下指南进行检查与修复：

### 步骤 3.1: 解决 `pubspec.yaml` 冲突
合并上游代码时，如果上游对 `pubspec.yaml` 中的 `dependencies` 或 `dependency_overrides` 进行了修改，可能会产生 Git 冲突：
1. **保留 dependencies 的原有逻辑**：上游的 `dependencies` 更新（如升级某些库的版本）应直接保留合入。
2. **确保 overrides 配置存在**：在冲突合并时，**必须保留**以下对 `rhttp` 和 `dio_compatibility_layer` 的路径覆盖配置：
   ```yaml
   dependency_overrides:
     permission_handler_apple: 9.4.10 # （保留原有其他覆盖）
     rhttp:
       path: ./lib/custom/mocks/rhttp
     dio_compatibility_layer:
       path: ./lib/custom/mocks/dio_compatibility_layer
   ```

### 步骤 3.2: 应对上游引入的新 rhttp API 接口
如果上游在之后的更新中调用了 `package:rhttp/rhttp.dart` 中新增的类、构造函数或方法，可能会导致 `flutter analyze` 报告找不到成员：
* **解决方法**：不需要修改原项目代码，而是直接编辑本地的模拟实现文件 [rhttp.dart](file:///Users/ryan/DEV/Flutter/pixez-flutter/lib/custom/mocks/rhttp/lib/rhttp.dart)。
* 根据报错提示，在 [rhttp.dart](file:///Users/ryan/DEV/Flutter/pixez-flutter/lib/custom/mocks/rhttp/lib/rhttp.dart) 的对应 Mock 类中添加空壳的方法签名/构造函数即可，使其保持签名对齐。

### 步骤 3.3: 验证合并正确性
完成 Git 合并与冲突解决后，必须依次运行以下命令：
```bash
# 1. 重新解析依赖并应用覆盖映射
flutter pub get

# 2. 运行静态分析确保没有任何类型签名不匹配问题
flutter analyze
```
只要 `flutter analyze` 无错通过，说明合并成功，且网络层将继续完全以原生 Dio 运行。
