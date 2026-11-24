package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

func TestCatalogViewsKeyboardFocusAndTextInput(t *testing.T) {
	model := New(Config{Width: 40, Height: 12, NoColor: true})
	press := func(key string) { updateModel(model, keyPress(key)) }
	assertTitle := func(want string) {
		t.Helper()
		line, _, _ := strings.Cut(model.View().Content, "\n")
		if !strings.Contains(line, want) || ansi.Strip(line) != line {
			t.Fatalf("wanted %q in plain title, got %q", want, line)
		}
	}

	assertTitle("[*] [Tree] - Favourites - Recent")
	press("[")
	assertTitle("[*] Tree - Favourites - [Recent]")
	if !strings.Contains(model.View().Content, "No recent sessions yet.") {
		t.Fatalf("missing Recent empty state:\n%s", model.View().Content)
	}
	updateModel(model, tea.KeyPressMsg(tea.Key{Code: tea.KeyF3}))
	assertTitle("[*] [Tree] - Favourites - Recent")
	press("]")
	assertTitle("[*] Tree - [Favourites] - Recent")
	if !strings.Contains(model.View().Content, "No favourites yet.") {
		t.Fatalf("missing Favourites empty state:\n%s", model.View().Content)
	}
	for _, pair := range [][2]string{{"[", "Previous view"}, {"]/F3", "Next view"}} {
		if !actionHelpContains(model.currentHelpLines(), pair[0], pair[1]) {
			t.Fatalf("catalog help omitted %q", pair)
		}
	}
	press("tab")
	assertTitle("[ ] Tree - [Favourites] - Recent")
	press("]")
	assertTitle("[ ] Tree - [Favourites] - Recent")
	updateModel(model, tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}))
	assertTitle("[*] Tree - [Favourites] - Recent")
	press("shift+tab")
	if model.focusOwner != focusOwnerTunnels {
		t.Fatalf("reverse focus = %v, want Tunnels", model.focusOwner)
	}
	press("tab")
	press("[")
	press("/")
	press("[")
	press("]")
	if model.search == nil || model.search.input.Value() != "[]" || model.browser.view != catalogTree {
		t.Fatalf("brackets did not stay in search: %+v", model.search)
	}
	press("esc")
	press("n")
	press("[")
	press("]")
	if model.form == nil || model.form.inputs[fieldName].Value() != "[]" || model.browser.view != catalogTree {
		t.Fatalf("brackets did not stay in the connection name field")
	}
}

func TestCatalogActiveViewColourAndCompactTitle(t *testing.T) {
	style := newStyles(false)
	for view, name := range catalogViewNames {
		for _, width := range []int{38, 29, 18} {
			heading := style.catalogTitle(catalogView(view), false, width)
			if !strings.Contains(heading, style.activeTitle.Render("["+name+"]")) || ansi.StringWidth(heading) > width {
				t.Errorf("width %d does not fit the highlighted active name in %q", width, heading)
			}
		}
	}
}

