package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const composeTextareaHeight = 6

// ComposeModel is a textarea with a context header for entering a note on the
// current heading.
type ComposeModel struct {
	textarea textarea.Model
	anchor   Anchor
	label    string
}

func NewComposeModel() ComposeModel {
	ta := textarea.New()
	ta.Placeholder = "Type your note..."
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Base = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	return ComposeModel{textarea: ta}
}

// Open configures the compose model for a new note.
func (c *ComposeModel) Open(anchor Anchor, label, existing string) {
	c.anchor = anchor
	c.label = label
	c.textarea.SetValue(existing)
	c.textarea.Focus()
}

func (c *ComposeModel) Close() { c.textarea.Blur() }

func (c *ComposeModel) SetSize(width, _ int) {
	c.textarea.SetWidth(width - 2)
	c.textarea.SetHeight(composeTextareaHeight)
}

func (c ComposeModel) Update(msg tea.Msg) (ComposeModel, tea.Cmd) {
	var cmd tea.Cmd
	c.textarea, cmd = c.textarea.Update(msg)
	return c, cmd
}

func (c *ComposeModel) Value() string { return c.textarea.Value() }

// View renders the compose overlay pinned to the bottom of the given area.
func (c *ComposeModel) View(width, height int) []string {
	var lines []string

	label := c.label
	if label == "" {
		label = "(document)"
	}
	header := "  " + headerStyle.Render("Note on: ") + labelStyle.Render(label)
	lines = append(lines, header)
	lines = append(lines, "  "+dividerStyle.Render(strings.Repeat("─", maxInt(0, width-4))))

	taLines := strings.Split(c.textarea.View(), "\n")
	used := len(lines) + len(taLines) + 1
	for i := used; i < height; i++ {
		lines = append(lines, "")
	}

	lines = append(lines, "  "+dividerStyle.Render(strings.Repeat("─", maxInt(0, width-4))))
	for _, line := range taLines {
		lines = append(lines, "  "+line)
	}

	return ensureLines(lines, width, height)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ensureLines truncates or pads `lines` to exactly `height` lines.
func ensureLines(lines []string, _, height int) []string {
	if len(lines) > height {
		return lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}
