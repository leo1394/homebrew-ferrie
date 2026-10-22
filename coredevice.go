package main

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strings"
    "time"
)

func coreDeviceTool() string {
    if runtime.GOOS != "darwin" { return "" }
    path, err := exec.LookPath("xcrun")
    if err != nil { return "" }
    return path
}

type coreHardware struct { UDID, Platform, Reality string }
type coreConnection struct { PairingState, TunnelState, State string }
type coreDeviceInfo struct {
    Hardware coreHardware `json:"hardwareProperties"`
    Connection coreConnection `json:"connectionProperties"`
    Device struct { Name string } `json:"deviceProperties"`
    Properties struct {
        Hardware coreHardware
        Connection coreConnection
        State struct { Name string }
    }
}

func (info coreDeviceInfo) device() device {
    hardware, connection, name := info.Hardware, info.Connection, info.Device.Name
    if info.Properties.Hardware.UDID != "" { hardware = info.Properties.Hardware }
    if info.Properties.Connection.PairingState != "" { connection = info.Properties.Connection }
    if info.Properties.State.Name != "" { name = info.Properties.State.Name }
    if hardware.Reality != "physical" || hardware.Platform != "iOS" || hardware.UDID == "" || connection.PairingState != "paired" { return device{} }
    state := "paired"
    if connection.State == "connected" || connection.TunnelState == "connected" { state = "device" }
    return device{platform:"ios", id:hardware.UDID, state:state, name:name, backend:"coredevice"}
}

func parseCoreDevices(data []byte) ([]device, error) {
    var payload struct { Result struct { Devices []coreDeviceInfo } }
    if err := json.Unmarshal(data, &payload); err != nil { return nil, err }
    var result []device
    for _, info := range payload.Result.Devices {
        if item := info.device(); item.id != "" { result = append(result, item) }
    }
    return result, nil
}

func (a *app) coreJSON(timeout time.Duration, args ...string) ([]byte, error) {
    folder, err := os.MkdirTemp("", "ferrie-coredevice-*")
    if err != nil { return nil, err }
    defer os.RemoveAll(folder)
    path := filepath.Join(folder, "result.json")
    ctx, cancel := context.WithTimeout(a.ctx, timeout)
    defer cancel()
    command := append([]string{"devicectl"}, args...)
    command = append(command, "--json-output", path, "--timeout", fmt.Sprint(int(timeout.Seconds())))
    if _, err := a.run(ctx, true, a.coreDevice, command...); err != nil { return nil, err }
    data, err := os.ReadFile(path)
    if err != nil { return nil, err }
    var payload struct { Info struct { Outcome string } }
    if err := json.Unmarshal(data, &payload); err != nil { return nil, err }
    if payload.Info.Outcome != "success" { return nil, errors.New("CoreDevice did not report success") }
    return data, nil
}

func (a *app) coreDevices() ([]device, error) {
    data, err := a.coreJSON(10*time.Second, "list", "devices")
    if err != nil { return nil, err }
    return parseCoreDevices(data)
}

func (a *app) checkReady(selected *device) error {
    if selected.backend == "coredevice" && selected.state == "paired" {
        data, err := a.coreJSON(20*time.Second, "device", "info", "details", "--device", selected.id)
        if err != nil { return fmt.Errorf("iPhone %s is paired but not reachable: %w. Unlock it, use the same network and enable wireless pairing in Xcode, or connect via USB", selected.id, err) }
        var payload struct { Result coreDeviceInfo }
        if err := json.Unmarshal(data, &payload); err != nil { return err }
        found := payload.Result.device()
        if found.id != selected.id || found.state != "device" { return fmt.Errorf("iPhone %s is paired but not connected. Unlock it and check its wireless connection, or connect via USB", selected.id) }
        *selected = found
    }
    return ready(*selected)
}

func (a *app) installCoreDevice(target string, selected device) error {
    // Recheck the selected device immediately before installation; cached pairing is not reachability.
    selected.state, selected.backend = "paired", "coredevice"
    if err := a.checkReady(&selected); err != nil { return err }
    folder, err := os.MkdirTemp("", "ferrie-ipa-*")
    if err != nil { return err }
    defer os.RemoveAll(folder)
    if err := extractZIP(target, folder); err != nil { return fmt.Errorf("Extract IPA: %w", err) }
    entries, err := os.ReadDir(filepath.Join(folder, "Payload"))
    if err != nil { return err }
    var apps []string
    for _, entry := range entries {
        if entry.IsDir() && strings.HasSuffix(entry.Name(), ".app") { apps = append(apps, filepath.Join(folder, "Payload", entry.Name())) }
    }
    if len(apps) != 1 { return errors.New("IPA must contain exactly one main app") }
    fmt.Fprintln(a.out, "Installing through Apple CoreDevice (USB/Wi-Fi)...")
    data, err := a.coreJSON(5*time.Minute, "device", "install", "app", "--device", selected.id, apps[0])
    if err != nil { return fmt.Errorf("CoreDevice installation failed; existing app was not uninstalled: %w", err) }
    var payload struct { Result struct { InstalledApplications []struct { BundleID string } } }
    if err := json.Unmarshal(data, &payload); err != nil { return err }
    id, err := packageID(target)
    if err != nil { return err }
    for _, installed := range payload.Result.InstalledApplications { if installed.BundleID == id { return nil } }
    return errors.New("CoreDevice did not confirm installation of the requested app")
}
