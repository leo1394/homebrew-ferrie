# Ferrie

<p align="center"><a href="README.md">简体中文</a> · <strong>English</strong></p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="MIT License"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-0078D4" alt="macOS, Linux, Windows">
</p>

![Ferrie](assets/ferrie-banner-en.png)

Ferrie is a cross-platform app installer for mobile app testers and developers. Use the same command on macOS, Linux, or Windows to install an Android or iOS package from a file or URL on a connected device:

```sh
ferrie --target ./app.apk
```

**[Install](#install) · [Quick start](#quick-start) · [Daily commands](#daily-commands) · [User guide](docs/usage-en.md)**

## Why Ferrie

The name comes from **ferry**: carrying an app package to its destination device.

Get a build, connect a test device, verify the update. App testers and developers repeat this workflow every day. Ferrie brings platform-specific installation commands into one familiar interface so you can focus on the app.

- **One command across platforms:** use the same `ferrie` command on macOS, Linux, and Windows for APK, APKS, AAB, and IPA packages.
- **Update existing apps:** Try updating first to preserve local data. Only after a failure and a matching installed app does Ferrie offer uninstall/reinstall, with a data-loss warning. The default is no.
- **Start with less setup:** a standalone executable with no Python, Node.js, or Go runtime required. Missing device tools are installed only after confirmation.


## Install

### Homebrew (recommended)

```sh
brew install leo1394/ferrie/ferrie
```

The stable formula prefers a matching Homebrew bottle; otherwise it downloads the prebuilt executable directly. Neither path compiles locally. Both include shell completions and require no language runtime dependencies. To upgrade:

```sh
brew update
brew upgrade ferrie
```

### Bash (Linux / macOS)

If the installed Homebrew is too old to install the Formula, build and install
the published release with Bash:

```sh
curl -fsSL https://raw.githubusercontent.com/leo1394/homebrew-ferrie/master/install.sh -o /tmp/ferrie-install.sh
bash /tmp/ferrie-install.sh
```

The installer verifies SHA256 and installs Ferrie to `~/.local/bin`. Add that directory to PATH if prompted. Run the installer again to update.

### Windows

Run in PowerShell:

```powershell
Invoke-WebRequest https://raw.githubusercontent.com/leo1394/homebrew-ferrie/master/install.ps1 -OutFile "$env:TEMP\ferrie-install.ps1"
powershell -ExecutionPolicy Bypass -File "$env:TEMP\ferrie-install.ps1"
```

The installer verifies SHA256, installs `ferrie.exe` to `%LOCALAPPDATA%\Ferrie`, and adds the directory to your user PATH. Open a new terminal to use Ferrie in PowerShell or CMD.

Prebuilt packages cover Intel (amd64) and ARM64 on macOS / Linux, plus Windows x64. See the [user guide](docs/usage-en.md#installation-options) for custom directories, pinned versions, and other Unix platforms.

## Quick start

**1. Connect a device.** Enable USB debugging and authorize your computer on Android. Unlock the device and trust your computer on iOS.

**2. Install a local package.** On first use, Ferrie lists any missing tools and asks before downloading them.

```sh
# Android
ferrie --target ./app.apk

# iOS
ferrie --target ./app.ipa
```

**3. Choose among multiple devices.** Ferrie prompts you to select a device and confirm installation. If you already know its ID, specify it directly:

```sh
ferrie --list
ferrie --target ./app.apk --device SERIAL
```

Only devices matching the package platform are considered. `--list` uses existing device tools and never downloads them; if tools are missing, it explains which platform cannot yet be discovered.

## Install from a URL

```sh
# Direct download: detects APK, APKS, AAB and IPA archives
ferrie --target "https://example.com/download/app.apk"

# Pgyer merged page: choose Android or iOS interactively
ferrie --target "https://www.pgyer.com/clobotics-rea-test"

# Select a platform and device in advance
ferrie --target "https://www.pgyer.com/clobotics-rea-test" --android --device SERIAL
```

| Link type | Behavior |
| --- | --- |
| Direct HTTP(S) package | Download, check the archive type, then use the local installation flow |
| Public Pgyer download / merged page | Resolve the public download entry; choose interactively or pass `--android` / `--ios` |
| Static download page / iOS OTA manifest | Extract package links or the manifest's IPA; prompt when multiple packages remain |
| Google Play app link | Open the listing on the selected Android device; complete installation on the device |
| App Store app link | Recognize the link and explain the limitation; install through App Store or provide a signed IPA |

`--target` automatically detects local paths and URLs. Quote URLs containing characters such as `&`. Downloads use built-in functionality with no additional runtime dependencies. Temporary packages are removed after success or failure. For pages requiring login, passwords, CAPTCHA or complex JavaScript, download in a browser and use `--target`. See [URL installation](docs/usage-en.md#url-installation).

## Daily commands

| Task | Command |
| --- | --- |
| Install / update an APK | `ferrie --target ./app.apk` |
| Install an IPA | `ferrie --target ./app.ipa` |
| Install an APKS archive | `ferrie --target ./app.apks` |
| Build and install from an AAB | `ferrie --target ./app.aab` |
| Install from a URL | `ferrie --target "https://example.com/app.apk"` |
| Select Android from a merged page | `ferrie --target "https://www.pgyer.com/clobotics-rea-test" --android` |
| List connected devices | `ferrie --list` |
| Install on a specific device | `ferrie --target ./app.apk --device SERIAL` |
| Show help | `ferrie --help` |
| Read the manual | `man ferrie` |
| Show the version | `ferrie version` |

Short options `-T`, `-d`, `-l`, `-h`, and `-v` are also supported. Non-interactive sessions with multiple devices must specify `--device`. Missing tools produce an error instead of a silent download.

Android installation automatically restarts adb once if discovery fails, returns no devices, or the target is offline.

## Tools, only when needed

Ferrie checks only the tools required by the current package and reuses existing installations. If something is missing, it asks once for the missing tools. **The default is not to install:**

```text
Missing tools: java, bundletool
Install into /Users/me/.ferrie (no administrator access)?
Download and install these tools? [y/N]:
```

| Package | Tools used |
| --- | --- |
| APK | adb |
| IPA | go-ios |
| AAB / APKS | adb, Java 17+, bundletool |

Approved downloads go into `~/.ferrie`, without administrator access or changes to the system PATH. **Java and bundletool are required only for AAB / APKS.** Device listing, help, version output, and completion never download tools.


## Devices and installation notes

- **Android:** USB debugging must be authorized. Devices marked offline or unauthorized cannot receive an installation.
- **iOS:** the IPA needs a valid signature / provisioning profile for the target device. Windows may require Apple device drivers; Linux requires the usbmuxd service.
- **Confirmed reinstallation:** uninstalling deletes local app data; an uninstall failure stops the retry. If the new installation fails after uninstalling, Ferrie cannot restore the old app or its data. AAB builds use bundletool’s debug key by default.
- **Platform differences:** Linux ARM64 requires an adb package from your distribution. Running the x64 binary on Windows ARM64 requires OS emulation support.

See the [user guide](docs/usage-en.md) for environment settings, exit codes, and troubleshooting.

The manual is included in Homebrew/Bottle and Unix installations. After building locally, run `bash install.sh --local`. If needed, set `export MANPATH="$HOME/.local/share/man:${MANPATH:-}"`; with a custom `FERRIE_DATA_DIR`, use `$FERRIE_DATA_DIR/man` instead.

## Development and contributing

Issues and improvements are welcome. When reporting an installation problem, include the Ferrie version, host operating system, package format, and error output.

Building from source requires Go 1.23+. Users of release binaries do not need Go:

```sh
go build -o bin/ferrie .
./bin/ferrie --help
go test ./...
go vet ./...
```

On Windows, use `go build -o bin/ferrie.exe .`. To install a local build, run `bash install.sh --local` or `./install.ps1 -Local`.

[Testing and development](docs/usage-en.md#development-checks) · [Release process (Chinese)](RELEASING.md)

## License

[MIT](LICENSE)
