package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	helpHeight  = 1
	gutterWidth = 3 // [cursor][note][ ] reserved on every line
	scrollOff   = 5 // lines kept visible above/below cursor before viewport follows
)

type Model struct {
	docPath   string
	docName   string
	source    string
	rendered  string
	headings  []Heading
	h2Indices []int // indices into headings where Level == 2
	hasH2     bool  // cached from source so tab bar height is stable before rerender

	viewport   viewport.Model
	cursorLine int // line index in rendered content

	notes *NoteStore

	composing bool
	compose   ComposeModel

	showHelp bool

	searching   bool
	searchInput textinput.Model
	query       string
	matches     []int // line indices with a match
	matchIdx    int   // index into matches; 0-based

	width, height int
	renderedWidth int
	ready         bool
	quitting      bool
	statusMsg     string
}

func NewModel(docPath, source string) Model {
	hasH2 := false
	for _, h := range parseSourceHeadings(source) {
		if h.level == 2 {
			hasH2 = true
			break
		}
	}
	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = "search"
	ti.CharLimit = 256
	return Model{
		docPath:     docPath,
		docName:     filepath.Base(docPath),
		source:      source,
		hasH2:       hasH2,
		notes:       NewNoteStore(docPath),
		compose:     NewComposeModel(),
		searchInput: ti,
	}
}

// SetInitialSize lets Run() prerender at the real terminal width before
// Bubble Tea is started. The first View() then paints content immediately
// instead of going through a black flash while WindowSizeMsg propagates.
func (m *Model) SetInitialSize(width, height int) {
	m.width = width
	m.height = height
	contentH := m.contentHeight()
	m.viewport = viewport.New(width, contentH)
	m.viewport.YPosition = 0
	m.rerender()
	m.compose.SetSize(width, contentH)
	m.ready = true
}

// contentHeight is the height available to the markdown viewport. It excludes
// the help line at the bottom and — when present — the H2 tab bar + divider
// at the top.
func (m Model) contentHeight() int {
	c := m.height - helpHeight - m.tabBarHeight()
	if c < 1 {
		return 1
	}
	return c
}

// tabBarHeight is 2 (tab row + divider) when the doc has H2s, 0 otherwise.
func (m Model) tabBarHeight() int {
	if !m.hasH2 {
		return 0
	}
	return 2
}

// renderWidth is the width glamour is told to wrap at. The gutter consumes
// the first three columns on every line, so we hand glamour the remainder.
func (m Model) renderWidth() int {
	w := m.width - gutterWidth
	if w < 1 {
		w = 1
	}
	return w
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleResize(msg), nil
	case tea.KeyMsg:
		switch {
		case m.composing:
			return m.handleComposeKey(msg)
		case m.searching:
			return m.handleSearchKey(msg)
		default:
			return m.handleKey(msg)
		}
	}
	return m, nil
}

func (m Model) handleResize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height
	contentH := m.contentHeight()

	if !m.ready {
		m.viewport = viewport.New(m.width, contentH)
		m.viewport.YPosition = 0
		m.ready = true
		m.rerender()
	} else {
		m.viewport.Width = m.width
		m.viewport.Height = contentH
		if m.width != m.renderedWidth {
			m.rerender()
		} else {
			m.refreshViewport()
		}
	}

	m.compose.SetSize(m.width, contentH)
	return m
}

// rerender re-runs the glamour render at the current width and rebuilds the
// heading + H2 indices. Use whenever width changes or notes change.
func (m *Model) rerender() {
	rendered, headings := ExtractHeadings(m.source, m.renderWidth())
	m.rendered = rendered
	m.headings = headings
	m.renderedWidth = m.width
	m.h2Indices = m.h2Indices[:0]
	for i, h := range headings {
		if h.Level == 2 {
			m.h2Indices = append(m.h2Indices, i)
		}
	}
	// Clamp cursor in case the rendered line count shrank.
	if n := m.lineCount(); m.cursorLine >= n {
		m.cursorLine = n - 1
	}
	if m.cursorLine < 0 {
		m.cursorLine = 0
	}
	m.refreshViewport()
}

// refreshViewport reapplies cursor + note markers and pushes the result into
// the viewport. Cheap — call after any key that could shift the cursor.
func (m *Model) refreshViewport() {
	m.viewport.SetContent(applyMarkers(m.rendered, m.headings, m.cursorLine, m.notes))
}

