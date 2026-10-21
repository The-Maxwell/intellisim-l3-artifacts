package common

import (
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/common"
)

func TestParseFormatRoundTrip(t *testing.T) {
	oid := bson.NewObjectID()
	parsed, err := ParseID(FormatID(oid))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed != oid {
		t.Fatalf("round-trip mismatch: %v != %v", parsed, oid)
	}
}

func TestParseIDInvalid(t *testing.T) {
	for _, bad := range []string{"", "zzz", "123"} {
		if _, err := ParseID(bad); !errors.Is(err, common.ErrInvalidID) {
			t.Fatalf("ParseID(%q): want ErrInvalidID, got %v", bad, err)
		}
	}
}

func TestOptionalIDUnset(t *testing.T) {
	oid, err := ParseOptionalID("")
	if err != nil || !oid.IsZero() {
		t.Fatalf("ParseOptionalID(\"\"): got %v, %v", oid, err)
	}
	if got := FormatOptionalID(bson.NilObjectID); got != "" {
		t.Fatalf("FormatOptionalID(zero): got %q", got)
	}
}
