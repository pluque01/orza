//go:build !race

package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/catalogrepo"
	"github.com/pluque01/orza/internal/credential"
)

type sc011SQLiteFixture struct {
	folders     *app.FolderService
	credentials *credential.Fake
}

func TestSC011SQLiteReloadPerformanceAcceptance(t *testing.T) {
	requireSC011Runner(t)
	fixture := newSC011SQLiteFixture(t)
	model := New(Config{Folders: fixture.folders, Width: 80, Height: 24, NoColor: true})

	warmup, snapshot := runSC011ReloadFrame(t, model)
	t.Logf("SQLite reload warm-up (unmeasured): %s", warmup)
	assertSC011SnapshotShape(t, snapshot)

	qualified := 0
	for run := 0; run < us5PerformanceSamples; run++ {
		duration, reloaded := runSC011ReloadFrame(t, model)
		assertSC011SnapshotShape(t, reloaded)
		t.Logf("SQLite reload run %02d: %s", run+1, duration)
		if duration <= us5ReloadLimit {
			qualified++
		}
	}
	t.Logf("SQLite reload qualified: %d/%d <= %s", qualified, us5PerformanceSamples, us5ReloadLimit)
	if qualified < 19 {
		t.Fatalf("SQLite reload qualified %d/%d <= %s, want at least 19", qualified, us5PerformanceSamples, us5ReloadLimit)
	}
	if calls := fixture.credentials.Calls(); len(calls) != 0 {
		t.Fatalf("credential store calls = %#v, want none", calls)
	}
}

func requireSC011Runner(t *testing.T) {
	t.Helper()
	if runtime.NumCPU() < 2 || runtime.GOMAXPROCS(0) < 2 {
		t.Skip("SC-011 requires at least two available logical CPUs")
	}
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		t.Skipf("SC-011 cannot verify available memory: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != "MemAvailable:" || fields[2] != "kB" {
			continue
		}
		kilobytes, parseErr := strconv.ParseUint(fields[1], 10, 64)
		if parseErr != nil {
			t.Skipf("SC-011 cannot parse available memory: %v", parseErr)
		}
		if kilobytes*1024 < 4<<30 {
			t.Skipf("SC-011 requires at least 4 GiB available memory; found %d bytes", kilobytes*1024)
		}
		return
	}
	t.Skip("SC-011 cannot determine available memory")
}

func newSC011SQLiteFixture(t *testing.T) sc011SQLiteFixture {
	t.Helper()
	directory := filepath.Join(t.TempDir(), "orza")
	if err := os.Mkdir(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := catalog.Open(filepath.Join(directory, catalog.CatalogFileName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	repository := catalogrepo.NewRepository(store)
	ctx := context.Background()
	folders := make([]app.Folder, 100)
	for index := range folders {
		parent := app.ItemSelector{Path: "/"}
		if index > 0 && index%9 != 0 {
			parent = app.ItemSelector{ID: folders[index-1].ID}
		}
		result, createErr := repository.CreateFolder(ctx, app.CreateFolderRequest{
			Parent: parent,
			Name:   fmt.Sprintf("folder-%03d", index),
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		folders[index] = result.Folder
	}
	for index := range 1000 {
		name := fmt.Sprintf("connection-%04d", index)
		_, createErr := repository.CreateConnection(ctx, app.CreateConnectionRequest{
			Parent:     app.ItemSelector{ID: folders[index%len(folders)].ID},
			Name:       name,
			Host:       name + ".example.invalid",
			Port:       22,
			AuthMethod: app.AuthMethodAgent,
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
	}

	catalogID, err := store.CatalogID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	credentials := credential.NewFake()
	saga, err := app.NewCredentialSaga(
		app.NewCatalogCredentialOperationRepository(store), credentials, credential.Scope(catalogID), nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	folderService, err := app.NewFolderService(repository, saga)
	if err != nil {
		t.Fatal(err)
	}
	return sc011SQLiteFixture{folders: folderService, credentials: credentials}
}

func runSC011ReloadFrame(t *testing.T, model *Model) (time.Duration, *catalogSnapshot) {
	t.Helper()
	started := time.Now()
	updated, command := model.Update(keyPress("r"))
	if updated != model || command == nil {
		t.Fatal("reload key did not start an operation")
	}
	rawMessage := command()
	message, ok := rawMessage.(operationResultMsg)
	if !ok {
		t.Fatalf("reload command result type = %T, want operationResultMsg", rawMessage)
	}
	if message.err != nil {
		t.Fatalf("reload command failed: %v", message.err)
	}
	updated, followup := model.Update(message)
	if updated != model || followup != nil {
		t.Fatal("reload result did not complete directly")
	}
	frame := model.View().Content
	duration := time.Since(started)
	if frame == "" || model.operation != nil {
		t.Fatal("reload did not produce a complete stable frame")
	}
	return duration, message.snapshot
}

func assertSC011SnapshotShape(t *testing.T, snapshot *catalogSnapshot) {
	t.Helper()
	if snapshot == nil || len(snapshot.nodes) != 1101 {
		got := 0
		if snapshot != nil {
			got = len(snapshot.nodes)
		}
		t.Fatalf("snapshot nodes = %d, want root + 100 folders + 1000 connections", got)
	}
	folders, connections, maxDepth := 0, 0, 0
	for _, node := range snapshot.nodes {
		if node.root {
			continue
		}
		depth := strings.Count(node.node.Path, "/")
		maxDepth = max(maxDepth, depth)
		switch node.node.Kind {
		case app.NodeKindFolder:
			folders++
		case app.NodeKindConnection:
			connections++
		}
	}
	if folders != 100 || connections != 1000 || maxDepth != 10 {
		t.Fatalf("snapshot shape = %d folders, %d connections, depth %d; want 100, 1000, 10", folders, connections, maxDepth)
	}
}
