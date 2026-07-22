package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestFuzzyScoreMatchesInOrderSubsequence(t *testing.T) {
	cases := []struct {
		query, candidate string
		want             bool
	}{
		{"wk", "work", true},
		{"work", "work", true},
		{"WORK", "work", true}, // case-insensitive
		{"", "anything", true}, // empty query matches everything
		{"kw", "work", false},  // out of order
		{"xyz", "work", false}, // not present
	}
	for _, c := range cases {
		got, _ := fuzzyScore(c.query, c.candidate)
		if got != c.want {
			t.Errorf("fuzzyScore(%q, %q) matched = %v, want %v", c.query, c.candidate, got, c.want)
		}
	}
}

func TestFuzzyScoreRanksContiguousAndEarlyMatchesBetter(t *testing.T) {
	// "per" is contiguous+early in "personal" but scattered+late in "peer-review".
	matched1, s1 := fuzzyScore("per", "personal")
	matched2, s2 := fuzzyScore("per", "peer-review")
	if !matched1 || !matched2 {
		t.Fatalf("expected both to match, got %v %v", matched1, matched2)
	}
	if s1 >= s2 {
		t.Errorf("score(personal)=%d, score(peer-review)=%d — want personal to score lower (better)", s1, s2)
	}
}

func TestFilterProfilesOrdersBestMatchFirstAndExcludesNonMatches(t *testing.T) {
	names := []string{"personal", "work", "peer-review"}
	got := filterProfiles(names, "per")
	want := []string{"personal", "peer-review"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("filterProfiles = %v, want %v", got, want)
	}
}

func TestFilterProfilesEmptyQueryReturnsAllSorted(t *testing.T) {
	names := []string{"work", "personal"}
	got := filterProfiles(names, "")
	if len(got) != 2 {
		t.Fatalf("filterProfiles(\"\") = %v, want both names present", got)
	}
}

func TestPickerStateTypingFiltersAndResetsCursor(t *testing.T) {
	st := newPickerState([]string{"work", "personal", "peer-review"})
	st, enter := st.apply(key{kind: keyRune, r: 'p'})
	if enter {
		t.Fatal("typing must not signal enter")
	}
	st, _ = st.apply(key{kind: keyRune, r: 'e'})
	st, _ = st.apply(key{kind: keyRune, r: 'r'})
	if st.query != "per" {
		t.Fatalf("query = %q, want %q", st.query, "per")
	}
	if got, want := len(st.filtered), 2; got != want {
		t.Fatalf("filtered = %v, want %d entries", st.filtered, want)
	}
	if st.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 after filtering", st.cursor)
	}
}

func TestPickerStateBackspaceWidensFilterAgain(t *testing.T) {
	st := newPickerState([]string{"work", "personal"})
	st, _ = st.apply(key{kind: keyRune, r: 'w'})
	if len(st.filtered) != 1 {
		t.Fatalf("after typing 'w', filtered = %v, want 1 entry", st.filtered)
	}
	st, _ = st.apply(key{kind: keyBackspace})
	if st.query != "" {
		t.Fatalf("query after backspace = %q, want empty", st.query)
	}
	if len(st.filtered) != 2 {
		t.Fatalf("after backspace, filtered = %v, want both entries back", st.filtered)
	}
}

func TestPickerStateBackspaceOnEmptyQueryIsNoop(t *testing.T) {
	st := newPickerState([]string{"work"})
	next, enter := st.apply(key{kind: keyBackspace})
	if enter {
		t.Fatal("backspace must not signal enter")
	}
	if next.query != "" {
		t.Fatalf("query = %q, want empty", next.query)
	}
}

func TestPickerStateArrowsMoveAndWrapCursor(t *testing.T) {
	st := newPickerState([]string{"a", "b", "c"})
	st, _ = st.apply(key{kind: keyDown})
	if st.cursor != 1 {
		t.Fatalf("cursor after one down = %d, want 1", st.cursor)
	}
	st, _ = st.apply(key{kind: keyDown})
	st, _ = st.apply(key{kind: keyDown}) // wraps past the end back to 0
	if st.cursor != 0 {
		t.Fatalf("cursor after wrapping down = %d, want 0", st.cursor)
	}
	st, _ = st.apply(key{kind: keyUp}) // wraps the other way
	if st.cursor != 2 {
		t.Fatalf("cursor after wrapping up = %d, want 2", st.cursor)
	}
}

