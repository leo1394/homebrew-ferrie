package main

import (
    "fmt"
    "io"
    "os"
    "strings"
    "time"
    "unicode/utf8"

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
    if p.file != nil { fmt.Fprint(p.out, "\r\x1b[2K") }
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

func downloadSize(size int64) string {
    if size < 1000 { return fmt.Sprintf("%dB", size) }
    if size < 1000000 { return fmt.Sprintf("%.1fKB", float64(size)/1000) }
    if size < 1000000000 { return fmt.Sprintf("%.1fMB", float64(size)/1000000) }
    return fmt.Sprintf("%.1fGB", float64(size)/1000000000)
}

func downloadCompleteLine(name string, received, total int64, width int) string {
    size := downloadSize(received)
    if total > 0 { size += "/" + downloadSize(total) }
    right := "Downloaded    " + size
    left := "✔︎ " + name
    // The check mark plus variation selector occupies one terminal column.
    gap := width - (utf8.RuneCountInString(left)-1) - len(right)
    if gap < 2 { gap = 2 }
    return left + strings.Repeat(" ", gap) + right
}

func (p *downloadProgress) complete(name string) {
    width := 80
    if p.file != nil {
        if columns, _, err := term.GetSize(int(p.file.Fd())); err == nil && columns > 1 { width = columns-1 }
    }
    fmt.Fprintln(p.out, downloadCompleteLine(name, p.received, p.total, width))
}
