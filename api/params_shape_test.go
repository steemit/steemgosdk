package api

import (
	"encoding/json"
	"testing"
)

// params_shape_test.go asserts the EXACT wire shape each G2 method emits.
//
// Background: an earlier audit found that GetOrderBook/GetFeedHistory called
// database_api with positional-array params, but database_api handlers take
// named objects — so the calls would fail against a real node. The permissive
// mockRPCServer (which only routes by method) missed this because it never
// inspected params. These tests use mockRPCServerCapture to record params and
// assert each method sends the shape its target API actually accepts:
//
//   - condenser_api methods: positional arrays (condenser_api handlers are
//     vector<variant>, indexed as args[0], args[1], ...)
//   - element order matches the Steem condenser_api.cpp handler signatures
//
// This guards against regressions where a method is retargeted to database_api
// (which needs named objects) or params are reordered.

// emptyResult is a canned JSON-RPC result that satisfies every method's
// unmarshal target without needing per-method fixtures — these tests only care
// about the REQUEST shape, not the response.
var emptyResult = map[string]interface{}{}

func TestGetAccounts_ParamsShape(t *testing.T) {
	var captured []capturedRequest
	server := mockRPCServerCapture(t, map[string]interface{}{
		"condenser_api.get_accounts": []interface{}{},
	}, &captured)
	api := NewAPI(server.URL)

	_, _ = api.GetAccounts([]string{"alice", "bob"})

	if len(captured) != 1 {
		t.Fatalf("expected 1 request, got %d", len(captured))
	}
	c := captured[0]
	if c.Method != "condenser_api.get_accounts" {
		t.Fatalf("expected method condenser_api.get_accounts, got %s", c.Method)
	}
	// get_accounts takes [["n1","n2"]] — a positional array wrapping the names.
	arr := assertParamsArray(t, c.Params)
	if len(arr) != 1 {
		t.Fatalf("expected outer array of 1 element (the names array), got %d", len(arr))
	}
	inner, ok := arr[0].([]interface{})
	if !ok {
		t.Fatalf("expected arr[0] to be a nested array, got %T", arr[0])
	}
	if len(inner) != 2 || inner[0] != "alice" || inner[1] != "bob" {
		t.Errorf("expected inner [alice,bob], got %v", inner)
	}
}

func TestGetFollowCount_ParamsShape(t *testing.T) {
	var captured []capturedRequest
	server := mockRPCServerCapture(t, map[string]interface{}{
		"condenser_api.get_follow_count": emptyResult,
	}, &captured)
	api := NewAPI(server.URL)

	_, _ = api.GetFollowCount("alice")

	c := captured[0]
	if c.Method != "condenser_api.get_follow_count" {
		t.Fatalf("expected method condenser_api.get_follow_count, got %s", c.Method)
	}
	arr := assertParamsArray(t, c.Params)
	if len(arr) != 1 || arr[0] != "alice" {
		t.Errorf("expected [alice], got %v", arr)
	}
}

func TestGetFollowers_ParamsShape(t *testing.T) {
	var captured []capturedRequest
	server := mockRPCServerCapture(t, map[string]interface{}{
		"condenser_api.get_followers": []interface{}{},
	}, &captured)
	api := NewAPI(server.URL)

	_, _ = api.GetFollowers("alice", "", "blog", 100)

	c := captured[0]
	if c.Method != "condenser_api.get_followers" {
		t.Fatalf("expected method condenser_api.get_followers, got %s", c.Method)
	}
	// [account, start, followType, limit] — order matters (condenser_api.cpp:1750).
	arr := assertParamsArray(t, c.Params)
	if len(arr) != 4 {
		t.Fatalf("expected 4 params, got %d", len(arr))
	}
	if arr[0] != "alice" {
		t.Errorf("expected param[0]=alice (account), got %v", arr[0])
	}
	if arr[1] != "" {
		t.Errorf("expected param[1]=\"\" (start), got %v", arr[1])
	}
	if arr[2] != "blog" {
		t.Errorf("expected param[2]=blog (followType), got %v", arr[2])
	}
	// JSON numbers unmarshal to float64.
	if arr[3] != float64(100) {
		t.Errorf("expected param[3]=100 (limit), got %v", arr[3])
	}
}

