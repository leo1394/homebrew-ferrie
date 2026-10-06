package main

import (
    "archive/zip"
    "bytes"
    "context"
    "errors"
    "fmt"
    "io"
    "net/http"
    "net/http/httptest"
    "os"
    "path/filepath"
    "reflect"
    "strings"
    "testing"
)

func TestMain(m *testing.M) {
    if os.Getenv("FERRIE_TEST_JAVA") == "1" && len(os.Args) == 2 && os.Args[1] == "-version" {
        fmt.Fprintln(os.Stderr, `openjdk version "17.0.1"`)
        os.Exit(0)
    }
    os.Exit(m.Run())
}

func testApp(t *testing.T) *app {
    t.Helper()
    return &app{ctx: context.Background(), in: strings.NewReader(""), out: &bytes.Buffer{}, err: &bytes.Buffer{}, cache: t.TempDir(),
        run: func(context.Context, bool, string, ...string) (string, error) { t.Fatal("Unexpected external command"); return "", nil }}
}

func fakeTool(t *testing.T, name string) string {
    t.Helper()
    path := filepath.Join(t.TempDir(), binaryName(name))
    if err := os.WriteFile(path, []byte("fixture"), 0o755); err != nil { t.Fatal(err) }
    t.Setenv("FERRIE_" + strings.ToUpper(name), path)
    return path
}

func TestParse(t *testing.T) {
    for _, args := range [][]string{{"--target", "app.apk"}, {"-T", "app.apk", "-d", "id"}, {"--target=app.apk", "--device=id"}, {"--list"}, {"--help"}, {"version"}, {"--version"}} {
        if _, err := parse(args); err != nil { t.Errorf("%v: %v", args, err) }
    }
    for _, args := range [][]string{nil, {"--target"}, {"--target", "--list"}, {"--device", "id"}, {"--list", "--device", "id"}, {"--target=x", "--list"}, {"--device=", "--target=x"}, {"--list=yes"}, {"--list", "--list"}, {"--lis"}, {"--target=x", "--target=y"}} {
        if _, err := parse(args); err == nil { t.Errorf("Accepted invalid arguments: %v", args) }
    }
}

func TestSuggestions(t *testing.T) {
    for typo, want := range map[string]string{"--targte":"--target", "--lst":"--list", "--devcie":"--device", "--hlep":"--help", "versoin":"version", "--verison":"--version"} {
        _, err := parse([]string{typo})
        if err == nil || !strings.Contains(err.Error(), "Did you mean '" + want + "'") { t.Errorf("%s: %v", typo, err) }
    }
    if similar(strings.Repeat("a", 1000)) != "" { t.Fatal("Unexpected suggestion for long input") }
}

func TestHelpAndCompletionHaveNoDependencies(t *testing.T) {
    a := testApp(t)
    for _, args := range [][]string{{"--help"}, {"--version"}, {"version"}, {"__completion", "bash"}, {"__completion", "zsh"}, {"__completion", "fish"}, {"__completion", "powershell"}} {
        if err := a.execute(args); err != nil { t.Fatal(err) }
    }
    if _, err := completion("unknown"); err == nil { t.Fatal("Expected invalid shell") }
}

func TestInvalidFileBeforeTools(t *testing.T) {
    a := testApp(t)
    for _, target := range []string{filepath.Join(t.TempDir(), "missing.apk"), t.TempDir()} {
        if err := a.execute([]string{"--target", target}); err == nil || !strings.Contains(err.Error(), "regular file") { t.Fatal(err) }
    }
    path := filepath.Join(t.TempDir(), "app.zip")
    os.WriteFile(path, nil, 0o600)
    if err := a.execute([]string{"--target", path}); err == nil || !strings.Contains(err.Error(), "Unsupported") { t.Fatal(err) }
}

