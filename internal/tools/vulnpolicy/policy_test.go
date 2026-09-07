package vulnpolicy

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var policyDate = time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)

func TestActiveExpiredStaleUnknownMalformedScannerErrorMatrix(t *testing.T) {
	tests := []struct {
		name       string
		scanner    string
		exceptions string
		wantError  bool
		wantOutput string
	}{
		{"NoFindings", "no-findings.jsonl", "empty.json", false, ""},
		{"Reachable", "reachable.jsonl", "empty.json", true, "reachable vulnerability GO-2026-5972"},
		{"Active", "reachable.jsonl", "active.json", false, "warning: active vulnerability exception GO-2026-5972"},
		{"Expired", "reachable.jsonl", "expired.json", true, "is expired"},
		{"Stale", "no-findings.jsonl", "active.json", true, "stale vulnerability exception for GO-2026-5972"},
		{"Duplicate", "reachable.jsonl", "duplicate.json", true, "duplicates an id"},
		{"MalformedException", "reachable.jsonl", "malformed.json", true, "file is malformed"},
		{"Unreachable", "unreachable.jsonl", "empty.json", false, "info: unreachable vulnerability GO-2026-6354"},
		{"ModuleOnly", "module-only.jsonl", "empty.json", false, "info: unreachable vulnerability GO-2026-5932"},
		{"ScannerError", "scanner-error.jsonl", "empty.json", true, "unknown event type"},
		{"ProtocolError", "malformed-stream.jsonl", "empty.json", true, "malformed JSON"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := Evaluate(openFixture(t, test.scanner), openFixture(t, test.exceptions), policyDate, &output)
			if (err != nil) != test.wantError {
				t.Fatalf("Evaluate() error = %v, wantError %v; output: %s", err, test.wantError, output.String())
			}
			if !strings.Contains(output.String(), test.wantOutput) {
				t.Fatalf("output %q does not contain %q", output.String(), test.wantOutput)
			}
		})
	}
}

func TestExactReachableIDMatching(t *testing.T) {
	exceptions := strings.ReplaceAll(string(readFixture(t, "active.json")), "GO-2026-5972", "GO-2026-5973")
	var output bytes.Buffer
	err := Evaluate(openFixture(t, "reachable.jsonl"), strings.NewReader(exceptions), policyDate, &output)
	if err == nil || !strings.Contains(output.String(), "GO-2026-5972 has no active exception") {
		t.Fatalf("expected exact-match failure, got error %v and output %q", err, output.String())
	}
}

func TestPrettyPrintedScannerStream(t *testing.T) {
	stream := "{\n  \"config\": {\n    \"protocol_version\": \"v1.0.0\"\n  }\n}\n{\n  \"SBOM\": {\n    \"go_version\": \"go1.26.5\"\n  }\n}\n{\n  \"progress\": {\n    \"message\": \"synthetic\"\n  }\n}\n"
	var output bytes.Buffer
	if err := Evaluate(strings.NewReader(stream), openFixture(t, "empty.json"), policyDate, &output); err != nil {
		t.Fatalf("Evaluate() rejected a valid pretty-printed stream: %v; output: %s", err, output.String())
	}
}

func TestDiagnosticsExcludeRationaleAndAreBounded(t *testing.T) {
	exceptions := strings.ReplaceAll(string(readFixture(t, "active.json")), "locked toolchain", "SYNTHETIC_SECRET_VALUE")
	var output bytes.Buffer
	if err := Evaluate(openFixture(t, "reachable.jsonl"), strings.NewReader(exceptions), policyDate, &output); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if strings.Contains(output.String(), "SYNTHETIC_SECRET_VALUE") {
		t.Fatal("diagnostic exposed rationale")
	}

	var stream strings.Builder
	stream.WriteString("{\"config\":{\"protocol_version\":\"v1.0.0\"}}\n")
	for index := 0; index < 150; index++ {
		stream.WriteString(fmtFinding(index))
	}
	output.Reset()
	if err := Evaluate(strings.NewReader(stream.String()), openFixture(t, "empty.json"), policyDate, &output); err == nil {
		t.Fatal("expected reachable findings to fail")
	}
	if lines := strings.Count(output.String(), "\n"); lines > maxDiagnosticLines {
		t.Fatalf("diagnostic has %d lines, limit is %d", lines, maxDiagnosticLines)
	}
}

func fmtFinding(index int) string {
	return "{\"finding\":{\"osv\":\"GO-2026-" + fourDigits(index) + "\",\"trace\":[{\"function\":\"synthetic\"}]}}\n"
}

func fourDigits(value int) string {
	digits := []byte{'0', '0', '0', '0'}
	for index := len(digits) - 1; index >= 0; index-- {
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits)
}

func openFixture(t *testing.T, name string) *os.File {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
