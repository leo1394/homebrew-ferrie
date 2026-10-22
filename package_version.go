package main

import (
    "archive/zip"
    "bytes"
    "encoding/xml"
    "fmt"
    "io"
    "path/filepath"
    "strings"
    "unicode"

    "github.com/shogo82148/androidbinary"
    "howett.net/plist"
)

// Version metadata is informational: missing metadata must not prevent installation.
func packageVersionLabel(target string) string {
    archive, err := zip.OpenReader(target)
    if err != nil { return "" }
    defer archive.Close()
    extension := strings.ToLower(filepath.Ext(target))
    for _, file := range archive.File {
        parts := strings.Split(file.Name, "/")
        apk := extension == ".apk" && file.Name == "AndroidManifest.xml"
        ipa := extension == ".ipa" && len(parts) == 3 && parts[0] == "Payload" && strings.HasSuffix(parts[1], ".app") && parts[2] == "Info.plist"
        if !apk && !ipa { continue }
        data, err := readMetadata(file)
        if err != nil { return "" }
        version, build := "", ""
        if ipa {
            var info struct { Version string `plist:"CFBundleShortVersionString"`; Build string `plist:"CFBundleVersion"` }
            if _, err := plist.Unmarshal(data, &info); err != nil { return "" }
            version, build = info.Version, info.Build
        } else {
            var manifest struct { Version string `xml:"versionName,attr"`; Build string `xml:"versionCode,attr"` }
            reader := io.Reader(bytes.NewReader(data))
            if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("<")) {
                binary, err := androidbinary.NewXMLFile(bytes.NewReader(data))
                if err != nil { return "" }
                reader = binary.Reader()
            }
            if err := xml.NewDecoder(reader).Decode(&manifest); err != nil { return "" }
            version, build = manifest.Version, manifest.Build
        }
        // Do not emit terminal controls or unresolved Android resource references.
        clean := func(value string) string {
            if strings.HasPrefix(value, "@") { return "" }
            return strings.Map(func(r rune) rune { if unicode.IsControl(r) { return -1 }; return r }, value)
        }
        version, build = clean(version), clean(build)
        if version != "" && build != "" { return fmt.Sprintf(" (%s+%s)", version, build) }
        if version != "" { return " (" + version + ")" }
        if build != "" { return " (build " + build + ")" }
    }
    return ""
}
