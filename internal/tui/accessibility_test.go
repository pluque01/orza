package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

func TestUS5ExactNoColorCues(t *testing.T) {
	style := newStyles(true)
	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "active region", got: style.regionTitle("Tree", true), want: "[*] Tree"},
		{name: "inactive region", got: style.regionTitle("Details", false), want: "[ ] Details"},
		{name: "focused", got: style.item("Name", itemSemantics{focused: true}), want: ">   Name"},
		{name: "invalid", got: style.item("Host", itemSemantics{invalid: true}), want: " !  Host"},
		{name: "primary", got: style.item("Save", itemSemantics{primary: true}), want: "  * Save"},
		{name: "combined marker order", got: style.item("Save", itemSemantics{focused: true, invalid: true, primary: true}), want: ">!* Save"},
		{name: "warning", got: style.warningMessage("target changed"), want: "Warning: target changed"},
		{name: "error", got: style.failureMessage("save failed"), want: "Error: save failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("cue = %q, want exact %q", tt.got, tt.want)
			}
			if strings.Contains(tt.got, "\x1b[") {
				t.Fatalf("no-color cue contains ANSI: %q", tt.got)
			}
		})
	}
}

func TestUS5ClosedKeyboardBindingsAndF2Fallback(t *testing.T) {
	keys := newKeyMap()
	tests := []struct {
		name string
		got  []string
		want []string
	}{
		{name: "Up", got: keys.Up.Keys(), want: []string{"up", "k"}},
		{name: "Down", got: keys.Down.Keys(), want: []string{"down", "j"}},
		{name: "Left", got: keys.Left.Keys(), want: []string{"left", "h"}},
		{name: "Right", got: keys.Right.Keys(), want: []string{"right", "l"}},
		{name: "Home", got: keys.Home.Keys(), want: []string{"home", "g"}},
		{name: "End", got: keys.End.Keys(), want: []string{"end", "G"}},
		{name: "Toggle", got: keys.Toggle.Keys(), want: []string{"enter", "space"}},
		{name: "Open", got: keys.Open.Keys(), want: []string{"enter"}},
		{name: "Back", got: keys.Back.Keys(), want: []string{"esc"}},
		{name: "Help", got: keys.Help.Keys(), want: []string{"?"}},
		{name: "form Help", got: keys.FormHelp.Keys(), want: []string{"f1"}},
		{name: "Quit", got: keys.Quit.Keys(), want: []string{"q", "ctrl+c"}},
		{name: "Next", got: keys.Next.Keys(), want: []string{"tab"}},
		{name: "Previous", got: keys.Previous.Keys(), want: []string{"shift+tab", "f2"}},
		{name: "New connection", got: keys.New.Keys(), want: []string{"n"}},
		{name: "New folder", got: keys.NewFolder.Keys(), want: []string{"f"}},
		{name: "Edit", got: keys.Edit.Keys(), want: []string{"e"}},
		{name: "Move", got: keys.Move.Keys(), want: []string{"m"}},
		{name: "Delete", got: keys.Delete.Keys(), want: []string{"d"}},
		{name: "Connect", got: keys.Connect.Keys(), want: []string{"c"}},
		{name: "Reload", got: keys.Reload.Keys(), want: []string{"r"}},
		{name: "Save", got: keys.Save.Keys(), want: []string{"ctrl+s"}},
		{name: "Confirm", got: keys.Confirm.Keys(), want: []string{"y"}},
		{name: "Detail", got: keys.Detail.Keys(), want: []string{"d"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !slices.Equal(tt.got, tt.want) {
				t.Fatalf("binding = %#v, want exact %#v", tt.got, tt.want)
			}
		})
	}

	form := validConnectionForm()
	form.setFocus(fieldHost)
	form.update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}))
	if form.focusedField() != fieldName {
		t.Fatalf("unmodified F2 focus = %v, want previous editable field Name", form.focusedField())
	}
	form.update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF2, Mod: tea.ModShift}))
	if form.focusedField() != fieldName {
		t.Fatalf("modified F2 was accepted as fallback, focus = %v", form.focusedField())
	}
	lines, _ := form.content(newStyles(true))
	if !strings.Contains(strings.Join(lines, "\n"), "Shift+Tab/F2 Previous") {
		t.Fatalf("form does not expose F2 fallback: %q", strings.Join(lines, "\n"))
	}
}

