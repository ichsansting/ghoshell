package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"
)

// profileNames returns a manifest's profile names, sorted — the candidate set
// the picker filters over (and the deterministic order before any typing).
func profileNames(m Manifest) []string {
	names := make([]string, 0, len(m.Profiles))
	for n := range m.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// fuzzyScore reports whether every rune of query appears in candidate, in
// order, case-insensitively (a subsequence match, fzf-style) and a score
// where lower ranks better: gaps between consecutive matched runes accrue,
// so contiguous and early matches outrank scattered ones.
func fuzzyScore(query, candidate string) (matched bool, score int) {
	q := []rune(strings.ToLower(query))
	c := []rune(strings.ToLower(candidate))
	if len(q) == 0 {
		return true, len(c)
	}
	qi, last := 0, -1
	for ci, r := range c {
		if qi >= len(q) {
			break
		}
		if r == q[qi] {
			if last >= 0 {
				score += ci - last - 1
			} else {
				score += ci
			}
			last = ci
			qi++
		}
	}
	return qi == len(q), score
}

// filterProfiles returns names matching query, best score first. Pure — the
// seam the picker's live-filtering is tested against.
func filterProfiles(names []string, query string) []string {
	type scored struct {
		name  string
		score int
	}
	var matches []scored
	for _, n := range names {
		if ok, score := fuzzyScore(query, n); ok {
			matches = append(matches, scored{n, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score < matches[j].score })
	out := make([]string, len(matches))
	for i, m := range matches {
		out[i] = m.name
	}
	return out
}

// pickerState is the picker's whole state: the full candidate set, the typed
// query, the current filtered/ranked matches, and which one is highlighted.
// Pure and immutable-by-convention (methods return a new state) so the
// keystroke logic is testable without a terminal.
type pickerState struct {
	names    []string
	query    string
	filtered []string
	cursor   int
}

func newPickerState(names []string) pickerState {
	return pickerState{names: names, filtered: filterProfiles(names, "")}
}

func (s pickerState) setQuery(q string) pickerState {
	s.query = q
	s.filtered = filterProfiles(s.names, q)
	s.cursor = 0
	return s
}

func (s pickerState) moveCursor(delta int) pickerState {
	if len(s.filtered) == 0 {
		return s
	}
	s.cursor = ((s.cursor+delta)%len(s.filtered) + len(s.filtered)) % len(s.filtered)
	return s
}

// selected returns the highlighted profile, or false if no match is under it
// (empty filtered list).
func (s pickerState) selected() (string, bool) {
	if s.cursor < 0 || s.cursor >= len(s.filtered) {
		return "", false
	}
	return s.filtered[s.cursor], true
}

type keyKind int

const (
	keyRune keyKind = iota
	keyBackspace
	keyEnter
	keyUp
	keyDown
	keyCancel
)

type key struct {
	kind keyKind
	r    rune
}

// apply advances the state by one keystroke. The bool return reports whether
// enter was pressed (the caller decides what to do with the current
// selection); cancel is surfaced to the caller as an error, not modeled here.
func (s pickerState) apply(k key) (next pickerState, enter bool) {
	switch k.kind {
	case keyRune:
		return s.setQuery(s.query + string(k.r)), false
	case keyBackspace:
		if len(s.query) == 0 {
			return s, false
		}
		r := []rune(s.query)
		return s.setQuery(string(r[:len(r)-1])), false
	case keyUp:
		return s.moveCursor(-1), false
	case keyDown:
		return s.moveCursor(1), false
	case keyEnter:
		return s, true
	default:
		return s, false
	}
}

// readKey decodes one keystroke off a raw-mode terminal stream: printable
// runes, backspace, enter, Ctrl-C, and the arrow keys' escape sequences
// (ESC [ A / ESC [ B). Any other escape sequence or a lone ESC is treated as
// cancel, since gho has no use for it and swallowing it silently would leave
// the picker stuck.
func readKey(r *bufio.Reader) (key, error) {
	ch, _, err := r.ReadRune()
	if err != nil {
		return key{}, err
	}
	switch ch {
	case '\r', '\n':
		return key{kind: keyEnter}, nil
	case 127, 8:
		return key{kind: keyBackspace}, nil
	case 3: // Ctrl-C
		return key{kind: keyCancel}, nil
	case 0x1b: // ESC, possibly the start of an arrow-key sequence
		next, _, err := r.ReadRune()
		if err != nil || next != '[' {
			return key{kind: keyCancel}, nil
		}
		dir, _, err := r.ReadRune()
		if err != nil {
			return key{kind: keyCancel}, nil
		}
		switch dir {
		case 'A':
			return key{kind: keyUp}, nil
		case 'B':
			return key{kind: keyDown}, nil
		default:
			return key{kind: keyCancel}, nil
		}
	default:
		return key{kind: keyRune, r: ch}, nil
	}
}

// maxShown caps how many matches the picker prints at once — plenty for a
// manifest's profile count without the render scrolling off a normal
// terminal.
const maxShown = 10

// renderPicker redraws the picker in place: it rewinds prevLines (the height
// of its own last render) before drawing, so retyping never scrolls the
// terminal. It returns the new height for the next call to rewind by.
func renderPicker(w io.Writer, prevLines int, st pickerState) int {
	if prevLines > 0 {
		fmt.Fprintf(w, "\x1b[%dA", prevLines)
	}
	fmt.Fprint(w, "\r\x1b[J")
	fmt.Fprintf(w, "profile> %s\n", st.query)
	lines := 1
	shown := st.filtered
	if len(shown) > maxShown {
		shown = shown[:maxShown]
	}
	for i, name := range shown {
		marker := "  "
		if i == st.cursor {
			marker = "> "
		}
		fmt.Fprintf(w, "%s%s\n", marker, name)
		lines++
	}
	if len(st.filtered) == 0 {
		fmt.Fprintln(w, "  (no match)")
		lines++
	}
	return lines
}

// pickProfilePrompt is the real, interactive picker: raw-mode keystrokes
// drive live fuzzy filtering over names, rendered to stderr (stdout stays
// clean for any future piping). Only called by launch when there is more
// than one profile — the single-profile auto-select lives at the call site,
// so this never needs to special-case it.
func pickProfilePrompt(names []string) (string, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", fmt.Errorf("enter raw mode: %w", err)
	}
	defer term.Restore(fd, oldState)

	st := newPickerState(names)
	r := bufio.NewReader(os.Stdin)
	prevLines := 0
	for {
		prevLines = renderPicker(os.Stderr, prevLines, st)
		k, err := readKey(r)
		if err != nil {
			return "", fmt.Errorf("read key: %w", err)
		}
		if k.kind == keyCancel {
			return "", fmt.Errorf("profile selection cancelled")
		}
		var enter bool
		st, enter = st.apply(k)
		if enter {
			if name, ok := st.selected(); ok {
				fmt.Fprintln(os.Stderr)
				return name, nil
			}
			// Enter with no match under the cursor: keep typing/browsing.
		}
	}
}
