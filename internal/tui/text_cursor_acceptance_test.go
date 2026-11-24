package tui

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestConnectionFormCursorMovesWithoutShiftingText(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {40, 12}} {
		for _, noColor := range []bool{false, true} {
			t.Run(fmt.Sprintf("%dx%d/noColor=%t", size[0], size[1], noColor), func(t *testing.T) {
				const host = "ab.invalid"
				connection := testConnection("host", syntheticRootID, "/host", 1)
				connection.Host = host
				form := newConnectionForm(&connection)
				form.setFocus(fieldHost)
				model := New(Config{Width: size[0], Height: size[1], NoColor: noColor})
				model.openConnectionForm(form, model.captureConnectionTarget(connection))

				model.Update(modifiedKey(tea.KeyHome, 0))
				atStart := model.View()
				model.Update(modifiedKey(tea.KeyRight, 0))
				atSecond := model.View()
				model.Update(modifiedKey(tea.KeyEnd, 0))
				atEnd := model.View()

				plain := ansi.Strip(atStart.Content)
				if ansi.Strip(atSecond.Content) != plain || ansi.Strip(atEnd.Content) != plain {
					t.Fatal("moving the Host cursor changed the visible text")
				}
				if noColor && (atStart.Content != plain || atSecond.Content != plain || atEnd.Content != plain) {
					t.Fatal("Host cursor added styling to a NO_COLOR frame")
				}
				for y, line := range strings.Split(plain, "\n") {
					before, _, ok := strings.Cut(line, host)
					if !ok {
						continue
					}
					start := ansi.StringWidth(before)
					end := start + len(host)
					if next := ansi.Cut(line, end, end+1); next != " " {
						t.Fatalf("the Host cursor needs a blank cell at the end, got %q", next)
					}
					startCell := cursorBackgroundAt(atStart, start, y)
					secondCell := cursorBackgroundAt(atSecond, start+1, y)
					endCell := cursorBackgroundAt(atEnd, end, y)
					if startCell == cursorBackgroundAt(atSecond, start, y) ||
						secondCell == cursorBackgroundAt(atStart, start+1, y) ||
						endCell == cursorBackgroundAt(atSecond, end, y) {
						t.Fatal("the block cursor did not follow the insertion point across the Host field")
					}
					return
				}
				t.Fatalf("Host text %q does not appear at a fixed position in the frame", host)
			})
		}
	}
}

type cursorBackground struct {
	color    color.Color
	reversed bool
}

func cursorBackgroundAt(view tea.View, x, y int) cursorBackground {
	styled := uv.NewStyledString(view.Content)
	bounds := styled.Bounds()
	buffer := uv.NewScreenBuffer(bounds.Dx(), bounds.Dy())
	styled.Draw(&buffer, bounds)
	style := buffer.CellAt(x, y).Style
	reversed := style.Attrs&uv.AttrReverse != 0
	if cursor := view.Cursor; cursor != nil && cursor.Shape == tea.CursorBlock && cursor.X == x && cursor.Y == y {
		if cursor.Color != nil {
			return cursorBackground{color: cursor.Color}
		}
		reversed = !reversed
	}
	if reversed {
		return cursorBackground{color: style.Fg, reversed: true}
	}
	return cursorBackground{color: style.Bg}
}
