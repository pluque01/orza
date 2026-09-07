package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/app"
)

func TestDetailConnectionProjection(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	connection := testConnection("connection", root.ID, "/team/production", 2)
	connection.Host = "prod.example"
	connection.Port = 2222
	connection.AuthMethod = app.AuthMethodKey
	connection.IdentityFile = "/home/operator/.ssh/id_production"
	connection.CredentialRef = "SECRET-CREDENTIAL-CANARY"
	snapshot := newCatalogSnapshot(root, 3)
	if !snapshot.addChildren(root.ID, app.ListChildrenResult{Connections: []app.Connection{connection}}) {
		t.Fatal("fixture snapshot rejected")
	}

	state, ok := newDetailState(snapshot, connection.ID)
	if !ok {
		t.Fatal("connection detail target rejected")
	}
	if state.targetID != connection.ID || state.kind != detailKindConnection || state.heading != "Connection" {
		t.Fatalf("identity = %q/%q/%q", state.targetID, state.kind, state.heading)
	}
	want := []string{
		"Kind: Connection",
		"Name: production",
		"Path: /team/production",
		"Endpoint: prod.example:2222",
		"User: (default)",
		"Method: key",
		"Identity: /home/operator/.ssh/id_production",
	}
	projection := state.project(20, 200)
	if !slices.Equal(projection.lines, want) {
		t.Fatalf("lines = %#v, want %#v", projection.lines, want)
	}
	if strings.Contains(strings.Join(projection.lines, "\n"), connection.CredentialRef) {
		t.Fatal("detail exposed credential reference")
	}

	connection.Username = "deploy"
	connection.AuthMethod = app.AuthMethodAgent
	connection.IdentityFile = ""
	snapshot = newCatalogSnapshot(root, 4)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Connections: []app.Connection{connection}})
	state, ok = state.withTarget(snapshot, connection.ID)
	if !ok {
		t.Fatal("updated connection detail target rejected")
	}
	joined := strings.Join(state.project(20, 200).lines, "\n")
	if !strings.Contains(joined, "User: deploy") || strings.Contains(joined, "Identity:") {
		t.Fatalf("optional/default fields projected incorrectly:\n%s", joined)
	}
}

func TestDetailFolderUsesOrderedImmediateConnectionChildren(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	folder := testFolder("team", root.ID, "/team", 1)
	nestedFolder := testFolder("nested-folder", folder.ID, "/team/nested", 1)
	alphaSecond := testConnection("connection-2", folder.ID, "/team/alpha", 1)
	alphaSecond.Host = "alpha-two.example"
	alphaFirst := testConnection("connection-1", folder.ID, "/team/alpha", 1)
	alphaFirst.Host = "alpha-one.example"
	zeta := testConnection("connection-z", folder.ID, "/team/zeta", 1)
	zeta.Host = "zeta.example"
	nested := testConnection("nested", nestedFolder.ID, "/team/nested/hidden", 1)
	nested.Host = "nested.example"

	snapshot := newCatalogSnapshot(root, 1)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{folder}})
	_ = snapshot.addChildren(folder.ID, app.ListChildrenResult{
		Folders:     []app.Folder{nestedFolder},
		Connections: []app.Connection{zeta, alphaSecond, alphaFirst},
	})
	_ = snapshot.addChildren(nestedFolder.ID, app.ListChildrenResult{Connections: []app.Connection{nested}})

	state, ok := newDetailState(snapshot, folder.ID)
	if !ok {
		t.Fatal("folder detail target rejected")
	}
	gotIDs := make([]app.NodeID, len(state.directConnections))
	for index, connection := range state.directConnections {
		gotIDs[index] = connection.id
	}
	wantIDs := []app.NodeID{alphaFirst.ID, alphaSecond.ID, zeta.ID}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("direct connection order = %q, want %q", gotIDs, wantIDs)
	}
	joined := strings.Join(state.project(20, 200).lines, "\n")
	for _, want := range []string{"Kind: Folder", "Name: team", "Path: /team", "Direct connections: 3", "alpha: alpha-one.example:22", "alpha: alpha-two.example:22", "zeta: zeta.example:22"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("folder detail omitted %q:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "hidden") || strings.Contains(joined, "nested.example") {
		t.Fatalf("folder detail included a nested descendant:\n%s", joined)
	}
}

func TestDetailRootAndFolderEmptyState(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	empty := testFolder("empty", root.ID, "/empty", 1)
	childFolder := testFolder("child-folder", empty.ID, "/empty/child", 1)
	snapshot := newCatalogSnapshot(root, 1)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{empty}})
	_ = snapshot.addChildren(empty.ID, app.ListChildrenResult{Folders: []app.Folder{childFolder}})

	rootState, ok := newDetailState(snapshot, root.ID)
	if !ok || rootState.kind != detailKindRoot || rootState.heading != "Root" {
		t.Fatalf("root state = %#v, valid %v", rootState, ok)
	}
	for name, id := range map[string]app.NodeID{"root": root.ID, "folder containing only a folder": empty.ID} {
		t.Run(name, func(t *testing.T) {
			state, valid := newDetailState(snapshot, id)
			if !valid {
				t.Fatal("detail target rejected")
			}
			joined := strings.Join(state.project(20, 200).lines, "\n")
			if !strings.Contains(joined, "Direct connections: 0") || !strings.Contains(joined, detailEmptyConnections) {
				t.Fatalf("missing explicit empty state:\n%s", joined)
			}
		})
	}
}

