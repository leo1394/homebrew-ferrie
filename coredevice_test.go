package main

import (
    "context"
    "errors"
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestCoreDeviceDiscovery(t *testing.T) {
    devices, err := parseCoreDevices([]byte(`{"result":{"devices":[
        {"identifier":"uuid","properties":{"hardware":{"udid":"phone","platform":"iOS","reality":"physical"},"state":{"name":"Phone"},"connection":{"pairingState":"paired","state":"disconnected","transportType":"localNetwork"}}},
        {"hardwareProperties":{"udid":"sim","platform":"iOS","reality":"simulated"},"connectionProperties":{"pairingState":"paired","tunnelState":"connected"}}
    ]}}`))
    if err != nil || len(devices) != 1 || devices[0].id != "phone" || devices[0].state != "paired" { t.Fatalf("%+v %v", devices, err) }
}

func TestCoreDeviceReachability(t *testing.T) {
    for _, connected := range []bool{false, true} {
        a := testApp(t)
        a.coreDevice = "xcrun"
        a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
            if !strings.Contains(strings.Join(args, " "), "device info details --device phone") { t.Fatal(args) }
            state := "disconnected"
            if connected { state = "connected" }
            for i, arg := range args { if arg == "--json-output" { os.WriteFile(args[i+1], []byte(`{"info":{"outcome":"success"},"result":{"hardwareProperties":{"udid":"phone","platform":"iOS","reality":"physical"},"connectionProperties":{"pairingState":"paired","tunnelState":"`+state+`"}}}`), 0o600) } }
            return "", nil
        }
        selected := device{platform:"ios", id:"phone", state:"paired", backend:"coredevice"}
        _, err := a.selectDevice([]device{selected}, "phone")
        if (err == nil) != connected { t.Fatalf("connected=%v err=%v", connected, err) }
    }
}

func TestCoreDeviceInstallationFallback(t *testing.T) {
    fakeTool(t, "ios")
    a := testApp(t)
    a.coreDevice = "xcrun"
    path := filepath.Join(t.TempDir(), "app.ipa")
    identityFixture(t, path)
    installs := 0
    goIOSVisible := false
    a.run = func(_ context.Context, _ bool, tool string, args ...string) (string, error) {
        if tool != "xcrun" {
            if args[0] == "install" && goIOSVisible { return "", errors.New("wireless transport unavailable") }
            if strings.Join(args, " ") != "list" { t.Fatal("Unexpected go-ios action", args) }
            if goIOSVisible { return `{"deviceList":["phone"]}`, nil }
            return `{"deviceList":[]}`, nil
        }
        info := `{"hardwareProperties":{"udid":"phone","platform":"iOS","reality":"physical"},"connectionProperties":{"pairingState":"paired","tunnelState":"connected"}}`
        result := info
        if args[1] == "list" { result = `{"devices":[`+info+`]}` }
        if args[2] == "install" {
            installs++
            if _, err := os.Stat(filepath.Join(args[6], "Info.plist")); err != nil { t.Fatal("Missing extracted .app", args, err) }
            result = `{"installedApplications":[{"bundleID":"com.example.app"}]}`
        }
        for i, arg := range args { if arg == "--json-output" { os.WriteFile(args[i+1], []byte(`{"info":{"outcome":"success"},"result":`+result+`}`), 0o600) } }
        return "", nil
    }
    if err := a.execute([]string{"--target", path, "--device", "phone"}); err != nil { t.Fatal(err) }
    if installs != 1 { t.Fatal(installs) }
    goIOSVisible = true
    found, err := a.discover("ios", false, false)
    if err != nil || len(found) != 1 { t.Fatalf("Duplicate device: %+v %v", found, err) }
    if err := a.execute([]string{"--target", path, "--device", "phone"}); err != nil { t.Fatal(err) }
    if installs != 2 { t.Fatal(installs) }
}
