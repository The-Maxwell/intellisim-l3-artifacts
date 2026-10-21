package common

import (
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/common"
)

// ParseID converts a hex entity id to an ObjectID. Malformed input (including "")
// yields common.ErrInvalidID (wrapped).
func ParseID(id string) (bson.ObjectID, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return bson.NilObjectID, fmt.Errorf("%w: %q", common.ErrInvalidID, id)
	}
	return oid, nil
}

// ParseOptionalID is ParseID with "" mapping to the zero ObjectID (= unset; with
// `_id,omitempty` the server generates an id on insert).
func ParseOptionalID(id string) (bson.ObjectID, error) {
	if id == "" {
		return bson.NilObjectID, nil
	}
	return ParseID(id)
}

// FormatID converts an ObjectID to its hex entity form.
func FormatID(id bson.ObjectID) string { return id.Hex() }

// FormatOptionalID converts an ObjectID to hex, mapping the zero ObjectID to "".
func FormatOptionalID(id bson.ObjectID) string {
	if id.IsZero() {
		return ""
	}
	return id.Hex()
}
