package tui

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/app"
)

func TestConflictReloadPreservesOpenForm(t *testing.T) {
	connection := app.Connection{Node: app.Node{ID: "11111111111111111111111111111111", Name: "prod", Path: "/prod", Revision: 1}, Host: "prod.test", Port: 22, AuthMethod: app.AuthMethodAgent}
	revision := app.CatalogRevision(6)
	model := New(Config{Width: 80, Height: 24, NoColor: true, Connections: ConnectionFuncs{ListFunc: func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
		revision++
		return app.ListConnectionsResult{Connections: []app.Connection{connection}, CatalogRevision: revision}, nil
	}}})
	updateModel(model, model.Init()())
	updateModel(model, keyPress("l"))
	updateModel(model, keyPress("e"))
	form := model.form
	form.inputs[fieldName].SetValue("pending")
	model.installFormConflict(conflictTypeRevisionChanged)
	_, command := model.Update(keyPress("r"))
	if command == nil {
		t.Fatal("conflict reload did not start")
	}
	updateModel(model, command())
	if model.form != form || model.form.inputs[fieldName].Value() != "pending" {
		t.Fatal("reload discarded the open form")
	}
	if model.status != "READY" || model.connectionEdit.conflict != nil {
		t.Fatalf("status/conflict = %q/%#v, want resolved", model.status, model.connectionEdit.conflict)
	}
}

func TestDestructiveConfirmationDefaultsToCancel(t *testing.T) {
	scope := app.ConnectionDeleteScope{Path: "/prod", Name: "prod", Host: "prod.example", Username: "deploy", Revision: 2, HasRememberedPassword: true}
	modal := newDeleteConfirmation(scope)
	if modal.choice != confirmationCancel || modal.confirmed(keyPress("enter")) {
		t.Fatal("destructive confirmation did not default to cancel")
	}
	if !modal.confirmed(keyPress("y")) {
		t.Fatal("documented explicit confirmation key was rejected")
	}
	view := strings.Join(deleteConnectionLines(modal, 80), "\n")
	if !strings.Contains(view, "/prod") || !strings.Contains(view, "saved password") {
		t.Fatalf("confirmation omitted scope: %q", view)
	}
}

func TestConnectConfirmationCapturesPathEndpointAndRevision(t *testing.T) {
	connection := app.Connection{Node: app.Node{ID: "connection", Kind: app.NodeKindConnection, Path: "/work/prod", Revision: 7}, Host: "prod.test", Port: 2202}
	modal := newConnectConfirmation(connection)
	connection.Path = "/changed"
	connection.Host = "changed.test"
	if modal.confirmed(keyPress("enter")) || !modal.confirmed(keyPress("y")) {
		t.Fatal("connect confirmation did not require explicit Y")
	}
	view := strings.Join(connectConfirmationLines(modal, 80), "\n")
	for _, value := range []string{"/work/prod", "prod.test:2202", "connection", "Revision: 7"} {
		if !strings.Contains(view, value) {
			t.Fatalf("connect confirmation omitted %q: %q", value, view)
		}
	}
	if strings.Contains(view, "changed") {
		t.Fatalf("connect confirmation followed mutable source: %q", view)
	}
}

func TestConnectConfirmationFormatsIPv6Endpoint(t *testing.T) {
	connection := app.Connection{Node: app.Node{ID: "connection", Path: "/ipv6", Revision: 1}, Host: "2001:db8::1", Port: 22}
	modal := newConnectConfirmation(connection)
	view := strings.Join(connectConfirmationLines(modal, 40), "\n")
	if !strings.Contains(view, "Endpoint: [2001:db8::1]:22") {
		t.Fatalf("confirmation has ambiguous IPv6 endpoint: %q", view)
	}
}

