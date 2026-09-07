package tui

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
)

// controlledTerms is the package's test-only vocabulary authority. The
// manifest duplicates it deliberately so every copy change requires human
// review rather than being silently regenerated from source.
var controlledTerms = map[string]string{
	"action.back":             "Back",
	"action.cancel":           "Cancel",
	"action.connect":          "Connect",
	"action.delete":           "Delete",
	"action.discard":          "Discard",
	"action.edit":             "Edit",
	"action.help":             "Help",
	"action.move":             "Move",
	"action.new_connection":   "New connection",
	"action.new_folder":       "New folder",
	"action.quit":             "Quit",
	"action.reload":           "Reload",
	"action.retry":            "Retry",
	"action.save":             "Save",
	"field.identity":          "Identity file",
	"field.method":            "Method",
	"field.remember_password": "Remember password",
	"region.actions":          "Actions",
	"region.details":          "Details",
	"region.tree":             "Tree",
	"security.passphrase":     "Private key passphrase",
	"security.password":       "Password",
}

var sourceTechnicalLiterals = map[string]string{
	"%w for %s: invalid payload": "internal error formatting",
	"%w for %s: want %v, got %T": "internal error formatting",
	"%w: %q":                     "internal error formatting",
	"%w: %s":                     "internal error formatting",
	"0123456789ABCDEF":           "hex encoding alphabet",
	"catalog tree contains duplicate or invalid nodes":                "internal invariant",
	"inline help must restore its payload control before modal close": "internal state error",
	"inline help requires an open non-help modal":                     "internal state error",
	"modal is closed":                             "internal state error",
	"modal kind has no registered payload":        "internal state error",
	"modal kind payload is already registered":    "internal state error",
	"modal opener must be an application surface": "internal state error",
	"modal payload has the wrong type":            "internal state error",
	"nested modal is not allowed":                 "internal state error",
	"replacing an open modal is not allowed":      "internal state error",
	"run TUI: nil context":                        "adapter error",
	"run TUI: invalid VT capability report":       "adapter error",
	"run TUI: unexpected final model":             "adapter error",
	"unknown modal kind":                          "internal state error",
	"__tui_root__":                                "internal synthetic identifier",
	"\x00controls\x00":                            "internal modal sentinel",
	"%d":                                          "numeric format",
	"%s/%d":                                       "identifier and revision format",
	"%s:%d":                                       "endpoint format",
	"\\n":                                         "inert control representation",
	"\\r":                                         "inert control representation",
	"\\t":                                         "inert control representation",
	"\\u":                                         "inert Unicode representation",
	"\\x":                                         "inert byte representation",
	"back":                                        "action identifier",
	"cancel":                                      "action identifier",
	"cancel_warning":                              "action identifier",
	"confirm":                                     "action identifier",
	"connect":                                     "action identifier",
	"connect_confirmation":                        "modal identifier",
	"ctrl+c":                                      "key identifier",
	"ctrl+r":                                      "key identifier",
	"ctrl+s":                                      "key identifier",
	"delete":                                      "action identifier",
	"delete_connection":                           "modal identifier",
	"delete_folder":                               "modal identifier",
	"discard":                                     "action identifier",
	"down":                                        "action identifier",
	"edit":                                        "action identifier",
	"end":                                         "action identifier",
	"enter":                                       "key identifier",
	"esc":                                         "key identifier",
	"f1":                                          "key identifier",
	"f2":                                          "key identifier",
	"focus_details":                               "action identifier",
	"focus_tree":                                  "action identifier",
	"folder_create":                               "modal identifier",
	"folder_edit":                                 "modal identifier",
	"g":                                           "key identifier",
	"h":                                           "key identifier",
	"help":                                        "action or modal identifier",
	"home":                                        "action identifier",
	"j":                                           "key identifier",
	"k":                                           "key identifier",
	"l":                                           "key identifier",
	"left":                                        "action or input alignment identifier",
	"move":                                        "action identifier",
	"move_here":                                   "action identifier",
	"move_picker":                                 "modal identifier",
	"new_connection":                              "action identifier",
	"new_folder":                                  "action identifier",
	"next_field":                                  "action identifier",
	"once":                                        "trust decision input token",
	"operation_error":                             "modal identifier",
	"p":                                           "key identifier",
	"persist":                                     "trust decision input token",
	"previous_field":                              "action identifier",
	"quit":                                        "action identifier",
	"reload":                                      "action identifier",
	"retry":                                       "action identifier",
	"right":                                       "action or input alignment identifier",
	"root":                                        "detail-kind identifier",
	"s":                                           "key identifier",
	"save":                                        "action identifier",
	"shift+tab":                                   "key identifier",
	"space":                                       "key identifier",
	"ssh_failure":                                 "modal identifier",
	"tab":                                         "key identifier",
	"toggle":                                      "action identifier",
	"unsaved_changes":                             "modal identifier",
	"up":                                          "action identifier",
	"y":                                           "key or trust decision input token",
}

