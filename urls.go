package main

import (
    "archive/zip"
    "bufio"
    "encoding/hex"
    "encoding/xml"
    "errors"
    "fmt"
    "html"
    "io"
    "net/http"
    "net/http/cookiejar"
    "net/url"
    "os"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "time"
)

const maxPackageBytes int64 = 8 << 30
const maxPageBytes int64 = 2 << 20

type urlSource struct {
    path, store, platform string
}

func (s urlSource) cleanup() {
    if s.path != "" { os.RemoveAll(filepath.Dir(s.path)) }
}

type urlResolver struct {
    app *app
    client *http.Client
    platform string
    seen map[string]bool
}

func (a *app) resolveURL(raw, platform string) (urlSource, error) {
    jar, _ := cookiejar.New(nil)
    r := urlResolver{app: a, platform: platform, seen: map[string]bool{}, client: &http.Client{
        Jar: jar, Timeout: 15*time.Minute,
        // Handle redirects ourselves, including Apple's itms-services manifest links.
        CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
    }}
    return r.resolve(raw, "", 0)
}

// Keep Windows drive paths and local names containing a colon on the file path.
func isURLTarget(raw string) bool {
    value := strings.ToLower(strings.TrimSpace(raw))
    if len(value) >= 2 && value[1] == ':' { return false }
    parsed, err := url.Parse(value)
    return (err == nil && len(parsed.Scheme) > 1 && strings.HasPrefix(value, parsed.Scheme+"://")) || strings.HasPrefix(value, "http:") ||
        strings.HasPrefix(value, "https:") || strings.HasPrefix(value, "itms-services:") || strings.HasPrefix(value, "blob:")
}

func parseSourceURL(raw string) (*url.URL, error) {
    u, err := url.Parse(strings.TrimSpace(raw))
    if err != nil { return nil, errors.New("Invalid URL") }
    if u.Scheme == "blob" { return nil, errors.New("blob: URLs only exist inside their browser session; use the underlying HTTP(S) download URL or --target PATH") }
    if u.Scheme == "itms-services" {
        manifest := u.Query().Get("url")
        if manifest == "" { return nil, errors.New("The iOS installation link has no manifest URL") }
        return parseSourceURL(manifest)
    }
    if (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
        return nil, errors.New("Use an HTTP(S) URL without embedded username/password")
    }
    u.Fragment = ""
    return u, nil
}

