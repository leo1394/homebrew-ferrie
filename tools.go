package main

import (
    "archive/tar"
    "archive/zip"
    "bufio"
    "compress/gzip"
    "context"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "runtime"
    "strconv"
    "strings"
    "time"
)

func (a *app) readLine() (string, error) {
    if a.reader == nil { a.reader = bufio.NewReader(a.in) }
    line, err := a.reader.ReadString('\n')
    return strings.TrimSpace(line), err
}

func (a *app) prepare(names ...string) error {
    var missing []string
    for _, name := range names {
        path, err := a.tool(name, false)
        if err != nil { return err }
        if path == "" { missing = append(missing, name) }
    }
    if len(missing) == 0 { return nil }
    fmt.Fprintf(a.err, "Missing tools: %s\nInstall into %s (no administrator access)?\n", strings.Join(missing, ", "), a.cache)
    if !a.interactive { return errors.New("Missing tools in non-interactive session; run Ferrie interactively to approve installation, or provide tools via FERRIE_* paths") }
    fmt.Fprint(a.out, "Download and install these tools? [y/N]: ")
    answer, err := a.readLine()
    if err != nil || (!strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes")) { return errors.New("Dependency installation cancelled") }
    for _, name := range missing {
        if _, err := a.tool(name, true); err != nil { return err }
    }
    return nil
}

func regular(path string) bool {
    info, err := os.Stat(path)
    return err == nil && info.Mode().IsRegular()
}

func binaryName(name string) string {
    if runtime.GOOS == "windows" { return name + ".exe" }
    return name
}

func (a *app) javaUsable(path string) bool {
    ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
    defer cancel()
    // Java writes its version to stderr.
    result, err := exec.CommandContext(ctx, path, "-version").CombinedOutput()
    if err != nil { return false }
    match := regexp.MustCompile(`version "([0-9]+)`).FindStringSubmatch(string(result))
    if len(match) != 2 { return false }
    major, _ := strconv.Atoi(match[1])
    return major >= 17
}

func (a *app) tool(name string, install bool) (string, error) {
    override := os.Getenv("FERRIE_" + strings.ToUpper(name))
    if override != "" {
        path, err := filepath.Abs(override)
        if err != nil || !regular(path) { return "", fmt.Errorf("FERRIE_%s must point to a file", strings.ToUpper(name)) }
        if name == "java" && !a.javaUsable(path) { return "", errors.New("FERRIE_JAVA must point to Java 17+") }
        return path, nil
    }
    home, _ := os.UserHomeDir()
    var candidates []string
    if name != "bundletool" {
        if path, err := exec.LookPath(name); err == nil { candidates = append(candidates, path) }
    }
    switch name {
    case "adb":
        for _, root := range []string{os.Getenv("ANDROID_HOME"), os.Getenv("ANDROID_SDK_ROOT"),
            filepath.Join(home, "Library", "Android", "sdk"), filepath.Join(home, "Android", "Sdk"),
            filepath.Join(os.Getenv("LOCALAPPDATA"), "Android", "Sdk"), a.cache, filepath.Join(home, ".app-installer")} {
            if root != "" { candidates = append(candidates, filepath.Join(root, "platform-tools", binaryName("adb"))) }
        }
    case "ios": candidates = append(candidates, filepath.Join(a.cache, "go-ios", binaryName("ios")))
    case "java":
        for _, root := range []string{os.Getenv("JAVA_HOME"), filepath.Join(a.cache, "java"), filepath.Join(home, ".app-installer", "java")} {
            if root != "" { candidates = append(candidates, filepath.Join(root, "bin", binaryName("java")), filepath.Join(root, "Contents", "Home", "bin", "java")) }
        }
        if runtime.GOOS == "darwin" {
            ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
            if output, err := exec.CommandContext(ctx, "/usr/libexec/java_home", "-v", "17+").Output(); err == nil {
                candidates = append(candidates, filepath.Join(strings.TrimSpace(string(output)), "bin", "java"))
            }
            cancel()
        }
    case "bundletool":
        candidates = append(candidates, filepath.Join(a.cache, "bundletool.jar"), filepath.Join(home, ".app-installer", "bundletool", "bundletool.jar"), filepath.Join(home, ".bundletool", "bundletool.jar"))
    default: return "", fmt.Errorf("Unknown tool %s", name)
    }
    for _, path := range candidates {
        if !regular(path) || (name == "java" && !a.javaUsable(path)) { continue }
        absolute, err := filepath.Abs(path)
        if err != nil { return "", err }
        return absolute, nil
    }
    if !install { return "", nil }
    if err := os.MkdirAll(a.cache, 0o755); err != nil { return "", err }
    folder, err := os.MkdirTemp(a.cache, ".download-")
    if err != nil { return "", err }
    defer os.RemoveAll(folder)
    switch name {
    case "adb": return a.installADB(folder)
    case "ios": return a.installIOS(folder)
    case "java": return a.installJava(folder)
    case "bundletool":
        file := filepath.Join(folder, "bundletool.jar")
        if err := a.download("https://github.com/google/bundletool/releases/download/1.18.1/bundletool-all-1.18.1.jar", file, ""); err != nil { return "", err }
        jar, err := zip.OpenReader(file)
        if err != nil { return "", fmt.Errorf("Invalid bundletool JAR: %w", err) }
        jar.Close()
        return moveTool(file, filepath.Join(a.cache, "bundletool.jar"))
    }
    return "", errors.New("Unknown tool")
}

func (a *app) download(url, destination, checksum string) error {
    fmt.Fprintln(a.err, "==> Fetching downloads for:", safeLinkLabel(url))
    request, err := http.NewRequestWithContext(a.ctx, http.MethodGet, url, nil)
    if err != nil { return err }
    client := &http.Client{Timeout: 10*time.Minute}
    response, err := client.Do(request)
    if err != nil { return err }
    defer response.Body.Close()
    if response.StatusCode != http.StatusOK { return fmt.Errorf("Download returned %s", response.Status) }
    file, err := os.Create(destination)
    if err != nil { return err }
    digest := sha256.New()
    progress := newDownloadProgress(a.err, response.ContentLength)
    size, copyErr := io.Copy(io.MultiWriter(file, digest, progress), response.Body)
    progress.finish()
    closeErr := file.Close()
    if copyErr != nil { return copyErr }
    if closeErr != nil { return closeErr }
    if response.ContentLength >= 0 && size != response.ContentLength { return errors.New("Incomplete tool download") }
    if checksum != "" && !strings.EqualFold(hex.EncodeToString(digest.Sum(nil)), checksum) { return errors.New("SHA256 verification failed") }
    progress.complete(filepath.Base(safeLinkLabel(url)))
    return nil
}

func moveTool(source, destination string) (string, error) {
    // The old path was not usable; replace it only after download/extraction succeeds.
    if err := os.RemoveAll(destination); err != nil { return "", err }
    if err := os.Rename(source, destination); err != nil { return "", err }
    return destination, nil
}

func (a *app) installADB(folder string) (string, error) {
    system := map[string]string{"darwin": "darwin", "linux": "linux", "windows": "windows"}[runtime.GOOS]
    if system == "" || (runtime.GOOS == "linux" && runtime.GOARCH != "amd64") {
        return "", errors.New("Google has no compatible Platform Tools download; install adb with your system package manager, then retry")
    }
    archive := filepath.Join(folder, "adb.zip")
    if err := a.download("https://dl.google.com/android/repository/platform-tools-latest-" + system + ".zip", archive, ""); err != nil { return "", err }
    unpack := filepath.Join(folder, "unpack")
    if err := extractZIP(archive, unpack); err != nil { return "", err }
    path := filepath.Join(unpack, "platform-tools", binaryName("adb"))
    if err := os.Chmod(path, 0o755); err != nil { return "", err }
    if _, err := a.run(a.ctx, true, path, "version"); err != nil { return "", err }
    destination, err := moveTool(filepath.Join(unpack, "platform-tools"), filepath.Join(a.cache, "platform-tools"))
    return filepath.Join(destination, binaryName("adb")), err
}

func (a *app) installIOS(folder string) (string, error) {
    system := map[string]string{"darwin": "mac", "linux": "linux", "windows": "win"}[runtime.GOOS]
    checksums := map[string]string{
        "mac": "100f225bfdd039081bcbdaf45029df3e67032673f3e0ca1fa3c050898c230c57",
        "linux": "a55fdb4c507391c0548252e01d1deb6ae3c7fd99cfbce842c6f80569cc125604",
        "win": "939c6bcaafed183a92afb9f79cc11b1f935fa6389bfc94d3902e3f52c4dff3fe",
    }
    if system == "" || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") { return "", errors.New("Provide a compatible go-ios executable via FERRIE_IOS") }
    archive := filepath.Join(folder, "ios.zip")
    if err := a.download("https://github.com/danielpaulus/go-ios/releases/download/v1.3.2/go-ios-" + system + ".zip", archive, checksums[system]); err != nil { return "", err }
    unpack := filepath.Join(folder, "go-ios")
    if err := extractZIP(archive, unpack); err != nil { return "", err }
    path := filepath.Join(unpack, binaryName("ios"))
    if runtime.GOOS == "linux" {
        if err := os.Rename(filepath.Join(unpack, "ios-" + runtime.GOARCH), path); err != nil { return "", err }
    }
    if err := os.Chmod(path, 0o755); err != nil { return "", err }
    if _, err := a.run(a.ctx, true, path, "--version"); err != nil { return "", err }
    destination, err := moveTool(unpack, filepath.Join(a.cache, "go-ios"))
    return filepath.Join(destination, binaryName("ios")), err
}

func (a *app) installJava(folder string) (string, error) {
    system := map[string]string{"darwin": "mac", "linux": "linux", "windows": "windows"}[runtime.GOOS]
    arch := map[string]string{"amd64": "x64", "arm64": "aarch64"}[runtime.GOARCH]
    if system == "" || arch == "" { return "", errors.New("Provide Java 17+ via JAVA_HOME or FERRIE_JAVA") }
    metadata := filepath.Join(folder, "java.json")
    if err := a.download("https://api.adoptium.net/v3/assets/latest/17/hotspot?architecture=" + arch + "&image_type=jre&os=" + system, metadata, ""); err != nil { return "", err }
    data, err := os.ReadFile(metadata)
    if err != nil { return "", err }
    var assets []struct { Binary struct { Package struct { Link, Checksum string } } }
    if err := json.Unmarshal(data, &assets); err != nil { return "", err }
    if len(assets) == 0 || len(assets[0].Binary.Package.Checksum) != 64 { return "", errors.New("No compatible Java download found") }
    pkg := assets[0].Binary.Package
    if !strings.HasPrefix(pkg.Link, "https://") { return "", errors.New("Java download must use HTTPS") }
    archive := filepath.Join(folder, "java.archive")
    if err := a.download(pkg.Link, archive, pkg.Checksum); err != nil { return "", err }
    unpack := filepath.Join(folder, "unpack")
    if strings.HasSuffix(pkg.Link, ".zip") { err = extractZIP(archive, unpack) } else { err = extractTar(archive, unpack) }
    if err != nil { return "", err }
    entries, err := os.ReadDir(unpack)
    if err != nil { return "", err }
    if len(entries) != 1 || !entries[0].IsDir() { return "", errors.New("Unexpected Java archive layout") }
    root := filepath.Join(unpack, entries[0].Name())
    relative := filepath.Join("bin", binaryName("java"))
    if runtime.GOOS == "darwin" { relative = filepath.Join("Contents", "Home", "bin", "java") }
    if !a.javaUsable(filepath.Join(root, relative)) { return "", errors.New("Downloaded Java is not usable") }
    destination, err := moveTool(root, filepath.Join(a.cache, "java"))
    return filepath.Join(destination, relative), err
}

func archivePath(root, name string) (string, error) {
    name = strings.ReplaceAll(name, `\`, "/")
    if !filepath.IsLocal(name) || strings.Contains(name, ":") { return "", fmt.Errorf("Unsafe archive path: %s", name) }
    return filepath.Join(root, name), nil
}

func copyArchiveFile(path string, reader io.Reader, mode os.FileMode) error {
    if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { return err }
    file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm()|0o600)
    if err != nil { return err }
    _, copyErr := io.Copy(file, reader)
    closeErr := file.Close()
    if copyErr != nil { return copyErr }
    return closeErr
}

func extractZIP(archive, root string) error {
    reader, err := zip.OpenReader(archive)
    if err != nil { return err }
    defer reader.Close()
    for _, member := range reader.File {
        path, err := archivePath(root, member.Name)
        if err != nil { return err }
        if member.Mode()&os.ModeSymlink != 0 { return errors.New("Unexpected symlink in ZIP archive") }
        if member.FileInfo().IsDir() {
            if err := os.MkdirAll(path, 0o755); err != nil { return err }
            continue
        }
        source, err := member.Open()
        if err != nil { return err }
        err = copyArchiveFile(path, source, member.Mode())
        source.Close()
        if err != nil { return err }
    }
    return nil
}

func extractTar(archive, root string) error {
    file, err := os.Open(archive)
    if err != nil { return err }
    defer file.Close()
    compressed, err := gzip.NewReader(file)
    if err != nil { return err }
    defer compressed.Close()
    reader := tar.NewReader(compressed)
    // Create symlinks last, so no later archive member can traverse a link.
    links := map[string]string{}
    for {
        member, err := reader.Next()
        if err == io.EOF { break }
        if err != nil { return err }
        path, err := archivePath(root, member.Name)
        if err != nil { return err }
        switch member.Typeflag {
        case tar.TypeDir:
            if err := os.MkdirAll(path, 0o755); err != nil { return err }
        case tar.TypeReg, tar.TypeRegA:
            if err := copyArchiveFile(path, reader, os.FileMode(member.Mode)); err != nil { return err }
        case tar.TypeSymlink:
            if filepath.IsAbs(member.Linkname) { return errors.New("Unsafe archive link") }
            if _, err := archivePath(root, filepath.Join(filepath.Dir(member.Name), member.Linkname)); err != nil { return err }
            links[path] = member.Linkname
        default: return fmt.Errorf("Unsupported archive entry: %s", member.Name)
        }
    }
    for path, target := range links {
        if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { return err }
        if err := os.Symlink(target, path); err != nil { return err }
    }
    return nil
}
