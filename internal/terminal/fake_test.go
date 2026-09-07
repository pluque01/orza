package terminal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestFakeRawRestoreAndCalls(t *testing.T) {
	t.Parallel()
	fake := NewFake(Size{Columns: 80, Rows: 24})
	ctx := context.Background()

	state, err := fake.Capture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := fake.MakeRaw(ctx); err != nil {
		t.Fatal(err)
	}
	if !fake.CurrentState().Raw() {
		t.Fatal("MakeRaw did not set raw mode")
	}
	if err := fake.Restore(ctx, state); err != nil {
		t.Fatal(err)
	}
	if fake.CurrentState().Raw() {
		t.Fatal("Restore did not restore captured mode")
	}

	calls := fake.Calls()
	want := []Operation{OperationCapture, OperationMakeRaw, OperationRestore}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v", calls)
	}
	for i := range want {
		if calls[i].Operation != want[i] {
			t.Fatalf("call %d = %q, want %q", i, calls[i].Operation, want[i])
		}
	}
}

func TestFakeSecretPromptCopiesAndDoesNotRecordSecret(t *testing.T) {
	t.Parallel()
	fake := NewFake(Size{})
	queued := []byte("prompt-canary")
	fake.QueueSecret(queued, nil)
	queued[0] = 'X'

	secret, err := fake.ReadSecret(context.Background(), SecretPrompt{Message: "Password: "})
	if err != nil {
		t.Fatal(err)
	}
	if string(secret) != "prompt-canary" {
		t.Fatalf("secret = %q", secret)
	}
	if formatted := fmt.Sprintf("%#v", fake); strings.Contains(formatted, "prompt-canary") {
		t.Fatalf("formatted fake exposed secret: %s", formatted)
	}
	if formatted := fmt.Sprintf("%#v", fake.Calls()); strings.Contains(formatted, "prompt-canary") {
		t.Fatalf("calls exposed secret: %s", formatted)
	}
}

func TestFakeResizeSequenceIsDeterministic(t *testing.T) {
	t.Parallel()
	fake := NewFake(Size{Columns: 80, Rows: 24})
	want := []Size{{Columns: 100, Rows: 30}, {Columns: 120, Rows: 40}}
	fake.QueueResize(want...)

	events, err := fake.ResizeEvents(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got []Size
	for size := range events {
		got = append(got, size)
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	size, err := fake.Size(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if size != want[len(want)-1] {
		t.Fatalf("size = %v, want %v", size, want[len(want)-1])
	}
}

func TestFakeOperationFaultDoesNotMutateState(t *testing.T) {
	t.Parallel()
	fake := NewFake(Size{})
	want := errors.New("raw mode failed")
	fake.SetFault(OperationMakeRaw, want)

	if err := fake.MakeRaw(context.Background()); !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if fake.CurrentState().Raw() {
		t.Fatal("failed MakeRaw mutated state")
	}
}

func TestFakeInjectsVTCapability(t *testing.T) {
	t.Parallel()
	fake := NewFake(Size{})
	want := VTCapability{Status: VTMissing, SafeFallback: true}
	fake.SetVTCapability(want)

	got, err := fake.VTCapability(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("VTCapability() = %+v, want %+v", got, want)
	}
}