func (r *urlResolver) resolve(raw, referer string, depth int) (urlSource, error) {
    empty := urlSource{}
    if depth >= 12 { return empty, errors.New("Too many download redirects/pages; use a direct package URL") }
    u, err := parseSourceURL(raw)
    if err != nil { return empty, err }
    if store := storePlatform(u); store != "" {
        if r.platform != "" && store != r.platform { return empty, errors.New("Store link does not match the selected platform") }
        return urlSource{store: u.String(), platform: store}, nil
    }
    key := r.platform + " " + u.String()
    if r.seen[key] { return empty, errors.New("Download page loop; use a direct package URL") }
    r.seen[key] = true
    req, err := http.NewRequestWithContext(r.app.ctx, http.MethodGet, u.String(), nil)
    if err != nil { return empty, errors.New("Invalid download request") }
    agent := "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 Chrome/120.0 Mobile Safari/537.36"
    if r.platform == "ios" { agent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1" }
    req.Header.Set("User-Agent", agent)
    if previous, err := url.Parse(referer); err == nil && previous.Host == u.Host {
        req.Header.Set("Referer", referer)
    }
    resp, err := r.client.Do(req)
    if err != nil {
        if r.app.ctx.Err() != nil { return empty, r.app.ctx.Err() }
        return empty, fmt.Errorf("Could not download from %s; check the network or obtain a fresh download link", u.Hostname())
    }
    defer resp.Body.Close()
    if resp.StatusCode >= 300 && resp.StatusCode < 400 {
        location := resp.Header.Get("Location")
        if location == "" { return empty, errors.New("Download redirect is missing its destination") }
        next, err := u.Parse(location)
        if err != nil { return empty, errors.New("Invalid download redirect") }
        resp.Body.Close()
        return r.resolve(next.String(), u.String(), depth+1)
    }
    if resp.StatusCode != http.StatusOK {
        return empty, fmt.Errorf("Download from %s returned HTTP %d; the link may be expired, private or require browser verification. Obtain a direct package URL or use --target PATH", u.Hostname(), resp.StatusCode)
    }
    body := bufio.NewReader(resp.Body)
    signature, _ := body.Peek(4)
    if len(signature) == 4 && string(signature[:2]) == "PK" {
        return r.savePackage(body, resp.ContentLength, u.Hostname())
    }
    data, err := io.ReadAll(io.LimitReader(body, maxPageBytes+1))
    if err != nil { return empty, errors.New("Could not read download page") }
    if int64(len(data)) > maxPageBytes { return empty, errors.New("Response is not a supported app package or is an oversized download page") }
    resp.Body.Close()
    page := string(data)
    if strings.Contains(page, "<plist") {
        next, err := manifestPackage(data)
        if err != nil { return empty, err }
        if r.platform == "android" { return empty, errors.New("iOS manifest does not match --android") }
        r.platform = "ios"
        target, err := u.Parse(next)
        if err != nil { return empty, errors.New("Invalid package URL in iOS manifest") }
        return r.resolve(target.String(), u.String(), depth+1)
    }
    if isPgyer(u) && jsValue(page, "aKey") != "" {
        return r.pgyer(u, page, depth)
    }
    candidates := pageLinks(u, page)
    if len(candidates) == 0 {
        return empty, errors.New("No public package link found. This page may need JavaScript, login, a password or CAPTCHA; download the package in a browser and use --target PATH, or supply its direct HTTP(S) URL")
    }
    android, ios := false, false
    for _, item := range candidates { android = android || item.platform == "android"; ios = ios || item.platform == "ios" }
    if android && ios && r.platform == "" {
        r.platform, err = r.app.selectPlatform()
        if err != nil { return empty, err }
    }
    filtered := []pageLink{}
    for _, item := range candidates {
        if r.platform == "" || item.platform == "" || item.platform == r.platform { filtered = append(filtered, item) }
    }
    if len(filtered) == 0 { return empty, errors.New("Page has no download for the selected platform") }
    index := 0
    if len(filtered) > 1 {
        if !r.app.interactive { return empty, errors.New("Page has multiple package links; use a direct package URL in non-interactive sessions") }
        for i, item := range filtered { fmt.Fprintf(r.app.out, "%d. %s\n", i+1, safeLinkLabel(item.url)) }
        fmt.Fprintf(r.app.out, "Select package [1-%d], or q to cancel: ", len(filtered))
        answer, err := r.app.readLine()
        number, parseErr := strconv.Atoi(strings.TrimSpace(answer))
        if err != nil || parseErr != nil || number < 1 || number > len(filtered) { return empty, errors.New("Package selection cancelled or invalid") }
        index = number-1
    }
    return r.resolve(filtered[index].url, u.String(), depth+1)
}

func (a *app) selectPlatform() (string, error) {
    if !a.interactive { return "", errors.New("This page provides Android and iOS; specify --android or --ios in non-interactive sessions") }
    fmt.Fprint(a.out, "Install which platform? [1] Android [2] iOS (q to cancel): ")
    answer, err := a.readLine()
    if err != nil { return "", errors.New("Platform selection cancelled") }
    switch strings.ToLower(strings.TrimSpace(answer)) {
    case "1", "android": return "android", nil
    case "2", "ios": return "ios", nil
    default: return "", errors.New("Platform selection cancelled or invalid")
    }
}

func (r *urlResolver) savePackage(body io.Reader, length int64, host string) (urlSource, error) {
    empty := urlSource{}
    if length > maxPackageBytes { return empty, errors.New("Package exceeds the 8 GiB download limit") }
    dir, err := os.MkdirTemp("", "ferrie-download-*")
    if err != nil { return empty, err }
    keep := false
    defer func() { if !keep { os.RemoveAll(dir) } }()
    path := filepath.Join(dir, "package.download")
    file, err := os.Create(path)
    if err != nil { return empty, err }
    fmt.Fprintf(r.app.out, "Downloading app package from %s...\n", host)
    size, copyErr := io.Copy(file, io.LimitReader(body, maxPackageBytes+1))
    closeErr := file.Close()
    if copyErr != nil { return empty, errors.New("Package download interrupted; retry with a fresh link") }
    if closeErr != nil { return empty, closeErr }
    if size > maxPackageBytes { return empty, errors.New("Package exceeds the 8 GiB download limit") }
    extension, err := packageExtension(path)
    if err != nil { return empty, err }
    platform := "android"
    if extension == ".ipa" { platform = "ios" }
    if r.platform != "" && r.platform != platform { return empty, errors.New("Downloaded package does not match the selected platform") }
    target := filepath.Join(dir, "app"+extension)
    if err := os.Rename(path, target); err != nil { return empty, err }
    keep = true
    fmt.Fprintf(r.app.out, "Downloaded %s package (%d bytes).\n", strings.TrimPrefix(extension, "."), size)
    return urlSource{path: target, platform: platform}, nil
}

func packageExtension(path string) (string, error) {
    archive, err := zip.OpenReader(path)
    if err != nil { return "", errors.New("Downloaded file is not a valid APK/APKS/AAB/IPA archive") }
    defer archive.Close()
    kinds := map[string]bool{}
    for _, file := range archive.File {
        switch file.Name {
        case "AndroidManifest.xml": kinds[".apk"] = true
        case "BundleConfig.pb": kinds[".aab"] = true
        case "toc.pb": kinds[".apks"] = true
        }
        parts := strings.Split(file.Name, "/")
        if len(parts) == 3 && parts[0] == "Payload" && strings.HasSuffix(parts[1], ".app") && parts[2] == "Info.plist" { kinds[".ipa"] = true }
    }
    if len(kinds) != 1 { return "", errors.New("Downloaded archive is not an unambiguous APK/APKS/AAB/IPA package") }
    for kind := range kinds { return kind, nil }
    return "", errors.New("Unsupported archive")
}

func safeLinkLabel(raw string) string {
    u, err := url.Parse(raw)
    if err != nil { return "download" }
    return u.Hostname()+u.EscapedPath()
}

func storePlatform(u *url.URL) string {
    host := strings.ToLower(u.Hostname())
    if host == "play.google.com" && u.Path == "/store/apps/details" { return "android" }
    if (host == "apps.apple.com" || host == "itunes.apple.com") && strings.Contains(u.Path, "/app/") { return "ios" }
    return ""
}

func (a *app) openStore(source urlSource, id string) error {
    if source.platform == "ios" {
        return errors.New("App Store links do not provide a downloadable signed IPA. Open this app's page in App Store on the iPhone/iPad to install; for Ferrie installation, obtain a signed IPA and use --target PATH_OR_DIRECT_IPA_URL")
    }
    u, _ := url.Parse(source.store)
    packageID := u.Query().Get("id")
    if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z][A-Za-z0-9_]*)+$`).MatchString(packageID) { return errors.New("Google Play link is missing a valid app id") }
    if err := a.prepare("adb"); err != nil { return err }
    devices, err := a.discoverForInstall("android", id)
    if err != nil { return err }
    selected, err := a.selectDevice(devices, id)
    if err != nil { return err }
    adb, err := a.tool("adb", false)
    if err != nil { return err }
    output, err := a.run(a.ctx, true, adb, "-s", selected.id, "shell", "am", "start", "-W", "-a", "android.intent.action.VIEW", "-d", "market://details?id="+packageID, "-p", "com.android.vending")
    if err != nil { return err }
    if !strings.Contains(output, "Status: ok") || strings.Contains(output, "Error:") || strings.Contains(output, "Exception") { return errors.New("Could not open Google Play; verify that it is installed and enabled on this device") }
    fmt.Fprintln(a.out, "Opened Google Play on the selected device. Complete installation on the device; Ferrie has not installed the app.")
    return nil
}

type pageLink struct { url, platform string }
var anchorPattern = regexp.MustCompile(`(?is)<a\b([^>]*)>`)
var attributePattern = regexp.MustCompile(`(?is)\b(href|download)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)

