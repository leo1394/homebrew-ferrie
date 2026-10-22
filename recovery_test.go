package main

import (
    "context"
    "errors"
    "path/filepath"
    "strings"
    "testing"
)

func TestInstallationRecoversADBDiscovery(t *testing.T) {
    for _, initial := range []string{"empty", "offline", "error"} {
        t.Run(initial, func(t *testing.T) {
            fakeTool(t, "adb")
            target := filepath.Join(t.TempDir(), "app.apk")
            identityFixture(t, target)
            a := testApp(t)
            var calls []string
            reads := 0
            a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
                line := strings.Join(args, " ")
                calls = append(calls, line)
                if line == "devices -l" {
                    reads++
                    if reads == 1 {
                        switch initial {
                        case "empty": return "List of devices attached\n", nil
                        case "offline": return "chosen offline\n", nil
                        case "error": return "", errors.New("USB read failed")
                        }
                    }
                    return "chosen device model:Test\n", nil
                }
                return "", nil
            }
            if err := a.execute([]string{"--target", target, "--device", "chosen"}); err != nil { t.Fatal(err) }
            want := []string{"devices -l", "kill-server", "start-server", "devices -l", "-s chosen install -r "+target}
            if strings.Join(calls, "\n") != strings.Join(want, "\n") { t.Fatal(calls) }
        })
    }
}

func TestADBRecoveryLeavesKnownStatesAlone(t *testing.T) {
    fakeTool(t, "adb")
    for _, item := range []struct { output, id string }{
        {"chosen device", ""}, {"chosen unauthorized", ""}, {"chosen recovery", ""},
        {"other device", "typo"}, {"chosen device\nother offline", ""},
        {"chosen unauthorized\nother offline", ""},
    } {
        a := testApp(t)
        count := 0
        a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
            count++
            if strings.Join(args, " ") != "devices -l" { t.Fatal("Unexpected recovery", args) }
            return item.output, nil
        }
        if _, err := a.discoverForInstall("android", item.id); err != nil || count != 1 { t.Fatal(count, err) }
    }
}

func TestADBRestartFailureStops(t *testing.T) {
    fakeTool(t, "adb")
    a := testApp(t)
    calls := 0
    a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
        calls++
        if calls == 1 { return "", nil }
        if calls != 2 || args[0] != "kill-server" { t.Fatal(args) }
        return "", errors.New("cannot stop server")
    }
    if _, err := a.discoverForInstall("android", ""); err == nil || !strings.Contains(err.Error(), "kill-server") { t.Fatal(err) }
    if calls != 2 { t.Fatal(calls) }
}

func TestADBRecoveryBoundedWithoutDevice(t *testing.T) {
    fakeTool(t, "adb")
    a := testApp(t)
    kills, starts := 0, 0
    a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
        if args[0] == "kill-server" { kills++ }
        if args[0] == "start-server" { starts++ }
        return "", nil
    }
    if _, err := a.discoverForInstall("android", ""); err == nil || !strings.Contains(err.Error(), "after one adb restart") { t.Fatal(err) }
    if kills != 1 || starts != 1 { t.Fatal(kills, starts) }
}

func TestDeviceListingDoesNotRecoverADB(t *testing.T) {
    fakeTool(t, "adb")
    fakeTool(t, "ios")
    for _, command := range []string{"--list", "__devices"} {
        a := testApp(t)
        a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
            if args[0] == "devices" { return "", nil }
            if args[0] == "list" { return `{"deviceList":[]}`, nil }
            t.Fatal("Listing restarted a device service", args); return "", nil
        }
        if err := a.execute([]string{command}); err != nil { t.Fatal(err) }
    }
}

func TestADBRecoveryCancellation(t *testing.T) {
    fakeTool(t, "adb")
    a := testApp(t)
    ctx, cancel := context.WithCancel(a.ctx)
    defer cancel()
    a.ctx = ctx
    a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
        if args[0] == "devices" { cancel(); return "", context.Canceled }
        t.Fatal("Restarted after cancellation"); return "", nil
    }
    if _, err := a.discoverForInstall("android", ""); !errors.Is(err, context.Canceled) { t.Fatal(err) }
}

func TestIOSMissingDeviceExplainsConnection(t *testing.T) {
    fakeTool(t, "ios")
    a := testApp(t)
    a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
        if strings.Join(args, " ") != "list" { t.Fatal("Unexpected device action", args) }
        return `{"deviceList":[]}`, nil
    }
    _, err := a.discoverForInstall("ios", "00008110-001C7C491E93801E")
    if err == nil || !strings.Contains(err.Error(), "go-ios") || !strings.Contains(err.Error(), "USB") { t.Fatalf("Missing connection guidance: %v", err) }
}
