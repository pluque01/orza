package domain

import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Name is an immutable, case-sensitive node name. Its exact input is preserved.
type Name struct {
	value string
}

func NewName(value string) (Name, error) {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" || strings.ContainsRune(value, '/') {
		return Name{}, invalid("name", ErrInvalidName)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return Name{}, invalid("name", ErrInvalidName)
		}
	}
	return Name{value: value}, nil
}

func (name Name) String() string {
	return name.value
}

// NodeKind distinguishes folders from SSH connections.
type NodeKind uint8

const (
	nodeKindInvalid NodeKind = iota
	NodeKindFolder
	NodeKindConnection
)

func ParseNodeKind(value string) (NodeKind, error) {
	switch value {
	case "folder":
		return NodeKindFolder, nil
	case "connection":
		return NodeKindConnection, nil
	default:
		return nodeKindInvalid, invalid("kind", ErrInvalidNodeKind)
	}
}

func (kind NodeKind) String() string {
	switch kind {
	case NodeKindFolder:
		return "folder"
	case NodeKindConnection:
		return "connection"
	default:
		return ""
	}
}

func (kind NodeKind) Valid() bool {
	return kind == NodeKindFolder || kind == NodeKindConnection
}

func (kind NodeKind) MarshalText() ([]byte, error) {
	if !kind.Valid() {
		return nil, invalid("kind", ErrInvalidNodeKind)
	}
	return []byte(kind.String()), nil
}

// Revision is an immutable optimistic-concurrency revision.
type Revision struct {
	value uint64
}

func InitialRevision() Revision {
	return Revision{value: 1}
}

func NewRevision(value uint64) (Revision, error) {
	if value == 0 {
		return Revision{}, invalid("revision", ErrInvalidRevision)
	}
	return Revision{value: value}, nil
}

func (revision Revision) Uint64() uint64 {
	return revision.value
}

func (revision Revision) String() string {
	return strconv.FormatUint(revision.value, 10)
}

func (revision Revision) Valid() bool {
	return revision.value > 0
}

func (revision Revision) Next() (Revision, error) {
	if !revision.Valid() {
		return Revision{}, invalid("revision", ErrInvalidRevision)
	}
	if revision.value == math.MaxUint64 {
		return Revision{}, invalid("revision", ErrRevisionOverflow)
	}
	return Revision{value: revision.value + 1}, nil
}