func TestSSHFailurePriorityAt80x24ReducedAndNoColor(t *testing.T) {
	attempt := app.SSHAttemptTarget{ID: "captured", Revision: 7, Path: "/work/prod", Host: "prod.test", Port: 2202}
	failure := app.NewSSHStartError(app.SSHFailureAuthenticationDenied, app.SSHFailureStageAuthentication, "server rejected available authentication methods", nil).Presentation()
	for _, size := range []struct{ width, height int }{{80, 24}, {60, 12}} {
		model := New(Config{Width: size.width, Height: size.height, NoColor: true})
		modal := newSSHFailureModal(attempt, failure)
		modal.detailVisible = true
		model.installSSHFailure(modal)
		view := model.View().Content
		wants := []string{"Category: authentication_denied", "Path: /work/prod", "Endpoint: prod.test:2202", "d Detail", "r Retry", "e Edit", "b/Esc Back", "q Quit"}
		if size.width == 80 {
			wants = append(wants, "Stage: authentication", "Recommendation:")
		}
		for _, want := range wants {
			if !strings.Contains(view, want) {
				t.Fatalf("%dx%d view omitted priority item %q: %q", size.width, size.height, want, view)
			}
		}
		if strings.Contains(view, "\x1b[") {
			t.Fatalf("no-color view contains ANSI: %q", view)
		}
	}
}

func TestSSHFailureAt80x24BoundsSamePrefixTargets(t *testing.T) {
	prefix := "/production/" + strings.Repeat("shared-path-segment/", 300)
	hostPrefix := strings.Repeat("shared-endpoint-segment.", 300)
	targets := []struct {
		id, pathSuffix, hostSuffix string
	}{
		{"connection-primary", "primary-distinguishing-suffix", "primary.example.test"},
		{"connection-standby", "standby-distinguishing-suffix", "standby.example.test"},
	}
	failure := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", nil).Presentation()

	for _, target := range targets {
		model := New(Config{Width: 80, Height: 24})
		model.installSSHFailure(newSSHFailureModal(app.SSHAttemptTarget{
			ID:   app.NodeID(target.id),
			Path: prefix + target.pathSuffix,
			Host: hostPrefix + target.hostSuffix,
			Port: 2222,
		}, failure))
		views := strings.Join(sshFailureLines(model.activeSSHFailure(), 70), "")
		for _, want := range []string{"Path: /production/", target.pathSuffix, "Endpoint: shared-endpoint", target.hostSuffix + ":2222", "ID/revision: " + target.id, "Category: timeout", "Stage: network_connection", "Recommendation:", "d Detail", "r Retry", "e Edit", "b/Esc Back", "q Quit"} {
			if !strings.Contains(views, want) {
				t.Fatalf("80x24 scrollable view omitted %q for %q", want, target.id)
			}
		}
		view := model.View().Content
		if lines := strings.Count(view, "\n") + 1; lines > 24 {
			t.Fatalf("80x24 failure has %d lines: %q", lines, view)
		}
		for _, line := range strings.Split(view, "\n") {
			if width := ansi.StringWidth(line); width > 80 {
				t.Fatalf("bounded line width = %d, want <= 80: %q", width, line)
			}
		}
	}
}

func TestConnectConfirmationsAt80x24BoundLongDistinctTargetsAndKeepControls(t *testing.T) {
	pathPrefix := "/production/" + strings.Repeat("shared-path-segment/", 300)
	hostPrefix := strings.Repeat("shared-endpoint-segment.", 300)
	current := app.Connection{Node: app.Node{ID: "current-stable-id", Path: pathPrefix + "current-distinguishing-suffix", Revision: 9}, Host: hostPrefix + "current.example.test", Port: 2202}
	previous := app.SSHAttemptTarget{ID: "previous-stable-id", Path: pathPrefix + "previous-distinguishing-suffix", Host: hostPrefix + "previous.example.test", Port: 22}

	tests := []struct {
		name  string
		modal *connectConfirmation
		want  []string
	}{
		{"normal", newConnectConfirmation(current), []string{"Path: /production/", "current-distinguishing-suffix", "current.example.test:2202", "ID: current-stable-id", "Revision: 9"}},
		{"retry", newRetryConnectConfirmation(previous, current), []string{"Previous path: /production/", "previous-distinguishing-suffix", "previous.example.test:22", "Previous ID/revision: previous-stable-id", "Path: /production/", "current-distinguishing-suffix", "current.example.test:2202", "ID: current-stable-id", "Revision: 9"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New(Config{Width: 80, Height: 24, NoColor: true})
			target := capturedTargetFromConnection(tt.modal.connection)
			model.openGenericModal(modalKindConnectConfirmation, &target, connectConfirmationPayload{confirmation: tt.modal})
			views := strings.Join(connectConfirmationLines(tt.modal, 70), "")
			for _, want := range append(tt.want, "y Confirm", "Enter/Esc Cancel") {
				if !strings.Contains(views, want) {
					t.Fatalf("80x24 scrollable confirmation omitted %q", want)
				}
			}
			view := model.View().Content
			if lines := strings.Count(view, "\n") + 1; lines > 24 {
				t.Fatalf("80x24 confirmation has %d lines: %q", lines, view)
			}
			for _, line := range strings.Split(view, "\n") {
				if width := ansi.StringWidth(line); width > 80 {
					t.Fatalf("bounded line width = %d, want <= 80: %q", width, line)
				}
			}
		})
	}
}

