# Ferrie user guide

[Back to README](../README-EN.md) · [简体中文](usage.md)

## Installation options

Remote installation requires the selected version to be published. The Unix installer uses Bash, curl, and `shasum` / `sha256sum`; no language runtime is required.

```sh
# Pin a version
bash /tmp/ferrie-install.sh 0.1.0

# Choose directories (absolute paths required)
FERRIE_INSTALL_DIR="$HOME/tools/bin" FERRIE_DATA_DIR="$HOME/tools/share" bash /tmp/ferrie-install.sh
```

The default executable is `~/.local/bin/ferrie`, with completion scripts under `~/.local/share`. Use `FERRIE_VERSION` to select a version and `FERRIE_REPOSITORY` to select a GitHub repository. On other Unix platforms, build from source, run `bash install.sh --local`, and provide compatible device tools yourself.

On Windows, choose a version and directory:

```powershell
powershell -ExecutionPolicy Bypass -File "$env:TEMP\ferrie-install.ps1" -Version 0.1.0 -InstallDir "$env:LOCALAPPDATA\Ferrie"
```

The installer also supports `-Repository`. Windows ARM64 uses the x64 executable and requires OS emulation support.

## Completion setup

Homebrew installs scripts into standard completion directories. Enable your shell's completion system as described in [Homebrew Shell Completion](https://docs.brew.sh/Shell-Completion). For standalone installations, load the scripts below. Substitute your own data directory if you changed it.

### Bash

Add this to `~/.bashrc`. On macOS, a login Bash shell needs to source that file from `~/.bash_profile`.

```sh
source "$HOME/.local/share/bash-completion/completions/ferrie"
```

### Zsh

Add the fpath entry before the existing `compinit` call in `~/.zshrc`. If completion is not enabled yet, include the last two lines too:

```sh
fpath=("$HOME/.local/share/zsh/site-functions" $fpath)
autoload -Uz compinit
compinit
```

### Fish

The default data directory is loaded automatically. You can also add this explicitly to `~/.config/fish/config.fish`:

```fish
source ~/.local/share/fish/vendor_completions.d/ferrie.fish
```

### PowerShell 7

On Windows, add this to `$PROFILE`:

```powershell
. "$env:LOCALAPPDATA\Ferrie\ferrie-completion.ps1"
```

For a Homebrew installation, use:

```powershell
. "$(brew --prefix)/share/pwsh/completions/ferrie.ps1"
```

Run `ferrie __completion bash` to print the script, or replace `bash` with `zsh`, `fish`, or `powershell`. Use the space-separated form `--target PATH` for path completion. The command also accepts `--target=PATH` and `--device=ID`.

## Environment settings

| Variable | Purpose |
| --- | --- |
| `FERRIE_HOME` | Tool cache directory; defaults to `~/.ferrie` |
| `FERRIE_ADB` | Path to the adb executable |
| `FERRIE_IOS` | Path to the go-ios executable |
| `FERRIE_JAVA` | Path to a Java 17+ executable |
| `FERRIE_BUNDLETOOL` | Path to the bundletool JAR |

Ferrie also reuses PATH, ANDROID_HOME, ANDROID_SDK_ROOT, JAVA_HOME, and Android, Java, and bundletool caches from the original script under `~/.app-installer`. Prefer absolute paths when specifying tool files.

If tools are missing, run `ferrie --target PATH` interactively and approve the download. Prepare tools in advance for non-interactive sessions. `--device` skips device selection only; it does not authorize dependency downloads.

## Troubleshooting

| Situation | What to check |
| --- | --- |
| `--list` reports missing tools | Run `ferrie --target PATH` to review and approve the required tools, or point to existing tools through environment variables |
| Android shows `unauthorized` | Unlock the device and accept the USB debugging authorization |
| Android shows `offline` | Reconnect the device and check USB debugging and the adb connection |
| Windows cannot find a device | Check the USB cable, connection mode, and vendor Android USB drivers / Apple device drivers |
| Linux cannot find an iOS device | Check usbmuxd, device trust, and USB access permissions |
| adb is missing on Linux ARM64 | Install it through your distribution; Google does not provide a matching Platform Tools binary |
| An update fails with a signing mismatch | Prefer a package signed with the same key to preserve data. After failure, Ferrie checks for a matching app and offers uninstall/reinstall with a data-loss warning; the default is no. |
| AAB signing requirements | bundletool uses a debug key by default; provide a correctly signed APK / APKS from your build pipeline |

An IPA needs a valid signature and provisioning profile for the target device. System drivers and services cannot be replaced by user-space tools in Ferrie's cache. See [Android bundletool](https://developer.android.com/tools/bundletool) and [go-ios](https://github.com/danielpaulus/go-ios).

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Command succeeded |
| `1` | Installation, environment, or device error; or interactive cancellation |
| `2` | Invalid arguments |
| `130` | Interrupted with Ctrl+C |

