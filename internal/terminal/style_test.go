package terminal

import "testing"

func TestConvenSelectionColors(t *testing.T) {
    style := Style{enabled: true}
    if got := style.Selection("selected", true); got != "\x1b[1;37;41mselected\x1b[0m" { t.Fatalf("Active color: %q", got) }
    if got := style.Selection("selected", false); got != "\x1b[1;32mselected\x1b[0m" { t.Fatalf("Selected color: %q", got) }
    if got := (Style{}).Selection("selected", true); got != "selected" { t.Fatalf("Disabled color: %q", got) }
}