// lineCount returns the number of lines in the rendered content.
func (m Model) lineCount() int {
	if m.rendered == "" {
		return 0
	}
	return strings.Count(m.rendered, "\n") + 1
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "?", "esc":
			m.showHelp = false
			return m, nil
		}
		return m, nil
	}
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.showHelp = true
		return m, nil
	case "enter":
		return m.openCompose()
	case "j", "down":
		return m.moveCursor(+1), nil
	case "k", "up":
		return m.moveCursor(-1), nil
	case "pgdown", "ctrl+d", " ":
		return m.moveCursor(m.halfPage()), nil
	case "pgup", "ctrl+u":
		return m.moveCursor(-m.halfPage()), nil
	case "]":
		return m.jumpH2(+1), nil
	case "[":
		return m.jumpH2(-1), nil
	case "g", "home":
		return m.gotoLine(0), nil
	case "G", "end":
		return m.gotoLine(m.lineCount() - 1), nil
	case "/":
		return m.openSearch()
	case "n":
		return m.cycleMatch(+1), nil
	case "N":
		return m.cycleMatch(-1), nil
	case "esc":
		if len(m.matches) > 0 || m.statusMsg != "" {
			m.matches = nil
			m.matchIdx = 0
			m.query = ""
			m.statusMsg = ""
			return m, nil
		}
	}
	return m, nil
}

