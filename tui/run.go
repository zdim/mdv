package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

// Run reads the markdown file at path and launches the TUI.
func Run(path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	// Detect the terminal size before Bubble Tea takes over so we can prerender
	// the markdown at the correct width. Otherwise the first WindowSizeMsg forces
	// a fresh glamour render at the real width, which the user sees as a black
	// flash on startup.
	w, h := initialSize()
	PrewarmRenderer(w)

	m := NewModel(path, string(body))
	m.SetInitialSize(w, h)

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func initialSize() (int, int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 || h <= 0 {
		return defaultRenderWidth, 24
	}
	return w, h
}
