//go:build !race

package sshclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"hash"
	"runtime"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
)

const (
	sc015StreamBytes    = 64 << 20
	sc015InterruptBytes = sc015StreamBytes / 2
	sc015RetainedLimit  = 1 << 20
)

func TestSC015DirectStreamAndRetentionMatrix20Runs(t *testing.T) {
	for _, interrupted := range []bool{false, true} {
		runSC015StreamCase(t, interrupted) // Warm-up is outside the measured runs.
	}
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	for run := range 20 {
		for _, test := range []struct {
			name        string
			interrupted bool
		}{
			{name: "success"},
			{name: "network-interruption", interrupted: true},
		} {
			t.Run(test.name, func(t *testing.T) {
				stdoutBytes, stderrBytes := runSC015StreamCase(t, test.interrupted)
				t.Logf("run %02d: stdout=%d stderr=%d", run+1, stdoutBytes, stderrBytes)
			})
		}
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	retained := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	if retained < 0 {
		retained = 0
	}
	t.Logf("retained heap growth after 20 success and 20 interrupted 128 MiB streams: %d bytes", retained)
	if retained >= sc015RetainedLimit {
		t.Fatalf("retained heap growth = %d bytes, want < %d", retained, sc015RetainedLimit)
	}
}

func runSC015StreamCase(t *testing.T, interrupted bool) (int64, int64) {
	t.Helper()
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueResize()
	stdout := newSC015Writer('O')
	stderr := newSC015Writer('E')
	wantBytes := sc015StreamBytes
	if interrupted {
		wantBytes = sc015InterruptBytes
	}
	var wantStdoutDigest, wantStderrDigest []byte
	remote := &fakeSession{wait: func() error {
		wantStdoutDigest = writeSC015Stream(t, stdout, 'O', wantBytes)
		wantStderrDigest = writeSC015Stream(t, stderr, 'E', wantBytes)
		if interrupted {
			return errors.New("injected network interruption")
		}
		return nil
	}}
	client := successfulClient(local, &fakeTransport{session: remote}, &fakeConn{})
	client.options.Stdout, client.options.Stderr = stdout, stderr

	result, err := client.Run(context.Background(), testRequest(local))
	if interrupted {
		if err == nil || result.StartedAt.IsZero() || result.Outcome != app.SessionOutcomeTransportFailure {
			t.Fatalf("interrupted Run() = %+v, %v", result, err)
		}
	} else if err != nil || result.State != app.SessionSucceeded || result.Outcome != app.SessionOutcomeSuccess || result.StartedAt.IsZero() {
		t.Fatalf("successful Run() = %+v, %v", result, err)
	}
	if stdout.invalid || stderr.invalid {
		t.Fatal("stream writer received a byte from the wrong remote stream")
	}
	if stdout.bytes != int64(wantBytes) || stderr.bytes != int64(wantBytes) {
		t.Fatalf("stream counts = stdout %d stderr %d, want %d each", stdout.bytes, stderr.bytes, wantBytes)
	}
	for name, check := range map[string]struct {
		writer *sc015Writer
		want   []byte
	}{"stdout": {stdout, wantStdoutDigest}, "stderr": {stderr, wantStderrDigest}} {
		if got := check.writer.digest(); !bytes.Equal(got, check.want) {
			t.Fatalf("%s order-preserving digest = %x, want %x", name, got, check.want)
		}
	}
	if remote.stdout != stdout || remote.stderr != stderr {
		t.Fatal("remote streams were not connected directly to non-retaining terminal writers")
	}
	assertTerminalRestored(t, local)
	return stdout.bytes, stderr.bytes
}

func writeSC015Stream(t *testing.T, writer interface{ Write([]byte) (int, error) }, stream byte, size int) []byte {
	t.Helper()
	chunk := make([]byte, 64<<10)
	digest := sha256.New()
	for offset := 0; offset < size; {
		remaining := size - offset
		length := min(remaining, len(chunk))
		fillSC015Pattern(chunk[:length], stream, int64(offset))
		_, _ = digest.Write(chunk[:length])
		written, err := writer.Write(chunk[:length])
		if err != nil || written != length {
			t.Fatalf("stream write = %d, %v; want %d", written, err, length)
		}
		offset += written
	}
	return digest.Sum(nil)
}

type sc015Writer struct {
	stream  byte
	bytes   int64
	invalid bool
	hash    hash.Hash
}

func newSC015Writer(stream byte) *sc015Writer {
	return &sc015Writer{stream: stream, hash: sha256.New()}
}

func (w *sc015Writer) Write(value []byte) (int, error) {
	for index, current := range value {
		if current != sc015PatternByte(w.stream, w.bytes+int64(index)) {
			w.invalid = true
			break
		}
	}
	_, _ = w.hash.Write(value)
	w.bytes += int64(len(value))
	return len(value), nil
}

func (w *sc015Writer) digest() []byte { return w.hash.Sum(nil) }

func sc015PatternByte(stream byte, offset int64) byte {
	value := uint64(offset) ^ uint64(stream)*0x9e3779b1
	value ^= value >> 17
	value *= 0x85ebca6b
	value ^= value >> 13
	return byte(value ^ value>>8 ^ value>>16 ^ value>>24)
}

func fillSC015Pattern(target []byte, stream byte, offset int64) {
	for index := range target {
		target[index] = sc015PatternByte(stream, offset+int64(index))
	}
}