// openSearch enters search mode, replacing the help bar with an input.
func (m Model) openSearch() (tea.Model, tea.Cmd) {
	m.searching = true
	m.searchInput.SetValue("")
	m.searchInput.Width = m.width - 2
	return m, m.searchInput.Focus()
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.query = strings.TrimSpace(m.searchInput.Value())
		m.searching = false
		m.searchInput.Blur()
		if m.query == "" {
			m.matches = nil
			return m, nil
		}
		m.matches = findMatches(m.rendered, m.query)
		if len(m.matches) == 0 {
			m.statusMsg = fmt.Sprintf("no match for %q", m.query)
			return m, nil
		}
		// Prefer the first match at or after the current cursor — feels less
		// jarring than always snapping back to the top of the doc.
		m.matchIdx = 0
		for i, line := range m.matches {
			if line >= m.cursorLine {
				m.matchIdx = i
				break
			}
		}
		return m.gotoLine(m.matches[m.matchIdx]), nil
	case "esc":
		m.searching = false
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		return m, nil
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

// cycleMatch moves to the next/prev match, wrapping around at the ends.
func (m Model) cycleMatch(delta int) Model {
	if len(m.matches) == 0 {
		return m
	}
	idx := (m.matchIdx + delta) % len(m.matches)
	if idx < 0 {
		idx += len(m.matches)
	}
	m.matchIdx = idx
	return m.gotoLine(m.matches[idx])
}

// findMatches returns line indices in `content` whose ANSI-stripped text
// contains query (case-insensitive).
func findMatches(content, query string) []int {
	if query == "" {
		return nil
	}
	q := strings.ToLower(query)
	var out []int
	for i, line := range strings.Split(content, "\n") {
		stripped := strings.ToLower(ansiRE.ReplaceAllString(line, ""))
		if strings.Contains(stripped, q) {
			out = append(out, i)
		}
	}
	return out
}

func (m Model) halfPage() int {
	h := m.viewport.Height / 2
	if h < 1 {
		h = 1
	}
	return h
}

// moveCursor shifts the cursor by delta and lets the viewport follow only if
// the cursor crosses the scrollOff margin near an edge.
func (m Model) moveCursor(delta int) Model {
	if delta == 0 {
		return m
	}
	total := m.lineCount()
	if total == 0 {
		return m
	}
	target := m.cursorLine + delta
	if target < 0 {
		target = 0
	}
	if target > total-1 {
		target = total - 1
	}
	m.cursorLine = target
	m.adjustViewportToCursor()
	m.refreshViewport()
	return m
}

// gotoLine sets the cursor to a specific line and snaps the viewport so the
// cursor sits near the top (scrollOff lines below the top edge). Used by
// jumps — [/], g/G — where the user expects a fresh anchor point, not the
// "barely move the page" feel of adjustViewportToCursor.
func (m Model) gotoLine(line int) Model {
	total := m.lineCount()
	if total == 0 {
		return m
	}
	if line < 0 {
		line = 0
	}
	if line > total-1 {
		line = total - 1
	}
	m.cursorLine = line
	m.snapViewportToCursor()
	m.refreshViewport()
	return m
}

// adjustViewportToCursor moves the viewport only when the cursor leaves the
// scrollOff band at the top or bottom edge.
func (m *Model) adjustViewportToCursor() {
	h := m.viewport.Height
	if h <= 0 {
		return
	}
	relRow := m.cursorLine - m.viewport.YOffset
	var newY int
	switch {
	case relRow < scrollOff:
		newY = m.cursorLine - scrollOff
	case relRow > h-1-scrollOff:
		newY = m.cursorLine - (h - 1 - scrollOff)
	default:
		return
	}
	m.setViewportY(newY)
}

// snapViewportToCursor positions the viewport so the cursor sits scrollOff
// lines from the top. Used by jumps.
func (m *Model) snapViewportToCursor() {
	m.setViewportY(m.cursorLine - scrollOff)
}

func (m *Model) setViewportY(y int) {
	if y < 0 {
		y = 0
	}
	h := m.viewport.Height
	maxY := m.lineCount() - h
	if maxY < 0 {
		maxY = 0
	}
	if y > maxY {
		y = maxY
	}
	m.viewport.SetYOffset(y)
}

// jumpH2 moves to the prev/next H2 tab. Sub-headings (H3/H4) are reached by
// scrolling — the cursor still tracks whichever heading is topmost.
func (m Model) jumpH2(delta int) Model {
	if len(m.h2Indices) == 0 {
		return m
	}
	cur := m.activeH2Position()
	var target int
	if cur < 0 {
		if delta > 0 {
			target = 0
		} else {
			return m
		}
	} else {
		target = cur + delta
		if target < 0 {
			target = 0
		}
		if target >= len(m.h2Indices) {
			target = len(m.h2Indices) - 1
		}
	}
	return m.gotoLine(m.headings[m.h2Indices[target]].Line)
}

// activeH2Position returns the index into h2Indices of the H2 the cursor is
// currently inside (the last H2 whose line <= cursorLine). -1 if the cursor
// is above all H2s.
func (m Model) activeH2Position() int {
	active := -1
	for i, hi := range m.h2Indices {
		if m.headings[hi].Line <= m.cursorLine {
			active = i
		} else {
			break
		}
	}
	return active
}

// currentHeadingIndex returns the heading the cursor is currently under — the
// last heading whose line <= cursorLine. -1 means the cursor is above the
// first heading.
func (m Model) currentHeadingIndex() int {
	idx := -1
	for i, h := range m.headings {
		if h.Line <= m.cursorLine {
			idx = i
		} else {
			break
		}
	}
	return idx
}

func (m Model) currentAnchor() (Anchor, string) {
	i := m.currentHeadingIndex()
	if i < 0 {
		return Anchor{}, "(document)"
	}
	path := HeadingPath(m.headings, i)
	return Anchor{Path: path}, strings.Join(path, " / ")
}

func (m Model) openCompose() (tea.Model, tea.Cmd) {
	anchor, label := m.currentAnchor()
	existing, _ := m.notes.Get(anchor)
	m.compose.Open(anchor, label, existing)
	m.compose.SetSize(m.width, m.contentHeight())
	m.composing = true
	return m, m.compose.textarea.Focus()
}

func (m Model) handleComposeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+s":
		text := strings.TrimSpace(m.compose.Value())
		m.notes.Set(m.compose.anchor, text)
		if err := m.notes.Save(); err != nil {
			m.statusMsg = fmt.Sprintf("save failed: %v", err)
		} else if text == "" {
			m.statusMsg = "note cleared"
		} else {
			m.statusMsg = "saved → " + filepath.Base(m.notes.Path())
		}
		m.composing = false
		m.compose.Close()
		m.refreshViewport()
		return m, nil
	case "esc":
		m.composing = false
		m.compose.Close()
		return m, nil
	}

	var cmd tea.Cmd
	m.compose, cmd = m.compose.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if !m.ready {
		return ""
	}
	if m.quitting {
		return ""
	}

	var parts []string
	switch {
	case m.composing:
		parts = append(parts, strings.Join(m.compose.View(m.width, m.contentHeight()), "\n"))
	case m.showHelp:
		parts = append(parts, m.renderHelp())
	default:
		if m.hasH2 {
			parts = append(parts, m.renderTabBar())
			parts = append(parts, dividerStyle.Render(strings.Repeat("─", m.width)))
		}
		parts = append(parts, m.viewport.View())
	}
	parts = append(parts, m.helpLine())
	return strings.Join(parts, "\n")
}

// renderTabBar renders the H2 outline as a row of tabs. The active tab is
// derived from cursor position. Bar is truncated from the right if it
// overflows the terminal width.
func (m Model) renderTabBar() string {
	if len(m.h2Indices) == 0 {
		return ""
	}
	active := m.activeH2Position()
	sep := tabSep.Render(" · ")
	var b strings.Builder
	b.WriteString(" ")
	for i, hi := range m.h2Indices {
		if i > 0 {
			b.WriteString(sep)
		}
		name := m.headings[hi].Name
		if i == active {
			b.WriteString(tabActive.Render(name))
		} else {
			b.WriteString(tabStyle.Render(name))
		}
	}
	return truncateAnsi(b.String(), m.width)
}