// These labels contain no natural-language shape, so the source scanner cannot
// discover them by letters. Keeping the exact rendered marker inventory here
// makes symbolic browser copy participate in manifest equality.
var sourceSymbolicCopyLiterals = map[string]struct{}{
	"!":    {},
	"*":    {},
	"/":    {},
	"?":    {},
	">":    {},
	"> ":   {},
	"[ ] ": {},
	"[*] ": {},
	"[/]":  {},
	"[-]":  {},
	"[+]":  {},
	"…":    {},
}

var sourceGeneratedRenderLiterals = map[string]struct{}{
	"b": {}, "c": {}, "d": {}, "e": {}, "f": {}, "m": {}, "n": {}, "q": {}, "r": {}, "x": {},
}

func TestControlledEnglishManifest(t *testing.T) {
	manifestTerms, manifestCopy := readControlledEnglishManifest(t)
	assertStringMapEqual(t, "terms", manifestTerms, controlledTerms)

	canonicalOwner := make(map[string]string, len(manifestTerms))
	for concept, term := range manifestTerms {
		if previous, duplicate := canonicalOwner[term]; duplicate {
			t.Errorf("canonical term %q is assigned to duplicate concepts %q and %q", term, previous, concept)
		}
		canonicalOwner[term] = concept
	}
	if len(manifestCopy) == 0 {
		t.Error("controlled copy manifest is empty")
	}
}

func TestControlledEnglishSourceClassification(t *testing.T) {
	_, classified := readControlledEnglishManifest(t)
	actual := make(map[string]struct{})
	for literal, positions := range productionStringLiterals(t) {
		if !looksLikeControlledLanguage(literal) {
			continue
		}
		if _, technical := sourceTechnicalLiterals[literal]; technical {
			continue
		}
		actual[literal] = struct{}{}
		if _, ok := classified[literal]; !ok {
			t.Errorf("unclassified controlled literal %q at %s", literal, strings.Join(positions, ", "))
		}
	}
	for literal := range sourceSymbolicCopyLiterals {
		actual[literal] = struct{}{}
		if _, ok := classified[literal]; !ok {
			t.Errorf("unclassified symbolic render literal %q", literal)
		}
	}
	for literal := range sourceGeneratedRenderLiterals {
		actual[literal] = struct{}{}
		if _, ok := classified[literal]; !ok {
			t.Errorf("unclassified generated render literal %q", literal)
		}
	}
	for _, literal := range controlledSSHPresentationCorpus() {
		actual[literal] = struct{}{}
		if _, ok := classified[literal]; !ok {
			t.Errorf("unclassified controlled SSH presentation literal %q", literal)
		}
	}
	for literal := range classified {
		if _, ok := actual[literal]; !ok {
			t.Errorf("manifest copy is missing from production source: %q", literal)
		}
	}
	assertStringSetEqual(t, "controlled copy", actual, classified)
}