func TestUnifiedConfirmationAndErrorOverlaysKeepTargetsAndControlsVisible(t *testing.T) {
	connection := app.Connection{Node: app.Node{ID: "connection", Path: "/prod", Revision: 2}, Host: "prod.test", Port: 22}
	scope := app.ConnectionDeleteScope{ID: "connection", Path: "/prod", Host: "prod.test", Username: "deploy"}
	folderScope := app.FolderDeleteScope{ID: "folder", Path: "/tree", Folders: 2, Connections: 1}
	tests := []struct {
		name  string
		setup func(*Model)
		want  []string
	}{
		{name: "connect", setup: func(model *Model) {
			model.openGenericModal(modalKindConnectConfirmation, nil, connectConfirmationPayload{newConnectConfirmation(connection)})
		}, want: []string{"Path: /prod", "Endpoint: prod.test:22", "Enter/Esc Cancel", "y Confirm"}},
		{name: "delete connection", setup: func(model *Model) {
			model.openGenericModal(modalKindDeleteConnection, nil, deleteConnectionPayload{newDeleteConfirmation(scope)})
		}, want: []string{"Path: /prod", "Target: deploy@prod.test", "Enter/Esc Cancel", "y Confirm"}},
		{name: "delete folder", setup: func(model *Model) {
			model.openGenericModal(modalKindDeleteFolder, nil, deleteFolderPayload{newFolderDeleteConfirmation(folderScope)})
		}, want: []string{"Path: /tree", "Scope: 2 folders, 1 connection", "Enter/Esc Cancel", "y Confirm"}},
		{name: "error", setup: func(model *Model) {
			model.openGenericModal(modalKindOperationError, nil, operationErrorPayload{newErrorModal("move", "/prod", app.ErrConflict)})
		}, want: []string{"Target: /prod", "Cause:", "r Reload", "b/Esc Back"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New(Config{Width: 42, Height: 12, NoColor: true})
			tt.setup(model)
			view := model.View().Content
			for _, want := range tt.want {
				if !strings.Contains(view, want) {
					t.Fatalf("unified overlay omitted %q: %q", want, view)
				}
			}
		})
	}
}

func TestTrustPromptPrecedesSecretAndDefaultsToReject(t *testing.T) {
	prompt := newTrustPrompt(app.TrustDecisionPrompt{
		Status: app.HostTrustChanged,
		Host:   app.PresentedHost{Endpoint: app.HostEndpoint{CanonicalHost: "host.example", Port: 22}, RemoteAddress: "192.0.2.10:22", KeyAlgorithm: "ssh-ed25519", FingerprintSHA256: "SHA256:new"},
		Known:  &app.TrustedHost{FingerprintSHA256: "SHA256:old"},
	})
	if prompt.decision != app.TrustReject {
		t.Fatalf("default trust decision = %q", prompt.decision)
	}
	view := prompt.view(newStyles(true))
	for _, value := range []string{"host.example:22", "192.0.2.10:22", "ssh-ed25519", "SHA256:new", "SHA256:old", "possible attack"} {
		if !strings.Contains(view, value) {
			t.Fatalf("trust prompt omitted %q: %q", value, view)
		}
	}

	secret := newSecretPrompt(app.SecretPassword, "operating system credential store")
	if secret.consent {
		t.Fatal("password storage consent started checked")
	}
	secret.setForTest([]byte("never-render-this"))
	if strings.Contains(secret.view(newStyles(true)), "never-render-this") {
		t.Fatal("secret was retained in or exposed by the view")
	}
	secret.clear()
	if len(secret.take()) != 0 {
		t.Fatal("cleared secret remained available")
	}
}

