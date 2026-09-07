package sshclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/pluque01/orza/internal/app"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	MaxPrivateKeyBytes    = 1 << 20
	MaxPassphraseAttempts = 3
)

var (
	ErrAgentUnavailable      = errors.New("SSH agent is unavailable")
	ErrInvalidAuthMethod     = errors.New("invalid SSH authentication method")
	ErrPrivateKeyTooLarge    = errors.New("private key file is too large")
	ErrPrivateKeyPermissions = errors.New("private key file permissions are too broad")
	ErrPassphraseAttempts    = errors.New("private key passphrase attempts exhausted")
	ErrSecretPromptMissing   = errors.New("secret prompt is required")
)

type SecretPrompt func(context.Context, app.SecretRequest) ([]byte, error)

// AgentClient combines the protocol client with ownership of its transport.
type AgentClient struct {
	agent.Agent
	connection io.Closer
	closeOnce  sync.Once
	closeErr   error
}

func newAgentClient(connection io.Closer, client agent.Agent) *AgentClient {
	return &AgentClient{Agent: client, connection: connection}
}

func (c *AgentClient) Close() error {
	if c == nil || c.connection == nil {
		return nil
	}
	c.closeOnce.Do(func() {
		c.closeErr = c.connection.Close()
	})
	return c.closeErr
}

// AuthResult owns exactly one selected authentication method and any resource
// (currently an agent connection) that must remain open during authentication.
type AuthResult struct {
	Method  ssh.AuthMethod
	closer  io.Closer
	failure *authFailureRecorder
}

type authFailureRecorder struct {
	mu  sync.Mutex
	err error
}

type recordingAgentSigner struct {
	signer  ssh.Signer
	failure *authFailureRecorder
}

func (s *recordingAgentSigner) PublicKey() ssh.PublicKey {
	return s.signer.PublicKey()
}

func (s *recordingAgentSigner) Sign(random io.Reader, data []byte) (*ssh.Signature, error) {
	signature, err := s.signer.Sign(random, data)
	s.record(err)
	return signature, err
}

func (s *recordingAgentSigner) record(err error) {
	if err != nil {
		s.failure.record(credentialFailure("agent", fmt.Errorf("sign with SSH agent: %w", err)))
	}
}

type recordingAgentAlgorithmSigner struct {
	*recordingAgentSigner
	signer ssh.AlgorithmSigner
}

func (s *recordingAgentAlgorithmSigner) SignWithAlgorithm(random io.Reader, data []byte, algorithm string) (*ssh.Signature, error) {
	signature, err := s.signer.SignWithAlgorithm(random, data, algorithm)
	s.record(err)
	return signature, err
}

type recordingAgentMultiAlgorithmSigner struct {
	*recordingAgentAlgorithmSigner
	signer ssh.MultiAlgorithmSigner
}

func (s *recordingAgentMultiAlgorithmSigner) Algorithms() []string {
	return s.signer.Algorithms()
}

func (r *authFailureRecorder) record(err error) {
	if err == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err == nil {
		r.err = err
	}
}

func (r *authFailureRecorder) load() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

func (r *AuthResult) authFailure() error {
	if r == nil {
		return nil
	}
	return r.failure.load()
}

func (r *AuthResult) Close() error {
	if r == nil || r.closer == nil {
		return nil
	}
	return r.closer.Close()
}

// BuildAuthMethod constructs only the method selected by the connection. The
// returned result must be closed after the SSH handshake/session is finished.
func BuildAuthMethod(ctx context.Context, connection app.Connection, prompt SecretPrompt) (*AuthResult, error) {
	return buildAuthMethod(ctx, connection, prompt, OpenAgent, readPrivateKey)
}

type agentOpener func(context.Context) (*AgentClient, error)
type keyReader func(string) ([]byte, error)

