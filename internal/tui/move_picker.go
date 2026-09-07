package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pluque01/orza/internal/app"
)

type moveTarget struct {
	folder  app.Folder
	enabled bool
}

type movePicker struct {
	source   app.Node
	targets  []moveTarget
	selected int
}

type movePickerPayload struct{ picker *movePicker }

func (payload movePickerPayload) validModalPayload() bool {
	return payload.picker != nil && payload.picker.source.ID != ""
}

func newMovePicker(source app.Node, folders []app.Folder) *movePicker {
	sort.Slice(folders, func(i, j int) bool { return folders[i].Path < folders[j].Path })
	picker := &movePicker{source: source, targets: make([]moveTarget, 0, len(folders))}
	for _, folder := range folders {
		enabled := true
		if source.Kind == app.NodeKindFolder && (folder.ID == source.ID || strings.HasPrefix(folder.Path, source.Path+"/")) {
			enabled = false
		}
		picker.targets = append(picker.targets, moveTarget{folder: folder, enabled: enabled})
	}
	picker.selectEnabled(1)
	return picker
}

func newTreeMovePicker(source app.Node, snapshot catalogSnapshot) *movePicker {
	picker := newMovePicker(source, (&browserModel{snapshot: snapshot}).folders())
	if source.Kind != app.NodeKindFolder {
		return picker
	}
	for index := range picker.targets {
		id := picker.targets[index].folder.ID
		for current := id; current != ""; current = snapshot.parents[current] {
			if current == source.ID {
				picker.targets[index].enabled = false
				break
			}
		}
	}
	picker.selected = 0
	picker.selectEnabled(1)
	return picker
}

func (p *movePicker) enabled(id app.NodeID) bool {
	for _, target := range p.targets {
		if target.folder.ID == id {
			return target.enabled
		}
	}
	return false
}

func (p *movePicker) selectEnabled(delta int) {
	if len(p.targets) == 0 {
		return
	}
	for count := 0; count < len(p.targets); count++ {
		if p.targets[p.selected].enabled {
			return
		}
		p.selected = (p.selected + delta + len(p.targets)) % len(p.targets)
	}
}

func (p *movePicker) move(delta int) {
	if len(p.targets) == 0 {
		return
	}
	for count := 0; count < len(p.targets); count++ {
		p.selected = (p.selected + delta + len(p.targets)) % len(p.targets)
		if p.targets[p.selected].enabled {
			return
		}
	}
}

func (p *movePicker) destination() *app.Folder {
	if p.selected < 0 || p.selected >= len(p.targets) || !p.targets[p.selected].enabled {
		return nil
	}
	return &p.targets[p.selected].folder
}

func (p *movePicker) modalLines(width int) ([]string, int) {
	lines := []string{"Move destination"}
	lines = append(lines, projectedModalLine("Source: ", p.source.Path, width))
	revision := "/" + fmt.Sprint(p.source.Revision)
	lines = append(lines, projectedModalLineWithSuffix("ID/revision: ", string(p.source.ID), revision, width))
	active := len(lines) + p.selected
	for index, target := range p.targets {
		marker := "  "
		if index == p.selected {
			marker = "> "
		}
		suffix := ""
		if !target.enabled {
			suffix = " [unavailable: source subtree]"
		}
		line := marker + safeText(target.folder.Path, max(0, width-len(marker)-len(suffix))) + suffix
		lines = append(lines, line)
	}
	lines = append(lines, "Enter Move  Esc Cancel  ? Help")
	return lines, active
}
