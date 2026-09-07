package domain

import (
	"crypto/rand"
	"encoding/hex"
)

const idSize = 16

// ID is an immutable 128-bit identifier.
type ID struct {
	value [idSize]byte
}

// NewID returns a cryptographically random, non-zero identifier.
func NewID() (ID, error) {
	for {
		var id ID
		if _, err := rand.Read(id.value[:]); err != nil {
			return ID{}, err
		}
		if !id.IsZero() {
			return id, nil
		}
	}
}

// ParseID accepts only the 32-character canonical lowercase hexadecimal form.
func ParseID(text string) (ID, error) {
	if len(text) != hex.EncodedLen(idSize) {
		return ID{}, invalid("id", ErrInvalidID)
	}
	for _, character := range text {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return ID{}, invalid("id", ErrInvalidID)
		}
	}

	var id ID
	if _, err := hex.Decode(id.value[:], []byte(text)); err != nil {
		return ID{}, invalid("id", ErrInvalidID)
	}
	return id, nil
}

func (id ID) String() string {
	return hex.EncodeToString(id.value[:])
}

// Bytes returns a copy of the identifier bytes.
func (id ID) Bytes() [idSize]byte {
	return id.value
}

func (id ID) IsZero() bool {
	return id == ID{}
}

func (id ID) MarshalText() ([]byte, error) {
	return []byte(id.String()), nil
}
