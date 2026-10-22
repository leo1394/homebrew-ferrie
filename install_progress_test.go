package main

import (
    "bytes"
    "context"
    "os"
    "path/filepath"
    "runtime"
    "strings"
    "testing"
)

func TestIOSInstallProgress(t *testing.T) {
    var out bytes.Buffer
    log := newIOSInstallLog(&out)
    log.Write([]byte(`{"level":"WARN","msg":"go-ios agent is not running. Start a tunnel"}`+"\n"))
    log.Write([]byte(`{"level":"INFO","msg":"installing","percentCom`))
    log.Write([]byte(`plete":60}`+"\n"))
    log.Write([]byte(`{"level":"INFO","msg":"installing","percentComplete":5}`+"\n"))
    if log.progress.received != 60 { t.Fatal(log.progress.received) }
    log.Write([]byte(`{"level":"INFO","msg":"installing","percentComplete":100}`))
    log.finish(false)
    if log.progress.received != 100 || out.Len() != 0 { t.Fatal(out.String(), log.progress.received) }
}

func TestIOSInstallFailureDiagnostics(t *testing.T) {
    var out bytes.Buffer
    log := newIOSInstallLog(&out)
    log.Write([]byte(`{"level":"WARN","msg":"go-ios agent is not running"}`+"\n"))
    log.Write([]byte(`{"level":"ERROR","msg":"install failed","err":"invalid signature"}`))
    detail := log.finish(true)
    if !strings.Contains(detail, "invalid signature") || !strings.Contains(out.String(), "Warning: go-ios agent") || strings.Contains(out.String(), `"level"`) { t.Fatal(out.String(), detail) }
}

func TestIOSInstallUnknownOutput(t *testing.T) {
    var out bytes.Buffer
    log := newIOSInstallLog(&out)
    log.Write([]byte("connection lost\n"))
    log.finish(true)
    if !strings.Contains(out.String(), "connection lost") { t.Fatal(out.String()) }
}

func TestIOSInstallRunnerExitStatus(t *testing.T) {
    if runtime.GOOS == "windows" { t.Skip("POSIX subprocess fixture") }
    for _, exitCode := range []string{"0", "1"} {
        tool := filepath.Join(t.TempDir(), "ios")
        script := "#!/bin/sh\nprintf '%s\\n' '{\"level\":\"INFO\",\"msg\":\"installing\",\"percentComplete\":100}' >&2\nexit " + exitCode + "\n"
        if err := os.WriteFile(tool, []byte(script), 0o755); err != nil { t.Fatal(err) }
        var out, errOut bytes.Buffer
        _, err := commandRunner(&out, &errOut)(context.Background(), false, tool, "install", "--path=test.ipa", "--udid=test")
        if (err != nil) != (exitCode == "1") { t.Fatal(err) }
        if out.Len() != 0 || errOut.Len() != 0 { t.Fatal(out.String(), errOut.String()) }
    }
}