func buildAuthMethod(ctx context.Context, connection app.Connection, prompt SecretPrompt, open agentOpener, readKey keyReader) (*AuthResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	switch connection.AuthMethod {
	case app.AuthMethodAgent:
		client, err := open(ctx)
		if err != nil {
			return nil, credentialFailure("agent", fmt.Errorf("open SSH agent: %w", err))
		}
		if client == nil || client.Agent == nil {
			if client != nil {
				_ = client.Close()
			}
			return nil, credentialFailure("agent", ErrAgentUnavailable)
		}
		signers, err := client.Signers()
		if err != nil {
			_ = client.Close()
			return nil, credentialFailure("agent", fmt.Errorf("list SSH agent signers: %w", err))
		}
		if len(signers) == 0 {
			_ = client.Close()
			return nil, credentialFailure("agent", ErrAgentUnavailable)
		}
		failure := &authFailureRecorder{}
		recorded := make([]ssh.Signer, len(signers))
		for index, signer := range signers {
			recorded[index] = recordingSigner(signer, failure)
		}
		return &AuthResult{
			Method:  ssh.PublicKeys(recorded...),
			closer:  client,
			failure: failure,
		}, nil

	case app.AuthMethodKey:
		if connection.IdentityFile == "" {
			return nil, credentialFailure("identity file", fmt.Errorf("%w: identity file is required", ErrInvalidAuthMethod))
		}
		keyBytes, err := readKey(connection.IdentityFile)
		if err != nil {
			return nil, credentialFailure("identity file", fmt.Errorf("read private key: %w", err))
		}
		defer clearBytes(keyBytes)

		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err == nil {
			return &AuthResult{Method: ssh.PublicKeys(signer)}, nil
		}
		var missing *ssh.PassphraseMissingError
		if !errors.As(err, &missing) {
			return nil, credentialFailure("identity file", fmt.Errorf("parse private key: %w", err))
		}
		if prompt == nil {
			return nil, credentialFailure("secret prompt", ErrSecretPromptMissing)
		}

		var parseErr error
		for attempt := 0; attempt < MaxPassphraseAttempts; attempt++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			passphrase, err := prompt(ctx, app.SecretRequest{
				Kind:   app.SecretPassphrase,
				Prompt: "Private key passphrase",
			})
			if err != nil {
				clearBytes(passphrase)
				return nil, credentialFailure("secret prompt", fmt.Errorf("request private key passphrase: %w", err))
			}
			signer, parseErr = ssh.ParsePrivateKeyWithPassphrase(keyBytes, passphrase)
			clearBytes(passphrase)
			if parseErr == nil {
				return &AuthResult{Method: ssh.PublicKeys(signer)}, nil
			}
		}
		return nil, credentialFailure("identity file", fmt.Errorf("%w: %v", ErrPassphraseAttempts, parseErr))

	case app.AuthMethodPassword:
		if prompt == nil {
			return nil, credentialFailure("secret prompt", ErrSecretPromptMissing)
		}
		failure := &authFailureRecorder{}
		callback := passwordCallback(ctx, connection.CredentialRef, prompt)
		method := ssh.PasswordCallback(func() (string, error) {
			password, err := callback()
			failure.record(err)
			return password, err
		})
		return &AuthResult{Method: method, failure: failure}, nil

	default:
		return nil, credentialFailure("identity file", fmt.Errorf("%w: %q", ErrInvalidAuthMethod, connection.AuthMethod))
	}
}

func recordingSigner(signer ssh.Signer, failure *authFailureRecorder) ssh.Signer {
	if signer == nil {
		return nil
	}
	recording := &recordingAgentSigner{signer: signer, failure: failure}
	if multi, ok := signer.(ssh.MultiAlgorithmSigner); ok {
		algorithm := &recordingAgentAlgorithmSigner{recordingAgentSigner: recording, signer: multi}
		return &recordingAgentMultiAlgorithmSigner{recordingAgentAlgorithmSigner: algorithm, signer: multi}
	}
	if algorithm, ok := signer.(ssh.AlgorithmSigner); ok {
		return &recordingAgentAlgorithmSigner{recordingAgentSigner: recording, signer: algorithm}
	}
	return recording
}

func passwordCallback(ctx context.Context, credentialRef string, prompt SecretPrompt) func() (string, error) {
	var mu sync.Mutex
	used := false
	return func() (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if used {
			return "", credentialFailure("secret prompt", errors.New("password was already requested"))
		}
		used = true
		secret, err := prompt(ctx, app.SecretRequest{
			Kind:       app.SecretPassword,
			Prompt:     "Password",
			Credential: credentialRef,
		})
		if err != nil {
			clearBytes(secret)
			return "", credentialFailure("secret prompt", err)
		}
		password := string(secret)
		clearBytes(secret)
		return password, nil
	}
}

func credentialFailure(detail string, cause error) error {
	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		return cause
	}
	return app.NewSSHStartError(app.SSHFailureCredentialUnavailable, app.SSHFailureStageCredential, detail, cause)
}

func readPrivateKey(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("identity file is not a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, ErrPrivateKeyPermissions
	}
	if info.Size() > MaxPrivateKeyBytes {
		return nil, ErrPrivateKeyTooLarge
	}

	key, err := io.ReadAll(io.LimitReader(file, MaxPrivateKeyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(key) > MaxPrivateKeyBytes {
		clearBytes(key)
		return nil, ErrPrivateKeyTooLarge
	}
	return key, nil
}

func clearBytes(secret []byte) {
	for index := range secret {
		secret[index] = 0
	}
}