## Development checks

```sh
go test ./...
go vet ./...
go build -o bin/ferrie .
bash tests/completion_test.sh
bash tests/installer_test.sh
```

On Windows, run `go build -o bin/ferrie.exe .`, then `./tests/windows_test.ps1`. GitHub Actions defines tests for all three platforms and Homebrew installation checks. Tests use simulated devices and do not install apps on real devices.

The original script is retained at [legacy/install-app.sh](../legacy/install-app.sh). The root `install.sh` installs Ferrie itself. Preserve the existing code style and do not run formatters. See [RELEASING.md](../RELEASING.md) for the release process in Chinese.

## URL installation

```sh
ferrie --target "https://example.com/app.apk"
ferrie --target "https://www.pgyer.com/3ceb0fd1d20e97fa0310d802323c07c8" --android
ferrie --target "https://www.pgyer.com/clobotics-rea-test" --ios --device UDID
ferrie --target "https://play.google.com/store/apps/details?id=com.example.app" --device SERIAL
```

- **Direct downloads:** HTTP(S), relative redirects, session cookies and extensionless URLs are supported. ZIP contents determine APK / APKS / AAB / IPA type; URL extensions and server filenames are not trusted. Ferrie tries updating first. Only after failure and detection of the same applicationId / bundleId does it ask for permission to uninstall/reinstall, warning that local data will be deleted.
- **Pgyer:** resolves public download entries and merged Android / iOS pages using the build and temporary token supplied by the current page. No API Key or paid API is used. Page structure, link expiry and access restrictions remain under the provider's control; expired, restricted or broken entries produce an error.
- **Other pages:** recognizes static `<a href>` links ending in `.apk`, `.apks`, `.aab`, `.ipa` or `.plist`, or links with a matching filename in the `download` attribute. Supports `itms-services` and XML OTA manifests containing a `software-package`. Does not execute JavaScript. For unsupported pages, supply a direct URL or download the file in a browser.
- **Platform selection:** merged pages prompt for Android / iOS unless specified. Non-interactive sessions require `--android` or `--ios`. Multiple packages for the same platform still require selection; use an exact direct URL for automation. `--device` selects hardware and does not replace platform selection.
- **Google Play:** adb opens the app listing on the selected device, which must have Google Play installed and enabled. Sign-in, purchases and installation happen on the device. Command success means the page was opened, not that the app was installed.
- **App Store:** app links are not IPA download URLs. Ferrie recognizes the link and returns an explanation. Install through App Store on the device, or provide a signed IPA suitable for the device.
- **Browser blob URLs:** literal `blob:` URLs exist only in their browser session. Supply the underlying HTTP(S) URL or a saved local package.

Downloads require no extra tools. Limits: 15 minutes per request, 12 page/redirect hops, 2 MiB per page and 8 GiB per package. Packages are stored in the system temporary directory and removed after normal completion, download failure or installation failure. Forced termination or shutdown can leave `ferrie-download-*` directories for manual cleanup. Download logs omit URL query parameters. `--target=URL` is supported; quote URLs. `--android` / `--ios` require a URL in `--target` and are mutually exclusive.

Protocol references: [Pgyer installation API](https://www.pgyer.com/doc/en/view/api_install), [Google Play links](https://developer.android.com/distribute/marketing-tools/linking-to-google-play).

## Android connection recovery

Before installation, Ferrie restarts the selected adb server once (`kill-server` / `start-server`) when discovery fails, returns an empty list, or reports the target as offline. Each server command has a 10-second timeout; device re-enumeration is then allowed up to 5 seconds. An explicit device ID is never replaced by another device. Without an explicit ID, discovery does not restart a server that still lists usable devices or devices awaiting authorization.

Ferrie prints a notice before restarting; other debugging sessions on that adb server may briefly disconnect. If recovery fails, check the USB connection, USB debugging and device authorization. Unauthorized devices still require approval on the device. Listing, completion and iOS installation do not trigger recovery. Failed uninstall/install commands are not automatically retried.

## Update first, confirm before reinstalling

Local and downloaded APK, IPA, APKS and AAB packages use the same flow:

1. Attempt installation/update without uninstalling first.
2. After failure, read the package's applicationId / bundleId and check for the same app on the selected device. Identity or device-query errors stop the fallback, preserving the original failure; nothing is uninstalled.
3. If a matching app exists, show its ID, the device ID and a local-data-loss warning, then ask `Uninstall this app and retry installation? [y/N]`. Only `y` or `yes` permits uninstalling. Enter, refusal, invalid input and EOF all decline. Non-interactive sessions never uninstall.
4. After confirmation, uninstall and retry the same package on the same device once. An uninstall failure prevents the retry. A failed retry does not loop and cannot restore the previous app or deleted data.

For signature mismatches, decline uninstalling and use a package signed with the same key if local data must be preserved.
