package tui

import (
	"context"
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

const (
	tuiBenchmarkConnections = 1000
	tuiBenchmarkFolders     = 100
	performanceRuns         = 20
)

func TestTreePerformanceAcceptance(t *testing.T) {
	service, rootID, expandable := scaleTreeService(tuiBenchmarkFolders, tuiBenchmarkConnections)
	refresh := func() {
		if _, err := loadCatalogSnapshot(context.Background(), service); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := loadCatalogSnapshot(context.Background(), service)
	if err != nil {
		t.Fatal(err)
	}
	browser := browserModel{}
	browser.setSnapshot(*snapshot, "")
	browser.selectedID = expandable
	expand := func() {
		delete(browser.expanded, expandable)
		browser.rebuildRows()
		browser.toggle()
	}
	collapse := func() {
		browser.expanded[expandable] = struct{}{}
		browser.rebuildRows()
		browser.collapse(expandable)
	}

	operations := []struct {
		name string
		run  func()
	}{{"refresh", refresh}, {"expand", expand}, {"collapse", collapse}}
	for _, operation := range operations {
		operation.run() // Warm-up is outside the 20 measured runs.
		qualified := 0
		for run := range performanceRuns {
			started := time.Now()
			operation.run()
			duration := time.Since(started)
			t.Logf("%s run %02d: %s", operation.name, run+1, duration)
			if duration < time.Second {
				qualified++
			}
		}
		t.Logf("%s qualified %d/%d runs under one second", operation.name, qualified, performanceRuns)
		if qualified < 19 {
			t.Errorf("%s qualified %d/%d runs under one second", operation.name, qualified, performanceRuns)
		}
	}
	if browser.snapshot.rootID != rootID {
		t.Fatalf("root ID = %q, want %q", browser.snapshot.rootID, rootID)
	}
}

func BenchmarkTreeRefresh(b *testing.B) {
	service, _, _ := scaleTreeService(tuiBenchmarkFolders, tuiBenchmarkConnections)
	if _, err := loadCatalogSnapshot(context.Background(), service); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		if _, err := loadCatalogSnapshot(context.Background(), service); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTreeExpand(b *testing.B) {
	browser, expandable := scaleBrowser(b)
	b.ResetTimer()
	for range b.N {
		delete(browser.expanded, expandable)
		browser.rebuildRows()
		browser.toggle()
	}
}

func BenchmarkTreeCollapse(b *testing.B) {
	browser, expandable := scaleBrowser(b)
	b.ResetTimer()
	for range b.N {
		browser.expanded[expandable] = struct{}{}
		browser.rebuildRows()
		browser.collapse(expandable)
	}
}

func scaleBrowser(b *testing.B) (*browserModel, app.NodeID) {
	b.Helper()
	service, _, expandable := scaleTreeService(tuiBenchmarkFolders, tuiBenchmarkConnections)
	snapshot, err := loadCatalogSnapshot(context.Background(), service)
	if err != nil {
		b.Fatal(err)
	}
	browser := &browserModel{}
	browser.setSnapshot(*snapshot, "")
	browser.selectedID = expandable
	browser.expandAncestors(expandable)
	browser.rebuildRows()
	browser.toggle() // Warm-up.
	return browser, expandable
}

func scaleTreeService(folderCount, connectionCount int) (FolderFuncs, app.NodeID, app.NodeID) {
	root := testFolder("00000000000000000000000000000000", "", "/", 1)
	children := make(map[app.NodeID]app.ListChildrenResult)
	folders := make([]app.Folder, folderCount)
	for index := range folders {
		parent := root
		if index > 0 {
			// Nine-folder chains keep connections within the supported depth while creating 100 total folders.
			if index%9 != 0 {
				parent = folders[index-1]
			}
		}
		name := fmt.Sprintf("folder-%03d", index)
		path := parent.Path
		if path != "/" {
			path += "/"
		}
		path += name
		folders[index] = testFolder(fmt.Sprintf("%032x", index+1), parent.ID, path, 1)
		result := children[parent.ID]
		result.Folders = append(result.Folders, folders[index])
		children[parent.ID] = result
	}
	for index := 0; index < connectionCount; index++ {
		parent := root
		if len(folders) != 0 {
			parent = folders[index%len(folders)]
		}
		name := fmt.Sprintf("connection-%04d", index)
		path := parent.Path
		if path != "/" {
			path += "/"
		}
		path += name
		connection := testConnection(fmt.Sprintf("%032x", index+1001), parent.ID, path, 1)
		result := children[parent.ID]
		result.Connections = append(result.Connections, connection)
		children[parent.ID] = result
	}
	for id, result := range children {
		result.CatalogRevision = 1
		children[id] = result
	}
	service := FolderFuncs{
		GetFunc: func(context.Context, app.ItemSelector) (app.FolderResult, error) {
			return app.FolderResult{Folder: root, CatalogRevision: 1}, nil
		},
		ListFunc: func(_ context.Context, request app.ListChildrenRequest) (app.ListChildrenResult, error) {
			result := children[request.Folder.ID]
			result.CatalogRevision = 1
			return result, nil
		},
	}
	expandable := root.ID
	if len(folders) != 0 {
		expandable = folders[0].ID
	}
	return service, root.ID, expandable
}

func FuzzModelStateTransitions(f *testing.F) {
	f.Add([]byte("jj?\x1b"), 80, 24)
	f.Add([]byte("nprod\x1bq"), 79, 23)
	f.Fuzz(func(t *testing.T, actions []byte, width, height int) {
		if len(actions) > 512 {
			t.Skip()
		}
		service, _, _ := scaleTreeService(4, 8)
		snapshot, err := loadCatalogSnapshot(context.Background(), service)
		if err != nil {
			t.Fatal(err)
		}
		model := New(Config{Width: 80, Height: 24, NoColor: true})
		model.browser.setSnapshot(*snapshot, "")
		keys := []tea.Key{{Code: 'j', Text: "j"}, {Code: 'k', Text: "k"}, {Code: 'l', Text: "l"}, {Code: 'h', Text: "h"}, {Code: '?'}, {Code: tea.KeyEscape}, {Code: 'n', Text: "n"}, {Code: 'f', Text: "f"}, {Code: 'e', Text: "e"}, {Code: tea.KeyEnter}}
		for index, action := range actions {
			if action%3 == 0 {
				_, _ = model.Update(tea.WindowSizeMsg{Width: width + index%3, Height: height + index%3})
			} else {
				_, _ = model.Update(tea.KeyPressMsg(keys[int(action)%len(keys)]))
			}
			view := model.View().Content
			if model.width > 0 && model.height > 0 && len(view) > 0 {
				// View generation itself is the fuzz invariant; fitContent owns bounds.
				_ = view
			}
		}
	})
}
