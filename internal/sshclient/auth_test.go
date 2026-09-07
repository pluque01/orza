package sshclient

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func TestBuildAuthMethodSelectsOnlyAgent(t *testing.T) {
	closed := &trackingCloser{}
	keyring := agent.NewKeyring()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := keyring.Add(agent.AddedKey{PrivateKey: private}); err != nil {
		t.Fatal(err)
	}
	openCalls := 0
	result, err := buildAuthMethod(context.Background(), app.Connection{AuthMethod: app.AuthMethodAgent}, nil,
		func(context.Context) (*AgentClient, error) {
			openCalls++
			return newAgentClient(closed, keyring), nil
		},
		func(string) ([]byte, error) {
			t.Fatal("agent authentication read a key")
			return nil, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if result.Method == nil || openCalls != 1 {
		t.Fatalf("BuildAuthMethod() method = %v, opens = %d", result.Method, openCalls)
	}
	if err := result.Close(); err != nil {
		t.Fatal(err)
	}
	if err := result.Close(); err != nil {
		t.Fatal(err)
	}
	if closed.calls != 1 {
		t.Fatalf("agent transport close calls = %d, want 1", closed.calls)
	}
}

func TestBuildAuthMethodRejectsAgentWithoutIdentities(t *testing.T) {
	closed := &trackingCloser{}
	_, err := buildAuthMethod(context.Background(), app.Connection{AuthMethod: app.AuthMethodAgent}, nil,
		func(context.Context) (*AgentClient, error) {
			return newAgentClient(closed, agent.NewKeyring()), nil
		}, nil)
	var failure *app.SSHStartError
	if !errors.As(err, &failure) || failure.Reason() != app.SSHFailureCredentialUnavailable || failure.Stage() != app.SSHFailureStageCredential || failure.Presentation().TechnicalDetail != "agent" || !errors.Is(err, ErrAgentUnavailable) {
		t.Fatalf("buildAuthMethod() error = %v", err)
	}
	if closed.calls != 1 {
		t.Fatalf("agent transport close calls = %d, want 1", closed.calls)
	}
}

func TestRecordingSignerPreservesAlgorithmCapabilities(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	base, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	algorithm, ok := base.(ssh.AlgorithmSigner)
	if !ok {
		t.Fatal("test signer does not support algorithm selection")
	}
	failure := &authFailureRecorder{}

	plain := recordingSigner(struct{ ssh.Signer }{base}, failure)
	if _, ok := plain.(ssh.AlgorithmSigner); ok {
		t.Fatal("plain signer gained algorithm selection")
	}

	algorithmOnly := recordingSigner(struct{ ssh.AlgorithmSigner }{algorithm}, failure)
	if _, ok := algorithmOnly.(ssh.AlgorithmSigner); !ok {
		t.Fatal("algorithm signer lost algorithm selection")
	}
	if _, ok := algorithmOnly.(ssh.MultiAlgorithmSigner); ok {
		t.Fatal("algorithm signer gained algorithm advertisement")
	}

	wantAlgorithms := []string{ssh.KeyAlgoED25519}
	multi := recordingSigner(&testMultiAlgorithmSigner{AlgorithmSigner: algorithm, algorithms: wantAlgorithms}, failure)
	recordedMulti, ok := multi.(ssh.MultiAlgorithmSigner)
	if !ok {
		t.Fatal("multi-algorithm signer lost algorithm advertisement")
	}
	if got := recordedMulti.Algorithms(); len(got) != 1 || got[0] != wantAlgorithms[0] {
		t.Fatalf("Algorithms() = %v, want %v", got, wantAlgorithms)
	}
}

func TestBuildAuthMethodParsesUnencryptedKeyWithoutPrompt(t *testing.T) {
	key := privateKeyPEM(t, nil)
	promptCalls := 0
	result, err := buildAuthMethod(context.Background(), app.Connection{
		AuthMethod:   app.AuthMethodKey,
		IdentityFile: "identity",
	}, func(context.Context, app.SecretRequest) ([]byte, error) {
		promptCalls++
		return nil, errors.New("unexpected prompt")
	}, nil, func(string) ([]byte, error) {
		return append([]byte(nil), key...), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Method == nil || promptCalls != 0 {
		t.Fatalf("key method = %v, prompt calls = %d", result.Method, promptCalls)
	}
}

func TestBuildAuthMethodRetriesEncryptedKeyAndClearsSecrets(t *testing.T) {
	correct := []byte("correct passphrase")
	key := privateKeyPEM(t, correct)
	returned := make([][]byte, 0, MaxPassphraseAttempts)
	attempts := 0
	result, err := buildAuthMethod(context.Background(), app.Connection{
		AuthMethod:   app.AuthMethodKey,
		IdentityFile: "identity",
	}, func(_ context.Context, request app.SecretRequest) ([]byte, error) {
		if request.Kind != app.SecretPassphrase {
			t.Fatalf("secret kind = %q, want passphrase", request.Kind)
		}
		attempts++
		secret := []byte("wrong")
		if attempts == 2 {
			secret = append([]byte(nil), correct...)
		}
		returned = append(returned, secret)
		return secret, nil
	}, nil, func(string) ([]byte, error) {
		return append([]byte(nil), key...), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Method == nil || attempts != 2 {
		t.Fatalf("key method = %v, attempts = %d", result.Method, attempts)
	}
	for index, secret := range returned {
		if !bytes.Equal(secret, make([]byte, len(secret))) {
			t.Fatalf("secret %d was not cleared", index)
		}
	}
}

func TestBuildAuthMethodBoundsPassphraseAttempts(t *testing.T) {
	key := privateKeyPEM(t, []byte("correct"))
	attempts := 0
	_, err := buildAuthMethod(context.Background(), app.Connection{
		AuthMethod:   app.AuthMethodKey,
		IdentityFile: "identity",
	}, func(context.Context, app.SecretRequest) ([]byte, error) {
		attempts++
		return []byte("wrong"), nil
	}, nil, func(string) ([]byte, error) {
		return append([]byte(nil), key...), nil
	})
	if !errors.Is(err, ErrPassphraseAttempts) || attempts != MaxPassphraseAttempts {
		t.Fatalf("BuildAuthMethod() error = %v, attempts = %d", err, attempts)
	}
}

func TestBuildAuthMethodPasswordIsLazyAndCarriesCredentialReference(t *testing.T) {
	promptCalls := 0
	result, err := buildAuthMethod(context.Background(), app.Connection{
		AuthMethod:    app.AuthMethodPassword,
		CredentialRef: "credential-id",
	}, func(context.Context, app.SecretRequest) ([]byte, error) {
		promptCalls++
		return []byte("secret"), nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Method == nil || promptCalls != 0 {
		t.Fatalf("password method = %v, eager prompts = %d", result.Method, promptCalls)
	}
}

func TestReadPrivateKeyRejectsOversizeReader(t *testing.T) {
	_, err := buildAuthMethod(context.Background(), app.Connection{
		AuthMethod:   app.AuthMethodKey,
		IdentityFile: "identity",
	}, nil, nil, func(string) ([]byte, error) {
		return nil, ErrPrivateKeyTooLarge
	})
	if !errors.Is(err, ErrPrivateKeyTooLarge) {
		t.Fatalf("BuildAuthMethod() error = %v, want key too large", err)
	}
}

func TestBuildAuthMethodRejectsUnknownSelection(t *testing.T) {
	_, err := buildAuthMethod(context.Background(), app.Connection{AuthMethod: "keyboard-interactive"}, nil, nil, nil)
	if !errors.Is(err, ErrInvalidAuthMethod) {
		t.Fatalf("BuildAuthMethod() error = %v, want invalid method", err)
	}
}

func TestBuildAuthMethodClassifiesLocalCredentialFailures(t *testing.T) {
	tests := []struct {
		name       string
		connection app.Connection
		prompt     SecretPrompt
		open       agentOpener
		readKey    keyReader
		detail     string
		cause      error
	}{
		{"agent", app.Connection{AuthMethod: app.AuthMethodAgent}, nil, func(context.Context) (*AgentClient, error) { return nil, ErrAgentUnavailable }, nil, "agent", ErrAgentUnavailable},
		{"identity read", app.Connection{AuthMethod: app.AuthMethodKey, IdentityFile: "canary-path"}, nil, nil, func(string) ([]byte, error) { return nil, ErrPrivateKeyTooLarge }, "identity file", ErrPrivateKeyTooLarge},
		{"key parser", app.Connection{AuthMethod: app.AuthMethodKey, IdentityFile: "canary-path"}, nil, nil, func(string) ([]byte, error) { return []byte("not a private key"), nil }, "identity file", nil},
		{"passphrase prompt", app.Connection{AuthMethod: app.AuthMethodKey, IdentityFile: "canary-path"}, func(context.Context, app.SecretRequest) ([]byte, error) { return nil, ErrSecretPromptMissing }, nil, func(string) ([]byte, error) { return privateKeyPEM(t, []byte("correct")), nil }, "secret prompt", ErrSecretPromptMissing},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := buildAuthMethod(context.Background(), test.connection, test.prompt, test.open, test.readKey)
			var failure *app.SSHStartError
			if !errors.As(err, &failure) || failure.Reason() != app.SSHFailureCredentialUnavailable || failure.Stage() != app.SSHFailureStageCredential || failure.Presentation().TechnicalDetail != test.detail {
				t.Fatalf("buildAuthMethod() error = %v", err)
			}
			if test.cause != nil && !errors.Is(err, test.cause) {
				t.Fatalf("error does not preserve %v: %v", test.cause, err)
			}
		})
	}
}

func TestPasswordCallbackPreservesLocalPromptFailure(t *testing.T) {
	wantErr := errors.New("localized prompt canary")
	secret := []byte("must-be-cleared")
	callback := passwordCallback(context.Background(), "credential-canary", func(_ context.Context, request app.SecretRequest) ([]byte, error) {
		if request.Kind != app.SecretPassword || request.Credential != "credential-canary" {
			t.Fatalf("request = %#v", request)
		}
		return secret, wantErr
	})
	_, err := callback()
	var failure *app.SSHStartError
	if !errors.As(err, &failure) || failure.Reason() != app.SSHFailureCredentialUnavailable || failure.Stage() != app.SSHFailureStageCredential || failure.Presentation().TechnicalDetail != "secret prompt" || !errors.Is(err, wantErr) {
		t.Fatalf("password callback error = %v", err)
	}
	if !bytes.Equal(secret, make([]byte, len(secret))) {
		t.Fatal("failed prompt secret was not cleared")
	}
}

func privateKeyPEM(t *testing.T, passphrase []byte) []byte {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var block *pem.Block
	if len(passphrase) == 0 {
		block, err = ssh.MarshalPrivateKey(private, "test")
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(private, "test", passphrase)
	}
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(block)
}

type testMultiAlgorithmSigner struct {
	ssh.AlgorithmSigner
	algorithms []string
}

func (s *testMultiAlgorithmSigner) Algorithms() []string { return s.algorithms }

type trackingCloser struct {
	calls int
}

func (c *trackingCloser) Close() error {
	c.calls++
	return nil
}

var _ io.Closer = (*trackingCloser)(nil)