func TestControlledEnglishDynamicByteIdentityAcrossSizes(t *testing.T) {
	corpus := []string{
		"ASCII-value",
		"e\u0301-combining",
		"界-CJK",
		"👩\u200d💻-ZWJ",
		"line\r\nnext",
		"\x1b[31mANSI\x1b[0m",
		"C0\x00-C1\u0085",
		"bidi\u202evalue",
		string([]byte{'i', 'n', 'v', 0xff, 0xfe}),
		strings.Repeat("oversized-界", 200),
	}
	for index, value := range corpus {
		folder := app.Folder{Node: app.Node{
			ID: app.NodeID("language-folder-" + strconv.Itoa(index)), ParentID: syntheticRootID,
			Kind: app.NodeKindFolder, Name: value, Path: "/" + value, Revision: 1,
		}}
		connection := app.Connection{
			Node: app.Node{ID: app.NodeID("language-byte-" + strconv.Itoa(index)), ParentID: folder.ID, Kind: app.NodeKindConnection, Name: value, Path: folder.Path + "/" + value, Revision: 1},
			Host: value, Port: 22, Username: value, AuthMethod: app.AuthMethodAgent,
		}
		snapshot := newCatalogSnapshot(rootFolder(), 1)
		if !snapshot.addChildren(snapshot.rootID, app.ListChildrenResult{Folders: []app.Folder{folder}}) ||
			!snapshot.addChildren(folder.ID, app.ListChildrenResult{Connections: []app.Connection{connection}}) {
			t.Fatalf("case %d: failed to build snapshot", index)
		}
		model := New(Config{Width: 80, Height: 24, NoColor: true})
		model.browser.setSnapshot(snapshot, connection.ID)
		model.ownedSelectionID = connection.ID
		model.syncDetail()
		for _, size := range us5ContractSizes {
			updateModel(model, tea.WindowSizeMsg{Width: size.width, Height: size.height})
			_ = model.View().Content
			storedFolder := model.browser.snapshot.nodes[folder.ID]
			stored := model.browser.snapshot.nodes[connection.ID]
			if storedFolder.node.Name != folder.Name || storedFolder.node.Path != folder.Path || storedFolder.folder == nil ||
				stored.node.Name != connection.Name || stored.node.Path != connection.Path || stored.connection == nil || stored.connection.Host != connection.Host || stored.connection.Username != connection.Username {
				t.Fatalf("case %d %s: render changed dynamic bytes", index, size.name)
			}
		}
	}
}

func controlledSSHPresentationCorpus() []string {
	reasons := []app.SSHFailureReason{
		app.SSHFailureTimeout, app.SSHFailureAuthenticationDenied, app.SSHFailureConnectionRefused,
		app.SSHFailureHostNotFound, app.SSHFailureNetworkUnreachable, app.SSHFailureHostTrust,
		app.SSHFailureCredentialUnavailable, app.SSHFailureSSHNegotiation, app.SSHFailureCanceled, app.SSHFailureUnexpected,
	}
	stages := []app.SSHFailureStage{
		app.SSHFailureStageTargetResolution, app.SSHFailureStageNetworkConnection, app.SSHFailureStageHostTrust,
		app.SSHFailureStageSSHNegotiation, app.SSHFailureStageCredential, app.SSHFailureStageAuthentication,
		app.SSHFailureStageSessionSetup, app.SSHFailureStageLocalTerminal, app.SSHFailureStageUnknown,
	}
	details := map[app.SSHFailureReason][]string{
		app.SSHFailureTimeout:               {"operation timed out"},
		app.SSHFailureAuthenticationDenied:  {"server rejected available authentication methods"},
		app.SSHFailureConnectionRefused:     {"remote endpoint refused connection"},
		app.SSHFailureHostNotFound:          {"host name could not be resolved"},
		app.SSHFailureNetworkUnreachable:    {"network is unreachable"},
		app.SSHFailureHostTrust:             {"unknown", "changed", "revoked", "rejected"},
		app.SSHFailureCredentialUnavailable: {"agent", "secure store", "identity file", "secret prompt"},
		app.SSHFailureSSHNegotiation:        {"handshake", "session", "pty", "shell"},
		app.SSHFailureCanceled:              {"operation canceled"},
	}
	values := make([]string, 0, len(reasons)*2+len(stages)+24)
	for _, reason := range reasons {
		values = append(values, string(reason))
		for _, detail := range append([]string(nil), details[reason]...) {
			presentation := app.NewSSHStartError(reason, controlledSSHStage(reason), detail, nil).Presentation()
			values = append(values, presentation.Summary, presentation.Recommendation, presentation.TechnicalDetail)
		}
		if len(details[reason]) == 0 {
			presentation := app.NewSSHStartError(reason, controlledSSHStage(reason), "", nil).Presentation()
			values = append(values, presentation.Summary, presentation.Recommendation)
		}
	}
	for _, stage := range stages {
		values = append(values, string(stage))
	}
	return append(values, string(app.AuthMethodAgent), string(app.AuthMethodKey), string(app.AuthMethodPassword))
}