func pageLinks(base *url.URL, page string) []pageLink {
    result := []pageLink{}
    seen := map[string]bool{}
    for _, anchor := range anchorPattern.FindAllStringSubmatch(page, -1) {
        href, filename := "", ""
        for _, attr := range attributePattern.FindAllStringSubmatch(anchor[1], -1) {
            value := attr[2]+attr[3]+attr[4]
            if strings.EqualFold(attr[1], "href") { href = html.UnescapeString(value) } else { filename = value }
        }
        u, err := base.Parse(href)
        if err != nil || href == "" { continue }
        ext := strings.ToLower(filepath.Ext(u.Path))
        if filename != "" { ext = strings.ToLower(filepath.Ext(filename)) }
        platform := storePlatform(u)
        if u.Scheme == "itms-services" || ext == ".ipa" || ext == ".plist" { platform = "ios" }
        if ext == ".apk" || ext == ".aab" || ext == ".apks" { platform = "android" }
        if platform == "" || seen[u.String()] { continue }
        if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "itms-services" { continue }
        seen[u.String()] = true
        result = append(result, pageLink{url: u.String(), platform: platform})
    }
    return result
}

// Parse each plist dictionary independently so image URLs cannot be mistaken for the IPA.
func manifestPackage(data []byte) (string, error) {
    decoder := xml.NewDecoder(strings.NewReader(string(data)))
    type dict struct { key string; values map[string]string }
    stack := []dict{}
    packages := []string{}
    for {
        token, err := decoder.Token()
        if err == io.EOF { break }
        if err != nil { return "", errors.New("Invalid iOS installation manifest") }
        switch node := token.(type) {
        case xml.StartElement:
            if node.Name.Local == "dict" { stack = append(stack, dict{values: map[string]string{}}) }
            if len(stack) > 0 && (node.Name.Local == "key" || node.Name.Local == "string") {
                var value string
                if err := decoder.DecodeElement(&value, &node); err != nil { return "", errors.New("Invalid iOS installation manifest") }
                top := &stack[len(stack)-1]
                if node.Name.Local == "key" { top.key = value } else { top.values[top.key] = value; top.key = "" }
            }
        case xml.EndElement:
            if node.Name.Local == "dict" && len(stack) > 0 {
                top := stack[len(stack)-1]
                if top.values["kind"] == "software-package" && top.values["url"] != "" { packages = append(packages, top.values["url"]) }
                stack = stack[:len(stack)-1]
            }
        }
    }
    if len(packages) != 1 { return "", errors.New("iOS manifest must contain exactly one software-package URL") }
    return packages[0], nil
}

