package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/app"
)

func TestForgetHostKeyConnectionWorkflow(t *testing.T) {
	connection := testConnection("connection", syntheticRootID, "/production", 3)
	connection.Host, connection.Port = "prod.example", 2202
	trusted := app.TrustedHost{HostEndpoint: app.HostEndpoint{CanonicalHost: "prod.example", Port: 2202}, Revision: 7}
	var scopeCalls, forgetCalls int
	model := loadedModel(t, nil, true)
	model.hostTrust = HostTrustFuncs{
		ForgetScopeFunc: func(_ context.Context, selector app.ItemSelector) (app.ForgetHostKeyScope, error) {
			scopeCalls++
			if selector.ID != connection.ID {
				t.Fatalf("scope selector = %#v", selector)
			}
			return app.ForgetHostKeyScope{Endpoint: trusted.HostEndpoint, TrustedHost: &trusted}, nil
		},
		ForgetFunc: func(_ context.Context, _ app.ForgetHostKeyRequest) (app.ForgetHostKeyResult, error) {
			forgetCalls++
			return app.ForgetHostKeyResult{Endpoint: trusted.HostEndpoint, Forgotten: true}, nil
		},
	}
	model.browser.setConnectionsPending(app.ListConnectionsResult{Connections: []app.Connection{connection}, CatalogRevision: 1}, connection.ID)
	model.ownedSelectionID = connection.ID

	_, command := model.Update(keyPress("x"))
	if command == nil || scopeCalls != 0 || model.modal.isOpen() {
		t.Fatal("forget action did not start a scoped operation")
	}
	updateModel(model, command())
	view := model.View().Content
	for _, want := range []string{"Forget app-owned host key?", "Host prod.example:2202", "Scope app-owned trust only", "future connections may prompt", "y Confirm", "Enter/Esc Cancel"} {
		if !renderedTextContains(view, want) {
			t.Fatalf("confirmation omitted %q:\n%s", want, view)
		}
	}

	updateModel(model, keyPress("enter"))
	if forgetCalls != 0 || model.modal.isOpen() {
		t.Fatal("Enter did not cancel the forget confirmation")
	}

	_, command = model.Update(keyPress("x"))
	updateModel(model, command())
	_, command = model.Update(keyPress("y"))
	if command == nil || forgetCalls != 0 {
		t.Fatal("Y did not start the captured trust deletion")
	}
	updateModel(model, command())
	if forgetCalls != 1 || !strings.Contains(model.View().Content, "App-owned host key forgotten") {
		t.Fatalf("completed forget result = calls %d:\n%s", forgetCalls, model.View().Content)
	}
}

func TestForgetHostKeyScopeWithoutTrustMakesNoChange(t *testing.T) {
	connection := testConnection("connection", syntheticRootID, "/production", 3)
	connection.Host, connection.Port = "prod.example", 2202
	forgetCalls := 0
	model := loadedModel(t, nil, true)
	model.hostTrust = HostTrustFuncs{
		ForgetScopeFunc: func(context.Context, app.ItemSelector) (app.ForgetHostKeyScope, error) {
			return app.ForgetHostKeyScope{Endpoint: app.HostEndpoint{CanonicalHost: connection.Host, Port: connection.Port}}, nil
		},
		ForgetFunc: func(context.Context, app.ForgetHostKeyRequest) (app.ForgetHostKeyResult, error) {
			forgetCalls++
			return app.ForgetHostKeyResult{}, nil
		},
	}
	model.browser.setConnectionsPending(app.ListConnectionsResult{Connections: []app.Connection{connection}, CatalogRevision: 1}, connection.ID)
	model.ownedSelectionID = connection.ID

	_, command := model.Update(keyPress("x"))
	updateModel(model, command())
	view := model.View().Content
	if !renderedTextContains(view, "No app-owned host key to forget") || !renderedTextContains(view, "Host prod.example:2202") {
		t.Fatalf("missing no-change explanation:\n%s", view)
	}
	updateModel(model, keyPress("y"))
	if forgetCalls != 0 {
		t.Fatal("Y changed trust when the scoped record was absent")
	}
	updateModel(model, keyPress("esc"))
	if model.modal.isOpen() {
		t.Fatal("Esc did not return to the connection screen")
	}
}