func controlledSSHStage(reason app.SSHFailureReason) app.SSHFailureStage {
	switch reason {
	case app.SSHFailureAuthenticationDenied:
		return app.SSHFailureStageAuthentication
	case app.SSHFailureConnectionRefused, app.SSHFailureNetworkUnreachable, app.SSHFailureTimeout:
		return app.SSHFailureStageNetworkConnection
	case app.SSHFailureHostNotFound:
		return app.SSHFailureStageTargetResolution
	case app.SSHFailureHostTrust:
		return app.SSHFailureStageHostTrust
	case app.SSHFailureCredentialUnavailable:
		return app.SSHFailureStageCredential
	case app.SSHFailureSSHNegotiation:
		return app.SSHFailureStageSSHNegotiation
	case app.SSHFailureCanceled:
		return app.SSHFailureStageLocalTerminal
	default:
		return app.SSHFailureStageUnknown
	}
}

func TestControlledEnglishCanonicalTermsAndDynamicValues(t *testing.T) {
	all := productionStringLiterals(t)
	for concept, canonical := range controlledTerms {
		found := false
		for literal := range all {
			if strings.Contains(literal, canonical) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("canonical term %q (%s) is absent from controlled source copy", canonical, concept)
		}
	}
	assertCanonicalActionLabels(t)

	dynamicValues := map[string]string{
		"catalog.name":     "USER-NAME-VALUE",
		"catalog.path":     "/USER/PATH/VALUE",
		"catalog.host":     "USER-HOST-VALUE",
		"catalog.username": "USER-LOGIN-VALUE",
	}
	connection := app.Connection{
		Node: app.Node{ID: "language-connection", ParentID: syntheticRootID, Kind: app.NodeKindConnection, Name: dynamicValues["catalog.name"], Path: dynamicValues["catalog.path"], Revision: 1},
		Host: dynamicValues["catalog.host"], Port: 22, Username: dynamicValues["catalog.username"], AuthMethod: app.AuthMethodAgent,
	}
	detailSnapshot := newCatalogSnapshot(rootFolder(), 1)
	if !detailSnapshot.addChildren(detailSnapshot.rootID, app.ListChildrenResult{Connections: []app.Connection{connection}}) {
		t.Fatal("failed to construct language detail snapshot")
	}
	detail, ok := newDetailState(detailSnapshot, connection.ID)
	if !ok {
		t.Fatal("failed to construct language detail")
	}
	rendered := strings.Join(detail.content(200), "\n")
	presentation := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", nil).Presentation()
	failure := newSSHFailureModal(app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}, presentation)
	rendered += "\n" + strings.Join(sshFailureLines(failure, 200), "\n")
	for surface, allowed := range dynamicValues {
		if !strings.Contains(rendered, allowed) {
			t.Errorf("rendered corpus did not preserve allowlisted dynamic %s value %q", surface, allowed)
		}
	}
}

