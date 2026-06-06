# Deployment Guide

本文档描述 PixEz Flutter 的本地构建、自动化构建、签名、产物和维护流程。

## 前置环境

基础要求：

- Flutter stable，Dart SDK 需满足 `pubspec.yaml` 中 `sdk: ">=3.10.0"`。
- Android 构建需要 JDK 17、Android SDK、NDK `28.2.13676358`。
- iOS/macOS 构建需要 macOS、Xcode、CocoaPods。
- Windows 构建需要 Windows、Visual Studio C++ 工具链、CMake。
- `plugins/rhttp` 构建需要 Rust 工具链。
- OpenHarmony 构建需要对应 DevEco/Hvigor 环境。

建议优先使用 `.fvmrc` 指定的 Flutter 版本。如果使用 FVM：

```sh
fvm flutter --version
fvm flutter pub get
```

未使用 FVM 时：

```sh
flutter --version
flutter pub get
```

## 依赖恢复

应用主体和本地插件都需要恢复依赖：

```sh
flutter pub get
cd plugins/rhttp
flutter pub get
cd ../..
```

CI 中 Windows 构建使用 lockfile：

```sh
flutter pub get --no-example --enforce-lockfile
cd plugins/rhttp
flutter pub get --no-example --enforce-lockfile
```

本地开发如遇 lockfile 与环境不一致，可先使用普通 `flutter pub get`，再评估是否需要提交 `pubspec.lock` 变化。

## 代码生成

项目使用 `json_serializable`、`freezed`、`riverpod_generator`、`mobx_codegen` 等生成工具。本地插件 `plugins/rhttp` 也有生成流程。

推荐顺序：

```sh
cd plugins/rhttp
dart run build_runner build --delete-conflicting-outputs
cd ../..
dart run build_runner build --delete-conflicting-outputs
```

修改以下内容后通常需要重新生成：

- `lib/models/` 中带序列化或 freezed 注解的模型。
- Riverpod 注解 provider/notifier。
- MobX store。
- `plugins/rhttp` 的 Dart/Rust Bridge 相关接口。
- ARB 国际化资源或 Flutter 生成配置。

## 本地校验

提交前建议至少运行：

```sh
flutter analyze
flutter test
```

针对具体平台构建前运行：

```sh
flutter doctor -v
```

如只修改文档，不需要执行 Flutter 构建；如修改代码、配置或资源，应根据影响平台选择对应构建命令验证。

## Android 构建

Android 配置位于：

- `android/app/build.gradle.kts`
- `android/key.properties`，本地签名文件，不应提交。

关键配置：

- `compileSdk = 36`
- `targetSdk = 35`
- `ndkVersion = "28.2.13676358"`
- ABI：`armeabi-v7a`、`arm64-v8a`、`x86_64`
- 普通包名：`com.perol.pixez`
- Google Play 包名：`com.perol.play.pixez`

普通 release APK：

```sh
flutter build apk --release
```

Google Play 包名构建：

```sh
flutter build apk --release --dart-define=IS_GOOGLEPLAY=true
```

Android App Bundle：

```sh
flutter build appbundle --release --dart-define=IS_GOOGLEPLAY=true
```

签名配置示例，放在 `android/key.properties`：

```properties
storePassword=your-store-password
keyPassword=your-key-password
keyAlias=your-key-alias
storeFile=/absolute/path/to/keystore.jks
```

如果 `android/key.properties` 不存在，release 构建不会应用自定义 release 签名配置。

## iOS 构建

iOS 配置位于：

- `ios/Podfile`
- `ios/Runner.xcodeproj`
- `ios/Runner.xcworkspace`
- `ios/TinkerExtension.entitlements`

当前 Podfile 设置：

- iOS 最低版本：`13.0`
- `use_frameworks!`
- 多个 permission_handler 权限宏被关闭，减少无用权限声明。

无签名构建：

```sh
flutter build ios --release --no-codesign
```

生成无签名 IPA 的流程与 GitHub Actions 一致：

```sh
flutter build ios --release --no-codesign
mkdir -p build/ios/iphoneos/Payload
mv build/ios/iphoneos/Runner.app build/ios/iphoneos/Payload
cd build/ios/iphoneos
zip -r pixez-ios.ipa Payload
```

正式上架或 TestFlight 分发需要在 Xcode 中配置 Team、Bundle Identifier、证书、描述文件和 entitlements，再使用 Xcode Archive 或 Flutter/Xcode 命令导出。

## Windows 构建

Windows 配置位于：

