// Package ui is the Bubble Tea Model for the cinematic replay TUI.
//
// Layout: left ~20% file tree, right ~80% highlighted editor with line
// numbers and a blinking caret, bottom status + keybind bar.
package ui

import "github.com/charmbracelet/lipgloss"

var (
	statusStyle = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	numStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Align(lipgloss.Right)
	caretStyle  = lipgloss.NewStyle().Reverse(true)
	barFill     = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	barEmpty    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	sepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	headerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true)

	// Playhead row: terminals cannot zoom one line, so cursorline + bold +
	// a magenta gutter bar is the cinematic stand-in.
	playLineBg   = lipgloss.Color("237")
	playAccentFg = lipgloss.Color("212")
	playRow      = lipgloss.NewStyle().Background(playLineBg)
	playNum      = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(playLineBg).Bold(true).Align(lipgloss.Right)
	playBar      = lipgloss.NewStyle().Foreground(playAccentFg).Background(playLineBg).Bold(true)

	treeActive = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Background(lipgloss.Color("4")).Bold(true)
	treePlayed = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	kindAdd    = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	kindMod    = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	kindDel    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

	// Review overlay (after a file has typed itself): GitHub-like gutter
	// plus a faint row wash. Colour is backup; the glyph is the signal.
	addRowBg   = lipgloss.Color("22")
	delRowBg   = lipgloss.Color("52")
	modRowBg   = lipgloss.Color("58")
	addRow     = lipgloss.NewStyle().Background(addRowBg)
	delRow     = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Faint(true).Background(delRowBg).Strikethrough(true)
	modRow     = lipgloss.NewStyle().Background(modRowBg)
	gutAdd     = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	gutDel     = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	gutMod     = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	kindRen    = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	dirStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	agentTag   = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
	creditRole = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	creditName = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
)
