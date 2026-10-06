package main

import (
    "archive/zip"
    "bytes"
    "context"
    "errors"
    "fmt"
    "io"
    "net/http"
    "net/http/httptest"
    "net/url"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func appArchive(t *testing.T, name string) []byte {
    t.Helper()
    var buffer bytes.Buffer
    archive := zip.NewWriter(&buffer)
    file, err := archive.Create(name)
    if err != nil { t.Fatal(err) }
    if name == "AndroidManifest.xml" { file.Write([]byte(`<manifest package="com.example.app"/>`)) } else { file.Write([]byte("fixture")) }
    if err := archive.Close(); err != nil { t.Fatal(err) }
    return buffer.Bytes()
}

func TestTargetURLDetection(t *testing.T) {
    for _, target := range []string{"https://example.com/app.apk?token=x", "HTTP://example.com/app", " https://example.com/app ", "itms-services://?action=download-manifest&url=https://example.com/app.plist", "blob:https://example.com/id", "https://", "https://%", "ftp://example.com/app.apk"} {
        if !isURLTarget(target) { t.Errorf("URL treated as file: %q", target) }
    }
    for _, target := range []string{"app.apk", "./app with spaces.ipa", "/tmp/app.apk", "~/app.apk", `C:\apps\app.apk`, "C:/apps/app.apk", `\\server\share\app.apk`, "build:debug.apk", "./cache/https://app.apk"} {
        if isURLTarget(target) { t.Errorf("File treated as URL: %q", target) }
    }
    if _, err := parse([]string{"--url", "https://example.com/app.apk"}); err == nil || !strings.Contains(err.Error(), "--target") { t.Fatalf("Missing migration hint: %v", err) }
    for _, target := range []string{"https://", "blob:https://example.com/id", "ftp://example.com/app.apk"} {
        err := testApp(t).execute([]string{"--target", target})
        if err == nil || strings.Contains(err.Error(), "regular file") { t.Fatalf("Wrong URL error for %q: %v", target, err) }
    }
}

func TestURLArguments(t *testing.T) {
    for _, args := range [][]string{{"--target", "https://example.com/app", "--android", "--device", "serial"}, {"--target=https://example.com/app", "--ios"}} {
        got, err := parse(args)
        if err != nil || got.target != "https://example.com/app" || got.platform == "" { t.Fatalf("%+v %v", got, err) }
    }
    for _, args := range [][]string{{"--target"}, {"--target", ""}, {"--url", "https://example.com/app"}, {"--android"}, {"--target", "a", "--android", "--ios"}, {"--target", "a", "--help"}, {"--target", "a", "--ios=yes"}, {"--target", "a", "--target", "b"}, {"--target", "a.apk", "--android"}} {
        if _, err := parse(args); err == nil { t.Fatalf("Accepted %v", args) }
    }
    if similar("--andriod") != "--android" { t.Fatal("Missing suggestion") }
}

func TestURLPackageTypesAndCleanup(t *testing.T) {
    for _, item := range []struct { name, extension, platform string }{
        {"AndroidManifest.xml", ".apk", "android"}, {"BundleConfig.pb", ".aab", "android"}, {"toc.pb", ".apks", "android"}, {"Payload/Test.app/Info.plist", ".ipa", "ios"},
    } {
        t.Run(item.extension, func(t *testing.T) {
            data := appArchive(t, item.name)
            server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
                // Filename and Content-Type are intentionally misleading; inspect content.
                w.Header().Set("Content-Disposition", `attachment; filename="../../wrong.exe"`)
                w.Header().Set("Content-Type", "application/octet-stream")
                w.Write(data)
            }))
            defer server.Close()
            source, err := testApp(t).resolveURL(server.URL+"/blob?token=secret", item.platform)
            if err != nil { t.Fatal(err) }
            if filepath.Ext(source.path) != item.extension { t.Fatal(source) }
            source.cleanup()
            if _, err := os.Stat(filepath.Dir(source.path)); !os.IsNotExist(err) { t.Fatal("Download directory retained") }
        })
    }
}

