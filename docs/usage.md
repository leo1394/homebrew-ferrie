# Ferrie 使用指南

[返回首页](../README.md) · [English](usage-en.md)

## 安装选项

远程安装方式需要对应版本已发布。Unix 安装器使用 Bash、curl 和 `shasum` / `sha256sum`，无需语言运行环境。

```sh
# 固定版本
bash /tmp/ferrie-install.sh 0.1.0

# 指定目录（必须为绝对路径）
FERRIE_INSTALL_DIR="$HOME/tools/bin" FERRIE_DATA_DIR="$HOME/tools/share" bash /tmp/ferrie-install.sh
```

默认可执行文件位置为 `~/.local/bin/ferrie`，补全脚本位于 `~/.local/share`。可用 `FERRIE_VERSION` 指定版本、`FERRIE_REPOSITORY` 指定 GitHub 仓库。其他 Unix 平台可从源码构建后使用 `bash install.sh --local`，并自行提供适用的设备工具。

Windows 可指定版本和目录：

```powershell
powershell -ExecutionPolicy Bypass -File "$env:TEMP\ferrie-install.ps1" -Version 0.1.0 -InstallDir "$env:LOCALAPPDATA\Ferrie"
```

安装器还支持 `-Repository`。Windows ARM64 使用 x64 可执行文件，需要系统提供仿真支持。

## 自动补全配置