func TestPickerStateArrowsOnEmptyFilterAreNoop(t *testing.T) {
	st := newPickerState([]string{"work"})
	st, _ = st.apply(key{kind: keyRune, r: 'z'}) // no match
	if len(st.filtered) != 0 {
		t.Fatalf("filtered = %v, want empty", st.filtered)
	}
	next, _ := st.apply(key{kind: keyDown})
	if next.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 (unchanged) on empty filter", next.cursor)
	}
}

func TestPickerStateEnterSignalsAndSelectedReturnsHighlighted(t *testing.T) {
	st := newPickerState([]string{"work", "personal"})
	st, _ = st.apply(key{kind: keyDown})
	st, enter := st.apply(key{kind: keyEnter})
	if !enter {
		t.Fatal("enter key must signal enter")
	}
	name, ok := st.selected()
	if !ok || name != "personal" {
		t.Fatalf("selected() = (%q, %v), want (\"personal\", true)", name, ok)
	}
}

func TestPickerStateEnterWithNoMatchesLeavesNothingSelected(t *testing.T) {
	st := newPickerState([]string{"work"})
	st, _ = st.apply(key{kind: keyRune, r: 'z'})
	_, enter := st.apply(key{kind: keyEnter})
	if !enter {
		t.Fatal("enter key must signal enter even with no matches")
	}
	if _, ok := st.selected(); ok {
		t.Fatal("selected() = ok on an empty filtered list, want false")
	}
}

func TestReadKeyDecodesPrintableRune(t *testing.T) {
	k, err := readKey(bufio.NewReader(strings.NewReader("w")))
	if err != nil {
		t.Fatalf("readKey: %v", err)
	}
	if k.kind != keyRune || k.r != 'w' {
		t.Errorf("readKey = %+v, want rune 'w'", k)
	}
}

func TestReadKeyDecodesControlAndArrowSequences(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  keyKind
	}{
		{"enter-cr", "\r", keyEnter},
		{"enter-lf", "\n", keyEnter},
		{"backspace-del", "\x7f", keyBackspace},
		{"ctrl-c", "\x03", keyCancel},
		{"arrow-up", "\x1b[A", keyUp},
		{"arrow-down", "\x1b[B", keyDown},
		{"lone-esc", "\x1b", keyCancel},
		{"unrecognized-escape", "\x1b[Z", keyCancel},
	}
	for _, c := range cases {
		k, err := readKey(bufio.NewReader(strings.NewReader(c.input)))
		if err != nil {
			t.Errorf("%s: readKey error: %v", c.name, err)
			continue
		}
		if k.kind != c.want {
			t.Errorf("%s: readKey kind = %v, want %v", c.name, k.kind, c.want)
		}
	}
}

func TestRenderPickerShowsQueryAndHighlightsCursor(t *testing.T) {
	st := newPickerState([]string{"work", "personal"})
	st, _ = st.apply(key{kind: keyDown})
	var buf bytes.Buffer
	renderPicker(&buf, 0, st)
	out := buf.String()
	if !strings.Contains(out, "profile> ") {
		t.Errorf("render missing prompt: %q", out)
	}
	if !strings.Contains(out, "> personal") {
		t.Errorf("render missing highlighted selection: %q", out)
	}
	if !strings.Contains(out, "  work") {
		t.Errorf("render missing unselected entry: %q", out)
	}
}

func TestRenderPickerReportsNoMatch(t *testing.T) {
	st := newPickerState([]string{"work"})
	st, _ = st.apply(key{kind: keyRune, r: 'z'})
	var buf bytes.Buffer
	renderPicker(&buf, 0, st)
	if !strings.Contains(buf.String(), "(no match)") {
		t.Errorf("render = %q, want a no-match indicator", buf.String())
	}
}

func TestProfileNamesSorted(t *testing.T) {
	m := Manifest{Profiles: map[string][]string{"work": nil, "personal": nil}}
	got := profileNames(m)
	want := []string{"personal", "work"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("profileNames = %v, want %v", got, want)
	}
}
