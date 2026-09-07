package hostkey

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var (
	ErrInvalidHostKey = errors.New("invalid presented host key")
	ErrHostNotTrusted = errors.New("host key was not trusted")
	ErrHostRevoked    = errors.New("host key is revoked")
)

type TrustDecision string

const (
	TrustReject  TrustDecision = "reject"
	TrustOnce    TrustDecision = "once"
	TrustPersist TrustDecision = "persist"
)

type DecisionResult struct {
	Decision  TrustDecision
	Status    app.HostTrustStatus
	Persisted *app.TrustedHost
}

type trustedHostRepository interface {
	GetTrustedHost(context.Context, app.HostEndpoint) (app.TrustedHost, error)
	TrustHost(context.Context, app.TrustHostRequest) (app.TrustedHost, error)
}

type Policy struct {
	repository trustedHostRepository
	standard   ssh.HostKeyCallback
}

var _ app.HostTrust = (*Policy)(nil)

// NewPolicy loads the supplied OpenSSH known_hosts files read-only. When no
// files are supplied, existing files in the user's standard locations are used.
func NewPolicy(repository trustedHostRepository, files ...string) (*Policy, error) {
	if repository == nil {
		return nil, errors.New("trusted host repository is required")
	}
	if len(files) == 0 {
		var err error
		files, err = DefaultKnownHostsFiles()
		if err != nil {
			return nil, err
		}
	}

	var callback ssh.HostKeyCallback
	if len(files) != 0 {
		var err error
		callback, err = knownhosts.New(files...)
		if err != nil {
			return nil, fmt.Errorf("load known_hosts: %w", err)
		}
	}
	return &Policy{repository: repository, standard: callback}, nil
}

// NewCatalogPolicy adapts the cycle-free catalog DTOs to the application host
// trust contract implemented by Policy.
func NewCatalogPolicy(repository *catalog.TrustedHostRepository, files ...string) (*Policy, error) {
	if repository == nil {
		return nil, errors.New("trusted host repository is required")
	}
	return NewPolicy(NewCatalogTrustedHostAdapter(repository), files...)
}

// NewCatalogTrustedHostAdapter exposes catalog trust persistence through the
// application port while keeping catalog DTOs independent from app types.
func NewCatalogTrustedHostAdapter(repository *catalog.TrustedHostRepository) app.TrustedHostRepository {
	if repository == nil {
		return nil
	}
	return catalogRepositoryAdapter{repository: repository}
}

type catalogRepositoryAdapter struct {
	repository *catalog.TrustedHostRepository
}

func (a catalogRepositoryAdapter) GetTrustedHost(ctx context.Context, endpoint app.HostEndpoint) (app.TrustedHost, error) {
	host, err := a.repository.GetTrustedHost(ctx, catalog.HostEndpoint{
		CanonicalHost: endpoint.CanonicalHost,
		Port:          endpoint.Port,
	})
	if err != nil {
		return app.TrustedHost{}, err
	}
	return app.TrustedHost{
		ID: host.ID,
		HostEndpoint: app.HostEndpoint{
			CanonicalHost: host.CanonicalHost,
			Port:          host.Port,
		},
		KeyAlgorithm:      host.KeyAlgorithm,
		PublicKey:         bytes.Clone(host.PublicKey),
		FingerprintSHA256: host.FingerprintSHA256,
		Revision:          app.Revision(host.Revision),
		AcceptedAt:        host.AcceptedAt,
	}, nil
}

func (a catalogRepositoryAdapter) TrustHost(ctx context.Context, request app.TrustHostRequest) (app.TrustedHost, error) {
	var expected *uint64
	if request.ExpectedRevision != nil {
		value := uint64(*request.ExpectedRevision)
		expected = &value
	}
	host, err := a.repository.TrustHost(ctx, catalog.TrustHostRequest{
		Host: catalog.PresentedHost{
			Endpoint: catalog.HostEndpoint{
				CanonicalHost: request.Host.Endpoint.CanonicalHost,
				Port:          request.Host.Endpoint.Port,
			},
			KeyAlgorithm:      request.Host.KeyAlgorithm,
			PublicKey:         bytes.Clone(request.Host.PublicKey),
			FingerprintSHA256: request.Host.FingerprintSHA256,
		},
		ExpectedRevision: expected,
	})
	if err != nil {
		return app.TrustedHost{}, err
	}
	return a.GetTrustedHost(ctx, app.HostEndpoint{CanonicalHost: host.CanonicalHost, Port: host.Port})
}

func DefaultKnownHostsFiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("locate user home for known_hosts: %w", err)
	}
	candidates := []string{
		filepath.Join(home, ".ssh", "known_hosts"),
		filepath.Join(home, ".ssh", "known_hosts2"),
	}
	files := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		info, statErr := os.Stat(candidate)
		switch {
		case statErr == nil && info.Mode().IsRegular():
			files = append(files, candidate)
		case statErr == nil:
			return nil, fmt.Errorf("known_hosts path is not a regular file: %s", candidate)
		case errors.Is(statErr, os.ErrNotExist):
			continue
		default:
			return nil, fmt.Errorf("inspect known_hosts: %w", statErr)
		}
	}
	return files, nil
}

