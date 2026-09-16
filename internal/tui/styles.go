package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const structuredFieldMinimumValueWidth = 8

type badgePlacement uint8

const (
	badgePlacementTitle badgePlacement = iota + 1
	badgePlacementContent
)

type typeBadge struct {
	label     string
	placement badgePlacement
	active    bool
	accented  bool
}

type displayField struct {
	label string
	value string
}

type fieldLayoutMode uint8

const (
	fieldModeAligned fieldLayoutMode = iota + 1
	fieldModeStacked
	fieldModeCompactInteractive
)

type structuredFieldGroup struct {
	fields            []displayField
	labelWidth        int
	prefixWidth       int
	contentWidth      int
	mode              fieldLayoutMode
	minimumValueWidth int
}

type styles struct {
	title, activeTitle, inactiveTitle   lipgloss.Style
	selected, focused, invalid, primary lipgloss.Style
	muted, status, warning, failure     lipgloss.Style
	scrollbarTrack, scrollbarThumb      lipgloss.Style
	badge                               lipgloss.Style
	accented                            bool
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
	badge := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("12"))
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
		badge:          badge,
		accented:       true,
	}
}

func (s styles) typeBadge(label string, placement badgePlacement, active bool) (typeBadge, bool) {
	if label == "" || strings.ContainsAny(label, "[]") || safeText(label, int(^uint(0)>>1)) != label {
		return typeBadge{}, false
	}
	if placement != badgePlacementTitle && placement != badgePlacementContent {
		return typeBadge{}, false
	}
	return typeBadge{
		label:     label,
		placement: placement,
		active:    placement == badgePlacementTitle && active,
		accented:  s.accented,
	}, true
}

func (s styles) renderTypeBadge(badge typeBadge) string {
	text := "[" + badge.label + "]"
	if badge.accented {
		return s.badge.Render(text)
	}
	return text
}

func (s styles) contentBadge(label string) string {
	badge, valid := s.typeBadge(label, badgePlacementContent, false)
	if !valid {
		return ""
	}
	return s.renderTypeBadge(badge)
}

func (s styles) descriptiveLabel(label string) string {
	return s.muted.Render(strings.TrimSuffix(label, ":"))
}

func newStructuredFieldGroup(fields []displayField, prefixWidth, contentWidth int, interactive bool) structuredFieldGroup {
	group := structuredFieldGroup{
		fields:       make([]displayField, len(fields)),
		prefixWidth:  max(0, prefixWidth),
		contentWidth: max(0, contentWidth),
	}
	copy(group.fields, fields)
	for index := range group.fields {
		group.fields[index].label = strings.TrimSuffix(group.fields[index].label, ":")
		group.labelWidth = max(group.labelWidth, ansi.StringWidth(group.fields[index].label))
	}
	if interactive {
		group.mode = fieldModeCompactInteractive
		return group
	}
	group.minimumValueWidth = structuredFieldMinimumValueWidth
	if group.availableValueWidth() >= group.minimumValueWidth {
		group.mode = fieldModeAligned
	} else {
		group.mode = fieldModeStacked
	}
	return group
}

func (group structuredFieldGroup) availableValueWidth() int {
	return max(0, group.contentWidth-group.prefixWidth-group.labelWidth-1)
}

func (group structuredFieldGroup) render(style styles) []string {
	prefix := strings.Repeat(" ", group.prefixWidth)
	lines := make([]string, 0, len(group.fields))
	if group.mode == fieldModeStacked {
		lines = make([]string, 0, len(group.fields)*2)
		for _, field := range group.fields {
			labelWidth := max(0, group.contentWidth-group.prefixWidth)
			valueWidth := max(0, group.contentWidth-group.prefixWidth-2)
			lines = append(lines,
				prefix+style.descriptiveLabel(viewportEllipsis(field.label, labelWidth)),
				prefix+"  "+viewportEllipsis(field.value, valueWidth),
			)
		}
		return lines
	}

	valueWidth := group.availableValueWidth()
	for _, field := range group.fields {
		padding := strings.Repeat(" ", max(0, group.labelWidth-ansi.StringWidth(field.label)))
		line := prefix + style.descriptiveLabel(field.label) + padding + " " + viewportEllipsis(field.value, valueWidth)
		lines = append(lines, viewportEllipsis(line, group.contentWidth))
	}
	return lines
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
	badge, valid := s.typeBadge(title, badgePlacementTitle, active)
	if !valid {
		return ""
	}
	if active {
		return s.activeTitle.Render("[*]") + " " + s.renderTypeBadge(badge)
	}
	return s.inactiveTitle.Render("[ ]") + " " + s.renderTypeBadge(badge)
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
