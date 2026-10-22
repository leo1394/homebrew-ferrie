package main

import (
    "encoding/json"
    "errors"
    "fmt"
    "net/url"
    "os"
    "path/filepath"
    "strings"

    "github.com/leo1394/homebrew-ferrie/internal/selector"
)

const recentURLLimit = 5

func historyURL(raw string) string {
    value := strings.TrimSpace(raw)
    parsed, err := url.Parse(value)
    if err != nil { return value }
    parsed.Scheme = strings.ToLower(parsed.Scheme)
    parsed.Host = strings.ToLower(parsed.Host)
    parsed.Fragment = ""
    return parsed.String()
}

func (a *app) recentURLs() ([]string, error) {
    data, err := os.ReadFile(filepath.Join(a.cache, "recent-urls.json"))
    if os.IsNotExist(err) { return nil, nil }
    if err != nil { return nil, fmt.Errorf("Read recent URLs: %w", err) }
    var stored []string
    if err := json.Unmarshal(data, &stored); err != nil { return nil, fmt.Errorf("Invalid recent-urls.json: %w", err) }
    result := []string{}
    seen := map[string]bool{}
    for _, raw := range stored {
        value := historyURL(raw)
        if _, err := parseSourceURL(value); err != nil || seen[value] { continue }
        seen[value] = true
        result = append(result, value)
        if len(result) == recentURLLimit { break }
    }
    return result, nil
}

func (a *app) rememberURL(raw string) error {
    recent, err := a.recentURLs()
    if err != nil { return err }
    value := historyURL(raw)
    next := []string{value}
    for _, previous := range recent {
        if previous != value && len(next) < recentURLLimit { next = append(next, previous) }
    }
    data, err := json.MarshalIndent(next, "", "  ")
    if err != nil { return err }
    if err := os.MkdirAll(a.cache, 0700); err != nil { return err }
    file, err := os.CreateTemp(a.cache, ".recent-urls-*")
    if err != nil { return err }
    defer os.Remove(file.Name())
    _, writeErr := file.Write(append(data, '\n'))
    closeErr := file.Close()
    if err := errors.Join(writeErr, closeErr); err != nil { return err }
    return os.Rename(file.Name(), filepath.Join(a.cache, "recent-urls.json"))
}

func (a *app) chooseRecentTarget(parsed *arguments) error {
    if !a.interactive { return errors.New("Recent URL selection requires an interactive terminal; use --target PATH_OR_URL") }
    devices, err := a.discover("", false, false)
    if err != nil { return err }
    if parsed.device == "" && len(devices) != 1 {
        return errors.New("Recent URLs require exactly one connected device or --device ID; otherwise use --target PATH_OR_URL")
    }
    selected, err := a.selectDevice(devices, parsed.device)
    if err != nil { return err }
    if parsed.platform != "" && parsed.platform != selected.platform { return fmt.Errorf("Device platform %s does not match --%s", selected.platform, parsed.platform) }
    recent, err := a.recentURLs()
    if err != nil { return err }
    if len(recent) == 0 { return errors.New("No recent URL installations; first install with --target URL") }
    candidates := make([]selector.Candidate, 0, len(recent))
    for _, value := range recent {
        // Never render URL credentials, queries or terminal control sequences.
        candidates = append(candidates, selector.Candidate{Name: safeLinkLabel(value)})
    }
    input, _ := a.in.(*os.File)
    pick := a.pickRecent
    if pick == nil { pick = selector.SelectOne }
    // Number rows to distinguish links whose only difference is their query.
    for i := range candidates { candidates[i].Name = fmt.Sprintf("%d. %s", i+1, candidates[i].Name) }
    choice, confirmed, err := pick(a.ctx, input, a.out, selector.Prompt{
        Title: "Select recent installation URL",
        EmptySelectionNotice: "Select one URL before confirming.",
    }, candidates)
    if err != nil { return fmt.Errorf("Recent URL selection: %w; use --target URL", err) }
    if !confirmed { return errors.New("Installation cancelled") }
    for i, candidate := range candidates {
        if choice == candidate {
            parsed.target, parsed.device, parsed.platform = recent[i], selected.id, selected.platform
            return nil
        }
    }
    return errors.New("Invalid recent URL selection")
}
