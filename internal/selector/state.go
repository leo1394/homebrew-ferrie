package selector

type Candidate struct {
	Name   string
	Path   string
	Detail string
	Tag    string
}

type Prompt struct {
	Title                string
	EmptySelectionNotice string
}

type pickerMode uint8

const (
	modePicking pickerMode = iota
	modeConfirming
	modeCancelled
)

type keyKind uint8

const (
	keyRune keyKind = iota
	keyUp
	keyDown
	keyEnter
	keyEscape
	keyBackspace
)

type key struct {
	kind keyKind
	rune rune
}

type pickerState struct {
	candidates   []Candidate
	cursor       int
	selected     []bool
	mode         pickerMode
	notice       string
	prompt       Prompt
}

func newSinglePickerState(candidates []Candidate, prompt Prompt) *pickerState {
    return &pickerState{
        candidates: candidates,
        selected: make([]bool, len(candidates)),
        mode: modePicking,
        prompt: prompt,
    }
}

func (s *pickerState) handle(input key) {
	if s.mode != modePicking {
		return
	}

	switch input.kind {
	case keyUp:
		s.move(-1)
	case keyDown:
		s.move(1)
	case keyEnter:
		if s.selectedCount() == 0 {
			s.notice = s.prompt.EmptySelectionNotice
			return
		}
		s.mode = modeConfirming
		s.notice = ""
	case keyEscape:
		s.mode = modeCancelled
	case keyRune:
		s.handlePickerRune(input.rune)
	}
}

func (s *pickerState) handlePickerRune(value rune) {
	switch value {
	case 'j':
		s.move(1)
	case 'k':
		s.move(-1)
	case 'f':
		s.toggleCurrent()
	case 'F':
		s.toggleCurrent()
		s.move(1)
	case 'q':
		s.mode = modeCancelled
	}
}

func (s *pickerState) move(delta int) {
	if len(s.candidates) == 0 {
		return
	}
	next := s.cursor + delta
	if next < 0 {
		next = 0
	}
	if next >= len(s.candidates) {
		next = len(s.candidates) - 1
	}
	s.cursor = next
	s.notice = ""
}

func (s *pickerState) toggleCurrent() {
	if len(s.candidates) == 0 {
		return
	}
	wasSelected := s.selected[s.cursor]
	for index := range s.selected {
		s.selected[index] = false
	}
	s.selected[s.cursor] = !wasSelected
	s.notice = ""
}

func (s *pickerState) selectedCount() int {
	count := 0
	for _, selected := range s.selected {
		if selected {
			count++
		}
	}
	return count
}

func (s *pickerState) selectedCandidates() []Candidate {
	candidates := make([]Candidate, 0, s.selectedCount())
	for index, selected := range s.selected {
		if selected {
			candidates = append(candidates, s.candidates[index])
		}
	}
	return candidates
}
