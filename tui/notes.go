package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Anchor identifies the heading a note is attached to as a path from the root.
// An empty path means the note is anchored to the document as a whole
// (e.g. content before the first heading).
type Anchor struct {
	Path []string
}

func (a Anchor) String() string {
	return strings.Join(a.Path, " / ")
}

func (a Anchor) Equal(b Anchor) bool {
	if len(a.Path) != len(b.Path) {
		return false
	}
	for i := range a.Path {
		if a.Path[i] != b.Path[i] {
			return false
		}
	}
	return true
}

// Note is one annotation on a heading.
type Note struct {
	Anchor Anchor
	Body   string
}

// NoteStore holds the notes for a single document and knows how to read/write
// the sibling <doc>.notes.md file.
type NoteStore struct {
	docPath string
	Notes   []Note
}

// siblingPath returns the path to the notes file next to the source doc.
// foo.md -> foo.notes.md; foo -> foo.notes.md.
func siblingPath(docPath string) string {
	dir := filepath.Dir(docPath)
	base := filepath.Base(docPath)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, stem+".notes.md")
}

// NewNoteStore loads existing notes from the sibling file if present.
func NewNoteStore(docPath string) *NoteStore {
	s := &NoteStore{docPath: docPath}
	s.Load()
	return s
}

func (s *NoteStore) Path() string { return siblingPath(s.docPath) }

// Load reads the sibling notes file. Missing file is not an error.
func (s *NoteStore) Load() {
	data, err := os.ReadFile(s.Path())
	if err != nil {
		return
	}
	s.Notes = parseNotesFile(string(data))
}

// Save writes notes to the sibling file. If there are no notes, the file is
// removed instead so a clean state leaves no artifacts.
func (s *NoteStore) Save() error {
	if len(s.Notes) == 0 {
		err := os.Remove(s.Path())
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	body := formatNotesFile(filepath.Base(s.docPath), s.Notes)
	return os.WriteFile(s.Path(), []byte(body), 0644)
}

// Set upserts a note for the given anchor. Empty body deletes.
func (s *NoteStore) Set(anchor Anchor, body string) {
	body = strings.TrimSpace(body)
	for i, n := range s.Notes {
		if n.Anchor.Equal(anchor) {
			if body == "" {
				s.Notes = append(s.Notes[:i], s.Notes[i+1:]...)
			} else {
				s.Notes[i].Body = body
			}
			return
		}
	}
	if body == "" {
		return
	}
	s.Notes = append(s.Notes, Note{Anchor: anchor, Body: body})
}

// Get returns the body for an anchor, or empty if none.
func (s *NoteStore) Get(anchor Anchor) (string, bool) {
	for _, n := range s.Notes {
		if n.Anchor.Equal(anchor) {
			return n.Body, true
		}
	}
	return "", false
}

// Count returns the number of notes.
func (s *NoteStore) Count() int { return len(s.Notes) }

// formatNotesFile renders the notes as a markdown document keyed by the source
// doc's basename. Format is round-trippable by parseNotesFile.
func formatNotesFile(docName string, notes []Note) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Notes on %s\n", docName)
	for _, n := range notes {
		anchor := n.Anchor.String()
		if anchor == "" {
			anchor = "(document)"
		}
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", anchor, strings.TrimSpace(n.Body))
	}
	return b.String()
}

// parseNotesFile reads the inverse of formatNotesFile. Tolerant of leading
// blank lines and trailing whitespace.
func parseNotesFile(content string) []Note {
	lines := strings.Split(content, "\n")
	var notes []Note
	var cur *Note
	var body []string
	flush := func() {
		if cur != nil {
			cur.Body = strings.TrimSpace(strings.Join(body, "\n"))
			if cur.Body != "" {
				notes = append(notes, *cur)
			}
		}
		cur = nil
		body = nil
	}
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "# "):
			// Document header — ignore.
			continue
		case strings.HasPrefix(line, "## "):
			flush()
			anchorStr := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			cur = &Note{Anchor: parseAnchor(anchorStr)}
		default:
			if cur != nil {
				body = append(body, line)
			}
		}
	}
	flush()
	return notes
}

func parseAnchor(s string) Anchor {
	if s == "" || s == "(document)" {
		return Anchor{}
	}
	parts := strings.Split(s, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return Anchor{Path: out}
}