func TestMovePickerProjectsDynamicValuesBeforeWrappingOrTruncation(t *testing.T) {
	unsafeValues := []string{
		"ansi\x1b[31mred\x1b[0m",
		"carriage\rreturn\nlinefeed",
		"c0\x00\x07\x7f",
		"c1\u0085\u009b",
		"bidi\u061c\u202e\u2066",
		string([]byte{'i', 'n', 'v', 'a', 'l', 'i', 'd', 0xff, 0xc3, '('}),
	}

	for _, unsafeValue := range unsafeValues {
		source := app.Node{ID: app.NodeID(unsafeValue), Kind: app.NodeKindConnection, Path: unsafeValue, Revision: 7}
		folder := app.Folder{Node: app.Node{ID: app.NodeID(unsafeValue + "-id"), Kind: app.NodeKindFolder, Path: unsafeValue, Revision: 9}}
		picker := newMovePicker(source, []app.Folder{folder})
		lines, _ := picker.modalLines(200)
		view := strings.Join(lines, "\n")
		projection := safeText(unsafeValue, 200)

		if strings.Count(view, projection) < 3 {
			t.Fatalf("move picker omitted a safe source/ID/destination projection %q:\n%s", projection, view)
		}
		for _, line := range lines {
			assertTerminalSafe(t, line, max(ansi.StringWidth(line), 1))
		}
		if picker.source.ID != source.ID || picker.source.Path != source.Path {
			t.Fatal("move rendering changed the captured source bytes")
		}
		if destination := picker.destination(); destination == nil || destination.ID != folder.ID || destination.Path != folder.Path || destination.Revision != folder.Revision {
			t.Fatalf("move rendering changed destination identity: %#v", destination)
		}
	}
}

func TestMovePickerOversizedValuesAreBoundedWithoutChangingTargets(t *testing.T) {
	oversized := "/" + strings.Repeat("destination-segment/", 100_000) + "tail"
	source := app.Node{ID: app.NodeID(oversized), Kind: app.NodeKindConnection, Path: oversized, Revision: 7}
	folder := app.Folder{Node: app.Node{ID: "destination-id", Kind: app.NodeKindFolder, Path: oversized, Revision: 9}}
	picker := newMovePicker(source, []app.Folder{folder})
	lines, _ := picker.modalLines(40)

	if len(lines) > 6 {
		t.Fatalf("oversized move values expanded to %d lines, want bounded single-line projections", len(lines))
	}
	for _, line := range lines {
		if width := ansi.StringWidth(line); width > 40 {
			t.Fatalf("oversized move line width = %d, want <= 40: %q", width, line)
		}
		assertTerminalSafe(t, line, 40)
	}
	if picker.source.ID != source.ID || picker.source.Path != source.Path || picker.destination() == nil || picker.destination().ID != folder.ID {
		t.Fatal("bounded move rendering changed captured target bytes or ID")
	}
}

