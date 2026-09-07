package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/terminal"
)

func TestRequiredVTCapabilityBoundary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		capability terminal.VTCapability
		wantErr    bool
	}{
		{name: "supported", capability: terminal.VTCapability{Status: terminal.VTSupported}},
		{name: "unverified runtime query", capability: terminal.VTCapability{Status: terminal.VTUnverified}},
		{name: "missing with safe fallback", capability: terminal.VTCapability{Status: terminal.VTMissing, SafeFallback: true}},
		{name: "missing without safe fallback", capability: terminal.VTCapability{Status: terminal.VTMissing}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := requireVT(test.capability)
			if (err != nil) != test.wantErr {
				t.Fatalf("requireVT() error = %v, want error %t", err, test.wantErr)
			}
			if test.wantErr && !errors.Is(err, terminal.ErrRequiredVT) {
				t.Fatalf("requireVT() error = %v, want %v", err, terminal.ErrRequiredVT)
			}
		})
	}
}

func TestRunMissingRequiredVTRestoresBeforeActionableFailure(t *testing.T) {
	t.Parallel()
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.SetVTCapability(terminal.VTCapability{Status: terminal.VTMissing})

	_, err := Run(context.Background(), Config{Terminal: local, Width: 80, Height: 24})
	if !errors.Is(err, terminal.ErrRequiredVT) || !strings.Contains(err.Error(), "VT-capable terminal") {
		t.Fatalf("Run() error = %v, want actionable required-VT failure", err)
	}
	want := []terminal.Operation{
		terminal.OperationInteractive,
		terminal.OperationCapture,
		terminal.OperationVTCapability,
		terminal.OperationRestore,
	}
	calls := local.Calls()
	if len(calls) != len(want) {
		t.Fatalf("terminal calls = %v, want %v", calls, want)
	}
	for i := range want {
		if calls[i].Operation != want[i] {
			t.Fatalf("terminal call %d = %q, want %q", i, calls[i].Operation, want[i])
		}
	}
}
