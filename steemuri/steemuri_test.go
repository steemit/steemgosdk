package steemuri

import (
	"strings"
	"testing"
)

var voteOp = []interface{}{"vote", map[string]interface{}{
	"voter": "foo", "author": "bar", "permlink": "baz", "weight": float64(10000),
}}

var transferOp = []interface{}{"transfer", map[string]interface{}{
	"from": "foo", "to": "bar", "amount": "10.000 STEEM", "memo": "",
}}

var resolveOpts = ResolveOptions{
	RefBlockNum:     1234,
	RefBlockPrefix:  5678900,
	Expiration:      "2020-01-01T00:00:00",
	Signers:         []string{"foo", "bar"},
	PreferredSigner: "foo",
}

func mustEncodeOp(t *testing.T, op interface{}, params Parameters, protocol EncodeProtocol) string {
	t.Helper()
	uri, err := EncodeOp(op, params, protocol)
	if err != nil {
		t.Fatal(err)
	}
	return uri
}

func TestEncodeOpDecodeRoundTrip(t *testing.T) {
	uri := mustEncodeOp(t, voteOp, Parameters{}, "")
	if !strings.HasPrefix(uri, "web+steem://sign/op/") {
		t.Fatalf("expected URI to start with web+steem://sign/op/, got %s", uri)
	}

	result, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}

	ops := result.Tx["operations"].([]interface{})
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}

	op := ops[0].([]interface{})
	if op[0] != "vote" {
		t.Fatalf("expected vote operation, got %v", op[0])
	}

	opData := op[1].(map[string]interface{})
	if opData["voter"] != "foo" {
		t.Fatalf("expected voter foo, got %v", opData["voter"])
	}

	if result.Params.Signer != "" || result.Params.Callback != "" || result.Params.NoBroadcast {
		t.Fatal("expected empty params")
	}
}

func TestEncodeOpWithProtocolSteem(t *testing.T) {
	uri := mustEncodeOp(t, voteOp, Parameters{}, ProtocolSteem)
	if !strings.HasPrefix(uri, "steem://sign/op/") {
		t.Fatalf("expected steem:// prefix, got %s", uri)
	}
	result, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}
	ops := result.Tx["operations"].([]interface{})
	op := ops[0].([]interface{})
	if op[0] != "vote" {
		t.Fatalf("expected vote, got %v", op[0])
	}
}

func TestEncodeOpWithProtocolExtSteem(t *testing.T) {
	uri := mustEncodeOp(t, voteOp, Parameters{}, ProtocolExtSteem)
	if !strings.HasPrefix(uri, "ext+steem://sign/op/") {
		t.Fatalf("expected ext+steem:// prefix, got %s", uri)
	}
	result, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}
	ops := result.Tx["operations"].([]interface{})
	op := ops[0].([]interface{})
	if op[0] != "vote" {
		t.Fatalf("expected vote, got %v", op[0])
	}
}

func TestEncodeOpsDecodeWithParams(t *testing.T) {
	params := Parameters{
		Callback: "https://example.com/wallet?tx={{id}}",
		Signer:   "foo",
	}
	ops := []interface{}{voteOp, transferOp}
	uri, err := EncodeOps(ops, params, "")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(uri, "web+steem://sign/ops/") {
		t.Fatalf("expected web+steem://sign/ops/ prefix, got %s", uri)
	}
	if !strings.Contains(uri, "?") {
		t.Fatal("expected query string in URI")
	}

	result, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}

	decodedOps := result.Tx["operations"].([]interface{})
	if len(decodedOps) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(decodedOps))
	}
	if result.Params.Signer != "foo" {
		t.Fatalf("expected signer foo, got %s", result.Params.Signer)
	}
	if result.Params.Callback != "https://example.com/wallet?tx={{id}}" {
		t.Fatalf("expected callback URL, got %s", result.Params.Callback)
	}
}

