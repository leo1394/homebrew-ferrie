package main

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "time"
)

type device struct { platform, id, state, name string }

func parseADB(output string) []device {
    var result []device
    for _, line := range strings.Split(output, "\n") {
        fields := strings.Fields(line)
        if len(fields) < 2 || strings.HasPrefix(line, "List of devices") || strings.HasPrefix(line, "*") { continue }
        item := device{platform: "android", id: fields[0], state: fields[1]}
        for _, field := range fields[2:] { if strings.HasPrefix(field, "model:") { item.name = strings.TrimPrefix(field, "model:") } }
        result = append(result, item)
    }
    return result
}

func parseIOS(output string) ([]device, error) {
    var payload struct { Devices []string `json:"deviceList"` }
    if err := json.Unmarshal([]byte(output), &payload); err != nil { return nil, fmt.Errorf("Invalid go-ios device list: %w", err) }
    result := make([]device, 0, len(payload.Devices))
    for _, id := range payload.Devices { if id != "" { result = append(result, device{platform: "ios", id: id, state: "device"}) } }
    return result, nil
}

func (a *app) discover(kind string, bootstrap, quiet bool) ([]device, error) {
    var devices []device
    var failures []error
    for _, backend := range []string{"android", "ios"} {
        if kind != "" && kind != backend { continue }
        name := "adb"
        args := []string{"devices", "-l"}
        if backend == "ios" { name, args = "ios", []string{"list"} }
        tool, err := a.tool(name, bootstrap)
        if err == nil && tool == "" {
            if !quiet { fmt.Fprintf(a.err, "Missing %s; run ferrie --target PATH to review and approve required tools.\n", name) }
            continue
        }
        var output string
        if err == nil {
            ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
            output, err = a.run(ctx, true, tool, args...)
            cancel()
        }
        if err == nil {
            if backend == "android" { devices = append(devices, parseADB(output)...) } else {
                var found []device
                found, err = parseIOS(output)
                devices = append(devices, found...)
            }
        }
        if err != nil {
            failures = append(failures, fmt.Errorf("%s discovery: %w", backend, err))
            if !quiet && kind == "" { fmt.Fprintf(a.err, "Warning: %s discovery unavailable: %v\n", backend, err) }
        }
    }
    if kind != "" || len(failures) == 2 { return devices, errors.Join(failures...) }
    return devices, nil
}

func (a *app) printDevices(devices []device) {
    fmt.Fprintln(a.out, "#  PLATFORM  DEVICE ID  STATE  MODEL")
    for i, device := range devices { fmt.Fprintf(a.out, "%d  %s  %s  %s  %s\n", i+1, device.platform, device.id, device.state, device.name) }
    if len(devices) == 0 { fmt.Fprintln(a.out, "No connected devices.") }
}

func (a *app) install(target string, selected device) error {
    fmt.Fprintf(a.out, "Installing %s on %s (%s)\n", filepath.Base(target), selected.id, selected.platform)
    if selected.platform == "ios" {
        tool, err := a.tool("ios", false)
        if err != nil { return err }
        if err := a.installWithFallback(target, selected, tool, func() error {
            _, err := a.run(a.ctx, false, tool, "install", "--path=" + target, "--udid=" + selected.id)
            return err
        }); err != nil { return err }
    } else {
        adb, err := a.tool("adb", false)
        if err != nil { return err }
        if strings.EqualFold(filepath.Ext(target), ".apk") {
            if err := a.installWithFallback(target, selected, adb, func() error {
                _, err := a.run(a.ctx, false, adb, "-s", selected.id, "install", "-r", target)
                return err
            }); err != nil { return err }
        } else {
            java, err := a.tool("java", false)
            if err != nil { return err }
            bundle, err := a.tool("bundletool", false)
            if err != nil { return err }
            folder, err := os.MkdirTemp("", "ferrie-")
            if err != nil { return err }
            defer os.RemoveAll(folder)
            apks := target
            if strings.EqualFold(filepath.Ext(target), ".aab") {
                apks = filepath.Join(folder, "app.apks")
                if _, err = a.run(a.ctx, false, java, "-jar", bundle, "build-apks", "--bundle=" + target,
                    "--output=" + apks, "--connected-device", "--device-id=" + selected.id, "--adb=" + adb); err != nil { return err }
            }
            if err := a.installWithFallback(apks, selected, adb, func() error {
                _, err := a.run(a.ctx, false, java, "-jar", bundle, "install-apks", "--apks=" + apks,
                    "--device-id=" + selected.id, "--adb=" + adb)
                return err
            }); err != nil { return err }
        }
    }
    fmt.Fprintln(a.out, "Installed successfully.")
    return nil
}

// Recovery is restricted to installation discovery. Listing/completion never restart adb.
func (a *app) discoverForInstall(kind, id string) ([]device, error) {
    devices, err := a.discover(kind, false, false)
    if kind == "ios" && err == nil && len(devices) == 0 {
        return nil, errors.New("No iOS device is reachable through go-ios. Connect via USB, unlock the device and trust this computer, then retry. A remembered device or an unlocked phone alone does not establish a wireless connection")
    }
    if kind != "android" || (err == nil && !needsADBRecovery(devices, id)) { return devices, err }
    if a.ctx.Err() != nil { return nil, a.ctx.Err() }
    adb, toolErr := a.tool("adb", false)
    if toolErr != nil { return nil, toolErr }
    if adb == "" { return nil, errors.New("Missing adb; cannot recover device discovery") }
    fmt.Fprintln(a.out, "Android device discovery unavailable. Restarting adb once; other adb sessions may briefly disconnect...")
    for _, command := range []string{"kill-server", "start-server"} {
        ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
        _, restartErr := a.run(ctx, true, adb, command)
        cancel()
        if restartErr != nil { return nil, fmt.Errorf("adb %s failed during recovery: %w", command, restartErr) }
    }
    // USB enumeration can finish shortly after start-server returns.
    ctx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
    defer cancel()
    probe := *a
    probe.ctx = ctx
    for {
        devices, err = probe.discover("android", false, false)
        if err == nil && !needsADBRecovery(devices, id) {
            fmt.Fprintln(a.out, "adb restarted; device list refreshed.")
            return devices, nil
        }
        timer := time.NewTimer(250*time.Millisecond)
        select {
        case <-ctx.Done():
            timer.Stop()
            if a.ctx.Err() != nil { return nil, a.ctx.Err() }
            if err != nil { return nil, fmt.Errorf("Android discovery still failed after one adb restart: %w", err) }
            return nil, errors.New("No usable Android device after one adb restart. Check the USB cable, enable USB debugging, and unlock the device to authorize this computer")
        case <-timer.C:
        }
    }
}

func needsADBRecovery(devices []device, id string) bool {
    if len(devices) == 0 { return true }
    if id != "" {
        for _, item := range devices { if item.id == id { return item.state == "offline" } }
        // A mistyped ID should not restart a working server with other devices.
        return false
    }
    offline := false
    for _, item := range devices {
        if item.state != "offline" { return false }
        offline = true
    }
    return offline
}