func TestDeviceSelection(t *testing.T) {
    first := device{platform:"android", id:"one", state:"device"}
    second := device{platform:"android", id:"two", state:"device"}
    a := testApp(t)
    if got, err := a.selectDevice([]device{first}, ""); err != nil || got.id != "one" { t.Fatal(got, err) }
    if got, err := a.selectDevice([]device{first, second}, "two"); err != nil || got.id != "two" { t.Fatal(got, err) }
    if _, err := a.selectDevice([]device{first}, "on"); err == nil { t.Fatal("ID prefix was accepted") }
    if _, err := a.selectDevice(nil, ""); err == nil { t.Fatal("No device accepted") }
    if _, err := a.selectDevice([]device{first, second}, ""); err == nil { t.Fatal("Noninteractive selection accepted") }
    for _, state := range []string{"offline", "unauthorized", "recovery", "no"} {
        if _, err := a.selectDevice([]device{{id:"bad", state:state}}, "bad"); err == nil { t.Fatal("Unready device accepted") }
    }
}

func TestInteractiveSelectionAndCancellation(t *testing.T) {
    devices := []device{{id:"one", state:"device"}, {id:"two", state:"device"}}
    for _, answer := range []string{"q\n", "0\n", "99\n", "abc\n", "1\nn\n", "1\n\n", ""} {
        a := testApp(t)
        a.interactive, a.in = true, strings.NewReader(answer)
        if _, err := a.selectDevice(devices, ""); err == nil { t.Errorf("Accepted %q", answer) }
    }
    a := testApp(t)
    a.interactive, a.in = true, strings.NewReader("2\ny\n")
    if got, err := a.selectDevice(devices, ""); err != nil || got.id != "two" { t.Fatal(got, err) }
    if !strings.Contains(a.out.(*bytes.Buffer).String(), "Install on two?") { t.Fatal("Confirmation omitted selected ID") }
}

func TestParseDeviceLists(t *testing.T) {
    devices := parseADB("List of devices attached\none device product:p model:Pixel transport_id:1\ntwo unauthorized\nthree offline\n")
    if len(devices) != 3 || devices[0].name != "Pixel" || devices[1].state != "unauthorized" { t.Fatal(devices) }
    ios, err := parseIOS(`{"deviceList":["UDID"]}`)
    if err != nil || len(ios) != 1 || ios[0].id != "UDID" { t.Fatal(ios, err) }
    if _, err := parseIOS("garbage"); err == nil { t.Fatal("Invalid JSON accepted") }
}

func TestDiscoverMixedAndPartialFailure(t *testing.T) {
    fakeTool(t, "adb")
    fakeTool(t, "ios")
    a := testApp(t)
    a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
        if args[0] == "devices" { return "List of devices attached\nA device\nB offline\n", nil }
        return `{"deviceList":["I"]}`, nil
    }
    found, err := a.discover("", false, false)
    if err != nil || len(found) != 3 { t.Fatal(found, err) }
    a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
        if args[0] == "devices" { return "A device", nil }
        return "", errors.New("usbmuxd unavailable")
    }
    found, err = a.discover("", false, false)
    if err != nil || len(found) != 1 || !strings.Contains(a.err.(*bytes.Buffer).String(), "Warning") { t.Fatal(found, err) }
    if _, err := a.discover("ios", false, false); err == nil { t.Fatal("Required platform error ignored") }
}

func TestAPKReplacement(t *testing.T) {
    adb := fakeTool(t, "adb")
    a := testApp(t)
    target := filepath.Join(t.TempDir(), "中文 space.apk")
    identityFixture(t, target)
    var got []string
    a.run = func(_ context.Context, _ bool, name string, args ...string) (string, error) { got = append([]string{name}, args...); return "", nil }
    if err := a.install(target, device{platform:"android", id:"chosen"}); err != nil { t.Fatal(err) }
    want := []string{adb, "-s", "chosen", "install", "-r", target}
    if !reflect.DeepEqual(got, want) { t.Fatal(got) }
}

func TestIOSInstallation(t *testing.T) {
    ios := fakeTool(t, "ios")
    a := testApp(t)
    target := filepath.Join(t.TempDir(), "app.ipa")
    identityFixture(t, target)
    a.run = func(_ context.Context, _ bool, name string, args ...string) (string, error) {
        if args[0] == "apps" { return "", nil }
        want := []string{"install", "--path="+target, "--udid=UDID"}
        if name != ios || !reflect.DeepEqual(args, want) { t.Fatal(name, args) }
        return "", nil
    }
    if err := a.install(target, device{platform:"ios", id:"UDID"}); err != nil { t.Fatal(err) }
}

