package tui

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

const (
	us5PerformanceSamples = 20
	us5LocalLimit         = 100 * time.Millisecond
	us5ReloadLimit        = time.Second
	us5RetentionCycles    = 10_000
	us5RetentionLimit     = 1 << 20
)

var us5RetentionSink int

func TestUS5PerformanceAcceptance1100NodeFixture(t *testing.T) {
	service, _, detailFolder := scaleTreeService(100, 1000)
	snapshot, err := loadCatalogSnapshot(context.Background(), service)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(snapshot.nodes); got != 1101 {
		t.Fatalf("fixture nodes = %d, want root + 100 folders + 1000 connections", got)
	}

	model := New(Config{Width: 80, Height: 24, NoColor: true})
	model.browser.setSnapshot(*snapshot, "")
	model.syncDetail()
	model.ownedSelectionID = model.browser.selectedID

	local := []struct {
		name  string
		setup func(int)
		run   func(int)
	}{
		{name: "selection", setup: func(int) {
			model.browser.selectedID = snapshot.rootID
			model.ownedSelectionID = snapshot.rootID
			model.focusOwner = focusOwnerTree
			model.syncDetail()
		}, run: func(int) {
			updateModel(model, keyPress("j"))
			_ = model.View().Content
		}},
		{name: "focus", setup: func(int) {
			model.focusOwner = focusOwnerTree
		}, run: func(int) {
			updateModel(model, keyPress("tab"))
			_ = model.View().Content
		}},
		{name: "scroll", setup: func(int) {
			model.browser.selectedID = detailFolder
			model.ownedSelectionID = detailFolder
			model.focusOwner = focusOwnerDetail
			model.syncDetail()
			model.detailState = model.detailState.withOffset(0)
			model.width, model.height = 80, 12
		}, run: func(int) {
			updateModel(model, keyPress("j"))
			_ = model.View().Content
		}},
		{name: "resize", setup: func(int) {
			model.width, model.height = 79, 24
		}, run: func(run int) {
			width := 80
			if run%2 != 0 {
				width = 79
			}
			updateModel(model, tea.WindowSizeMsg{Width: width, Height: 24})
			_ = model.View().Content
		}},
	}

	for _, operation := range local {
		operation.setup(0)
		operation.run(0)
		qualified, slowest := 0, time.Duration(0)
		for run := 0; run < us5PerformanceSamples; run++ {
			operation.setup(run)
			started := time.Now()
			operation.run(run)
			duration := time.Since(started)
			t.Logf("%s run %02d: %s", operation.name, run+1, duration)
			if duration > slowest {
				slowest = duration
			}
			if duration <= us5LocalLimit {
				qualified++
			}
		}
		t.Logf("%s: %d/%d <= %s, slowest %s", operation.name, qualified, us5PerformanceSamples, us5LocalLimit, slowest)
		if qualified < 19 {
			t.Errorf("%s: %d/%d samples <= %s, want at least 19", operation.name, qualified, us5PerformanceSamples, us5LocalLimit)
		}
	}

	// This bounded unit acceptance uses the deterministic 1,100-node service.
	// The final SC-011 gate separately includes the temporary SQLite repository.
	reloaded, loadErr := loadCatalogSnapshot(context.Background(), service)
	if loadErr != nil {
		t.Fatalf("reload warm-up failed: %v", loadErr)
	}
	if len(reloaded.nodes) != 1101 {
		t.Fatalf("reload warm-up nodes = %d, want 1101", len(reloaded.nodes))
	}
	qualified, slowest := 0, time.Duration(0)
	for run := 0; run < us5PerformanceSamples; run++ {
		started := time.Now()
		reloaded, loadErr := loadCatalogSnapshot(context.Background(), service)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		model.browser.setSnapshot(*reloaded, model.browser.selectedID)
		model.syncDetail()
		_ = model.View().Content
		duration := time.Since(started)
		t.Logf("reload run %02d: %s", run+1, duration)
		if duration > slowest {
			slowest = duration
		}
		if duration <= us5ReloadLimit {
			qualified++
		}
	}
	t.Logf("reload: %d/%d <= %s, slowest %s", qualified, us5PerformanceSamples, us5ReloadLimit, slowest)
	if qualified < 19 {
		t.Errorf("reload: %d/%d samples <= %s, want at least 19", qualified, us5PerformanceSamples, us5ReloadLimit)
	}
}