func TestEncodeTxDecodeRoundTrip(t *testing.T) {
	fullTx := map[string]interface{}{
		"ref_block_num":    1,
		"ref_block_prefix": 2,
		"expiration":       "2020-01-01T00:00:00",
		"extensions":       []interface{}{},
		"operations":       []interface{}{voteOp},
	}
	uri, err := EncodeTx(fullTx, Parameters{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(uri, "web+steem://sign/tx/") {
		t.Fatalf("expected web+steem://sign/tx/ prefix, got %s", uri)
	}

	result, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}

	if result.Tx["ref_block_num"].(float64) != 1 {
		t.Fatalf("expected ref_block_num 1, got %v", result.Tx["ref_block_num"])
	}
	if result.Tx["ref_block_prefix"].(float64) != 2 {
		t.Fatalf("expected ref_block_prefix 2, got %v", result.Tx["ref_block_prefix"])
	}
	ops := result.Tx["operations"].([]interface{})
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
}

func TestResolveTransactionReplacesPlaceholders(t *testing.T) {
	opWithPlaceholder := []interface{}{"vote", map[string]interface{}{
		"voter": "__signer", "author": "bar", "permlink": "baz", "weight": float64(10000),
	}}
	uri := mustEncodeOp(t, opWithPlaceholder, Parameters{}, "")
	decoded, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveTransaction(decoded.Tx, decoded.Params, resolveOpts)
	if err != nil {
		t.Fatal(err)
	}

	if resolved.Signer != "foo" {
		t.Fatalf("expected signer foo, got %s", resolved.Signer)
	}

	ops := resolved.Tx["operations"].([]interface{})
	op := ops[0].([]interface{})
	opData := op[1].(map[string]interface{})
	if opData["voter"] != "foo" {
		t.Fatalf("expected voter foo, got %v", opData["voter"])
	}

	if resolved.Tx["ref_block_num"] != "1234" {
		t.Fatalf("expected ref_block_num 1234, got %v", resolved.Tx["ref_block_num"])
	}
	if resolved.Tx["ref_block_prefix"] != "5678900" {
		t.Fatalf("expected ref_block_prefix 5678900, got %v", resolved.Tx["ref_block_prefix"])
	}
	if resolved.Tx["expiration"] != "2020-01-01T00:00:00" {
		t.Fatalf("expected expiration 2020-01-01T00:00:00, got %v", resolved.Tx["expiration"])
	}
}

func TestResolveTransactionSignerNotAvailable(t *testing.T) {
	uri := mustEncodeOp(t, voteOp, Parameters{Signer: "baz"}, "")
	decoded, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ResolveTransaction(decoded.Tx, decoded.Params, resolveOpts)
	if err == nil {
		t.Fatal("expected error for unavailable signer")
	}
	if !strings.Contains(err.Error(), "signer 'baz' not available") {
		t.Fatalf("unexpected error message: %s", err.Error())
	}
}

func TestResolveCallback(t *testing.T) {
	u := "https://example.com/cb?sig={{sig}}&id={{id}}&block={{block}}&txn={{txn}}"
	ctx := TransactionConfirmation{Sig: "abc", ID: "def", Block: 100, Txn: 2}
	resolved := ResolveCallback(u, ctx)
	expected := "https://example.com/cb?sig=abc&id=def&block=100&txn=2"
	if resolved != expected {
		t.Fatalf("expected %s, got %s", expected, resolved)
	}
}

func TestDecodeInvalidProtocol(t *testing.T) {
	_, err := Decode("https://sign/op/x")
	if err == nil {
		t.Fatal("expected error for invalid protocol")
	}
	if !strings.Contains(err.Error(), "invalid protocol") {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestDecodeInvalidAction(t *testing.T) {
	_, err := Decode("web+steem://other/op/x")
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
	if !strings.Contains(err.Error(), "invalid action") {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}

func TestB64uRoundTrip(t *testing.T) {
	original := `{"test":"hello world","num":42}`
	encoded := b64uEnc(original)
	if strings.ContainsAny(encoded, "+/=") {
		t.Fatalf("b64u encoded string should not contain +, /, or =: %s", encoded)
	}
	decoded, err := b64uDec(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != original {
		t.Fatalf("expected %s, got %s", original, decoded)
	}
}

func TestNoBroadcastRoundTrip(t *testing.T) {
	uri := mustEncodeOp(t, voteOp, Parameters{NoBroadcast: true}, "")
	result, err := Decode(uri)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Params.NoBroadcast {
		t.Fatal("expected NoBroadcast to be true")
	}
}

func TestEncodeJSONError(t *testing.T) {
	// channels are not JSON-serializable
	ch := make(chan int)
	_, err := EncodeOp(ch, Parameters{}, "")
	if err == nil {
		t.Fatal("expected error for non-serializable type")
	}
	if !strings.Contains(err.Error(), "failed to marshal") {
		t.Fatalf("unexpected error: %s", err.Error())
	}
}
