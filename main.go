package main

import (
    "bufio"
    "context"
    "errors"
    "fmt"
    "io"
    "os"
    "os/exec"
    "os/signal"
    "path/filepath"
    "strconv"
    "strings"
    "time"
)

const version = "0.2.2"
const versionDate = "2026-10-06"
const repositoryURL = "https://github.com/leo1394/homebrew-ferrie"

var options = []string{"--target", "--android", "--ios", "--list", "--device", "--help", "version", "--version"}

const help = `Ferrie — install Android/iOS apps on connected devices.

Usage:
  ferrie --target PATH_OR_URL [--android | --ios] [--device ID]
  ferrie --list
  ferrie --help
  ferrie version | --version

Options:
  -T, --target PATH_OR_URL
                    Install a local package or download from a URL
      --android     Choose Android for a URL target
      --ios         Choose iOS for a URL target
  -d, --device ID    Select this exact device ID
  -l, --list         List connected Android and iOS devices
  -h, --help         Show help
  -v, --version      Show version

Try updating first to preserve app data. If installation fails, uninstall/retry
requires an existing app with the same ID and your confirmation of data loss.
Multiple compatible devices require selection and confirmation.
Non-interactive sessions with multiple devices must use --device ID.
Missing device tools are installed into ~/.ferrie only after your confirmation.
No language runtime needed. --list and completion never download tools.
AAB/APKS additionally use a managed Java runtime and bundletool.
`

type arguments struct {
    target, device, action, platform string
}

type usageError struct { message string }
func (e usageError) Error() string { return e.message }

func parse(args []string) (arguments, error) {
    result := arguments{}
    seen := map[string]bool{}
    for i := 0; i < len(args); i++ {
        key, value, inline := strings.Cut(args[i], "=")
        switch key {
        case "-T": key = "--target"
        case "-d": key = "--device"
        case "-l": key = "--list"
        case "-h": key = "--help"
        case "-v", "version": key = "--version"
        }
        switch key {
        case "--target", "--device":
            if !inline {
                if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
                    return result, usageError{key + " requires a value"}
                }
                i++
                value = args[i]
            }
            if strings.TrimSpace(value) == "" { return result, usageError{key + " requires a non-empty value"} }
            switch key {
            case "--target": result.target = value
            case "--device": result.device = value
            }
        case "--android", "--ios":
            if inline { return result, usageError{key + " does not take a value"} }
            if result.platform != "" { return result, usageError{"Choose only one of --android and --ios"} }
            result.platform = strings.TrimPrefix(key, "--")
        case "--list", "--help", "--version":
            if inline { return result, usageError{key + " does not take a value"} }
            if result.action != "" { return result, usageError{"Choose only one action"} }
            result.action = key
        case "--url":
            return result, usageError{"--url has been replaced by --target; use --target URL"}
        default:
            message := "Unknown argument: " + key
            if suggestion := similar(key); suggestion != "" { message += "\nDid you mean '" + suggestion + "'?" }
            return result, usageError{message}
        }
        if seen[key] { return result, usageError{"Repeated argument: " + key} }
        seen[key] = true
    }
    if result.action == "" && result.target == "" { return result, usageError{"Use --target PATH_OR_URL or --list; see ferrie --help"} }
    if result.action != "" && (result.target != "" || result.platform != "" || result.device != "") { return result, usageError{"Do not combine --list/--help/--version with installation arguments"} }
    if result.platform != "" && !isURLTarget(result.target) { return result, usageError{"--android/--ios requires a URL in --target"} }
    if result.device != "" && result.target == "" { return result, usageError{"--device requires --target"} }
    return result, nil
}

// Damerau-Levenshtein includes adjacent transpositions, as in homebrew-gits.
func distance(left, right string) int {
    a, b := []rune(left), []rune(right)
    matrix := make([][]int, len(a)+1)
    for i := range matrix {
        matrix[i] = make([]int, len(b)+1)
        matrix[i][0] = i
    }
    for j := range matrix[0] { matrix[0][j] = j }
    for i := 1; i <= len(a); i++ {
        for j := 1; j <= len(b); j++ {
            cost := 1
            if a[i-1] == b[j-1] { cost = 0 }
            matrix[i][j] = min(matrix[i-1][j]+1, matrix[i][j-1]+1, matrix[i-1][j-1]+cost)
            if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
                matrix[i][j] = min(matrix[i][j], matrix[i-2][j-2]+1)
            }
        }
    }
    return matrix[len(a)][len(b)]
}

func similar(input string) string {
    if len(input) > 64 { return "" }
    best, suggestion := 3, ""
    for _, option := range options {
        if score := distance(strings.ToLower(input), option); score < best {
            best, suggestion = score, option
        }
    }
    return suggestion
}

type runner func(context.Context, bool, string, ...string) (string, error)

type app struct {
    ctx context.Context
    in io.Reader
    reader *bufio.Reader
    out, err io.Writer
    interactive bool
    cache string
    run runner
}

func commandRunner(out, errOut io.Writer) runner {
    return func(ctx context.Context, capture bool, name string, args ...string) (string, error) {
        cmd := exec.CommandContext(ctx, name, args...)
        var stdout, stderr strings.Builder
        if capture { cmd.Stdout, cmd.Stderr = &stdout, &stderr } else { cmd.Stdout, cmd.Stderr = out, errOut }
        if err := cmd.Run(); err != nil {
            return "", fmt.Errorf("%s: %w %s", filepath.Base(name), err, strings.TrimSpace(stderr.String()))
        }
        return stdout.String(), nil
    }
}

