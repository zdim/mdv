package tui

import "github.com/charmbracelet/lipgloss"

var (
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	headerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	dividerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	markerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	cursorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	statusStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	pathStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	tabStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	tabActive    = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true).Underline(true)
	tabSep       = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
)
