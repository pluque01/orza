package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
)

func TestWriteSuccessJSONEnvelope(t *testing.T) {
	var output bytes.Buffer
	revision := app.CatalogRevision(42)

	err := WriteSuccess(&output, true, map[string]string{"path": "/prod"}, &revision)
	if err != nil {
		t.Fatalf("WriteSuccess() error = %v", err)
	}
	want := "{\"ok\":true,\"data\":{\"path\":\"/prod\"},\"catalogRevision\":42}\n"
	if got := output.String(); got != want {
		t.Fatalf("WriteSuccess() = %q, want %q", got, want)
	}
}

func TestWriteStartupErrorContract(t *testing.T) {
	diagnostic := app.NewSSHStartError(app.SSHFailureTimeout, app.SSHFailureStageNetworkConnection, "operation timed out", errors.New("password=canary")).Presentation()
	err := newStartupError(CodeTransport, "Connection timed out", "/prod", "prod.example:22", diagnostic, errors.New("password=canary"))

	t.Run("human", func(t *testing.T) {
		var output bytes.Buffer
		started := time.Now()
		if writeErr := WriteError(&output, false, err); writeErr != nil {
			t.Fatal(writeErr)
		}
		if time.Since(started) >= time.Second {
			t.Fatal("diagnostic presentation took one second or longer")
		}
		want := "error: SSH session could not start\nsummary: Operation timed out\ntarget: /prod\nendpoint: prod.example:22\ncategory: timeout\nstage: network_connection\nrecommendation: Check connectivity and timeout settings, then retry.\ndetail: operation timed out\n"
		if got := output.String(); got != want {
			t.Fatalf("WriteError() = %q, want %q", got, want)
		}
		if strings.Contains(output.String(), "canary") {
			t.Fatalf("cause leaked: %q", output.String())
		}
	})

	t.Run("json", func(t *testing.T) {
		var output bytes.Buffer
		if writeErr := WriteError(&output, true, err); writeErr != nil {
			t.Fatal(writeErr)
		}
		got := output.String()
		decoder := json.NewDecoder(strings.NewReader(got))
		var object json.RawMessage
		if decodeErr := decoder.Decode(&object); decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if decodeErr := decoder.Decode(&object); !errors.Is(decodeErr, io.EOF) {
			t.Fatalf("stderr contained more than one JSON value: %q", got)
		}
		want := "{\"ok\":false,\"error\":{\"code\":\"transport_failure\",\"message\":\"Connection timed out\",\"target\":\"/prod\",\"endpoint\":\"prod.example:22\",\"category\":\"timeout\",\"stage\":\"network_connection\",\"recommendation\":\"Check connectivity and timeout settings, then retry.\",\"technicalDetail\":\"operation timed out\"}}\n"
		if got != want {
			t.Fatalf("WriteError() = %q, want %q", got, want)
		}
		if strings.Contains(got, "canary") {
			t.Fatalf("cause leaked: %q", got)
		}
	})
}

func TestWriteStartupErrorOmitsUnavailableOptionalFields(t *testing.T) {
	diagnostic := app.NewSSHStartError(app.SSHFailureUnexpected, app.SSHFailureStageUnknown, "password=canary", nil).Presentation()
	err := newStartupError(CodeTransport, "SSH startup failed", "/prod", "", diagnostic, nil)
	var output bytes.Buffer
	if writeErr := WriteError(&output, true, err); writeErr != nil {
		t.Fatal(writeErr)
	}
	if strings.Contains(output.String(), "endpoint") || strings.Contains(output.String(), "technicalDetail") || strings.Contains(output.String(), "canary") {
		t.Fatalf("optional or unsafe field was emitted: %q", output.String())
	}
}

func TestWriteSuccessHumanOutput(t *testing.T) {
	var output bytes.Buffer
	if err := WriteSuccess(&output, false, "created /prod", nil); err != nil {
		t.Fatalf("WriteSuccess() error = %v", err)
	}
	if got, want := output.String(), "created /prod\n"; got != want {
		t.Fatalf("WriteSuccess() = %q, want %q", got, want)
	}
}

func TestWriteErrorRedactsCause(t *testing.T) {
	const secret = "super-secret-password"
	err := NewError(CodeConflict, "the connection changed; reload before retrying", "/prod", errors.New(secret))

	for _, jsonOutput := range []bool{false, true} {
		var output bytes.Buffer
		if writeErr := WriteError(&output, jsonOutput, err); writeErr != nil {
			t.Fatalf("WriteError(json=%v) error = %v", jsonOutput, writeErr)
		}
		if strings.Contains(output.String(), secret) {
			t.Fatalf("WriteError(json=%v) leaked cause: %q", jsonOutput, output.String())
		}
	}
}

func TestWriteUnknownErrorIsGeneric(t *testing.T) {
	var output bytes.Buffer
	if err := WriteError(&output, true, errors.New("password=hunter2")); err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}
	want := "{\"ok\":false,\"error\":{\"code\":\"internal\",\"message\":\"operation failed\"}}\n"
	if got := output.String(); got != want {
		t.Fatalf("WriteError() = %q, want %q", got, want)
	}
}