func (a *app) execute(args []string) error {
    if len(args) == 1 && args[0] == "__man" {
        _, err := io.WriteString(a.out, manual())
        return err
    }
    if len(args) == 2 && args[0] == "__completion" {
        script, err := completion(args[1])
        if err != nil { return err }
        _, err = io.WriteString(a.out, script)
        return err
    }
    if len(args) == 1 && args[0] == "__devices" {
        devices, _ := a.discover("", false, true)
        for _, device := range devices {
            if device.state == "device" { fmt.Fprintln(a.out, device.id) }
        }
        return nil
    }
    parsed, err := parse(args)
    if err != nil { return err }
    switch parsed.action {
    case "--help": fmt.Fprint(a.out, help); return nil
    case "--version": fmt.Fprintf(a.out, "ferrie version %s (%s)\n%s\n", version, versionDate, repositoryURL); return nil
    case "--list":
        devices, err := a.discover("", false, false)
        a.printDevices(devices)
        return err
    }
    if isURLTarget(parsed.target) {
        source, err := a.resolveURL(parsed.target, parsed.platform)
        if err != nil { return err }
        defer source.cleanup()
        if source.store != "" { return a.openStore(source, parsed.device) }
        parsed.target = source.path
    }
    target := parsed.target
    if strings.HasPrefix(target, "~/") || strings.HasPrefix(target, `~\`) {
        home, err := os.UserHomeDir()
        if err != nil { return err }
        target = filepath.Join(home, target[2:])
    }
    target, err = filepath.Abs(target)
    if err != nil { return err }
    stat, err := os.Stat(target)
    if err != nil || !stat.Mode().IsRegular() { return fmt.Errorf("Target file does not exist or is not a regular file: %s", target) }
    kind := "android"
    switch strings.ToLower(filepath.Ext(target)) {
    case ".ipa": kind = "ios"
    case ".apk", ".apks", ".aab":
    default: return errors.New("Unsupported package; expected .apk, .apks, .aab or .ipa")
    }
    required := []string{"adb"}
    if kind == "ios" { required = []string{"ios"} } else if !strings.EqualFold(filepath.Ext(target), ".apk") { required = append(required, "java", "bundletool") }
    if err := a.prepare(required...); err != nil { return err }
    devices, err := a.discoverForInstall(kind, parsed.device)
    if err != nil { return err }
    device, err := a.selectDevice(devices, parsed.device)
    if err != nil { return err }
    return a.install(target, device)
}

func (a *app) selectDevice(devices []device, id string) (device, error) {
    selected := device{}
    if id != "" {
        for _, item := range devices { if item.id == id { selected = item; break } }
        if selected.id == "" { return selected, fmt.Errorf("Device %q is not connected or is incompatible with this package", id) }
    } else {
        switch len(devices) {
        case 0: return selected, errors.New("No compatible devices. Enable USB debugging, or unlock and trust this computer on iOS")
        case 1: selected = devices[0]
        default:
            a.printDevices(devices)
            if !a.interactive { return selected, errors.New("Multiple devices connected; use --device ID in non-interactive sessions") }
            fmt.Fprintf(a.out, "Select device [1-%d], or q to cancel: ", len(devices))
            answer, err := a.readLine()
            if err != nil { return selected, errors.New("Installation cancelled") }
            answer = strings.TrimSpace(answer)
            if strings.EqualFold(answer, "q") { return selected, errors.New("Installation cancelled") }
            index, err := strconv.Atoi(answer)
            if err != nil || index < 1 || index > len(devices) { return selected, errors.New("Invalid device selection") }
            selected = devices[index-1]
            if err := ready(selected); err != nil { return selected, err }
            fmt.Fprintf(a.out, "Install on %s? [y/N]: ", selected.id)
            answer, err = a.readLine()
            answer = strings.ToLower(strings.TrimSpace(answer))
            if err != nil || (answer != "y" && answer != "yes") { return selected, errors.New("Installation cancelled") }
        }
    }
    return selected, ready(selected)
}

func ready(d device) error {
    if d.state != "device" { return fmt.Errorf("Device %q is %s. Authorize/reconnect it first", d.id, d.state) }
    return nil
}

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()
    home, err := os.UserHomeDir()
    if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
    cache := os.Getenv("FERRIE_HOME")
    if cache == "" { cache = filepath.Join(home, ".ferrie") }
    stat, _ := os.Stdin.Stat()
    a := app{ctx: ctx, in: os.Stdin, out: os.Stdout, err: os.Stderr,
             interactive: stat != nil && stat.Mode()&os.ModeCharDevice != 0,
             cache: cache, run: commandRunner(os.Stdout, os.Stderr)}
    // Stop even while waiting for interactive input.
    go func() { <-ctx.Done(); time.Sleep(100*time.Millisecond); os.Exit(130) }()
    if err := a.execute(os.Args[1:]); err != nil {
        fmt.Fprintln(os.Stderr, "ferrie:", err)
        if ctx.Err() != nil { os.Exit(130) }
        var usage usageError
        if errors.As(err, &usage) { os.Exit(2) }
        os.Exit(1)
    }
}