// truncateAnsi cuts an ANSI-styled string to at most `width` visible columns,
// preserving escape sequences and appending an ellipsis if truncated.
func truncateAnsi(s string, width int) string {
	if width <= 0 {
		return ""
	}
	visible := 0
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			j := i + 1
			for j < len(s) && !isAnsiTerminator(s[j]) {
				j++
			}
			if j < len(s) {
				j++
			}
			b.WriteString(s[i:j])
			i = j
			continue
		}
		if visible >= width-1 {
			b.WriteString("…")
			return b.String()
		}
		b.WriteByte(s[i])
		visible++
		i++
	}
	return b.String()
}

func isAnsiTerminator(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func (m Model) helpLine() string {
	if m.composing {
		return helpStyle.Render("  ctrl+s save  esc cancel")
	}
	if m.searching {
		return "  " + m.searchInput.View()
	}
	if m.showHelp {
		return helpStyle.Render("  ? or esc to close")
	}
	keys := helpStyle.Render("  j/k move  [ ] tab  / search  enter note  ? help  q quit")
	_, label := m.currentAnchor()
	pathPart := "  " + pathStyle.Render("on: ") + label
	right := ""
	switch {
	case len(m.matches) > 0:
		right = "  " + statusStyle.Render(fmt.Sprintf("%d/%d  %s  (n/N esc)", m.matchIdx+1, len(m.matches), m.query))
	case m.statusMsg != "":
		right = "  " + statusStyle.Render(m.statusMsg)
	default:
		if n := m.notes.Count(); n > 0 {
			right = "  " + statusStyle.Render(fmt.Sprintf("%d note%s → %s", n, plural(n), filepath.Base(m.notes.Path())))
		}
	}
	return keys + pathPart + right
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// renderHelp renders the keybinding reference as a bordered panel that fills
// the content area. Pads with blank lines so the bottom help bar stays pinned.
func (m Model) renderHelp() string {
	type entry struct{ k, d string }
	sections := []struct {
		title   string
		entries []entry
	}{
		{"Navigation", []entry{
			{"j / ↓", "line down"},
			{"k / ↑", "line up"},
			{"space, pgdn, ctrl+d", "half-page down"},
			{"pgup, ctrl+u", "half-page up"},
			{"g / home", "top"},
			{"G / end", "bottom"},
			{"[ / ]", "prev / next H2 tab"},
		}},
		{"Search", []entry{
			{"/", "search (case-insensitive)"},
			{"n / N", "next / prev match"},
			{"esc", "clear matches"},
		}},
		{"Notes", []entry{
			{"enter", "open note on current heading"},
			{"ctrl+s", "save note (compose)"},
			{"esc", "cancel compose"},
		}},
		{"Other", []entry{
			{"?", "toggle this help"},
			{"q, ctrl+c", "quit"},
		}},
	}

	var b strings.Builder
	for i, sec := range sections {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(headerStyle.Render(sec.title))
		b.WriteString("\n")
		for _, e := range sec.entries {
			b.WriteString("  ")
			b.WriteString(labelStyle.Render(e.k))
			b.WriteString("  ")
			b.WriteString(e.d)
			b.WriteString("\n")
		}
	}

	panel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("238")).
		Padding(1, 2).
		Render(strings.TrimRight(b.String(), "\n"))

	lines := strings.Split(panel, "\n")
	contentH := m.contentHeight()
	for len(lines) < contentH {
		lines = append(lines, "")
	}
	if len(lines) > contentH {
		lines = lines[:contentH]
	}
	return strings.Join(lines, "\n")
}

// applyMarkers prepends a constant 3-column gutter to every line so cursor
// motion never shifts content horizontally. Column 0 is the line cursor,
// column 1 is the note marker on annotated heading lines, column 2 is the
// separator before content.
func applyMarkers(content string, headings []Heading, cursorLine int, notes *NoteStore) string {
	notedLines := make(map[int]bool, len(headings))
	if notes != nil {
		for i, h := range headings {
			path := HeadingPath(headings, i)
			if _, ok := notes.Get(Anchor{Path: path}); ok {
				notedLines[h.Line] = true
			}
		}
	}
	lines := strings.Split(content, "\n")
	for i := range lines {
		c, n := " ", " "
		if i == cursorLine {
			c = cursorStyle.Render("▸")
		}
		if notedLines[i] {
			n = markerStyle.Render("●")
		}
		lines[i] = c + n + " " + lines[i]
	}
	return strings.Join(lines, "\n")
}
