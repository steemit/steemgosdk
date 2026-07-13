package api

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// AccountHistoryEntry models a single element of a
// condenser_api.get_account_history response.
//
// The wire form is a two-element JSON array: [index, body], where body itself
// is an object whose "op" field is [type, payload] and whose "timestamp" field
// is an ISO8601 string. Example:
//
//	[42, {"op":["transfer",{"from":"a","to":"b","amount":"1.000 STEEM"}],"timestamp":"2026-01-01T00:00:00Z"}]
//
// The custom UnmarshalJSON flattens this tuple into typed fields so callers can
// work with Index/Op/Timestamp directly rather than nested raw arrays.
type AccountHistoryEntry struct {
	Index     int64
	Op        AccountHistoryOp
	Timestamp string
}

// AccountHistoryOp is the [type, payload] pair carried in each account-history
// entry's "op" field. Payload is kept as a generic map because operation
// payloads vary by type (e.g. transfer carries to/amount/memo, vote carries
// voter/author/permlink/weight).
type AccountHistoryOp struct {
	Type    string
	Payload map[string]interface{}
}

// UnmarshalJSON parses the wire form [index, {"op":[type, payload], "timestamp": ts}].
func (e *AccountHistoryEntry) UnmarshalJSON(data []byte) error {
	// A JSON null leaves the entry at its zero value (standard json behavior).
	if string(data) == "null" {
		return nil
	}

	var tuple []json.RawMessage
	if err := json.Unmarshal(data, &tuple); err != nil {
		return errors.Wrap(err, "account_history entry must be a [index, body] array")
	}
	if len(tuple) != 2 {
		return errors.Errorf("account_history entry must have exactly 2 elements, got %d", len(tuple))
	}

	// tuple[0] -> Index
	if err := json.Unmarshal(tuple[0], &e.Index); err != nil {
		return errors.Wrap(err, "invalid account_history index")
	}

	// tuple[1] -> body object with "op" and "timestamp"
	var body struct {
		Op        []json.RawMessage `json:"op"`
		Timestamp string            `json:"timestamp"`
	}
	if err := json.Unmarshal(tuple[1], &body); err != nil {
		return errors.Wrap(err, "invalid account_history body")
	}

	e.Timestamp = body.Timestamp

	if len(body.Op) != 2 {
		return errors.Errorf("account_history op must have exactly 2 elements [type, payload], got %d", len(body.Op))
	}
	if err := json.Unmarshal(body.Op[0], &e.Op.Type); err != nil {
		return errors.Wrap(err, "invalid account_history op type")
	}
	// An operation payload is always a JSON object; decode into a generic map so
	// callers can read type-specific fields without a per-op type switch here.
	if err := json.Unmarshal(body.Op[1], &e.Op.Payload); err != nil {
		return errors.Wrap(err, "invalid account_history op payload")
	}

	return nil
}
