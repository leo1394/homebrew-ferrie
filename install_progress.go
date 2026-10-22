package main

import (
    "encoding/json"
    "fmt"
    "io"
    "strings"
    "sync"
)

// Both go-ios streams share this writer; exec serializes writes to the same writer.
// Keep a mutex as well so partial lines and progress always move together.
type iosInstallLog struct {
    mu sync.Mutex
    progress *downloadProgress
    pending string
    warnings []string
    diagnostic string
}

func newIOSInstallLog(out io.Writer) *iosInstallLog {
    return &iosInstallLog{progress:newDownloadProgress(out, 100)}
}

func (log *iosInstallLog) Write(data []byte) (int, error) {
    log.mu.Lock()
    defer log.mu.Unlock()
    log.pending += string(data)
    for {
        end := strings.IndexByte(log.pending, '\n')
        if end < 0 { break }
        log.line(strings.TrimSpace(log.pending[:end]))
        log.pending = log.pending[end+1:]
    }
    // Bound malformed output without a newline.
    if len(log.pending) > 65536 { log.line(log.pending[:65536]); log.pending = "" }
    return len(data), nil
}

func (log *iosInstallLog) line(line string) {
    if line == "" { return }
    var event struct {
        Level, Msg, Status string
        Err json.RawMessage
        Percent *float64 `json:"percentComplete"`
    }
    if err := json.Unmarshal([]byte(line), &event); err != nil || event.Msg == "" {
        log.print(line)
        log.diagnostic = line
        return
    }
    if event.Percent != nil && *event.Percent >= 0 && *event.Percent <= 100 {
        percent := int64(*event.Percent)
        if percent > log.progress.received {
            log.progress.received = percent
            if log.progress.file != nil { log.progress.render() }
        }
    }
    level := strings.ToUpper(event.Level)
    message := event.Msg
    if len(event.Err) > 0 && string(event.Err) != "null" {
        var detail string
        if json.Unmarshal(event.Err, &detail) != nil { detail = string(event.Err) }
        message += ": " + detail
    }
    if level == "WARN" || level == "WARNING" {
        // This optional-agent warning is common even for successful Wi-Fi installs.
        if strings.HasPrefix(event.Msg, "go-ios agent is not running") {
            if len(log.warnings) < 8 { log.warnings = append(log.warnings, message) }
        } else { log.print("Warning: " + message) }
    } else if level == "ERROR" || level == "FATAL" {
        log.diagnostic = message
        log.print("Error: " + message)
    }
}

func (log *iosInstallLog) print(message string) {
    log.progress.finish()
    fmt.Fprintln(log.progress.out, message)
    if log.progress.file != nil { log.progress.render() }
}

func (log *iosInstallLog) finish(failed bool) string {
    log.mu.Lock()
    defer log.mu.Unlock()
    log.line(strings.TrimSpace(log.pending))
    log.pending = ""
    log.progress.finish()
    if failed {
        for _, warning := range log.warnings { fmt.Fprintln(log.progress.out, "Warning: " + warning) }
    }
    return log.diagnostic
}
