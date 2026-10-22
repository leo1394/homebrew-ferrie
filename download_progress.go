package main

import (
    "fmt"
    "io"
    "os"
    "strings"
    "time"

    "golang.org/x/term"
)

type downloadProgress struct {
    out io.Writer
    file *os.File
    total, received int64
    last time.Time
}

func newDownloadProgress(out io.Writer, total int64) *downloadProgress {
    progress := &downloadProgress{out: out, total: total}
    if file, ok := out.(*os.File); ok && term.IsTerminal(int(file.Fd())) && os.Getenv("TERM") != "dumb" {
        progress.file = file
        progress.render()
    }
    return progress
}

func (p *downloadProgress) Write(data []byte) (int, error) {
    p.received += int64(len(data))
    if p.file != nil && time.Since(p.last) >= 100*time.Millisecond { p.render() }
    return len(data), nil
}

func (p *downloadProgress) render() {
    width, _, err := term.GetSize(int(p.file.Fd()))
    if err != nil || width < 2 { width = 80 }
    // Leave the final column empty to avoid terminal auto-wrap while refreshing.
    fmt.Fprintf(p.out, "\r\x1b[2K%s", downloadProgressLine(p.received, p.total, width-1))
    p.last = time.Now()
}

func (p *downloadProgress) finish() {
    if p.file != nil { p.render(); fmt.Fprintln(p.out) }
}

func downloadProgressLine(received, total int64, width int) string {
    label := fmt.Sprintf("%d bytes", received)
    filled := 0
    if total > 0 {
        percent := float64(received) / float64(total)
        if percent > 1 { percent = 1 }
        label = fmt.Sprintf("%.1f%%", percent*100)
        filled = int(float64(width-len(label)-1) * percent)
    }
    available := width-len(label)-1
    if available < 0 { return label }
    if filled < 0 { filled = 0 }
    return strings.Repeat("#", filled) + strings.Repeat(" ", available-filled+1) + label
}