// NewPresentedHost derives display metadata from the key rather than trusting
// metadata supplied by a server or caller.
func NewPresentedHost(endpoint app.HostEndpoint, remoteAddress string, key ssh.PublicKey) (app.PresentedHost, error) {
	endpoint.CanonicalHost = normalizeHost(endpoint.CanonicalHost)
	if endpoint.CanonicalHost == "" || strings.IndexByte(endpoint.CanonicalHost, 0) >= 0 || endpoint.Port == 0 || key == nil {
		return app.PresentedHost{}, ErrInvalidHostKey
	}
	return app.PresentedHost{
		Endpoint:          endpoint,
		RemoteAddress:     remoteAddress,
		KeyAlgorithm:      key.Type(),
		PublicKey:         bytes.Clone(key.Marshal()),
		FingerprintSHA256: ssh.FingerprintSHA256(key),
	}, nil
}

func (p *Policy) CheckHost(ctx context.Context, presented app.PresentedHost) (app.HostTrustResult, error) {
	presented, key, err := validatePresented(presented)
	if err != nil {
		return app.HostTrustResult{}, err
	}
	if p == nil || p.repository == nil {
		return app.HostTrustResult{}, errors.New("host-key policy is not configured")
	}

	standardStatus := app.HostTrustUnknown
	if p.standard != nil {
		address := net.JoinHostPort(presented.Endpoint.CanonicalHost, strconv.Itoa(int(presented.Endpoint.Port)))
		remote := stringAddress(address)
		if presented.RemoteAddress != "" {
			remote = stringAddress(presented.RemoteAddress)
		}
		standardErr := p.standard(address, remote, key)
		switch typed := standardErr.(type) {
		case nil:
			return app.HostTrustResult{Status: app.HostTrustKnown}, nil
		case *knownhosts.RevokedError:
			return app.HostTrustResult{Status: app.HostTrustRevoked}, nil
		case *knownhosts.KeyError:
			if len(typed.Want) != 0 {
				standardStatus = app.HostTrustChanged
			}
		default:
			return app.HostTrustResult{}, fmt.Errorf("check known_hosts: %w", standardErr)
		}
	}

	trusted, repositoryErr := p.repository.GetTrustedHost(ctx, presented.Endpoint)
	if repositoryErr == nil {
		known := trusted
		if sameKey(trusted, presented) {
			return app.HostTrustResult{Status: app.HostTrustKnown, Known: &known}, nil
		}
		return app.HostTrustResult{Status: app.HostTrustChanged, Known: &known}, nil
	}
	if !errors.Is(repositoryErr, sql.ErrNoRows) && !errors.Is(repositoryErr, catalog.ErrTrustedHostNotFound) {
		return app.HostTrustResult{}, fmt.Errorf("read application host trust: %w", repositoryErr)
	}
	return app.HostTrustResult{Status: standardStatus}, nil
}

// DecideHost applies an explicit decision to the exact key that was checked.
// TrustOnce never writes, and a revoked key cannot be accepted or persisted.
func (p *Policy) DecideHost(ctx context.Context, presented app.PresentedHost, decision TrustDecision, expected *app.Revision) (DecisionResult, error) {
	checked, err := p.CheckHost(ctx, presented)
	if err != nil {
		return DecisionResult{}, err
	}
	result := DecisionResult{Decision: decision, Status: checked.Status}
	if checked.Status == app.HostTrustRevoked {
		return result, ErrHostRevoked
	}
	if checked.Status == app.HostTrustKnown {
		result.Persisted = checked.Known
		return result, nil
	}

	switch decision {
	case TrustOnce:
		return result, nil
	case TrustPersist:
		trusted, err := p.repository.TrustHost(ctx, app.TrustHostRequest{Host: presented, ExpectedRevision: expected})
		if err != nil {
			return DecisionResult{}, fmt.Errorf("persist host trust: %w", err)
		}
		result.Persisted = &trusted
		return result, nil
	case TrustReject:
		return result, ErrHostNotTrusted
	default:
		return DecisionResult{}, fmt.Errorf("%w: unknown decision", ErrHostNotTrusted)
	}
}

func validatePresented(presented app.PresentedHost) (app.PresentedHost, ssh.PublicKey, error) {
	presented.Endpoint.CanonicalHost = normalizeHost(presented.Endpoint.CanonicalHost)
	if presented.Endpoint.CanonicalHost == "" || strings.IndexByte(presented.Endpoint.CanonicalHost, 0) >= 0 || presented.Endpoint.Port == 0 {
		return app.PresentedHost{}, nil, ErrInvalidHostKey
	}
	key, err := ssh.ParsePublicKey(presented.PublicKey)
	if err != nil {
		return app.PresentedHost{}, nil, fmt.Errorf("%w: public key", ErrInvalidHostKey)
	}
	if presented.KeyAlgorithm != key.Type() || presented.FingerprintSHA256 != ssh.FingerprintSHA256(key) {
		return app.PresentedHost{}, nil, fmt.Errorf("%w: inconsistent metadata", ErrInvalidHostKey)
	}
	presented.PublicKey = bytes.Clone(key.Marshal())
	return presented, key, nil
}

func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func sameKey(trusted app.TrustedHost, presented app.PresentedHost) bool {
	return trusted.KeyAlgorithm == presented.KeyAlgorithm &&
		trusted.FingerprintSHA256 == presented.FingerprintSHA256 &&
		bytes.Equal(trusted.PublicKey, presented.PublicKey)
}

type stringAddress string

func (a stringAddress) Network() string { return "tcp" }
func (a stringAddress) String() string  { return string(a) }
