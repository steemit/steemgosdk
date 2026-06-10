package broadcast

import (
	"encoding/json"
	"testing"

	"github.com/steemit/steemutil/protocol"
)

func TestFeedPublishOperationFields(t *testing.T) {
	op := buildFeedPublishOperation("witness1", ExchangeRate{
		Base:  "0.500 SBD",
		Quote: "1.000 STEEM",
	})

	if op.Publisher != "witness1" {
		t.Fatalf("publisher = %q, want witness1", op.Publisher)
	}
	if op.ExchangeRate.Base != "0.500 SBD" {
		t.Fatalf("base = %q, want 0.500 SBD", op.ExchangeRate.Base)
	}
	if op.ExchangeRate.Quote != "1.000 STEEM" {
		t.Fatalf("quote = %q, want 1.000 STEEM", op.ExchangeRate.Quote)
	}
	if op.Type() != protocol.TypeFeedPublish {
		t.Fatalf("type = %q, want %q", op.Type(), protocol.TypeFeedPublish)
	}

	data, err := json.Marshal(op)
	if err != nil {
		t.Fatalf("marshal operation: %v", err)
	}
	payload := string(data)
	if payload == "" {
		t.Fatal("expected non-empty JSON payload")
	}
}