func TestGetFollowing_ParamsShape(t *testing.T) {
	var captured []capturedRequest
	server := mockRPCServerCapture(t, map[string]interface{}{
		"condenser_api.get_following": []interface{}{},
	}, &captured)
	api := NewAPI(server.URL)

	_, _ = api.GetFollowing("alice", "bob", "blog", 50)

	c := captured[0]
	if c.Method != "condenser_api.get_following" {
		t.Fatalf("expected method condenser_api.get_following, got %s", c.Method)
	}
	arr := assertParamsArray(t, c.Params)
	if len(arr) != 4 || arr[0] != "alice" || arr[1] != "bob" || arr[2] != "blog" || arr[3] != float64(50) {
		t.Errorf("expected [alice,bob,blog,50], got %v", arr)
	}
}

func TestGetAccountHistory_ParamsShape(t *testing.T) {
	var captured []capturedRequest
	server := mockRPCServerCapture(t, map[string]interface{}{
		"condenser_api.get_account_history": []interface{}{},
	}, &captured)
	api := NewAPI(server.URL)

	_, _ = api.GetAccountHistory("alice", int64(-1), 1000)

	c := captured[0]
	if c.Method != "condenser_api.get_account_history" {
		t.Fatalf("expected method condenser_api.get_account_history, got %s", c.Method)
	}
	// [account, from, limit] — from=-1 means newest (conveyor accountHistoryGenerator page 1).
	arr := assertParamsArray(t, c.Params)
	if len(arr) != 3 {
		t.Fatalf("expected 3 params, got %d", len(arr))
	}
	if arr[0] != "alice" {
		t.Errorf("expected param[0]=alice, got %v", arr[0])
	}
	if arr[1] != float64(-1) {
		t.Errorf("expected param[1]=-1 (from), got %v", arr[1])
	}
	if arr[2] != float64(1000) {
		t.Errorf("expected param[2]=1000 (limit), got %v", arr[2])
	}
}

func TestLookupAccounts_ParamsShape(t *testing.T) {
	var captured []capturedRequest
	server := mockRPCServerCapture(t, map[string]interface{}{
		"condenser_api.lookup_accounts": []string{},
	}, &captured)
	api := NewAPI(server.URL)

	_, _ = api.LookupAccounts("alic", 10)

	c := captured[0]
	if c.Method != "condenser_api.lookup_accounts" {
		t.Fatalf("expected method condenser_api.lookup_accounts, got %s", c.Method)
	}
	// [lowerBound, limit] — limit is the SECOND param (condenser_api.cpp:919).
	arr := assertParamsArray(t, c.Params)
	if len(arr) != 2 {
		t.Fatalf("expected 2 params, got %d", len(arr))
	}
	if arr[0] != "alic" {
		t.Errorf("expected param[0]=alic (lowerBound), got %v", arr[0])
	}
	if arr[1] != float64(10) {
		t.Errorf("expected param[1]=10 (limit), got %v", arr[1])
	}
}

// TestGetOrderBook_ParamsShape is the regression test for the database_api bug.
// It MUST target condenser_api (positional array [limit]), NOT database_api
// (named object {"limit":N}) — the latter would fail on a real node because
// steemutil's RpcSendData.Params ([]any) can only emit arrays.
func TestGetOrderBook_ParamsShape(t *testing.T) {
	var captured []capturedRequest
	server := mockRPCServerCapture(t, map[string]interface{}{
		"condenser_api.get_order_book": emptyResult,
	}, &captured)
	api := NewAPI(server.URL)

	_, _ = api.GetOrderBook(500)

	c := captured[0]
	if c.Method != "condenser_api.get_order_book" {
		t.Fatalf("REGRESSION: expected condenser_api.get_order_book, got %s — "+
			"database_api takes a named object {\"limit\":N} which this SDK cannot emit", c.Method)
	}
	arr := assertParamsArray(t, c.Params)
	if len(arr) != 1 {
		t.Fatalf("expected [500], got %v", arr)
	}
	if arr[0] != float64(500) {
		t.Errorf("expected param[0]=500, got %v", arr[0])
	}
}

