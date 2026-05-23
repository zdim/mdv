package tui

import (
	"reflect"
	"testing"
)

func TestNotesRoundTrip(t *testing.T) {
	notes := []Note{
		{Anchor: Anchor{Path: []string{"Implementation", "Phase 2", "Database changes"}}, Body: "Should we shard first?"},
		{Anchor: Anchor{Path: []string{"Implementation", "Phase 2", "Migration"}}, Body: "Timeline is tight.\n\nBackfill is 4h per quarter."},
		{Anchor: Anchor{}, Body: "Overall the plan looks reasonable."},
	}
	body := formatNotesFile("plan.md", notes)
	got := parseNotesFile(body)
	if !reflect.DeepEqual(got, notes) {
		t.Fatalf("round-trip mismatch.\nsource: %#v\ngot:    %#v\nmarkdown:\n%s", notes, got, body)
	}
}

func TestParseAnchor(t *testing.T) {
	cases := []struct {
		in   string
		want Anchor
	}{
		{"", Anchor{}},
		{"(document)", Anchor{}},
		{"Foo", Anchor{Path: []string{"Foo"}}},
		{"Foo / Bar", Anchor{Path: []string{"Foo", "Bar"}}},
		{" Foo /  Bar / Baz ", Anchor{Path: []string{"Foo", "Bar", "Baz"}}},
	}
	for _, c := range cases {
		got := parseAnchor(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseAnchor(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestHeadingPath(t *testing.T) {
	hs := []Heading{
		{Level: 1, Name: "Doc"},
		{Level: 2, Name: "Intro"},
		{Level: 2, Name: "Plan"},
		{Level: 3, Name: "Phase 1"},
		{Level: 3, Name: "Phase 2"},
		{Level: 4, Name: "Database"},
	}
	cases := []struct {
		i    int
		want []string
	}{
		{0, []string{"Doc"}},
		{2, []string{"Doc", "Plan"}},
		{4, []string{"Doc", "Plan", "Phase 2"}},
		{5, []string{"Doc", "Plan", "Phase 2", "Database"}},
	}
	for _, c := range cases {
		got := HeadingPath(hs, c.i)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("HeadingPath(%d) = %v, want %v", c.i, got, c.want)
		}
	}
}
