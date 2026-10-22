package main

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "reflect"
    "strings"
    "testing"

    "github.com/leo1394/homebrew-ferrie/internal/selector"
)

func TestRecentURLHistory(t *testing.T) {
    a := testApp(t)
    for i := 0; i < 7; i++ {
        if err := a.rememberURL(fmt.Sprintf("https://example.com/%d", i)); err != nil { t.Fatal(err) }
    }
    if err := a.rememberURL(" HTTPS://EXAMPLE.COM/4#fragment "); err != nil { t.Fatal(err) }
    recent, err := a.recentURLs()
    expected := []string{"https://example.com/4", "https://example.com/6", "https://example.com/5", "https://example.com/3", "https://example.com/2"}
    if err != nil || !reflect.DeepEqual(recent, expected) { t.Fatalf("%v %v", recent, err) }
    info, err := os.Stat(filepath.Join(a.cache, "recent-urls.json"))
    if err != nil { t.Fatal(err) }
    if info.Mode().Perm() & 0077 != 0 { t.Fatalf("History is not private: %v", info.Mode()) }
    other := testApp(t)
    other.cache = a.cache
    persisted, err := other.recentURLs()
    if err != nil || !reflect.DeepEqual(persisted, expected) { t.Fatalf("%v %v", persisted, err) }
}

func TestRecentTargetDeviceGates(t *testing.T) {
    fakeTool(t, "adb")
    fakeTool(t, "ios")
    for _, item := range []struct { name, adb, ios, id, platform, want string }{
        {"none", "", `{"deviceList":[]}`, "", "", "exactly one"},
        {"many", "a device\nb device", `{"deviceList":[]}`, "", "", "exactly one"},
        {"offline", "a offline", `{"deviceList":[]}`, "", "", "offline"},
        {"unknown", "a device", `{"deviceList":[]}`, "b", "", "not connected"},
        {"mismatch", "a device", `{"deviceList":[]}`, "", "ios", "does not match"},
        {"single", "a device", `{"deviceList":[]}`, "", "", ""},
        {"explicit", "a device\nb device", `{"deviceList":[]}`, "b", "", ""},
        {"ios", "a device", `{"deviceList":["iphone"]}`, "iphone", "", ""},
    } {
        t.Run(item.name, func(t *testing.T) {
            a := testApp(t)
            a.interactive = true
            if err := a.rememberURL("https://example.com/app?token=secret"); err != nil { t.Fatal(err) }
            a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
                if args[0] == "list" { return item.ios, nil }
                return item.adb, nil
            }
            called := false
            a.pickRecent = func(_ context.Context, _ *os.File, _ io.Writer, _ selector.Prompt, items []selector.Candidate) (selector.Candidate, bool, error) {
                called = true
                if strings.Contains(items[0].Name, "secret") { t.Fatal("Query leaked") }
                return items[0], true, nil
            }
            args := arguments{device: item.id, platform: item.platform}
            err := a.chooseRecentTarget(&args)
            if item.want != "" {
                if err == nil || !strings.Contains(err.Error(), item.want) || called { t.Fatalf("%v called=%v", err, called) }
            } else {
                if err != nil || !called || args.target != "https://example.com/app?token=secret" || args.device == "" || args.platform == "" { t.Fatalf("%+v %v", args, err) }
                if item.id != "" && args.device != item.id { t.Fatal("Changed selected device") }
            }
        })
    }
}

func TestRecentTargetCancelEmptyAndNoninteractive(t *testing.T) {
    a := testApp(t)
    if err := a.execute(nil); err == nil || !strings.Contains(err.Error(), "interactive") { t.Fatal(err) }
    fakeTool(t, "adb")
    fakeTool(t, "ios")
    a.interactive = true
    a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
        if args[0] == "list" { return `{"deviceList":[]}`, nil }
        return "serial device", nil
    }
    if err := a.execute(nil); err == nil || !strings.Contains(err.Error(), "No recent") { t.Fatal(err) }
    if err := a.rememberURL("https://example.com/app"); err != nil { t.Fatal(err) }
    a.pickRecent = func(context.Context, *os.File, io.Writer, selector.Prompt, []selector.Candidate) (selector.Candidate, bool, error) {
        return selector.Candidate{}, false, nil
    }
    if err := a.execute(nil); err == nil || !strings.Contains(err.Error(), "cancelled") { t.Fatal(err) }
}

func TestSuccessfulURLInstallRecordsAndReplays(t *testing.T) {
    data := appArchive(t, "AndroidManifest.xml")
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
    defer server.Close()
    fakeTool(t, "adb")
    fakeTool(t, "ios")
    a := testApp(t)
    installs := 0
    a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
        if args[0] == "devices" { return "serial device", nil }
        if args[0] == "list" { return `{"deviceList":[]}`, nil }
        if len(args) == 5 && args[0] == "-s" && args[1] == "serial" && args[2] == "install" && args[3] == "-r" {
            installs++
            return "Success", nil
        }
        t.Fatalf("Unexpected command %v", args)
        return "", nil
    }
    if err := a.execute([]string{"--target", server.URL}); err != nil { t.Fatal(err) }
    recent, err := a.recentURLs()
    if err != nil || !reflect.DeepEqual(recent, []string{server.URL}) { t.Fatalf("%v %v", recent, err) }
    a.interactive = true
    a.pickRecent = func(_ context.Context, _ *os.File, _ io.Writer, _ selector.Prompt, items []selector.Candidate) (selector.Candidate, bool, error) {
        return items[0], true, nil
    }
    if err := a.execute([]string{"--device", "serial"}); err != nil { t.Fatal(err) }
    if installs != 2 { t.Fatalf("Got %d installs", installs) }
    recent, err = a.recentURLs()
    if err != nil || len(recent) != 1 { t.Fatalf("%v %v", recent, err) }
}