func TestInstallFailureDoesNotReportSuccess(t *testing.T) {
    fakeTool(t, "adb")
    a := testApp(t)
    target := filepath.Join(t.TempDir(), "app.apk")
    identityFixture(t, target)
    a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
        if args[2] == "shell" { return "", nil }
        return "", errors.New("signature mismatch")
    }
    if err := a.install(target, device{platform:"android", id:"one"}); err == nil { t.Fatal("Failure ignored") }
    if strings.Contains(a.out.(*bytes.Buffer).String(), "successfully") { t.Fatal("False success") }
}

func TestBundleInstall(t *testing.T) {
    fakeTool(t, "adb")
    fakeTool(t, "bundletool")
    self, _ := os.Executable()
    t.Setenv("FERRIE_JAVA", self)
    t.Setenv("FERRIE_TEST_JAVA", "1")
    for _, extension := range []string{".AAB", ".apks"} {
        a := testApp(t)
        var commands [][]string
        target := filepath.Join(t.TempDir(), "app"+extension)
        if extension == ".apks" { identityFixture(t, target) }
        a.run = func(_ context.Context, _ bool, _ string, args ...string) (string, error) {
            if args[0] == "-s" { return "", nil }
            commands = append(commands, args)
            for _, arg := range args { if strings.HasPrefix(arg, "--output=") { identityFixture(t, strings.TrimPrefix(arg, "--output=")) } }
            return "", nil
        }
        if err := a.install(target, device{platform:"android", id:"selected"}); err != nil { t.Fatal(err) }
        want := 1
        if extension == ".AAB" { want = 2 }
        if len(commands) != want { t.Fatal(commands) }
        for _, command := range commands {
            if !strings.Contains(strings.Join(command, " "), "--device-id=selected") { t.Fatal(command) }
        }
        if extension == ".AAB" {
            for _, arg := range commands[0] {
                if strings.HasPrefix(arg, "--output=") {
                    if _, err := os.Stat(filepath.Dir(strings.TrimPrefix(arg, "--output="))); !os.IsNotExist(err) { t.Fatal("Temporary output was not removed") }
                }
            }
        }
    }
}

func TestFailedBuildDoesNotInstall(t *testing.T) {
    fakeTool(t, "adb")
    fakeTool(t, "bundletool")
    self, _ := os.Executable()
    t.Setenv("FERRIE_JAVA", self)
    t.Setenv("FERRIE_TEST_JAVA", "1")
    a := testApp(t)
    count := 0
    a.run = func(context.Context, bool, string, ...string) (string, error) { count++; return "", errors.New("failed") }
    if err := a.install("app.aab", device{platform:"android", id:"one"}); err == nil || count != 1 { t.Fatal(count, err) }
}

func TestDependenciesRequireConsent(t *testing.T) {
    a := testApp(t)
    // bundletool is searched in these homes; isolate from the developer's cache.
    t.Setenv("HOME", t.TempDir())
    t.Setenv("USERPROFILE", t.TempDir())
    t.Setenv("FERRIE_BUNDLETOOL", "")
    for _, answer := range []string{"", "\n", "n\n"} {
        a.in, a.reader, a.interactive = strings.NewReader(answer), nil, true
        if err := a.prepare("bundletool"); err == nil || !strings.Contains(err.Error(), "cancelled") { t.Fatal(err) }
        entries, _ := os.ReadDir(a.cache)
        if len(entries) != 0 { t.Fatal("Refusal changed the tool cache") }
    }
    a.interactive = false
    if err := a.prepare("bundletool"); err == nil || !strings.Contains(err.Error(), "non-interactive") { t.Fatal(err) }
}

func TestExistingDependencyNeedsNoConsent(t *testing.T) {
    fakeTool(t, "adb")
    a := testApp(t)
    if err := a.prepare("adb"); err != nil { t.Fatal(err) }
    if a.out.(*bytes.Buffer).Len() != 0 { t.Fatal("Unexpected prompt") }
}

func TestCompletionNeverInstallsTools(t *testing.T) {
    a := testApp(t)
    t.Setenv("FERRIE_ADB", filepath.Join(t.TempDir(), "missing"))
    t.Setenv("FERRIE_IOS", filepath.Join(t.TempDir(), "missing"))
    if err := a.execute([]string{"__devices"}); err != nil { t.Fatal(err) }
    if a.out.(*bytes.Buffer).Len() != 0 || a.err.(*bytes.Buffer).Len() != 0 { t.Fatal("Completion emitted errors or prompts") }
}

