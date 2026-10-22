package main

import (
    "os"
    "path/filepath"
    "testing"

    "howett.net/plist"
)

func TestPackageVersionLabel(t *testing.T) {
    info, err := plist.Marshal(map[string]string{"CFBundleShortVersionString":"2.3.1", "CFBundleVersion":"42"}, plist.BinaryFormat)
    if err != nil { t.Fatal(err) }
    for _, item := range []struct { extension, entry string; data []byte; want string }{
        {".apk", "AndroidManifest.xml", []byte(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" android:versionName="1.2.3" android:versionCode="123"/>`), " (1.2.3+123)"},
        {".ipa", "Payload/Test.app/Info.plist", info, " (2.3.1+42)"},
        {".ipa", "Payload/Test.app/PlugIns/Extra.appex/Info.plist", info, ""},
        {".apk", "AndroidManifest.xml", []byte(`<manifest/>`), ""},
        {".apk", "AndroidManifest.xml", []byte(`invalid`), ""},
    } {
        path := filepath.Join(t.TempDir(), "app"+item.extension)
        if err := os.WriteFile(path, identityArchive(t, map[string][]byte{item.entry:item.data}), 0o600); err != nil { t.Fatal(err) }
        if got := packageVersionLabel(path); got != item.want { t.Fatalf("got %q want %q", got, item.want) }
    }
}
