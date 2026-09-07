package domain

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const DefaultSSHPort uint16 = 22

var (
	ErrInvalidHost           = errors.New("invalid host")
	ErrInvalidPort           = errors.New("invalid port")
	ErrInvalidUsername       = errors.New("invalid username")
	ErrInvalidAuthMethod     = errors.New("invalid authentication method")
	ErrInvalidAuthentication = errors.New("invalid authentication fields")
	ErrInvalidIdentityFile   = errors.New("invalid identity file")
	ErrInvalidCredentialRef  = errors.New("invalid credential reference")
)

// AuthMethod identifies the non-secret material used to authenticate a connection.
type AuthMethod uint8

const (
	authMethodInvalid AuthMethod = iota
	AuthMethodAgent
	AuthMethodKey
	AuthMethodPassword
)

func ParseAuthMethod(value string) (AuthMethod, error) {
	switch value {
	case "agent":
		return AuthMethodAgent, nil
	case "key":
		return AuthMethodKey, nil
	case "password":
		return AuthMethodPassword, nil
	default:
		return authMethodInvalid, invalid("auth_method", ErrInvalidAuthMethod)
	}
}

func (method AuthMethod) String() string {
	switch method {
	case AuthMethodAgent:
		return "agent"
	case AuthMethodKey:
		return "key"
	case AuthMethodPassword:
		return "password"
	default:
		return ""
	}
}

func (method AuthMethod) Valid() bool {
	return method == AuthMethodAgent || method == AuthMethodKey || method == AuthMethodPassword
}

// ConnectionDetails contains only catalog-safe SSH connection fields. CredentialRef
// is an opaque identifier for a platform credential store, never secret material.
type ConnectionDetails struct {
	host          string
	port          uint16
	username      string
	authMethod    AuthMethod
	identityFile  string
	credentialRef string
}

// NewConnectionDetails validates authentication field combinations. A zero port
// denotes an omitted port and receives the SSH default.
func NewConnectionDetails(host string, port uint64, username string, authMethod AuthMethod, identityFile, credentialRef string) (ConnectionDetails, error) {
	if !validRequiredText(host) {
		return ConnectionDetails{}, invalid("host", ErrInvalidHost)
	}
	if port == 0 {
		port = uint64(DefaultSSHPort)
	}
	if port > 65535 {
		return ConnectionDetails{}, invalid("port", ErrInvalidPort)
	}
	if username != "" && !validText(username) {
		return ConnectionDetails{}, invalid("username", ErrInvalidUsername)
	}
	if !authMethod.Valid() {
		return ConnectionDetails{}, invalid("auth_method", ErrInvalidAuthMethod)
	}

	switch authMethod {
	case AuthMethodAgent:
		if identityFile != "" || credentialRef != "" {
			return ConnectionDetails{}, invalid("authentication", ErrInvalidAuthentication)
		}
	case AuthMethodKey:
		if strings.TrimSpace(identityFile) == "" || credentialRef != "" {
			return ConnectionDetails{}, invalid("authentication", ErrInvalidAuthentication)
		}
	case AuthMethodPassword:
		if identityFile != "" {
			return ConnectionDetails{}, invalid("authentication", ErrInvalidAuthentication)
		}
	}

	if identityFile != "" && !validText(identityFile) {
		return ConnectionDetails{}, invalid("identity_file", ErrInvalidIdentityFile)
	}
	if credentialRef != "" {
		if _, err := ParseID(credentialRef); err != nil {
			return ConnectionDetails{}, invalid("credential_ref", ErrInvalidCredentialRef)
		}
	}

	return ConnectionDetails{
		host:          host,
		port:          uint16(port),
		username:      username,
		authMethod:    authMethod,
		identityFile:  identityFile,
		credentialRef: credentialRef,
	}, nil
}

func (details ConnectionDetails) Host() string {
	return details.host
}

func (details ConnectionDetails) Port() uint16 {
	return details.port
}

func (details ConnectionDetails) Username() string {
	return details.username
}

func (details ConnectionDetails) AuthMethod() AuthMethod {
	return details.authMethod
}

func (details ConnectionDetails) IdentityFile() string {
	return details.identityFile
}

func (details ConnectionDetails) CredentialRef() string {
	return details.credentialRef
}

func validRequiredText(value string) bool {
	return strings.TrimSpace(value) != "" && validText(value)
}

func validText(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}
