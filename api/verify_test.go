package api

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/steemit/steemutil/rpc"
)

// G1 verification tests use a fixed key pair (sourced from steemutil's
// wif/testdata.go data[0]):
//
//	WIF       5JWHY5DxTF6qN5grTtChDCYBmWHfY9zaSsw4CxEKN5eZpH9iBma
//	PublicKey STM7jNh5ejQoqHqWcGWFJ1v4F5CzsG3EiBuz1VooCng1cH5QpJD27
//
// The mock get_accounts returns this pubkey as the account's single posting
// key_auth with weight 1 (clearing the weight_threshold of 1). rpc.Sign signs
// with the matching WIF, so VerifySignedRequest must succeed.
const (
	g1TestWIF    = "5JWHY5DxTF6qN5grTtChDCYBmWHfY9zaSsw4CxEKN5eZpH9iBma"
	g1TestPubKey = "STM7jNh5ejQoqHqWcGWFJ1v4F5CzsG3EiBuz1VooCng1cH5QpJD27"
	g1TestAcct   = "testaccount"
)

// mockAccountResponse builds a get_accounts response for one account with a
// single posting key_auth. weightThreshold and keyWeight control whether the
// key clears the threshold; keyAuths controls the count (for multisig tests).
func mockAccountResponse(name, pubKey string, weightThreshold uint32, keyWeight int, keyAuths int) []map[string]interface{} {
	auths := make([]interface{}, 0, keyAuths)
	for i := 0; i < keyAuths; i++ {
		// Use the same pubKey for all entries; count is what matters here.
		// For the single-key happy path keyAuths==1.
		auths = append(auths, []interface{}{pubKey, keyWeight})
	}
	return []map[string]interface{}{
		{
			"name": name,
			"posting": map[string]interface{}{
				"weight_threshold": weightThreshold,
				"account_auths":    []interface{}{},
				"key_auths":        auths,
			},
		},
	}
}

func TestVerifySignedRequest_Success(t *testing.T) {
	server := mockRPCServer(t, map[string]interface{}{
		"condenser_api.get_accounts": mockAccountResponse(g1TestAcct, g1TestPubKey, 1, 1, 1),
	})
	api := NewAPI(server.URL)

	// Sign a request with the matching WIF. rpc.Sign uses the current time, so
	// the 60s freshness window is satisfied when we validate immediately after.
	signedReq, err := rpc.Sign(
		&rpc.RpcRequest{
			Method: "some_method",
			Params: []interface{}{"hello", 42},
			ID:     1,
		},
		g1TestAcct,
		[]string{g1TestWIF},
	)
	if err != nil {
		t.Fatalf("rpc.Sign failed: %v", err)
	}

	params, acct, err := api.VerifySignedRequest(signedReq)
	if err != nil {
		t.Fatalf("VerifySignedRequest failed: %v", err)
	}
	if acct != g1TestAcct {
		t.Errorf("expected account %s, got %s", g1TestAcct, acct)
	}
	// The decoded plaintext params should round-trip to what we signed.
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(params))
	}
	if params[0] != "hello" {
		t.Errorf("expected params[0]=hello, got %v", params[0])
	}
}

func TestVerifySignedRequest_TamperedSignature(t *testing.T) {
	server := mockRPCServer(t, map[string]interface{}{
		"condenser_api.get_accounts": mockAccountResponse(g1TestAcct, g1TestPubKey, 1, 1, 1),
	})
	api := NewAPI(server.URL)

	signedReq, err := rpc.Sign(
		&rpc.RpcRequest{
			Method: "some_method",
			Params: []interface{}{"hello"},
			ID:     1,
		},
		g1TestAcct,
		[]string{g1TestWIF},
	)
	if err != nil {
		t.Fatalf("rpc.Sign failed: %v", err)
	}

	// Flip the first hex byte of the signature so the recovered pubkey no
	// longer matches. (We change a byte in the r/s body, not the recovery id,
	// so the signature is still well-formed but cryptographically wrong.)
	sig := signedReq.Params.Signed.Signatures[0]
	sigBytes, err := hex.DecodeString(sig)
	if err != nil {
		t.Fatalf("hex decode failed: %v", err)
	}
	sigBytes[1] ^= 0xFF // flip one byte in r
	signedReq.Params.Signed.Signatures[0] = hex.EncodeToString(sigBytes)

	_, _, err = api.VerifySignedRequest(signedReq)
	if err == nil {
		t.Fatal("expected verification to fail with tampered signature, got nil")
	}
	// steemutil wraps the inner error; the underlying message should mention
	// verification/signature.
	if !strings.Contains(strings.ToLower(err.Error()), "invalid signature") &&
		!strings.Contains(strings.ToLower(err.Error()), "verification failed") {
		t.Errorf("expected signature-related error, got: %v", err)
	}
}