func TestFormConflictProjectsEveryCapturedTargetValueBeforeTruncation(t *testing.T) {
	unsafeValues := []string{
		"ansi\x1b[31mred\x1b[0m",
		"carriage\rreturn\nlinefeed",
		"c0\x00\x07\x7f",
		"c1\u0085\u009b",
		"bidi\u061c\u202e\u2066",
		string([]byte{'i', 'n', 'v', 'a', 'l', 'i', 'd', 0xff, 0xc3, '('}),
	}

	for _, unsafeValue := range unsafeValues {
		target := capturedTarget{id: app.NodeID(unsafeValue), revision: 11, kind: app.NodeKindConnection, path: unsafeValue}
		model := &Model{styles: newStyles(true), connectionEdit: &connectionEditState{conflict: &conflictState{kind: conflictTypeRevisionChanged, target: target, detailVisible: true}}}
		lines := model.formConflictLines(200)
		view := strings.Join(lines, "\n")
		projection := safeText(unsafeValue, 200)

		if strings.Count(view, projection) != 2 {
			t.Fatalf("form conflict omitted a safe path/ID projection %q:\n%s", projection, view)
		}
		for _, line := range lines {
			assertTerminalSafe(t, line, max(ansi.StringWidth(line), 1))
		}
		if conflict := model.connectionEdit.conflict; conflict.target.id != target.id || conflict.target.path != target.path || conflict.target.revision != target.revision {
			t.Fatalf("form conflict rendering changed captured identity: %#v", conflict.target)
		}

		picker := newMovePicker(app.Node{ID: "source", Kind: app.NodeKindConnection, Path: "/source", Revision: 1}, []app.Folder{{Node: app.Node{ID: "destination", Path: "/destination"}}})
		state := modalState{
			kind:     modalKindMovePicker,
			payload:  movePickerPayload{picker: picker},
			conflict: &conflictState{kind: conflictTypeRevisionChanged, target: target, detailVisible: true},
		}
		modalLines, _ := modalContent(state, newStyles(true), 200, nil)
		modalView := strings.Join(modalLines, "\n")
		if strings.Count(modalView, projection) != 2 {
			t.Fatalf("modal conflict omitted a safe path/ID projection %q:\n%s", projection, modalView)
		}
		for _, line := range modalLines {
			line = strings.TrimPrefix(line, modalControlsPrefix)
			assertTerminalSafe(t, line, max(ansi.StringWidth(line), 1))
		}
		if state.conflict.target.id != target.id || state.conflict.target.path != target.path || state.conflict.target.revision != target.revision {
			t.Fatalf("modal conflict rendering changed captured identity: %#v", state.conflict.target)
		}
	}
}

func TestFormConflictOversizedValuesAreBoundedWithoutChangingTarget(t *testing.T) {
	oversized := "/" + strings.Repeat("captured-segment/", 100_000) + "tail"
	target := capturedTarget{id: app.NodeID(oversized), revision: 11, kind: app.NodeKindConnection, path: oversized}
	model := &Model{styles: newStyles(true), connectionEdit: &connectionEditState{conflict: &conflictState{kind: conflictTypeRevisionChanged, target: target, detailVisible: true}}}
	lines := model.formConflictLines(40)

	for _, line := range lines {
		assertTerminalSafe(t, line, 40)
	}
	if conflict := model.connectionEdit.conflict; conflict.target.id != target.id || conflict.target.path != target.path || conflict.target.revision != target.revision {
		t.Fatal("bounded form-conflict rendering changed captured target bytes")
	}
	picker := newMovePicker(app.Node{ID: "source", Kind: app.NodeKindConnection, Path: "/source", Revision: 1}, []app.Folder{{Node: app.Node{ID: "destination", Path: "/destination"}}})
	state := modalState{
		kind:     modalKindMovePicker,
		payload:  movePickerPayload{picker: picker},
		conflict: &conflictState{kind: conflictTypeRevisionChanged, target: target, detailVisible: true},
	}
	modalLines, _ := modalContent(state, newStyles(true), 40, nil)
	for _, line := range modalLines {
		line = strings.TrimPrefix(line, modalControlsPrefix)
		lineWidth := ansi.StringWidth(line)
		assertTerminalSafe(t, line, max(lineWidth, 1))
		if (strings.HasPrefix(line, "Target: ") || strings.HasPrefix(line, "ID/revision: ")) && lineWidth > 40 {
			t.Fatalf("oversized modal-conflict identity width = %d, want <= 40: %q", lineWidth, line)
		}
	}
	if state.conflict.target.id != target.id || state.conflict.target.path != target.path || state.conflict.target.revision != target.revision {
		t.Fatal("bounded modal-conflict rendering changed captured target bytes")
	}
}