func TestUS5ReducedConnectionFormActionsKeepSafetyPrimaryAndF2Fallback(t *testing.T) {
	model := New(Config{Width: 40, Height: 12, NoColor: true})
	model.screen = screenConnectionForm
	model.focusOwner = focusOwnerConnectionForm
	model.form = validConnectionForm()

	view := model.View().Content
	for _, control := range []string{
		"Esc Cancel", "Ctrl+C Quit", "F1 Help", "Ctrl+S Save", "Tab Next", "Shift+Tab/F2 Previous",
	} {
		if !strings.Contains(view, control) {
			t.Fatalf("40x12 connection form omitted priority control %q:\n%s", control, view)
		}
	}
	if strings.Contains(view, "\x1b[") {
		t.Fatalf("no-color connection form contains ANSI: %q", view)
	}
}

func TestUS5PrintableCommandsRemainLocalToEditableField(t *testing.T) {
	for _, printable := range []string{"p", "q", "?"} {
		t.Run(printable, func(t *testing.T) {
			form := validConnectionForm()
			form.setFocus(fieldName)
			before := form.inputs[fieldName].Value()
			form.update(tea.KeyPressMsg(tea.Key{Code: []rune(printable)[0], Text: printable}))
			if got := form.inputs[fieldName].Value(); got != before+printable {
				t.Fatalf("printable %q produced %q, want local text %q", printable, got, before+printable)
			}
		})
	}
}

func TestUS5NoColorSafeTextAcrossDisplayModes(t *testing.T) {
	unsafeName := "\x1b\n\u202Eprod界hidden"
	unsafePath := "/team/" + unsafeName
	root := testFolder("root", "", "/", 1)
	connection := testConnection("connection", root.ID, unsafePath, 2)
	connection.Name = unsafeName
	snapshot := newCatalogSnapshot(root, 1)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Connections: []app.Connection{connection}})
	model := New(Config{Width: 80, Height: 24, NoColor: true})
	model.browser.setSnapshot(snapshot, connection.ID)
	model.ownedSelectionID = connection.ID
	model.syncDetail()

	sizes := append(append([]us5Size(nil), us5ContractSizes...),
		us5Size{name: "39x12", width: 39, height: 12},
		us5Size{name: "40x11", width: 40, height: 11},
	)
	for _, size := range sizes {
		t.Run(size.name, func(t *testing.T) {
			updateModel(model, tea.WindowSizeMsg{Width: size.width, Height: size.height})
			view := model.View().Content
			if strings.Contains(view, "\x1b[") {
				t.Fatalf("no-color %s frame contains ANSI/control sequence: %q", size.name, view)
			}
			if size.width >= minimumLayoutWidth && size.height >= minimumLayoutHeight {
				for _, cue := range []string{"[*] Tree", "[ ] Details", "[ ] Actions", "> ", "[ssh]"} {
					if !strings.Contains(view, cue) {
						t.Fatalf("%s frame omitted textual cue %q:\n%s", size.name, cue, view)
					}
				}
				for _, escaped := range []string{`\x1B`, `\n`, `\u202E`} {
					if !strings.Contains(view, escaped) {
						t.Fatalf("%s frame omitted inert control projection %q:\n%s", size.name, escaped, view)
					}
				}
			} else if strings.Contains(view, unsafeName) || strings.Contains(view, "[ssh]") {
				t.Fatalf("undersized %s frame exposed catalog content: %q", size.name, view)
			}
			assertUS5FrameBounded(t, view, size.width, size.height)
		})
	}
}