func TestUS5RenderResizeHistoryRetention10000Cycles(t *testing.T) {
	if raceEnabled {
		t.Skip("SC-017 retained-memory thresholds require an uninstrumented binary")
	}
	modelType := reflect.TypeFor[Model]()
	for index := 0; index < modelType.NumField(); index++ {
		name := strings.ToLower(modelType.Field(index).Name)
		if strings.Contains(name, "history") || strings.Contains(name, "frames") || strings.Contains(name, "resizes") {
			t.Fatalf("Model retains a history-like field %q", modelType.Field(index).Name)
		}
	}

	model := newUS5UndersizedModel()
	longTarget := "/" + strings.Repeat("retained-history-canary-界/", 80)
	root := testFolder("retention-root", "", "/", 1)
	folder := testFolder("retention-folder", root.ID, "/"+strings.Repeat("tree-canary-界/", 80), 2)
	connections := make([]app.Connection, 64)
	for index := range connections {
		connections[index] = testConnection(fmt.Sprintf("retention-%02d", index), folder.ID, fmt.Sprintf("%s/detail-%02d-%s", folder.Path, index, strings.Repeat("界", 80)), 3)
	}
	snapshot := newCatalogSnapshot(root, 1)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{folder}})
	_ = snapshot.addChildren(folder.ID, app.ListChildrenResult{Connections: connections})
	model.browser.setSnapshot(snapshot, folder.ID)
	model.browser.expanded[folder.ID] = struct{}{}
	model.browser.rebuildRows()
	model.ownedSelectionID = folder.ID
	model.syncDetail()
	longHelp := make([]string, 64)
	for index := range longHelp {
		longHelp[index] = fmt.Sprintf("Help %02d %s", index, strings.Repeat("help-canary-界 ", 40))
	}
	errorState := newErrorModal("reload catalog", longTarget, context.DeadlineExceeded)
	render := func(cycle int) {
		size := us5ContractSizes[cycle%len(us5ContractSizes)]
		updateModel(model, tea.WindowSizeMsg{Width: size.width, Height: size.height})
		model.modal = modalState{}
		model.focusOwner = focusOwnerTree
		switch cycle % 4 {
		case 1:
			model.focusOwner = focusOwnerDetail
			model.detailState = model.detailState.withOffset(cycle % 32)
		case 2:
			model.openGenericModal(modalKindHelp, nil, helpPayload{lines: longHelp})
			model.modal.viewport = newViewportState(cycle % 32)
		case 3:
			model.openGenericModal(modalKindOperationError, nil, operationErrorPayload{modal: errorState})
		}
		view := model.View().Content
		assertUS5FrameBounded(t, view, size.width, size.height)
		us5RetentionSink += len(view)
	}

	for cycle := 0; cycle < 256; cycle++ {
		render(cycle)
	}
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	for cycle := 0; cycle < us5RetentionCycles; cycle++ {
		render(cycle)
	}
	model.modal = modalState{}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(model)

	growth := uint64(0)
	if after.HeapAlloc > before.HeapAlloc {
		growth = after.HeapAlloc - before.HeapAlloc
	}
	t.Logf("retained heap growth after %d cycles: %d bytes", us5RetentionCycles, growth)
	if growth >= us5RetentionLimit {
		t.Fatalf("retained heap grew %d bytes, want < %d bytes", growth, us5RetentionLimit)
	}
}