func TestSharedInputPreservesSelection(t *testing.T) {
    a := testApp(t)
    a.in, a.interactive = strings.NewReader("y\n2\ny\n"), true
    if answer, err := a.readLine(); err != nil || answer != "y" { t.Fatal(answer, err) }
    if got, err := a.selectDevice([]device{{id:"one", state:"device"}, {id:"two", state:"device"}}, ""); err != nil || got.id != "two" { t.Fatal(got, err) }
}

func TestDownloadChecksumAndHTTPFailure(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/bad" { w.WriteHeader(404); return }
        io.WriteString(w, "test")
    }))
    defer server.Close()
    a := testApp(t)
    destination := filepath.Join(t.TempDir(), "download")
    checksum := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
    if err := a.download(server.URL, destination, checksum); err != nil { t.Fatal(err) }
    if err := a.download(server.URL, destination, strings.Repeat("0", 64)); err == nil { t.Fatal("Checksum mismatch ignored") }
    if err := a.download(server.URL + "/bad", destination, ""); err == nil { t.Fatal("HTTP failure ignored") }
}

func TestArchiveTraversal(t *testing.T) {
    for _, name := range []string{"../escape", "/absolute", `..\escape`, "C:/outside"} {
        if _, err := archivePath(t.TempDir(), name); err == nil { t.Fatal(name) }
    }
    archive := filepath.Join(t.TempDir(), "bad.zip")
    file, _ := os.Create(archive)
    writer := zip.NewWriter(file)
    entry, _ := writer.Create("../outside")
    entry.Write([]byte("bad"))
    writer.Close()
    file.Close()
    if err := extractZIP(archive, t.TempDir()); err == nil { t.Fatal("Archive traversal accepted") }
}

func TestRealCommandFailure(t *testing.T) {
    runner := commandRunner(io.Discard, io.Discard)
    if _, err := runner(context.Background(), true, "ferrie-command-that-does-not-exist"); err == nil { t.Fatal("Missing tool accepted") }
}

func TestVersionFile(t *testing.T) {
    data, err := os.ReadFile("VERSION.txt")
    if err != nil || strings.TrimSpace(string(data)) != version { t.Fatal("Version mismatch", err) }
}

type roundTripFunc func(*http.Request) (*http.Response, error)
func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestApprovedDependencyDownloadsOnceAndReusesCache(t *testing.T) {
    t.Setenv("HOME", t.TempDir())
    t.Setenv("USERPROFILE", t.TempDir())
    t.Setenv("FERRIE_BUNDLETOOL", "")
    var archive bytes.Buffer
    writer := zip.NewWriter(&archive)
    entry, _ := writer.Create("META-INF/MANIFEST.MF")
    entry.Write([]byte("Manifest-Version: 1.0\n"))
    writer.Close()
    previous := http.DefaultTransport
    t.Cleanup(func() { http.DefaultTransport = previous })
    downloads := 0
    http.DefaultTransport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
        downloads++
        if request.URL.Host != "github.com" { t.Fatal(request.URL) }
        return &http.Response{StatusCode:200, Body:io.NopCloser(bytes.NewReader(archive.Bytes())), Header:make(http.Header)}, nil
    })
    a := testApp(t)
    a.in, a.interactive = strings.NewReader("y\n"), true
    if err := a.prepare("bundletool"); err != nil { t.Fatal(err) }
    if !regular(filepath.Join(a.cache, "bundletool.jar")) || downloads != 1 { t.Fatal("Approved dependency not installed", downloads) }
    a.interactive = false
    if err := a.prepare("bundletool"); err != nil || downloads != 1 { t.Fatal("Cached dependency prompted or downloaded again", err) }
}

func TestVersionOutput(t *testing.T) {
    const want = "ferrie version 0.2.2 (2026-10-06)\nhttps://github.com/leo1394/homebrew-ferrie\n"
    for _, argument := range []string{"--version", "version", "-v"} {
        a := testApp(t)
        if err := a.execute([]string{argument}); err != nil { t.Fatal(err) }
        if got := a.out.(*bytes.Buffer).String(); got != want { t.Errorf("%s: got %q, want %q", argument, got, want) }
    }
}
