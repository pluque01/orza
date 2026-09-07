package catalog

import (
	"database/sql"
	"fmt"
	"testing"
	"time"
)

func FuzzTrustedHostRowDecoding(f *testing.F) {
	f.Add("host.example", int64(22), int64(1), "2026-01-02T03:04:05Z", []byte("key"))
	f.Add("", int64(0), int64(0), "not-a-time", []byte(nil))
	f.Add("host", int64(65536), int64(-1), "", []byte{0xff})
	f.Fuzz(func(t *testing.T, host string, port, revision int64, accepted string, key []byte) {
		row := fuzzCatalogRow{
			"11111111111111111111111111111111", host, port, "ssh-ed25519", key,
			"SHA256:fuzz", revision, accepted,
		}
		decoded, err := scanTrustedHost(row)
		if err != nil {
			return
		}
		if port < 1 || port > 65535 || revision < 1 {
			t.Fatalf("scanTrustedHost accepted port=%d revision=%d", port, revision)
		}
		parsed, parseErr := time.Parse(time.RFC3339Nano, accepted)
		if parseErr != nil || decoded.CanonicalHost != host || decoded.Port != uint16(port) || decoded.Revision != uint64(revision) || !decoded.AcceptedAt.Equal(parsed) {
			t.Fatalf("decoded trusted host = %#v, parse error = %v", decoded, parseErr)
		}
	})
}

func FuzzCredentialOperationRowDecoding(f *testing.F) {
	f.Add(uint64(1), "save", "prepared", "2026-01-02T03:04:05.000000006Z", "", "new", "password", "")
	f.Add(uint64(0), "unknown", "", "bad-time", "old", "", "", "identity")
	f.Fuzz(func(t *testing.T, revision uint64, kind, phase, createdAt, oldRef, newRef, targetAuth, identity string) {
		row := fuzzCatalogRow{
			"11111111111111111111111111111111",
			"22222222222222222222222222222222",
			revision, kind, nullableFuzzValue(oldRef), nullableFuzzValue(newRef), phase,
			nullableFuzzValue(targetAuth), nullableFuzzValue(identity), createdAt,
		}
		decoded, err := scanCredentialOperation(row)
		if err != nil {
			return
		}
		parsed, parseErr := time.Parse(time.RFC3339Nano, createdAt)
		if parseErr != nil || decoded.ExpectedRevision != revision || decoded.Kind != kind || decoded.Phase != phase || !decoded.CreatedAt.Equal(parsed) {
			t.Fatalf("decoded credential operation = %#v, parse error = %v", decoded, parseErr)
		}
	})
}

func FuzzCredentialOperationStateValidation(f *testing.F) {
	f.Add("save", "prepared", uint64(1), "", "33333333333333333333333333333333", "password", "")
	f.Add("replace", "secret_changed", uint64(0), "old", "new", "key", "/key")
	f.Fuzz(func(t *testing.T, kind, phase string, revision uint64, oldRef, newRef, targetAuth, identity string) {
		operation := CredentialOperation{
			ID: "11111111111111111111111111111111", ConnectionID: "22222222222222222222222222222222",
			ExpectedRevision: revision, Kind: kind, OldRef: oldRef, NewRef: newRef, Phase: phase,
			TargetAuthMethod: targetAuth, TargetIdentityFile: identity, CreatedAt: time.Unix(1, 0),
		}
		if err := validateCredentialOperation(operation); err == nil {
			if phase != "prepared" || revision == 0 {
				t.Fatalf("invalid operation state accepted: %#v", operation)
			}
			switch kind {
			case "save", "replace", "remove", "delete_connection":
			default:
				t.Fatalf("unknown operation kind accepted: %q", kind)
			}
		}
	})
}

type fuzzCatalogRow []any

func (row fuzzCatalogRow) Scan(destinations ...any) error {
	if len(row) != len(destinations) {
		return fmt.Errorf("row has %d values for %d destinations", len(row), len(destinations))
	}
	for index, destination := range destinations {
		value := row[index]
		switch target := destination.(type) {
		case *string:
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("column %d is not text", index)
			}
			*target = text
		case *int64:
			number, ok := value.(int64)
			if !ok {
				return fmt.Errorf("column %d is not int64", index)
			}
			*target = number
		case *uint64:
			number, ok := value.(uint64)
			if !ok {
				return fmt.Errorf("column %d is not uint64", index)
			}
			*target = number
		case *[]byte:
			bytes, ok := value.([]byte)
			if !ok {
				return fmt.Errorf("column %d is not bytes", index)
			}
			*target = append((*target)[:0], bytes...)
		case *sql.NullString:
			switch value := value.(type) {
			case nil:
				*target = sql.NullString{}
			case string:
				*target = sql.NullString{String: value, Valid: true}
			default:
				return fmt.Errorf("column %d is not nullable text", index)
			}
		default:
			return fmt.Errorf("unsupported destination %T", destination)
		}
	}
	return nil
}

func nullableFuzzValue(value string) any {
	if value == "" {
		return nil
	}
	return value
}