- `windows/`
- `pubspec.yaml` 的 `msix_config`
- `.github/workflows/build_windows.yml`

普通 release 构建：

```powershell
flutter build windows --release
```

CI 中会使用 GitVersion 输出覆盖版本：

```powershell
flutter build windows `
  --release `
  --no-pub `
  --build-name=<major.minor.patch> `
  --build-number=<buildMetaData>
```

产物目录：

```text
build/windows/x64/runner/Release/
```

生成 MSIX：

```powershell
dart run msix:create --build-windows false
```

正式签名 MSIX 需要证书。GitHub Actions 通过以下 secrets 恢复证书：

- `CERTIFICATE`
- `CERTIFICATE_PASSWORD`

没有证书时，CI 仍会上传 loose binary；MSIX 和安装器步骤会跳过。

## macOS 构建

macOS 配置位于：

- `macos/Podfile`
- `macos/Runner.xcodeproj`
- `macos/Runner.xcworkspace`

当前 Podfile 设置 macOS 最低版本为 `10.15`。

构建命令：

```sh
flutter build macos --release
```

正式分发需要配置 Developer ID/Application signing、notarization 和 entitlements。仓库当前未提供 macOS 自动发布工作流。

## OpenHarmony 构建

OpenHarmony 工程位于 `ohos/`，包括：

- `ohos/build-profile.json5`
- `ohos/hvigorfile.ts`
- `ohos/oh-package.json5`
- `ohos/entry/`
- `ohos/AppScope/`

构建需使用 OpenHarmony/DevEco 对应工具链。仓库当前未提供 OpenHarmony GitHub Actions 工作流，发布流程以本地平台工具为准。

## Linux 构建

项目启动代码已对 Linux 桌面初始化 sqlite FFI 和数据库路径。仓库当前未提供 Linux 平台目录和自动化工作流；如需正式支持 Linux，应先补齐平台工程并在设计文档中说明目标发行格式。

## GitHub Actions

### iOS

`.github/workflows/build_ios.yml`

- 触发方式：`workflow_dispatch`
- Runner：`macos-26`
- 步骤：checkout、安装 Flutter stable、依赖恢复、`plugins/rhttp` 生成、应用生成、`flutter build ios --release --no-codesign`、打包 IPA、上传 artifact。
- 产物：`pixez-ios.ipa`

### Windows

`.github/workflows/build_windows.yml`

- 触发方式：`push`、`pull_request`、`workflow_dispatch`
- Runner：`windows-latest`
- 步骤：checkout、GitVersion、Flutter stable、Rust、依赖恢复、插件生成、应用生成、Windows release 构建、上传 loose binary。
- 可选步骤：存在证书 secret 时生成 MSIX 和 MSIX installer。
- tag 触发时，`publish` job 会把产物发布到 GitHub Release。

## 版本号

应用版本声明在 `pubspec.yaml`：

```yaml
version: 1.9.82+499
```

平台构建中的版本来源：

- Android 默认配置还包含 `versionCode` 和 `versionName`，Flutter 构建可通过 `pubspec.yaml` 或命令行覆盖。
- iOS 使用 Flutter build name/build number 映射到 `CFBundleShortVersionString` 和 `CFBundleVersion`。
- Windows CI 使用 GitVersion 输出传入 `--build-name` 和 `--build-number`，MSIX 使用 `assemblySemVer`。

发布前应确认各平台版本号策略一致，避免应用商店拒绝上传或用户无法升级。

## 发布前检查清单

- 依赖已恢复，`plugins/rhttp` 和应用主体生成代码已同步。
- `flutter analyze` 通过。
- 关键平台 `flutter test` 或目标测试通过。
- 目标平台 release 构建通过。
- Android/iOS/macOS/Windows 签名材料已配置且未提交到仓库。
- 权限、entitlements、包名、URL scheme、app uri handler 与目标渠道一致。
- `pubspec.yaml`、平台配置和 CI 版本号一致。
- 发布产物记录 SHA256 或使用 GitHub Actions artifact digest。

## 维护与回滚

- 发布产物应保留构建日志、Flutter 版本、Git commit、版本号和 artifact digest。
- 线上问题优先用同一 commit 和同一构建参数复现。
- 需要回滚时，优先重新发布上一稳定 tag/commit 的产物，而不是手工修改产物。
- 涉及上游同步时，优先保留原项目目录结构；二开功能集中在独立目录，通过少量入口接入。
- 删除二开功能时，应先关闭配置开关，再移除入口、资源和自定义目录，最后清理依赖。