func TestCatalogFavouriteActionsKeepASelectedRow(t *testing.T) {
	var toggled []app.NodeID
	connections := ConnectionFuncs{ToggleFavouriteFunc: func(_ context.Context, id app.NodeID) (bool, error) {
		toggled = append(toggled, id)
		return id == "b", nil
	}}
	model := New(Config{Connections: connections, NoColor: true})
	a, b := app.Connection{Node: app.Node{ID: "a", Name: "a", Path: "/a"}, Favourite: true}, app.Connection{Node: app.Node{ID: "b", Name: "b", Path: "/b"}, LastOpenedAt: time.Now()}
	model.browser.setConnectionsPending(app.ListConnectionsResult{Connections: []app.Connection{b, a}, CatalogRevision: 1}, "")
	f4 := tea.KeyPressMsg(tea.Key{Code: tea.KeyF4})
	if _, cmd := model.Update(f4); cmd != nil {
		t.Fatal("root accepted F4")
	}
	toggle := func() {
		t.Helper()
		_, cmd := model.Update(f4)
		if cmd == nil {
			t.Fatal("connection did not accept F4")
		}
		updateModel(model, cmd())
	}
	updateModel(model, keyPress("]"))
	if model.browser.selectedID != a.ID || !strings.Contains(model.View().Content, "F4 Remove from favourites") {
		t.Fatalf("existing favourite is not selected:\n%s", model.View().Content)
	}
	updateModel(model, keyPress("]"))
	if model.browser.selectedID != b.ID || !strings.Contains(model.View().Content, "F4 Add to favourites") {
		t.Fatalf("expected a recent row with Add action:\n%s", model.View().Content)
	}
	toggle()
	updateModel(model, keyPress("["))
	updateModel(model, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	if model.browser.selectedID != b.ID {
		t.Fatalf("Down selected %q, want %q", model.browser.selectedID, b.ID)
	}
	updateModel(model, tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	if model.browser.selectedID != a.ID {
		t.Fatalf("Up selected %q, want %q", model.browser.selectedID, a.ID)
	}
	toggle()
	if model.browser.selectedID != b.ID || strings.Contains(model.View().Content, "/a") {
		t.Fatalf("removing favourite did not advance selection:\n%s", model.View().Content)
	}
	updateModel(model, keyPress("["))
	if !strings.Contains(model.View().Content, "[ssh] * b") || strings.Contains(model.View().Content, "[ssh] * a") || len(toggled) != 2 {
		t.Fatalf("tree favourite markers disagree with toggles %q:\n%s", toggled, model.View().Content)
	}
}

func TestCatalogConnectReconfirmsChangedDestinationFromEveryView(t *testing.T) {
	for view, name := range catalogViewNames {
		t.Run(name, func(t *testing.T) {
			connection := app.Connection{
				Node: app.Node{ID: "saved", Name: "saved", Path: "/saved", Revision: 1},
				Host: "before.invalid", Port: 22, Favourite: true, LastOpenedAt: time.Now(),
			}
			model := New(Config{Connections: ConnectionFuncs{GetFunc: testConnectionLookup(connection)}, Width: 80, Height: 24, NoColor: true})
			stale := connection
			stale.Path, stale.Host = "/stale", "stale.invalid"
			model.browser.setConnectionsPending(app.ListConnectionsResult{Connections: []app.Connection{stale}, CatalogRevision: 1}, "")
			for range view {
				updateModel(model, keyPress("]"))
			}
			if view == int(catalogTree) {
				updateModel(model, keyPress("j"))
			}
			openConnectForTest(t, model)
			initial := model.View().Content
			if !strings.Contains(initial, "/saved") || !strings.Contains(initial, "before.invalid:22") {
				t.Fatalf("initial confirmation omitted identity:\n%s", initial)
			}
			updateModel(model, keyPress("y"))
			runtime := model.sessionRuntime
			if runtime == nil || runtime.request.ExpectedPath != "/saved" || *runtime.request.Expected != 1 {
				t.Fatal("confirmed request did not capture original path and revision")
			}
			changed := connection
			changed.Path, changed.Host, changed.Port, changed.Revision = "/moved/saved", "after.invalid", 2222, 2
			updateModel(model, sessionFinishedMsg{id: runtime.id, attempt: runtime.attempt, result: app.ConnectResult{Connection: changed}, err: app.ErrConflict})
			if model.operation != nil || model.sessionRuntime != nil {
				t.Fatal("changed target started without a fresh confirmation")
			}
			updated := model.View().Content
			for _, want := range []string{"Target changed. Confirm new target?", "/moved/saved", "after.invalid:2222", "y Confirm"} {
				if !strings.Contains(updated, want) {
					t.Fatalf("updated confirmation omitted %q:\n%s", want, updated)
				}
			}
			updateModel(model, keyPress("y"))
			defer model.cancelCurrentOperation(false)
			request := model.sessionRuntime.request
			if request.Connection.ID != connection.ID || request.ExpectedPath != changed.Path || *request.Expected != changed.Revision {
				t.Fatalf("reconfirmation pinned %+v, want %q revision %d", request, changed.Path, changed.Revision)
			}
		})
	}
}