func TestURLMergedPage(t *testing.T) {
    apk, ipa := appArchive(t, "AndroidManifest.xml"), appArchive(t, "Payload/Test.app/Info.plist")
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
        switch req.URL.Path {
        case "/": fmt.Fprint(w, `<a href='/app.apk'>Android</a><a href="/app.ipa">iOS</a>`)
        case "/app.apk": w.Write(apk)
        case "/app.ipa": w.Write(ipa)
        }
    }))
    defer server.Close()
    if _, err := testApp(t).resolveURL(server.URL, ""); err == nil || !strings.Contains(err.Error(), "--android or --ios") { t.Fatal(err) }
    for _, platform := range []string{"android", "ios"} {
        source, err := testApp(t).resolveURL(server.URL, platform)
        if err != nil { t.Fatal(err) }
        defer source.cleanup()
        if source.platform != platform { t.Fatal(source) }
    }
    a := testApp(t)
    a.interactive, a.in = true, strings.NewReader("2\n")
    source, err := a.resolveURL(server.URL, "")
    if err != nil { t.Fatal(err) }
    defer source.cleanup()
    if source.platform != "ios" { t.Fatal(source) }
}

func TestURLRedirectCookiesAndManifest(t *testing.T) {
    ipa := appArchive(t, "Payload/Test.app/Info.plist")
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
        switch req.URL.Path {
        case "/":
            http.SetCookie(w, &http.Cookie{Name: "session", Value: "test"})
            http.Redirect(w, req, "itms-services://?action=download-manifest&url="+url.QueryEscape("http://"+req.Host+"/manifest"), http.StatusFound)
        case "/manifest":
            if _, err := req.Cookie("session"); err != nil { t.Error("Cookie not retained") }
            fmt.Fprint(w, `<?xml version="1.0"?><plist><dict><key>items</key><array><dict><key>assets</key><array><dict><key>kind</key><string>display-image</string><key>url</key><string>/icon.png</string></dict><dict><key>url</key><string>/package</string><key>kind</key><string>software-package</string></dict></array></dict></array></dict></plist>`)
        case "/package": w.Write(ipa)
        default: t.Error("Followed wrong asset"); http.NotFound(w, req)
        }
    }))
    defer server.Close()
    source, err := testApp(t).resolveURL(server.URL, "")
    if err != nil { t.Fatal(err) }
    defer source.cleanup()
    if source.platform != "ios" { t.Fatal(source) }
}

func TestURLFailures(t *testing.T) {
    for _, raw := range []string{"blob:https://example.com/id", "file:///tmp/app.apk", "ftp://example.com/app.apk", "https://user:secret@example.com/a", "not a url", "https://%"} {
        if _, err := testApp(t).resolveURL(raw, ""); err == nil { t.Fatalf("Accepted %s", raw) }
    }
    apk, unrelated := appArchive(t, "AndroidManifest.xml"), appArchive(t, "readme.txt")
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
        switch req.URL.Path {
        case "/loop": http.Redirect(w, req, "/loop", http.StatusFound)
        case "/login.apk": fmt.Fprint(w, "<html>Please log in</html>")
        case "/wrong": w.Write(apk)
        case "/zip": w.Write(unrelated)
        case "/truncated": w.Write([]byte("PK\x03\x04bad"))
        case "/huge": w.Header().Set("Content-Length", "99999999999"); w.Write([]byte("PK\x03\x04"))
        default: http.Error(w, "secret-server-data", http.StatusForbidden)
        }
    }))
    defer server.Close()
    for _, path := range []string{"/loop", "/login.apk", "/wrong", "/zip", "/truncated", "/huge", "/denied?secret=value"} {
        _, err := testApp(t).resolveURL(server.URL+path, "ios")
        if err == nil { t.Fatal("Accepted", path) }
        if strings.Contains(err.Error(), "secret") { t.Fatal("Leaked URL or response", err) }
    }
}

