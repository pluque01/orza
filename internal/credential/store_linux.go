//go:build linux

package credential

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/big"

	dbus "github.com/keybase/dbus"
	secretservice "github.com/keybase/go-keychain/secretservice"
	"golang.org/x/crypto/hkdf"
)

const (
	linuxApplicationAttribute = "orza.application"
	linuxScopeAttribute       = "orza.scope"
	linuxReferenceAttribute   = "orza.reference"
	linuxCredentialLabel      = "Orza credential"
)

// Store uses the default Secret Service collection and encrypted sessions.
type Store struct {
	backend   linuxBackend
	operation chan struct{}
}

func NewStore() *Store { return newLinuxStore(&secretServiceBackend{}) }

func newLinuxStore(backend linuxBackend) *Store {
	operation := make(chan struct{}, 1)
	operation <- struct{}{}
	return &Store{backend: backend, operation: operation}
}

func (s *Store) Set(ctx context.Context, key Key, secret []byte) error {
	if err := s.begin(ctx); err != nil {
		return err
	}
	defer s.end()

	value := clone(secret)
	defer wipe(value)

	session, err := s.backend.OpenSession(ctx)
	if err != nil {
		return linuxError(ctx, "open encrypted session", err)
	}
	defer s.backend.CloseSession(session)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.backend.Unlock(ctx); err != nil {
		return linuxError(ctx, "unlock default collection", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	attributes := linuxAttributes(key)
	if err := s.backend.Create(ctx, session, attributes, value, true); err != nil {
		return linuxError(ctx, "write credential", err)
	}

	actual, err := s.read(ctx, session, attributes)
	if err != nil {
		return linuxError(ctx, "verify credential write", err)
	}
	defer wipe(actual)
	if !bytes.Equal(actual, value) {
		return fmt.Errorf("%w: Secret Service credential value mismatch", ErrUnavailable)
	}
	return ctx.Err()
}

func (s *Store) Get(ctx context.Context, key Key) ([]byte, error) {
	if err := s.begin(ctx); err != nil {
		return nil, err
	}
	defer s.end()

	session, err := s.backend.OpenSession(ctx)
	if err != nil {
		return nil, linuxError(ctx, "open encrypted session", err)
	}
	defer s.backend.CloseSession(session)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := s.backend.Unlock(ctx); err != nil {
		return nil, linuxError(ctx, "unlock default collection", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	secret, err := s.read(ctx, session, linuxAttributes(key))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, linuxError(ctx, "read credential", err)
	}
	if err := ctx.Err(); err != nil {
		wipe(secret)
		return nil, err
	}
	result := clone(secret)
	wipe(secret)
	return result, nil
}

func (s *Store) Delete(ctx context.Context, key Key) error {
	if err := s.begin(ctx); err != nil {
		return err
	}
	defer s.end()

	if err := s.backend.Unlock(ctx); err != nil {
		return linuxError(ctx, "unlock default collection", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	attributes := linuxAttributes(key)
	item, err := s.find(ctx, attributes)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return linuxError(ctx, "find credential for deletion", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.backend.Delete(ctx, item); err != nil {
		return linuxError(ctx, "delete credential", err)
	}

	_, err = s.find(ctx, attributes)
	if !errors.Is(err, ErrNotFound) {
		if err == nil {
			err = errors.New("credential remains present")
		}
		return linuxError(ctx, "verify credential deletion", err)
	}
	return ctx.Err()
}

func (s *Store) read(ctx context.Context, session *linuxSession, attributes map[string]string) ([]byte, error) {
	item, err := s.find(ctx, attributes)
	if err != nil {
		return nil, err
	}
	return s.backend.Read(ctx, session, item)
}

func (s *Store) find(ctx context.Context, attributes map[string]string) (string, error) {
	items, err := s.backend.Search(ctx, attributes)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", ErrNotFound
	}
	if len(items) != 1 {
		return "", errors.New("secret service returned multiple matching credentials")
	}
	actual, err := s.backend.Attributes(ctx, items[0])
	if err != nil {
		return "", err
	}
	for name, value := range attributes {
		if actual[name] != value {
			return "", errors.New("secret service returned a credential with nonmatching attributes")
		}
	}
	return items[0], nil
}

func (s *Store) begin(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.operation:
		if err := ctx.Err(); err != nil {
			s.end()
			return err
		}
		return nil
	}
}

func (s *Store) end() { s.operation <- struct{}{} }

func linuxAttributes(key Key) map[string]string {
	return map[string]string{
		linuxApplicationAttribute: "orza",
		linuxScopeAttribute:       string(key.Scope),
		linuxReferenceAttribute:   string(key.Reference),
	}
}

func linuxError(ctx context.Context, operation string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	var dismissed secretservice.PromptDismissedError
	if errors.As(err, &dismissed) {
		return fmt.Errorf("%w: Secret Service %s prompt dismissed", ErrUnavailable, operation)
	}
	return fmt.Errorf("%w: Secret Service %s: %v", ErrUnavailable, operation, err)
}

type linuxSession struct {
	session *secretservice.Session
}

type linuxBackend interface {
	OpenSession(context.Context) (*linuxSession, error)
	CloseSession(*linuxSession)
	Unlock(context.Context) error
	Search(context.Context, map[string]string) ([]string, error)
	Attributes(context.Context, string) (map[string]string, error)
	Create(context.Context, *linuxSession, map[string]string, []byte, bool) error
	Read(context.Context, *linuxSession, string) ([]byte, error)
	Delete(context.Context, string) error
}

type secretServiceBackend struct {
	service *secretservice.SecretService
}

func (b *secretServiceBackend) getService() (*secretservice.SecretService, error) {
	if b.service != nil {
		return b.service, nil
	}
	service, err := secretservice.NewService()
	if err != nil {
		return nil, err
	}
	b.service = service
	return service, nil
}

func (b *secretServiceBackend) OpenSession(ctx context.Context) (*linuxSession, error) {
	service, err := b.getService()
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	private, public, err := linuxDHKeypair()
	if err != nil {
		return nil, err
	}
	var algorithmOutput dbus.Variant
	var path dbus.ObjectPath
	call, ctxErr := joinedDBusCall(
		ctx,
		service.ServiceObj(),
		"org.freedesktop.Secret.Service.OpenSession",
		secretservice.AuthenticationDHAES,
		dbus.MakeVariant(public.Bytes()),
	)
	if call == nil {
		return nil, ctxErr
	}
	callErr := call.Store(&algorithmOutput, &path)
	if ctxErr != nil {
		if callErr == nil && path.IsValid() {
			service.Obj(path).Call("org.freedesktop.Secret.Session.Close", secretservice.NilFlags)
		}
		return nil, ctxErr
	}
	if callErr != nil {
		return nil, fmt.Errorf("failed to open secretservice session: %w", callErr)
	}
	theirPublicBytes, ok := algorithmOutput.Value().([]byte)
	if !ok {
		service.Obj(path).Call("org.freedesktop.Secret.Session.Close", secretservice.NilFlags)
		return nil, errors.New("failed to coerce algorithm output value to byteslice")
	}
	aesKey, err := linuxDHKey(theirPublicBytes, private)
	if err != nil {
		service.Obj(path).Call("org.freedesktop.Secret.Session.Close", secretservice.NilFlags)
		return nil, err
	}
	session := &secretservice.Session{
		Mode:    secretservice.AuthenticationDHAES,
		Path:    path,
		Public:  public,
		Private: private,
		AESKey:  aesKey,
	}
	return &linuxSession{session: session}, nil
}

func (b *secretServiceBackend) CloseSession(session *linuxSession) {
	if session != nil && session.session != nil && b.service != nil {
		b.service.Obj(session.session.Path).Call("org.freedesktop.Secret.Session.Close", secretservice.NilFlags)
		wipe(session.session.AESKey)
		session.session.AESKey = nil
		session.session.Private = nil
	}
}

func (b *secretServiceBackend) Unlock(ctx context.Context) error {
	service, err := b.getService()
	if err != nil {
		return err
	}
	collection, exists, err := defaultCollection(ctx, service)
	if err != nil || !exists {
		return err
	}
	var unlocked []dbus.ObjectPath
	var prompt dbus.ObjectPath
	call, ctxErr := joinedDBusCall(
		ctx,
		service.ServiceObj(),
		"org.freedesktop.Secret.Service.Unlock",
		[]dbus.ObjectPath{collection},
	)
	if call == nil {
		return ctxErr
	}
	if err := call.Store(&unlocked, &prompt); err != nil {
		if ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("failed to unlock items: %w", err)
	}
	if ctxErr != nil {
		return b.dismissPrompt(prompt, ctxErr)
	}
	if err := b.promptAndWait(ctx, prompt); err != nil {
		return fmt.Errorf("failed to prompt: %w", err)
	}
	return nil
}

func (b *secretServiceBackend) Search(ctx context.Context, attributes map[string]string) ([]string, error) {
	service, err := b.getService()
	if err != nil {
		return nil, err
	}
	collection, exists, err := defaultCollection(ctx, service)
	if err != nil || !exists {
		return nil, err
	}
	var paths []dbus.ObjectPath
	err = service.Obj(collection).CallWithContext(
		ctx,
		"org.freedesktop.Secret.Collection.SearchItems",
		secretservice.NilFlags,
		secretservice.Attributes(attributes),
	).Store(&paths)
	if err != nil {
		return nil, fmt.Errorf("failed to search collection: %w", err)
	}
	items := make([]string, len(paths))
	for i, path := range paths {
		items[i] = string(path)
	}
	return items, nil
}

func defaultCollection(ctx context.Context, service *secretservice.SecretService) (dbus.ObjectPath, bool, error) {
	var collection dbus.ObjectPath
	err := service.ServiceObj().CallWithContext(
		ctx,
		"org.freedesktop.Secret.Service.ReadAlias",
		secretservice.NilFlags,
		"default",
	).Store(&collection)
	if err != nil {
		return "", false, err
	}
	if !defaultCollectionExists(collection) {
		return "", false, nil
	}
	return collection, true, nil
}

func defaultCollectionExists(collection dbus.ObjectPath) bool {
	return collection != dbus.ObjectPath(secretservice.NullPrompt)
}

func (b *secretServiceBackend) Attributes(ctx context.Context, item string) (map[string]string, error) {
	service, err := b.getService()
	if err != nil {
		return nil, err
	}
	var attributesV dbus.Variant
	err = service.Obj(dbus.ObjectPath(item)).CallWithContext(
		ctx,
		"org.freedesktop.DBus.Properties.Get",
		secretservice.NilFlags,
		"org.freedesktop.Secret.Item",
		"Attributes",
	).Store(&attributesV)
	if err != nil {
		return nil, fmt.Errorf("failed to get attributes: %w", err)
	}
	attributes, ok := attributesV.Value().(map[string]string)
	if !ok {
		return nil, errors.New("failed to coerce item attributes")
	}
	result := make(map[string]string, len(attributes))
	for name, value := range attributes {
		result[name] = value
	}
	return result, nil
}

func (b *secretServiceBackend) Create(ctx context.Context, session *linuxSession, attributes map[string]string, value []byte, replace bool) error {
	secret, err := session.session.NewSecret(value)
	if err != nil {
		return err
	}
	defer wipe(secret.Value)
	defer wipe(secret.Parameters)
	var item dbus.ObjectPath
	var prompt dbus.ObjectPath
	call, ctxErr := joinedDBusCall(
		ctx,
		b.service.Obj(secretservice.DefaultCollection),
		"org.freedesktop.Secret.Collection.CreateItem",
		secretservice.NewSecretProperties(linuxCredentialLabel, attributes),
		secret,
		replace,
	)
	if call == nil {
		return ctxErr
	}
	if err := call.Store(&item, &prompt); err != nil {
		if ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("failed to create item: %w", err)
	}
	if ctxErr != nil {
		return b.dismissPrompt(prompt, ctxErr)
	}
	return b.promptAndWait(ctx, prompt)
}

func (b *secretServiceBackend) Read(ctx context.Context, session *linuxSession, item string) ([]byte, error) {
	var secretValues []interface{}
	err := b.service.Obj(dbus.ObjectPath(item)).CallWithContext(
		ctx,
		"org.freedesktop.Secret.Item.GetSecret",
		secretservice.NilFlags,
		session.session.Path,
	).Store(&secretValues)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}
	var secret secretservice.Secret
	if err := dbus.Store(secretValues, &secret.Session, &secret.Parameters, &secret.Value, &secret.ContentType); err != nil {
		return nil, fmt.Errorf("failed to unmarshal get secret result: %w", err)
	}
	defer wipe(secret.Parameters)
	defer wipe(secret.Value)
	if secret.Session != session.session.Path {
		return nil, errors.New("secret service returned a secret for a different session")
	}
	return decryptLinuxSecret(secret.Parameters, secret.Value, session.session.AESKey)
}

func (b *secretServiceBackend) Delete(ctx context.Context, item string) error {
	var prompt dbus.ObjectPath
	call, ctxErr := joinedDBusCall(
		ctx,
		b.service.Obj(dbus.ObjectPath(item)),
		"org.freedesktop.Secret.Item.Delete",
	)
	if call == nil {
		return ctxErr
	}
	if err := call.Store(&prompt); err != nil {
		if ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("failed to delete item: %w", err)
	}
	if ctxErr != nil {
		return b.dismissPrompt(prompt, ctxErr)
	}
	return b.promptAndWait(ctx, prompt)
}

func (b *secretServiceBackend) promptAndWait(ctx context.Context, prompt dbus.ObjectPath) error {
	if prompt == dbus.ObjectPath(secretservice.NullPrompt) {
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return b.dismissPrompt(prompt, err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := b.service.PromptAndWait(prompt)
		done <- err
	}()
	select {
	case err := <-done:
		var dismissed secretservice.PromptDismissedError
		if err != nil && !errors.As(err, &dismissed) {
			_ = b.dismissPrompt(prompt, err)
		}
		return err
	case <-ctx.Done():
		return b.dismissAndJoinPrompt(prompt, done, ctx.Err())
	}
}

func (b *secretServiceBackend) dismissPrompt(prompt dbus.ObjectPath, ctxErr error) error {
	if prompt == dbus.ObjectPath(secretservice.NullPrompt) {
		return ctxErr
	}
	_, _ = joinedDBusCall(
		context.Background(),
		b.service.Obj(prompt),
		"org.freedesktop.Secret.Prompt.Dismiss",
	)
	return ctxErr
}

func (b *secretServiceBackend) dismissAndJoinPrompt(prompt dbus.ObjectPath, done <-chan error, ctxErr error) error {
	_ = b.dismissPrompt(prompt, ctxErr)
	<-done
	return ctxErr
}

// Keybase's CallWithContext cancels the local call tracker, but D-Bus cannot
// cancel work already sent to the service. Mutations therefore observe
// cancellation but join the remote reply before the Store admits more work.
func joinedDBusCall(ctx context.Context, object dbus.BusObject, method string, args ...interface{}) (*dbus.Call, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	call := object.Go(method, secretservice.NilFlags, nil, args...)
	select {
	case completed := <-call.Done:
		return completed, ctx.Err()
	case <-ctx.Done():
		return <-call.Done, ctx.Err()
	}
}

var (
	linuxDHOne  = big.NewInt(1)
	linuxDHP, _ = new(big.Int).SetString(
		"FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7EDEE386BFB5A899FA5AE9F24117C4B1FE649286651ECE65381FFFFFFFFFFFFFFFF",
		16,
	)
	linuxDHPMinusOne = new(big.Int).Sub(new(big.Int).Set(linuxDHP), linuxDHOne)
)

func linuxDHKeypair() (*big.Int, *big.Int, error) {
	for {
		private, err := cryptorand.Int(cryptorand.Reader, linuxDHPMinusOne)
		if err != nil {
			return nil, nil, err
		}
		if private.Sign() > 0 {
			public := new(big.Int).Exp(big.NewInt(2), private, linuxDHP)
			return private, public, nil
		}
	}
}

func linuxDHKey(theirPublicBytes []byte, private *big.Int) ([]byte, error) {
	theirPublic := new(big.Int).SetBytes(theirPublicBytes)
	if theirPublic.Cmp(linuxDHOne) <= 0 || theirPublic.Cmp(linuxDHPMinusOne) >= 0 {
		return nil, errors.New("secret service DH parameter out of bounds")
	}
	shared := new(big.Int).Exp(theirPublic, private, linuxDHP).Bytes()
	defer wipe(shared)
	key := make([]byte, 16)
	if _, err := io.ReadFull(hkdf.New(sha256.New, shared, nil, nil), key); err != nil {
		wipe(key)
		return nil, err
	}
	return key, nil
}

func decryptLinuxSecret(iv, ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(iv) != aes.BlockSize {
		return nil, errors.New("secret service returned an invalid AES IV")
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("secret service returned invalid AES ciphertext")
	}
	plaintext := clone(ciphertext)
	defer wipe(plaintext)
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, plaintext)
	padding := int(plaintext[len(plaintext)-1])
	if padding == 0 || padding > aes.BlockSize || padding > len(plaintext) {
		return nil, errors.New("secret service returned invalid AES padding")
	}
	for _, value := range plaintext[len(plaintext)-padding:] {
		if int(value) != padding {
			return nil, errors.New("secret service returned invalid AES padding")
		}
	}
	return clone(plaintext[:len(plaintext)-padding]), nil
}

var _ CredentialStore = (*Store)(nil)
