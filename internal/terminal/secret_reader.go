package terminal

import (
	"context"
	"errors"
	"io"
	"os"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

var (
	errSecretInput        = errors.New("secret input failed")
	errSecretOutput       = errors.New("secret prompt output failed")
	errSecretReaderCancel = errors.New("secret input cancellation failed")
	errSecretReaderUnsafe = errors.New("secret input cannot guarantee cancellation")
	errSecretReaderClose  = errors.New("secret input cleanup failed")
	errSecretRestore      = errors.New("terminal restoration failed after secret input")
)

type secretCancelReader interface {
	io.ReadCloser
	Cancel() bool
}

type secretEventStreamer interface {
	StreamEvents(context.Context, chan<- uv.Event) error
}

type secretReaderPair struct {
	cancelReader           secretCancelReader
	streamer               secretEventStreamer
	cancellationGuaranteed bool
}

type secretReaderFactory func(io.Reader) (secretReaderPair, error)

func newTerminalSecretReader(reader secretCancelReader) secretReaderPair {
	terminalReader := uv.NewTerminalReader(reader, os.Getenv("TERM"))
	terminalReader.SetLogger(nil)
	return secretReaderPair{cancelReader: reader, streamer: terminalReader, cancellationGuaranteed: true}
}

func prepareSecretReader(input io.Reader, newReader secretReaderFactory) (secretReaderPair, error) {
	pair, err := newReader(input)
	if err != nil || pair.cancelReader == nil || pair.streamer == nil {
		return secretReaderPair{}, errors.Join(errSecretInput, closeIdleSecretReader(pair))
	}
	if !pair.cancellationGuaranteed {
		closeErr := pair.cancelReader.Close()
		if closeErr != nil {
			return secretReaderPair{}, errors.Join(errSecretReaderUnsafe, errSecretReaderClose)
		}
		return secretReaderPair{}, errSecretReaderUnsafe
	}
	return pair, nil
}

func closeIdleSecretReader(pair secretReaderPair) error {
	if pair.cancelReader == nil {
		return nil
	}
	if err := pair.cancelReader.Close(); err != nil {
		return errSecretReaderClose
	}
	return nil
}

func runSecretPrompt(
	ctx context.Context,
	prompt SecretPrompt,
	output io.Writer,
	newline string,
	restore func(context.Context) error,
	pair secretReaderPair,
) ([]byte, error) {
	editor := new(secretEditor)
	defer editor.wipe()

	var resultErr error
	if pair.cancelReader == nil || pair.streamer == nil || !pair.cancellationGuaranteed {
		resultErr = errSecretInput
	}

	writeOK := true
	if resultErr == nil {
		writeOK = writeSecretOutput(output, prompt.Message)
		if !writeOK {
			resultErr = errSecretOutput
		}
	}
	pasteEnabled := false
	if resultErr == nil {
		if err := uv.EncodeBracketedPaste(output, true); err != nil {
			writeOK = false
			resultErr = errSecretOutput
		} else {
			pasteEnabled = true
		}
	}

	var (
		streamCancel context.CancelFunc
		events       chan uv.Event
		streamDone   chan error
		action       secretEditorAction
	)
	if resultErr == nil {
		streamCtx, cancel := context.WithCancel(ctx)
		streamCancel = cancel
		events = make(chan uv.Event, 32)
		streamDone = make(chan error, 1)
		go func() {
			streamDone <- pair.streamer.StreamEvents(streamCtx, events)
		}()

		for action == secretContinue && resultErr == nil {
			select {
			case <-ctx.Done():
				resultErr = ctx.Err()
			case event := <-events:
				action = editor.handle(event)
				if action == secretContinue && !renderSecretEditor(output, prompt.Message, editor) {
					writeOK = false
					resultErr = errSecretOutput
				}
			case err := <-streamDone:
				streamDone = nil
				if err != nil {
					resultErr = errSecretInput
				} else if ctx.Err() != nil {
					resultErr = ctx.Err()
				} else {
					resultErr = errSecretInput
				}
			}
		}
		if action == secretCancel && resultErr == nil {
			resultErr = context.Canceled
		}
		if action == secretQuit && resultErr == nil {
			resultErr = ErrSecretQuit
		}
	}

	var cleanupErr error
	if streamCancel != nil {
		streamCancel()
	}
	if pair.cancelReader != nil {
		if !pair.cancelReader.Cancel() {
			cleanupErr = errors.Join(cleanupErr, errSecretReaderCancel)
		}
	}
	if streamDone != nil {
		for streamDone != nil {
			select {
			case <-events:
			case err := <-streamDone:
				streamDone = nil
				if err != nil && resultErr == nil {
					resultErr = errSecretInput
				}
			}
		}
	}
	if pair.cancelReader != nil {
		if err := pair.cancelReader.Close(); err != nil {
			cleanupErr = errors.Join(cleanupErr, errSecretReaderClose)
		}
	}
	if pasteEnabled {
		if err := uv.EncodeBracketedPaste(output, false); err != nil {
			writeOK = false
			cleanupErr = errors.Join(cleanupErr, errSecretOutput)
		}
	}
	if !writeSecretOutput(output, ansi.ResetStyle+ansi.ShowCursor+newline) {
		writeOK = false
		cleanupErr = errors.Join(cleanupErr, errSecretOutput)
	}
	if err := restore(context.WithoutCancel(ctx)); err != nil {
		cleanupErr = errors.Join(cleanupErr, errSecretRestore)
	}

	if resultErr != nil || cleanupErr != nil || !writeOK || action != secretSubmit {
		return nil, errors.Join(resultErr, cleanupErr)
	}
	return editor.bytes(), nil
}

func renderSecretEditor(output io.Writer, prompt string, editor *secretEditor) bool {
	masked, cursor := editor.render()
	status := ""
	if editor.status != "" {
		status = "  " + editor.status
	}
	line := "\r" + ansi.EraseLineRight + prompt + masked + status
	if distance := len(masked) - cursor + len(status); distance > 0 {
		line += ansi.CursorBackward(distance)
	}
	return writeSecretOutput(output, line)
}

func writeSecretOutput(output io.Writer, value string) bool {
	_, err := io.WriteString(output, value)
	return err == nil
}