Homebrew 把补全脚本放入标准目录。先按 [Homebrew Shell Completion](https://docs.brew.sh/Shell-Completion) 启用对应 Shell 的补全系统。使用脚本安装时，按以下方式加载；自定义数据目录时替换路径。

### Bash

在 `~/.bashrc` 中加入。macOS 的登录 Bash 需要从 `~/.bash_profile` 加载该文件。

```sh
source "$HOME/.local/share/bash-completion/completions/ferrie"
```

### Zsh

在 `~/.zshrc` 的现有 `compinit` 调用前加入 fpath。尚未启用补全时，一并加入后两行：

```sh
fpath=("$HOME/.local/share/zsh/site-functions" $fpath)
autoload -Uz compinit
compinit
```

### Fish

默认数据目录下会自动加载，也可在 `~/.config/fish/config.fish` 中显式加入：

```fish
source ~/.local/share/fish/vendor_completions.d/ferrie.fish
```

### PowerShell 7

Windows 在 `$PROFILE` 中加入：

```powershell
. "$env:LOCALAPPDATA\Ferrie\ferrie-completion.ps1"
```

Homebrew 安装的 PowerShell 使用：

```powershell
. "$(brew --prefix)/share/pwsh/completions/ferrie.ps1"
```

可执行 `ferrie __completion bash` 输出补全脚本，也可将 `bash` 换成 `zsh`、`fish` 或 `powershell`。建议使用空格形式 `--target PATH` 进行路径补全；命令本身也接受 `--target=PATH`、`--device=ID`。

## 环境配置

| 环境变量 | 用途 |
| --- | --- |
| `FERRIE_HOME` | 工具缓存目录，默认为 `~/.ferrie` |
| `FERRIE_ADB` | 指定 adb 可执行文件 |
| `FERRIE_IOS` | 指定 go-ios 可执行文件 |
| `FERRIE_JAVA` | 指定 Java 17+ 可执行文件 |
| `FERRIE_BUNDLETOOL` | 指定 bundletool JAR 文件 |

Ferrie 也会复用 PATH、ANDROID_HOME、ANDROID_SDK_ROOT、JAVA_HOME，以及旧脚本 `~/.app-installer` 下的 Android、Java 和 bundletool 缓存。指定文件路径时，建议使用绝对路径。

工具缺失时，交互运行 `ferrie --target PATH`，确认后才下载。非交互环境应提前完成准备；指定 `--device` 只跳过设备选择，不代表同意下载依赖。

## 常见问题

| 情况 | 处理方式 |
| --- | --- |
| `--list` 提示缺少工具 | 运行 `ferrie --target PATH` 查看所需工具并确认准备，或通过环境变量指定已有工具 |
| Android 显示 `unauthorized` | 解锁设备，接受 USB 调试授权后重试 |
| Android 显示 `offline` | 重新连接设备，检查 USB 调试和 adb 连接 |
| Windows 找不到设备 | 检查 USB 线、连接模式及设备厂商的 Android USB 驱动 / Apple 设备驱动 |
| Linux 找不到 iOS 设备 | 检查 usbmuxd 服务、设备信任状态及 USB 访问权限 |
| Linux ARM64 缺少 adb | 通过发行版包管理器安装；Google 未提供对应 Platform Tools 预编译包 |
| 覆盖安装签名不匹配 | 优先使用同签名的包保留数据；Ferrie 在失败后检测同 ID 应用，并询问是否卸载重装（会清除本地数据，默认否） |
| AAB 签名要求 | bundletool 默认使用 debug key；需要正式签名时，提供构建流程生成的 APK / APKS |

IPA 需具有适用于目标设备的有效签名和描述文件。系统驱动 / 服务无法由 Ferrie 缓存中的用户态工具替代。参考 [Android bundletool](https://developer.android.com/tools/bundletool) 和 [go-ios](https://github.com/danielpaulus/go-ios)。

## 退出码

| 退出码 | 含义 |
| --- | --- |
| `0` | 命令成功 |
| `1` | 安装、环境或设备错误，或交互取消 |
| `2` | 参数错误 |
| `130` | 用户按 Ctrl+C 中断 |

## 开发验证

```sh
go test ./...
go vet ./...
go build -o bin/ferrie .
bash tests/completion_test.sh
bash tests/installer_test.sh
```

Windows 先运行 `go build -o bin/ferrie.exe .`，再运行 `./tests/windows_test.ps1`。GitHub Actions 配置了三平台测试和 Homebrew 安装验证。测试使用模拟设备，不会安装应用到真机。

原始脚本保留于 [legacy/install-app.sh](../legacy/install-app.sh)；根目录的 `install.sh` 用于安装 Ferrie 本身。遵循现有代码样式，不运行格式化器。发布步骤见 [RELEASING.md](../RELEASING.md)。

## 链接安装

```sh
ferrie --target "https://example.com/app.apk"
ferrie --target "https://www.pgyer.com/3ceb0fd1d20e97fa0310d802323c07c8" --android
ferrie --target "https://www.pgyer.com/clobotics-rea-test" --ios --device UDID
ferrie --target "https://play.google.com/store/apps/details?id=com.example.app" --device SERIAL
```

- **直链**：支持 HTTP(S)、相对重定向、下载会话 Cookie，以及无文件扩展名的下载地址。依据 ZIP 内部结构识别 APK / APKS / AAB / IPA，不信任 URL 后缀或服务器文件名。优先直接覆盖安装；仅在失败且存在同 applicationId / bundleId 应用时，询问是否卸载重装并提示本地数据丢失风险。
- **蒲公英**：支持公开页面的下载入口与 Android / iOS 合并页，使用页面当前提供的构建和临时下载令牌，不调用需 API Key 的付费接口。第三方页面结构、链接有效期及访问限制由平台控制；过期、受限或失效入口会返回错误。
- **普通网页**：识别静态 `<a href>` 中以 `.apk`、`.apks`、`.aab`、`.ipa`、`.plist` 结尾的链接，或带对应文件名 `download` 属性的链接；支持 `itms-services` 与 XML OTA 清单中的 `software-package`。不运行网页 JavaScript。无法解析时，提供直链或浏览器下载后的本地文件。
- **平台选择**：合并页同时提供两个平台且未指定平台时，询问 Android / iOS；非交互环境必须加 `--android` 或 `--ios`。多个同平台包仍需交互选择，自动化请使用精确直链。`--device` 负责设备选择，不代替平台选择。
- **Google Play**：通过 adb 在选定设备打开应用详情页，需要设备安装并启用 Google Play。登录、付费与最终安装在设备上完成；命令成功仅表示页面已打开，不表示应用已安装。
- **App Store**：应用链接无法作为 IPA 直链使用。Ferrie 识别链接后返回说明，请在目标设备的 App Store 安装；需要通过电脑安装时，提供适用于设备的已签名 IPA。
- **浏览器 blob 地址**：字面 `blob:` URL 仅存在于浏览器会话中。请提供它对应的 HTTP(S) 地址或保存后的本地包。

下载无需新增工具。每个请求超时 15 分钟，最多 12 层页面 / 重定向，网页上限 2 MiB，安装包上限 8 GiB。包存入系统临时目录，在正常结束、下载失败或安装失败后删除；强制结束进程或关机可能留下 `ferrie-download-*` 临时目录，可手动清理。下载日志不输出 URL 查询参数。URL 支持 `--target=URL`，建议加引号；`--android` / `--ios` 只用于 URL 类型的 `--target`，且不能同时使用。

协议参考：[蒲公英安装接口说明](https://www.pgyer.com/doc/en/view/api_install)、[Google Play 应用链接](https://developer.android.com/distribute/marketing-tools/linking-to-google-play)。

## Android 连接自动恢复

安装前，如果 adb 设备列表为空、目标设备为 offline，或设备识别命令报错，Ferrie 会使用当前选中的 adb 执行一次 `kill-server` / `start-server`，随后等待最多 5 秒重新枚举设备。每次服务命令限时 10 秒。指定设备 ID 后不会自动改装到另一台设备。未指定 ID 且列表中仍有可用或待授权设备时，不重启服务。

重启前会显示提示；同一 adb 服务上的其他调试会话可能短暂断开。仍无法识别时，返回检查 USB 连接、USB 调试及手机授权的提示。未授权设备需要在手机上确认，重启不能代替授权。`--list`、自动补全和 iOS 安装不触发此恢复流程；卸载与安装命令失败后不自动重试。

## 覆盖安装与确认重装

本地包和 URL 下载包使用相同流程，适用于 APK、IPA、APKS、AAB：

1. 先直接安装 / 覆盖更新，不预先卸载应用。
2. 安装失败后，读取包内 applicationId / bundleId 并检查选定设备是否已有同 ID 应用。身份解析或设备查询失败时，保留原始错误并停止，不卸载。
3. 存在同 ID 应用时，显示设备 ID、应用 ID 和本地数据丢失风险，并询问 `Uninstall this app and retry installation? [y/N]`。只有明确输入 `y` 或 `yes` 才卸载；回车、拒绝、无效输入和输入结束均不卸载。非交互终端不执行卸载。
4. 确认后卸载，并用同一安装包、同一设备重试一次。卸载失败不重试；重装失败不循环，也无法恢复旧应用与已删除的数据。

签名不匹配时，若要保留本地数据，请拒绝卸载并换用与现有应用签名一致的安装包。
