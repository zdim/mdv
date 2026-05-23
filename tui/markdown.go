package tui

import (
	"regexp"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// ansiRE matches CSI escape sequences emitted by glamour styling so we can
// match heading lines by their plain text.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// Heading is a markdown heading paired with the line offset it ended up at in
// glamour's rendered output, so the viewport can scroll to it.
type Heading struct {
	Level int
	Name  string
	Line  int // line offset in the rendered (terminal-styled) output
}

// HeadingPath returns the heading path from root to the heading at index i:
// each ancestor with a strictly lower level, followed by the heading itself.
func HeadingPath(headings []Heading, i int) []string {
	if i < 0 || i >= len(headings) {
		return nil
	}
	cur := headings[i]
	path := []string{cur.Name}
	level := cur.Level
	for j := i - 1; j >= 0; j-- {
		h := headings[j]
		if h.Level < level {
			path = append([]string{h.Name}, path...)
			level = h.Level
			if level == 1 {
				break
			}
		}
	}
	return path
}

var (
	mdRendererMu sync.Mutex
	mdRenderer   *glamour.TermRenderer
	mdWidth      int
)

const defaultRenderWidth = 80

// PrewarmRenderer initializes glamour ahead of Bubble Tea taking over stdin,
// at the width that will actually be used. Termenv probes the terminal on
// first init by writing to stdout and reading stdin; doing it here prevents
// that probe from racing with user keystrokes. Matching the prewarm width to
// the real terminal width also avoids a wasteful re-init on first paint.
func PrewarmRenderer(width int) {
	if width <= 0 {
		width = defaultRenderWidth
	}
	mdRendererMu.Lock()
	defer mdRendererMu.Unlock()
	if mdRenderer != nil && mdWidth == width {
		return
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return
	}
	mdRenderer = r
	mdWidth = width
	_, _ = r.Render("# warm\n")
}

// renderMarkdown returns terminal-styled output for the given markdown body,
// wrapped to width. On any rendering error, returns the raw body unchanged.
func renderMarkdown(body string, width int) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	if width <= 0 {
		width = defaultRenderWidth
	}

	mdRendererMu.Lock()
	if mdRenderer == nil || mdWidth != width {
		r, err := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width),
		)
		if err == nil {
			mdRenderer = r
			mdWidth = width
		}
	}
	r := mdRenderer
	mdRendererMu.Unlock()

	if r == nil {
		return body
	}
	out, err := r.Render(body)
	if err != nil {
		return body
	}
	return out
}

// sourceHeading is a heading parsed from raw markdown.
type sourceHeading struct {
	level int
	name  string
}

// parseSourceHeadings extracts H1–H6 ATX headings in document order. Lines
// inside fenced code blocks are ignored.
func parseSourceHeadings(body string) []sourceHeading {
	var out []sourceHeading
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		level := atxLevel(line)
		if level == 0 {
			continue
		}
		prefix := strings.Repeat("#", level) + " "
		out = append(out, sourceHeading{
			level: level,
			name:  strings.TrimSpace(strings.TrimPrefix(line, prefix)),
		})
	}
	return out
}

// atxLevel returns the heading level (1–6) for an ATX heading line, or 0 if
// the line isn't a heading.
func atxLevel(line string) int {
	i := 0
	for i < len(line) && i < 6 && line[i] == '#' {
		i++
	}
	if i == 0 {
		return 0
	}
	if i >= len(line) || line[i] != ' ' {
		return 0
	}
	return i
}

// locateHeadings walks glamour-rendered content and returns each source
// heading's rendered line offset, in document order. Matching uses the
// ANSI-stripped line text and accepts either "## Name" (default glamour) or
// just "Name" (in case styling hides the prefix).
func locateHeadings(rendered string, sources []sourceHeading) []Heading {
	if len(sources) == 0 {
		return nil
	}
	lines := strings.Split(rendered, "\n")
	out := make([]Heading, 0, len(sources))
	next := 0
	for i, line := range lines {
		if next >= len(sources) {
			break
		}
		target := sources[next]
		stripped := strings.TrimSpace(ansiRE.ReplaceAllString(line, ""))
		prefix := strings.Repeat("#", target.level) + " "
		if stripped == prefix+target.name || stripped == target.name {
			out = append(out, Heading{Line: i, Level: target.level, Name: target.name})
			next++
		}
	}
	return out
}

// ExtractHeadings parses source markdown, renders it, and locates each
// heading's line in the rendered output. Returns both the rendered content and
// the located headings. The rendered content has the literal `#+ ` heading
// prefixes replaced with a level-distinct left bar so the level reads visually
// instead of as raw markdown syntax.
func ExtractHeadings(body string, width int) (string, []Heading) {
	sources := parseSourceHeadings(body)
	rendered := renderMarkdown(body, width)
	headings := locateHeadings(rendered, sources)
	rendered = prettifyHeadings(rendered, headings)
	return rendered, headings
}

var headingMarkerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

// levelMarker returns the prefix substitute for a heading level: a Unicode
// block character whose visual weight matches the level (H1 heaviest, H4
// thinnest), in a dim color so it reads as a guide rather than a focus. The
// styled heading text supplies the actual prominence.
func levelMarker(level int) string {
	var g string
	switch level {
	case 1:
		g = "█"
	case 2:
		g = "▌"
	case 3:
		g = "▎"
	case 4:
		g = "▏"
	default:
		return "  "
	}
	return headingMarkerStyle.Render(g) + " "
}

// prettifyHeadings strips the literal `#+ ` prefix from every known heading
// line in the rendered content and prepends a level-distinct bar marker. ANSI
// styling around the heading text is preserved.
func prettifyHeadings(content string, headings []Heading) string {
	if len(headings) == 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	for _, h := range headings {
		if h.Line < 0 || h.Line >= len(lines) {
			continue
		}
		prefix := strings.Repeat("#", h.Level) + " "
		line := lines[h.Line]
		idx := strings.Index(line, prefix)
		if idx < 0 {
			continue
		}
		lines[h.Line] = line[:idx] + levelMarker(h.Level) + line[idx+len(prefix):]
	}
	return strings.Join(lines, "\n")
}
