package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/app"
)

var feature008RegionFixtures = []struct {
	name  string
	label string
	owner focusOwner
}{
	{name: "tree", label: "Tree", owner: focusOwnerTree},
	{name: "details", label: "Details", owner: focusOwnerDetail},
	{name: "actions", label: "Actions", owner: focusOwnerNone},
}

var feature008DetailFixtures = []struct {
	kind  detailKind
	label string
}{
	{kind: detailKindRoot, label: "Root"},
	{kind: detailKindFolder, label: "Folder"},
	{kind: detailKindConnection, label: "Connection"},
}

var feature008ModalFixtures = []struct {
	kind  modalKind
	label string
}{
	{kind: modalKindFolderCreate, label: "Create Folder"},
	{kind: modalKindFolderEdit, label: "Edit Folder"},
	{kind: modalKindMovePicker, label: "Move"},
	{kind: modalKindDeleteConnection, label: "Delete"},
	{kind: modalKindDeleteFolder, label: "Delete"},
	{kind: modalKindConnectConfirmation, label: "Connect"},
	{kind: modalKindUnsavedChanges, label: "Unsaved Changes"},
	{kind: modalKindHelp, label: "Help"},
	{kind: modalKindOperationError, label: "Operation Error"},
	{kind: modalKindSSHFailure, label: "SSH Failure"},
}

var feature008ConnectionFormFixtures = []struct {
	name  string
	edit  bool
	label string
}{
	{name: "new", label: "New connection"},
	{name: "edit", edit: true, label: "Edit connection"},
}

var feature008FolderFormFixtures = []struct {
	name  string
	kind  modalKind
	label string
}{
	{name: "create", kind: modalKindFolderCreate, label: "Create Folder"},
	{name: "edit", kind: modalKindFolderEdit, label: "Edit Folder"},
}

var feature008MovePickerFixtures = []struct {
	kind  modalKind
	label string
}{
	{kind: modalKindMovePicker, label: "Move"},
}

var feature008TrustPromptFixtures = []struct {
	status app.HostTrustStatus
	label  string
}{
	{status: app.HostTrustUnknown, label: "Verify host identity"},
	{status: app.HostTrustChanged, label: "Verify host identity"},
	{status: app.HostTrustRevoked, label: "Verify host identity"},
}

var feature008SecretPromptFixtures = []struct {
	kind  app.SecretKind
	label string
}{
	{kind: app.SecretPassword, label: "Password"},
	{kind: app.SecretPassphrase, label: "Private key passphrase"},
}

func TestPanelStyleFixtureInventory(t *testing.T) {
	if got := fixtureLabels(feature008RegionFixtures, func(fixture struct {
		name  string
		label string
		owner focusOwner
	}) string {
		return fixture.label
	}); !slices.Equal(got, []string{"Tree", "Details", "Actions"}) {
		t.Fatalf("region fixtures = %#v", got)
	}
	if got := fixtureLabels(feature008DetailFixtures, func(fixture struct {
		kind  detailKind
		label string
	}) string {
		return fixture.label
	}); !slices.Equal(got, []string{"Root", "Folder", "Connection"}) {
		t.Fatalf("detail fixtures = %#v", got)
	}
	if len(feature008ModalFixtures) != 10 || len(newModalRegistry().payloadTypes) != 10 {
		t.Fatalf("modal fixture/registry count = %d/%d, want 10/10", len(feature008ModalFixtures), len(newModalRegistry().payloadTypes))
	}
	for _, fixture := range feature008ModalFixtures {
		if !fixture.kind.valid() || modalTitle(fixture.kind) != fixture.label {
			t.Fatalf("modal fixture = %#v, title %q", fixture, modalTitle(fixture.kind))
		}
	}
	if len(feature008ConnectionFormFixtures) != 2 || len(feature008FolderFormFixtures) != 2 || len(feature008MovePickerFixtures) != 1 || len(feature008TrustPromptFixtures) != 3 || len(feature008SecretPromptFixtures) != 2 {
		t.Fatalf("surface fixture counts = connection %d folder %d move %d trust %d secret %d", len(feature008ConnectionFormFixtures), len(feature008FolderFormFixtures), len(feature008MovePickerFixtures), len(feature008TrustPromptFixtures), len(feature008SecretPromptFixtures))
	}
}

func TestStructuralRegionTitlesArePlainTextWithoutBackground(t *testing.T) {
	plain := newStyles(true)
	color := newStyles(false)
	for _, fixture := range feature008RegionFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			active := fixture.owner != focusOwnerNone
			plainTitle := plain.regionTitle(fixture.label, active)
			coloredTitle := color.regionTitle(fixture.label, active)
			if !strings.Contains(plainTitle, fixture.label) || strings.Contains(plainTitle, "["+fixture.label+"]") {
				t.Fatalf("plain structural title = %q", plainTitle)
			}
			if strings.Contains(coloredTitle, ";4") {
				t.Fatalf("colored structural title has a background: %q", coloredTitle)
			}
		})
	}
}

func fixtureLabels[Fixture any](fixtures []Fixture, label func(Fixture) string) []string {
	labels := make([]string, len(fixtures))
	for index, fixture := range fixtures {
		labels[index] = label(fixture)
	}
	return labels
}