func TestURLUsesExistingInstallAndCleansUp(t *testing.T) {
    data := appArchive(t, "AndroidManifest.xml")
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { w.Write(data) }))
    defer server.Close()
    adb := fakeTool(t, "adb")
    a := testApp(t)
    var downloaded string
    a.run = func(_ context.Context, capture bool, name string, args ...string) (string, error) {
        if name != adb { t.Fatal(name) }
        if strings.Join(args, " ") == "devices -l" { return "List of devices attached\nserial device model:Test\n", nil }
        if len(args) > 2 && args[2] == "shell" { return "", nil }
        if len(args) != 5 || args[0] != "-s" || args[1] != "serial" || args[2] != "install" || args[3] != "-r" { t.Fatal(args) }
        downloaded = args[4]
        if _, err := os.Stat(downloaded); err != nil { t.Fatal(err) }
        return "", errors.New("fixture installation failure")
    }
    if err := a.execute([]string{"--target", server.URL, "--device", "serial"}); err == nil { t.Fatal("Failure swallowed") }
    if downloaded == "" { t.Fatal("Install not called") }
    if _, err := os.Stat(downloaded); !os.IsNotExist(err) { t.Fatal("Download leaked on installation failure") }
}

type urlTransport func(*http.Request) (*http.Response, error)
func (f urlTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestPgyerPublicPage(t *testing.T) {
    original := http.DefaultTransport
    defer func() { http.DefaultTransport = original }()
    apk := appArchive(t, "AndroidManifest.xml")
    seenInstall := false
    http.DefaultTransport = urlTransport(func(req *http.Request) (*http.Response, error) {
        body := ""
        if strings.HasPrefix(req.URL.Path, "/app/install/") {
            seenInstall = true
            if req.URL.Query().Get("installToken") != "a+b/c=" || req.URL.Query().Get("timeSign") != "6162636465666768696a6b6c" { t.Fatal("Incorrect token encoding") }
            if req.Header.Get("Referer") == "" { t.Fatal("Missing referer") }
            body = string(apk)
        } else {
            body = `<div class="merge-download-options-view"></div><script>aKey='0123456789abcdef0123456789abcdef';aType='android';installToken='a%2Bb%2Fc%3D';timeSign='4142434445464748494a4b4c';</script>`
        }
        return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
    })
    source, err := testApp(t).resolveURL("https://www.pgyer.com/example", "android")
    if err != nil { t.Fatal(err) }
    defer source.cleanup()
    if !seenInstall { t.Fatal("Missing public install request") }
    if _, err := testApp(t).resolveURL("https://www.pgyer.com/example", ""); err == nil { t.Fatal("Merged page needs explicit platform") }
    if _, err := testApp(t).resolveURL("https://www.pgyer.com/example", "ios"); err == nil { t.Fatal("Wrong build accepted") }
}

func TestStoreLinks(t *testing.T) {
    a := testApp(t)
    source, err := a.resolveURL("https://apps.apple.com/us/app/example/id12345", "ios")
    if err != nil { t.Fatal(err) }
    if err := a.openStore(source, ""); err == nil || !strings.Contains(err.Error(), "IPA") { t.Fatal(err) }
    if _, err := a.resolveURL("https://play.google.com/store/apps/details?id=com.example.app", "ios"); err == nil { t.Fatal("Platform mismatch accepted") }
    adb := fakeTool(t, "adb")
    called := false
    storeOutput := "Starting: Intent\nStatus: ok"
    a.run = func(_ context.Context, capture bool, name string, args ...string) (string, error) {
        if name != adb { t.Fatal(name) }
        if strings.Join(args, " ") == "devices -l" { return "List of devices attached\nserial device\n", nil }
        expected := "-s serial shell am start -W -a android.intent.action.VIEW -d market://details?id=com.example.app -p com.android.vending"
        if strings.Join(args, " ") != expected { t.Fatal(args) }
        called = true
        return storeOutput, nil
    }
    if err := a.execute([]string{"--target", "https://play.google.com/store/apps/details?id=com.example.app"}); err != nil { t.Fatal(err) }
    if !called || strings.Contains(a.out.(*bytes.Buffer).String(), "Installed successfully") { t.Fatal("Store status is incorrect") }
    storeOutput = "Starting: Intent"
    if err := a.execute([]string{"--target", "https://play.google.com/store/apps/details?id=com.example.app"}); err == nil { t.Fatal("Unconfirmed store launch reported as success") }
}