func isPgyer(u *url.URL) bool {
    host := strings.ToLower(u.Hostname())
    return host == "www.pgyer.com" || host == "pgyer.com"
}

func jsValue(page, name string) string {
    match := regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\s*=\s*['"]([^'"]*)['"]`).FindStringSubmatch(page)
    if len(match) < 2 { return "" }
    return match[1]
}

func (r *urlResolver) pgyer(u *url.URL, page string, depth int) (urlSource, error) {
    empty := urlSource{}
    if strings.Contains(page, "merge-download-options-view") && r.platform == "" {
        platform, err := r.app.selectPlatform()
        if err != nil { return empty, err }
        r.platform = platform
        // The same public page selects the corresponding build from the mobile user agent.
        return r.resolve(u.String(), u.String(), depth+1)
    }
    platform := jsValue(page, "aType")
    if platform != "android" && platform != "ios" { return empty, errors.New("This Pgyer page does not provide an Android/iOS app") }
    if r.platform != "" && platform != r.platform { return empty, errors.New("Pgyer build does not match the selected platform; use the merged app page or the correct build URL") }
    r.platform = platform
    key := jsValue(page, "aKey")
    if !regexp.MustCompile(`^[a-fA-F0-9]{32}$`).MatchString(key) { return empty, errors.New("Unrecognized Pgyer build; use a direct package URL") }
    for _, flag := range []string{"isTeamInstall", "isTestFlight", "accountNotEnough", "isInstallEnd"} {
        if regexp.MustCompile(`\b`+flag+`\s*=\s*(?:true|'true'|"true")\s*[,;]`).MatchString(page) {
            return empty, errors.New("This Pgyer build requires its browser installation flow (team access, TestFlight or distribution restrictions); open the page or obtain a signed package directly")
        }
    }
    if amount, _ := strconv.ParseFloat(jsValue(page, "downloadPayMoney"), 64); amount > 0 {
        return empty, errors.New("This Pgyer download requires payment in the browser; provide the downloaded package with --target PATH")
    }
    token := jsValue(page, "installToken")
    if token == "" { return empty, errors.New("Pgyer has not provided a public download token; open the page to complete any password/login/verification, then use a direct package URL or --target PATH") }
    // Mirror the public page's URL construction; no private/paid API or credentials.
    decoded, err := url.QueryUnescape(token)
    if err != nil { return empty, errors.New("Invalid Pgyer download token") }
    query := url.Values{"time": {strconv.FormatInt(time.Now().UnixMilli(), 10)}, "installToken": {decoded}}
    if code := jsValue(page, "authcode"); code != "" {
        value, err := strconv.ParseInt(code, 10, 32)
        if err != nil { return empty, errors.New("Unrecognized Pgyer page token") }
        nonce := time.Now().UnixMilli() % 1000000
        query.Set("finalCode", fmt.Sprintf("%d%06d", value^nonce, nonce))
    }
    for _, field := range []string{"sig", "lang"} { if value := jsValue(page, field); value != "" { query.Set(field, value) } }
    stamp := jsValue(page, "timeSign")
    if len(stamp) >= 24 {
        bytes, err := hex.DecodeString(stamp[:24])
        if err != nil { return empty, errors.New("Unrecognized Pgyer download signature") }
        query.Set("timeSign", hex.EncodeToString([]byte(strings.ToLower(string(bytes)))))
    }
    next := *u
    next.Path, next.RawPath, next.RawQuery = "/app/install/"+key, "", query.Encode()
    return r.resolve(next.String(), u.String(), depth+1)
}