func TestVerifySignedRequest_WrongAccount(t *testing.T) {
	// get_accounts returns a DIFFERENT posting pubkey than the one the WIF
	// corresponds to, so recovery won't match.
	wrongPub := "STM7W7ACQDZJZ6rZGKeT9auipnSiSxFxJ4k71QXmrhY9HbvYsNnQ2"
	server := mockRPCServer(t, map[string]interface{}{
		"condenser_api.get_accounts": mockAccountResponse(g1TestAcct, wrongPub, 1, 1, 1),
	})
	api := NewAPI(server.URL)

	signedReq, err := rpc.Sign(
		&rpc.RpcRequest{
			Method: "some_method",
			Params: []interface{}{"hello"},
			ID:     1,
		},
		g1TestAcct,
		[]string{g1TestWIF},
	)
	if err != nil {
		t.Fatalf("rpc.Sign failed: %v", err)
	}

	_, _, err = api.VerifySignedRequest(signedReq)
	if err == nil {
		t.Fatal("expected verification to fail with wrong account pubkey, got nil")
	}
}

func TestVerifySignedRequest_MultisigRejected(t *testing.T) {
	// Steem's verifier only supports accounts with a single posting key.
	// Two key_auths -> "Unsupported posting key configuration".
	server := mockRPCServer(t, map[string]interface{}{
		"condenser_api.get_accounts": mockAccountResponse(g1TestAcct, g1TestPubKey, 1, 1, 2),
	})
	api := NewAPI(server.URL)

	signedReq, err := rpc.Sign(
		&rpc.RpcRequest{
			Method: "some_method",
			Params: []interface{}{"hello"},
			ID:     1,
		},
		g1TestAcct,
		[]string{g1TestWIF},
	)
	if err != nil {
		t.Fatalf("rpc.Sign failed: %v", err)
	}

	_, _, err = api.VerifySignedRequest(signedReq)
	if err == nil {
		t.Fatal("expected failure for multi-posting-key account, got nil")
	}
	if !strings.Contains(err.Error(), "Unsupported") {
		t.Errorf("expected 'Unsupported posting key configuration' error, got: %v", err)
	}
}

func TestVerifySignedRequest_AccountNotFound(t *testing.T) {
	// get_accounts returns an empty array -> fetcher returns "no such account",
	// which VerifySignedRpc reports as "No such account".
	server := mockRPCServer(t, map[string]interface{}{
		"condenser_api.get_accounts": []interface{}{},
	})
	api := NewAPI(server.URL)

	signedReq, err := rpc.Sign(
		&rpc.RpcRequest{
			Method: "some_method",
			Params: []interface{}{"hello"},
			ID:     1,
		},
		g1TestAcct,
		[]string{g1TestWIF},
	)
	if err != nil {
		t.Fatalf("rpc.Sign failed: %v", err)
	}

	_, _, err = api.VerifySignedRequest(signedReq)
	if err == nil {
		t.Fatal("expected failure for account not found, got nil")
	}
	if !strings.Contains(err.Error(), "No such account") {
		t.Errorf("expected 'No such account' error, got: %v", err)
	}
}

func TestVerifySignedRequest_ShortAccountName(t *testing.T) {
	// Account names shorter than 3 chars are rejected by VerifySignedRpc with
	// "Invalid account name". No get_accounts mock needed (fetcher is never
	// reached), but provide one anyway so a missing-mock error doesn't mask
	// the real assertion if the check ordering ever changes.
	server := mockRPCServer(t, map[string]interface{}{
		"condenser_api.get_accounts": mockAccountResponse("ab", g1TestPubKey, 1, 1, 1),
	})
	api := NewAPI(server.URL)

	signedReq, err := rpc.Sign(
		&rpc.RpcRequest{
			Method: "some_method",
			Params: []interface{}{"hello"},
			ID:     1,
		},
		"ab", // 2 chars, below the 3-char minimum
		[]string{g1TestWIF},
	)
	if err != nil {
		t.Fatalf("rpc.Sign failed: %v", err)
	}

	_, _, err = api.VerifySignedRequest(signedReq)
	if err == nil {
		t.Fatal("expected failure for short account name, got nil")
	}
	if !strings.Contains(err.Error(), "Invalid account name") {
		t.Errorf("expected 'Invalid account name' error, got: %v", err)
	}
}