func TestDetailTargetChangeResetsViewportOffset(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	first := testConnection("first", root.ID, "/first", 1)
	second := testConnection("second", root.ID, "/second", 1)
	snapshot := newCatalogSnapshot(root, 1)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Connections: []app.Connection{first, second}})

	state, ok := newDetailState(snapshot, first.ID)
	if !ok {
		t.Fatal("first target rejected")
	}
	state = state.withOffset(4)
	sameTarget, ok := state.withTarget(snapshot, first.ID)
	if !ok || sameTarget.viewport.logicalOffset != 4 {
		t.Fatalf("same-target offset = %d, want 4", sameTarget.viewport.logicalOffset)
	}
	changedTarget, ok := sameTarget.withTarget(snapshot, second.ID)
	if !ok || changedTarget.viewport.logicalOffset != 0 {
		t.Fatalf("changed-target offset = %d, want 0", changedTarget.viewport.logicalOffset)
	}
	if stale, valid := changedTarget.withTarget(snapshot, "missing"); valid || stale.targetID != "" {
		t.Fatalf("missing target retained stale detail: %#v, valid %v", stale, valid)
	}
}

func TestDetailProjectionMakesUntrustedTextInertAndBounded(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	connection := testConnection("connection", root.ID, "/unsafe\npath", 1)
	connection.Name = "prod\x1b[31m"
	connection.Host = "host\u202Eevil"
	connection.Username = "user\rname"
	connection.CredentialRef = "SECRET-CANARY"
	snapshot := newCatalogSnapshot(root, 1)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Connections: []app.Connection{connection}})
	state, ok := newDetailState(snapshot, connection.ID)
	if !ok {
		t.Fatal("unsafe-valued target rejected")
	}
	projection := state.project(20, 24)
	joined := strings.Join(projection.lines, "\n")
	for _, forbidden := range []string{"\x1b", "\r", "\u202e", connection.CredentialRef} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("projection contains unsafe/secret canary %q: %q", forbidden, joined)
		}
	}
	for _, escaped := range []string{`\x1B`, `\n`, `\u202E`, `\r`} {
		if !strings.Contains(strings.Join(state.content(200), "\n"), escaped) {
			t.Fatalf("safe content omitted inert escape %q: %#v", escaped, state.content(200))
		}
	}
}

func TestDetailProjectionUsesSnapshotDirectChildIndex(t *testing.T) {
	root := testFolder("root", "", "/", 1)
	target := testFolder("target", root.ID, "/target", 1)
	direct := testConnection("direct", target.ID, "/target/direct", 1)
	poison := testConnection("not-indexed", target.ID, "/target/not-indexed", 1)
	snapshot := newCatalogSnapshot(root, 1)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{target}})
	_ = snapshot.addChildren(target.ID, app.ListChildrenResult{Connections: []app.Connection{direct}})
	snapshot.nodes[poison.ID] = treeNode{node: poison.Node, connection: &poison}
	for index := 0; index < 10_000; index++ {
		connection := testConnection(fmt.Sprintf("unrelated-%05d", index), root.ID, fmt.Sprintf("/unrelated-%05d", index), 1)
		snapshot.nodes[connection.ID] = treeNode{node: connection.Node, connection: &connection}
	}

	state, ok := newDetailState(snapshot, target.ID)
	if !ok {
		t.Fatal("target rejected")
	}
	if len(state.directConnections) != 1 || state.directConnections[0].id != direct.ID {
		t.Fatalf("direct connections = %#v, want only %q", state.directConnections, direct.ID)
	}
}

func BenchmarkDetailDirectChildren(b *testing.B) {
	root := testFolder("root", "", "/", 1)
	target := testFolder("target", root.ID, "/target", 1)
	directA := testConnection("direct-a", target.ID, "/target/a", 1)
	directB := testConnection("direct-b", target.ID, "/target/b", 1)
	snapshot := newCatalogSnapshot(root, 1)
	_ = snapshot.addChildren(root.ID, app.ListChildrenResult{Folders: []app.Folder{target}})
	_ = snapshot.addChildren(target.ID, app.ListChildrenResult{Connections: []app.Connection{directB, directA}})
	for index := 0; index < 100_000; index++ {
		connection := testConnection(fmt.Sprintf("unrelated-%06d", index), root.ID, fmt.Sprintf("/unrelated-%06d", index), 1)
		snapshot.nodes[connection.ID] = treeNode{node: connection.Node, connection: &connection}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		state, ok := newDetailState(snapshot, target.ID)
		if !ok || len(state.directConnections) != 2 {
			b.Fatal("detail projection failed")
		}
	}
}
