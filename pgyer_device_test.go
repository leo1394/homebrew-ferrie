package main

import (
    "context"
    "io"
    "net/http"
    "os"
    "strings"
    "testing"
)

func TestPgyerUsesDeviceBeforeResolving(t *testing.T) {
    original := http.DefaultTransport
    defer func() { http.DefaultTransport = original }()
    fakeTool(t, "adb")
    fakeTool(t, "ios")
    for _, platform := range []string{"android", "ios"} {
        for _, slug := range []string{"clobotics-sam-master-ios-test", "clobotics-sam-master-test"} {
            t.Run(platform+"/"+slug, func(t *testing.T) {
                a := testApp(t)
                installed := false
                a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
                    if args[0] == "devices" {
                        if platform == "android" { return "serial device", nil }
                        return "", nil
                    }
                    if args[0] == "list" {
                        if platform == "ios" { return `{"deviceList":["serial"]}`, nil }
                        return `{"deviceList":[]}`, nil
                    }
                    if platform == "ios" && args[0] == "install" && strings.Contains(args[1], ".ipa") && args[2] == "--udid=serial" { installed = true; return "", nil }
                    if platform == "android" && len(args) == 5 && args[1] == "serial" && args[2] == "install" && strings.HasSuffix(args[4], ".apk") { installed = true; return "Success", nil }
                    t.Fatalf("Unexpected install %v", args)
                    return "", nil
                }
                kind := "AndroidManifest.xml"
                if platform == "ios" { kind = "Payload/Test.app/Info.plist" }
                data := appArchive(t, kind)
                http.DefaultTransport = urlTransport(func(req *http.Request) (*http.Response, error) {
                    gotIOS := strings.Contains(req.Header.Get("User-Agent"), "iPhone")
                    if gotIOS != (platform == "ios") { t.Fatalf("%s device received wrong User-Agent: %s", platform, req.Header.Get("User-Agent")) }
                    resp := &http.Response{StatusCode: 200, Header: http.Header{}, Request: req}
                    if strings.HasPrefix(req.URL.Path, "/app/install/") {
                        resp.Body = io.NopCloser(strings.NewReader(string(data)))
                    } else if req.URL.Path != "/selected-build" {
                        resp.StatusCode = 302
                        resp.Header.Set("Location", "/selected-build")
                        resp.Body = io.NopCloser(strings.NewReader(""))
                    } else {
                        // Real mobile pages have aType/aKey, without merge-download-options-view.
                        resp.Body = io.NopCloser(strings.NewReader("<script>aType='"+platform+"';aKey='0123456789abcdef0123456789abcdef';installToken='public';</script>"))
                    }
                    return resp, nil
                })
                if err := a.execute([]string{"--target", "https://www.pgyer.com/"+slug}); err != nil { t.Fatal(err) }
                if !installed { t.Fatal("Package was not installed") }
                if _, err := os.Stat(a.cache+"/recent-urls.json"); err != nil { t.Fatal(err) }
            })
        }
    }
}

func TestPgyerPlatformDeviceSelection(t *testing.T) {
    fakeTool(t, "adb")
    fakeTool(t, "ios")
    for _, item := range []struct { name, id, flag, want, failure string }{
        {"android ID", "android-id", "", "android", ""},
        {"ios ID", "ios-id", "", "ios", ""},
        {"mismatch", "ios-id", "android", "", "does not match"},
        {"unknown ID", "missing", "", "", "not connected"},
        {"ambiguous", "", "", "", "Multiple devices"},
        {"explicit platform", "", "ios", "ios", ""},
    } {
        t.Run(item.name, func(t *testing.T) {
            a := testApp(t)
            a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
                if args[0] == "devices" { return "android-id device", nil }
                return `{"deviceList":["ios-id"]}`, nil
            }
            args := arguments{target: "https://www.pgyer.com/clobotics-sam-master-test", device: item.id, platform: item.flag}
            err := a.prepareURLPlatform(&args)
            if item.failure != "" {
                if err == nil || !strings.Contains(err.Error(), item.failure) { t.Fatalf("%v", err) }
            } else if err != nil || args.platform != item.want { t.Fatalf("%+v %v", args, err) }
        })
    }
    a := testApp(t)
    // Download-only and generic URLs do not query devices.
    for _, args := range []arguments{
        {target: "https://www.pgyer.com/example", platform: "ios", downloadDir: t.TempDir()},
        {target: "https://example.com/app.apk"},
    } {
        if err := a.prepareURLPlatform(&args); err != nil { t.Fatal(err) }
    }
}