// TestGetFeedHistory_ParamsShape is the regression test for the database_api bug.
// It MUST target condenser_api with an empty positional array, NOT database_api
// (named object) — same reason as GetOrderBook.
func TestGetFeedHistory_ParamsShape(t *testing.T) {
	var captured []capturedRequest
	server := mockRPCServerCapture(t, map[string]interface{}{
		"condenser_api.get_feed_history": emptyResult,
	}, &captured)
	api := NewAPI(server.URL)

	_, _ = api.GetFeedHistory()

	c := captured[0]
	if c.Method != "condenser_api.get_feed_history" {
		t.Fatalf("REGRESSION: expected condenser_api.get_feed_history, got %s — "+
			"database_api takes a named object which this SDK cannot emit", c.Method)
	}
	// condenser_api.get_feed_history takes no args -> empty positional array [].
	arr := assertParamsArray(t, c.Params)
	if len(arr) != 0 {
		t.Errorf("expected empty params [], got %v", arr)
	}
}

// TestParamsShape_IsArrayNotObject is a meta-assertion that every G2 method
// emits a JSON array for params (never a bare object), since the SDK's
// jsonrpc2 layer (RpcSendData.Params []any) can only produce arrays. This is
// the structural invariant that constrains us to condenser_api. If this ever
// breaks, it means someone added a database_api call that needs a named object.
func TestParamsShape_IsArrayNotObject(t *testing.T) {
	cases := []struct {
		name   string
		method string
		call   func(a *API) error
	}{
		{"GetAccounts", "condenser_api.get_accounts", func(a *API) error { _, e := a.GetAccounts([]string{"x"}); return e }},
		{"GetFollowCount", "condenser_api.get_follow_count", func(a *API) error { _, e := a.GetFollowCount("x"); return e }},
		{"GetFollowers", "condenser_api.get_followers", func(a *API) error { _, e := a.GetFollowers("x", "", "blog", 1); return e }},
		{"GetFollowing", "condenser_api.get_following", func(a *API) error { _, e := a.GetFollowing("x", "", "blog", 1); return e }},
		{"GetAccountHistory", "condenser_api.get_account_history", func(a *API) error { _, e := a.GetAccountHistory("x", -1, 1); return e }},
		{"LookupAccounts", "condenser_api.lookup_accounts", func(a *API) error { _, e := a.LookupAccounts("x", 1); return e }},
		{"GetOrderBook", "condenser_api.get_order_book", func(a *API) error { _, e := a.GetOrderBook(1); return e }},
		{"GetFeedHistory", "condenser_api.get_feed_history", func(a *API) error { _, e := a.GetFeedHistory(); return e }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var captured []capturedRequest
			server := mockRPCServerCapture(t, map[string]interface{}{tc.method: emptyResult}, &captured)
			api := NewAPI(server.URL)

			_ = tc.call(api)

			if len(captured) != 1 {
				t.Fatalf("expected 1 request, got %d", len(captured))
			}
			// params must be a JSON array (starts with '['), never a bare object ('{').
			p := string(captured[0].Params)
			if len(p) == 0 || p[0] != '[' {
				t.Errorf("REGRESSION: %s must emit a JSON array for params, got %s — "+
					"a bare object means a database_api call was added, which this SDK cannot serve",
					tc.name, p)
			}
			// Also confirm it's valid JSON and decodes as an array.
			var check []interface{}
			if err := json.Unmarshal(captured[0].Params, &check); err != nil {
				t.Errorf("params is not a valid JSON array: %v (raw: %s)", err, p)
			}
		})
	}
}
