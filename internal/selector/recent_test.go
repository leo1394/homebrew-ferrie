package selector

import (
    "bytes"
    "strings"
    "testing"
)

func TestRecentSingleChoiceControls(t *testing.T) {
    state := newSinglePickerState([]Candidate{{Name: "first"}, {Name: "second"}}, Prompt{Title: "Recent URLs", EmptySelectionNotice: "Select one URL before confirming."})
    state.handle(key{kind: keyEnter})
    if state.mode != modePicking || state.notice == "" { t.Fatal("Enter must require selection") }
    state.handle(key{kind: keyRune, rune: 'F'})
    if state.cursor != 1 || !state.selected[0] { t.Fatal("F must select and move") }
    state.handle(key{kind: keyRune, rune: 'f'})
    if state.selectedCount() != 1 || !state.selected[1] { t.Fatal("Must remain single choice") }
    state.handle(key{kind: keyRune, rune: 'k'})
    state.handle(key{kind: keyDown})
    if state.cursor != 1 { t.Fatal("Navigation differs") }
    state.handle(key{kind: keyEnter})
    if state.mode != modeConfirming || state.selectedCandidates()[0].Name != "second" { t.Fatal("Wrong selection") }
    for _, input := range []key{{kind: keyEscape}, {kind: keyRune, rune: 'q'}} {
        other := newSinglePickerState([]Candidate{{Name: "first"}}, Prompt{})
        other.handle(input)
        if other.mode != modeCancelled { t.Fatal("Cancel differs") }
    }
}

func TestRecentPickerRenderingMatchesConven(t *testing.T) {
    state := newSinglePickerState([]Candidate{{Name: "https://example.com/app"}}, Prompt{Title: "Select recent installation URL"})
    state.handle(key{kind: keyRune, rune: 'f'})
    var out bytes.Buffer
    if err := renderPicker(&out, state, 100, 24); err != nil { t.Fatal(err) }
    expected := "Select recent installation URL\r\n\r\n> [✔︎] https://example.com/app\r\n\r\nShowing 1-1 of 1 · selected 1\r\n[f] selection · [Enter] confirm · [q/Esc] cancel\r\n"
    if out.String() != expected { t.Fatalf("Unexpected rendering: %q", out.String()) }
    if strings.Contains(out.String(), "Confirm?") { t.Fatal("Unexpected second confirmation") }
}
