package main

import (
    "bytes"
    "strings"
    "testing"
)

func TestDownloadProgressAlignment(t *testing.T) {
    for _, width := range []int{20, 80, 140} {
        line := downloadProgressLine(785, 1000, width)
        if len(line) != width || !strings.HasPrefix(line, "#") || !strings.HasSuffix(line, "78.5%") { t.Fatalf("width %d: %q", width, line) }
    }
    if line := downloadProgressLine(1200, 1000, 80); !strings.HasSuffix(line, "100.0%") || len(line) != 80 { t.Fatal(line) }
    if line := downloadProgressLine(123, -1, 80); strings.Contains(line, "%") || !strings.HasSuffix(line, "123 bytes") { t.Fatal(line) }
}

func TestDownloadProgressDoesNotPolluteRedirectedOutput(t *testing.T) {
    var out bytes.Buffer
    progress := newDownloadProgress(&out, 100)
    if n, err := progress.Write(make([]byte, 50)); n != 50 || err != nil { t.Fatal(n, err) }
    progress.finish()
    if out.Len() != 0 || progress.received != 50 { t.Fatal(out.String(), progress.received) }
}