func assertCanonicalActionLabels(t *testing.T) {
	t.Helper()
	expected := map[actionID]string{
		actionBack:          controlledTerms["action.back"],
		actionCancel:        controlledTerms["action.cancel"],
		actionConnect:       controlledTerms["action.connect"],
		actionDelete:        controlledTerms["action.delete"],
		actionDiscard:       controlledTerms["action.discard"],
		actionEdit:          controlledTerms["action.edit"],
		actionHelp:          controlledTerms["action.help"],
		actionMove:          controlledTerms["action.move"],
		actionNewConnection: controlledTerms["action.new_connection"],
		actionNewFolder:     controlledTerms["action.new_folder"],
		actionQuit:          controlledTerms["action.quit"],
		actionReload:        controlledTerms["action.reload"],
		actionRetry:         controlledTerms["action.retry"],
		actionSave:          controlledTerms["action.save"],
	}
	inventories := [][]actionDescriptor{
		rootActionDescriptors, folderActionDescriptors, connectionActionDescriptors,
		operationActionDescriptors, conflictActionDescriptors, connectionFormActionDescriptors,
	}
	seen := make(map[actionID]bool)
	for _, inventory := range inventories {
		for _, descriptor := range inventory {
			canonical, controlled := expected[descriptor.id]
			if !controlled {
				continue
			}
			seen[descriptor.id] = true
			if descriptor.label != canonical {
				t.Errorf("action concept %q uses inconsistent term %q, want %q", descriptor.id, descriptor.label, canonical)
			}
		}
	}
	for id := range expected {
		if id == actionRetry || id == actionDiscard {
			continue // Modal-local controls use these canonical terms outside the object descriptor slices.
		}
		if !seen[id] {
			t.Errorf("canonical action concept %q has no descriptor", id)
		}
	}
}

func readControlledEnglishManifest(t *testing.T) (map[string]string, map[string]struct{}) {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", "controlled_english.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	terms := make(map[string]string)
	copySet := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		switch fields[0] {
		case "term":
			if len(fields) != 3 || fields[1] == "" || fields[2] == "" {
				t.Fatalf("manifest line %d: term requires concept and canonical text", lineNumber)
			}
			if _, duplicate := terms[fields[1]]; duplicate {
				t.Fatalf("manifest line %d: duplicate concept %q", lineNumber, fields[1])
			}
			terms[fields[1]] = fields[2]
		case "copy":
			if len(fields) != 2 {
				t.Fatalf("manifest line %d: copy requires one quoted Go string", lineNumber)
			}
			value, unquoteErr := strconv.Unquote(fields[1])
			if unquoteErr != nil {
				t.Fatalf("manifest line %d: invalid quoted copy: %v", lineNumber, unquoteErr)
			}
			if _, duplicate := copySet[value]; duplicate {
				t.Fatalf("manifest line %d: duplicate copy %q", lineNumber, value)
			}
			copySet[value] = struct{}{}
		default:
			t.Fatalf("manifest line %d: unknown record type %q", lineNumber, fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return terms, copySet
}

func productionStringLiterals(t *testing.T) map[string][]string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	set := token.NewFileSet()
	literals := make(map[string][]string)
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(set, path, nil, 0)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		imports := make(map[token.Pos]struct{}, len(file.Imports))
		for _, spec := range file.Imports {
			imports[spec.Path.Pos()] = struct{}{}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			if _, isImport := imports[literal.Pos()]; isImport {
				return true
			}
			value, unquoteErr := strconv.Unquote(literal.Value)
			if unquoteErr != nil {
				t.Fatalf("%s: %v", set.Position(literal.Pos()), unquoteErr)
			}
			position := set.Position(literal.Pos())
			literals[value] = append(literals[value], position.String())
			return true
		})
	}
	return literals
}

func looksLikeControlledLanguage(value string) bool {
	hasLetter := false
	for _, r := range value {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
	}
	return hasLetter
}

func assertStringMapEqual(t *testing.T, name string, got, want map[string]string) {
	t.Helper()
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s[%q] = %q, want %q", name, key, got[key], value)
		}
	}
	for key := range got {
		if _, ok := want[key]; !ok {
			t.Errorf("extra %s entry %q", name, key)
		}
	}
}

func assertStringSetEqual(t *testing.T, name string, got, want map[string]struct{}) {
	t.Helper()
	for value := range want {
		if _, ok := got[value]; !ok {
			t.Errorf("%s missing %q", name, value)
		}
	}
	for value := range got {
		if _, ok := want[value]; !ok {
			t.Errorf("%s has extra %q", name, value)
		}
	}
}