func TestLoadingModalSuppressesStalePayloadActions(t *testing.T) {
	connection := app.Connection{Node: app.Node{ID: "connection", Kind: app.NodeKindConnection, Path: "/captured", Revision: 7}, Host: "captured.test", Port: 22}
	failure := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "timeout", nil).Presentation()
	tests := []struct {
		name  string
		state modalState
		want  []string
		stale []string
	}{
		{
			name:  "move",
			state: modalState{kind: modalKindMovePicker, operationStatus: "Loading: Move — /captured", payload: movePickerPayload{picker: newMovePicker(connection.Node, []app.Folder{{Node: app.Node{ID: "folder", Path: "/destination"}}})}},
			want:  []string{"Move destination", "Source: /captured", "ID/revision: connection/7", "/destination"},
			stale: []string{"Enter Move"},
		},
		{
			name:  "confirmation",
			state: modalState{kind: modalKindConnectConfirmation, operationStatus: "Loading: SSH start — captured.test:22", payload: connectConfirmationPayload{confirmation: newConnectConfirmation(connection)}},
			want:  []string{"Connect confirmation", "Path: /captured", "Endpoint: captured.test:22", "ID: connection", "Revision: 7"},
			stale: []string{"y Confirm", "Enter/Esc Cancel"},
		},
		{
			name:  "SSH failure",
			state: modalState{kind: modalKindSSHFailure, operationStatus: "Loading: Reload — /captured", payload: sshFailurePayload{modal: newSSHFailureModal(app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}, failure)}},
			want:  []string{"SSH startup failed", "Path: /captured", "ID/revision: connection/7"},
			stale: []string{"d Detail", "r Retry", "e Edit", "b/Esc Back"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lines, _ := modalContent(test.state, newStyles(true), 80, []string{"help"})
			view := strings.Join(lines, "\n")
			for _, want := range append([]string{test.state.operationStatus}, test.want...) {
				if !strings.Contains(view, want) {
					t.Fatalf("loading modal omitted captured content %q:\n%s", want, view)
				}
			}
			for _, stale := range test.stale {
				if strings.Contains(view, stale) {
					t.Fatalf("loading modal retained stale payload action %q:\n%s", stale, view)
				}
			}
			priority := modalPriorityStart(lines)
			if priority < 0 {
				t.Fatalf("loading modal omitted priority controls:\n%s", view)
			}
			controls := lines[priority:]
			wantControls := []string{modalControlLine("Esc Cancel"), modalControlLine("? Help"), modalControlLine("q Quit")}
			if !reflect.DeepEqual(controls, wantControls) {
				t.Fatalf("loading controls = %#v, want %#v", controls, wantControls)
			}
		})
	}
}

func TestOperationErrorAndSSHFailureDescriptorsUseLowercaseContractKeys(t *testing.T) {
	operationConflict := strings.Join(operationErrorLines(newErrorModal("delete", "/captured", app.ErrConflict), 80), "\n")
	operationRetry := newErrorModal("delete", "/captured", app.ErrInvalidRequest)
	operationRetry.retry = &operationRetryIntent{kind: operationRetryCatalog, reloadKind: asyncOperationReload, owner: operationOwnerRoot}
	operationRetryView := strings.Join(operationErrorLines(operationRetry, 80), "\n")
	failure := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "timeout", nil).Presentation()
	sshModal := newSSHFailureModal(app.SSHAttemptTarget{ID: "connection", Revision: 7, Path: "/captured", Host: "captured.test", Port: 22}, failure)
	sshView := strings.Join(sshFailureLines(sshModal, 80), "\n")

	for name, test := range map[string]struct {
		view string
		want []string
	}{
		"operation conflict": {operationConflict, []string{"r Reload", "b/Esc Back", "q Quit"}},
		"operation retry":    {operationRetryView, []string{"r Retry", "b/Esc Back", "q Quit"}},
		"SSH failure":        {sshView, []string{"d Detail", "r Retry", "e Edit", "b/Esc Back", "q Quit"}},
	} {
		t.Run(name, func(t *testing.T) {
			for _, want := range test.want {
				if !strings.Contains(test.view, want) {
					t.Fatalf("descriptor omitted %q:\n%s", want, test.view)
				}
			}
			for _, uppercase := range []string{"D Detail", "R Reload", "R Retry", "E Edit", "B Back", "Q Quit"} {
				if strings.Contains(test.view, uppercase) {
					t.Fatalf("descriptor retained uppercase key %q:\n%s", uppercase, test.view)
				}
			}
		})
	}
}
