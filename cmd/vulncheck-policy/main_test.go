package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	exceptions := filepath.Join("..", "..", "internal", "tools", "vulnpolicy", "testdata", "empty.json")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-exceptions", exceptions, "-date", "2026-09-07"}, strings.NewReader("{\"config\":{}}\n"), &stdout, &stderr, func() time.Time {
		return time.Time{}
	})
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, stderr.String())
	}
}

func TestRunRejectsInvalidDate(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-date", "not-a-date"}, strings.NewReader(""), &stdout, &stderr, time.Now); code != 2 {
		t.Fatalf("run() = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "YYYY-MM-DD") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
