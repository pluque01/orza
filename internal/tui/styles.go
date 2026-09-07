package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type styles struct {
	title, activeTitle, inactiveTitle   lipgloss.Style
	selected, focused, invalid, primary lipgloss.Style
	muted, status, warning, failure     lipgloss.Style
	scrollbarTrack, scrollbarThumb      lipgloss.Style
}

type itemSemantics struct {
	selected bool
	focused  bool
	invalid  bool
	primary  bool
}

func newStyles(noColor bool) styles {
	if noColor {
		return styles{}
	}

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	selected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	status := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	warning := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	failure := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	return styles{
		title:          title,
		activeTitle:    title,
		inactiveTitle:  muted,
		selected:       selected,
		focused:        selected,
		invalid:        failure,
		primary:        status,
		muted:          muted,
		status:         status,
		warning:        warning,
		failure:        failure,
		scrollbarTrack: muted,
		scrollbarThumb: selected,
	}
}

func (s styles) scrollbarCell(thumb bool) string {
	if thumb {
		return s.scrollbarThumb.Render(oneCellGlyph("█", "#"))
	}
	return s.scrollbarTrack.Render(oneCellGlyph("│", "|"))
}

func oneCellGlyph(preferred, fallback string) string {
	if ansi.StringWidth(preferred) == 1 {
		return preferred
	}
	if ansi.StringWidth(fallback) == 1 {
		return fallback
	}
	return "|"
}

func (s styles) regionTitle(title string, active bool) string {
	if active {
		return s.activeTitle.Render("[*] " + title)
	}
	return s.inactiveTitle.Render("[ ] " + title)
}

func (s styles) item(text string, semantics itemSemantics) string {
	focusMarker := " "
	if semantics.selected || semantics.focused {
		focusMarker = s.focused.Render(">")
	}
	validityMarker := " "
	if semantics.invalid {
		validityMarker = s.invalid.Render("!")
	}
	priorityMarker := " "
	if semantics.primary {
		priorityMarker = s.primary.Render("*")
	}

	style := lipgloss.NewStyle()
	switch {
	case semantics.invalid:
		style = s.invalid
	case semantics.selected:
		style = s.selected
	case semantics.focused:
		style = s.focused
	case semantics.primary:
		style = s.primary
	}
	return focusMarker + validityMarker + priorityMarker + " " + style.Render(text)
}

func (s styles) warningMessage(message string) string {
	return s.warning.Render("Warning: " + message)
}

func (s styles) failureMessage(message string) string {
	return s.failure.Render("Error: " + message)
}
