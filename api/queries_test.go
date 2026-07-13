package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockRPCServer returns a test server that routes JSON-RPC requests by the
// "method" field in the request body, returning the corresponding canned
// response. This makes G2/G1 tests deterministic and offline (no dependency on
// api.steemit.com). The returned server is closed automatically via t.Cleanup.
//
// Each entry in responses maps an "<apiName>.<method>" string to a JSON-RPC
// response object (will be marshalled as the full {jsonrpc,result} envelope,
// with the result field taken verbatim from the provided value).
func mockRPCServer(t *testing.T, responses map[string]interface{}) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var req struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		result, ok := responses[req.Method]
		if !ok {
			http.Error(w, "no mock for method: "+req.Method, http.StatusBadRequest)
			return
		}
		// Emit a minimal JSON-RPC 2.0 success envelope with the canned result.
		envelope := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  result,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(envelope)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestGetAccounts(t *testing.T) {
	// Real-shaped condenser_api.get_accounts response: an array with one
	// ExtendedAccount whose posting authority has a single key_auth pair
	// ["STM...", 1]. This also exercises steemutil's KeyAuth.UnmarshalJSON
	// (nested array-of-pairs flattening).
	server := mockRPCServer(t, map[string]interface{}{
		"condenser_api.get_accounts": []map[string]interface{}{
			{
				"name":    "testaccount",
				"created": "2016-01-01T00:00:00Z",
				"posting": map[string]interface{}{
					"weight_threshold": 1,
					"account_auths":    []interface{}{},
					"key_auths":        []interface{}{[]interface{}{"STM7jNh5ejQoqHqWcGWFJ1v4F5CzsG3EiBuz1VooCng1cH5QpJD27", 1}},
				},
				"active": map[string]interface{}{
					"weight_threshold": 1,
					"account_auths":    []interface{}{},
					"key_auths":        []interface{}{[]interface{}{"STM7W7ACQDZJZ6rZGKeT9auipnSiSxFxJ4k71QXmrhY9HbvYsNnQ2", 1}},
				},
				"balance":       "1000.000 STEEM",
				"voting_power":  9000,
				"reputation":    "1234567890",
			},
		},
	})
	api := NewAPI(server.URL)

	accts, err := api.GetAccounts([]string{"testaccount"})
	if err != nil {
		t.Fatalf("GetAccounts failed: %v", err)
	}
	if len(accts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(accts))
	}
	if accts[0].Name != "testaccount" {
		t.Errorf("expected name testaccount, got %s", accts[0].Name)
	}
	// Verify key_auths nested-array parsing (the load-bearing wire quirk).
	if len(accts[0].Posting.KeyAuths) != 1 {
		t.Fatalf("expected 1 posting key_auth, got %d", len(accts[0].Posting.KeyAuths))
	}
	if accts[0].Posting.KeyAuths[0].PubKey != "STM7jNh5ejQoqHqWcGWFJ1v4F5CzsG3EiBuz1VooCng1cH5QpJD27" {
		t.Errorf("unexpected posting pubkey: %s", accts[0].Posting.KeyAuths[0].PubKey)
	}
	if accts[0].Posting.KeyAuths[0].Weight != 1 {
		t.Errorf("expected posting key weight 1, got %d", accts[0].Posting.KeyAuths[0].Weight)
	}
	if accts[0].Posting.WeightThreshold != 1 {
		t.Errorf("expected weight_threshold 1, got %d", accts[0].Posting.WeightThreshold)
	}
}

func TestGetFollowCount(t *testing.T) {
	server := mockRPCServer(t, map[string]interface{}{
		"condenser_api.get_follow_count": map[string]interface{}{
			"account":         "testaccount",
			"follower_count":  42,
			"following_count": 7,
		},
	})
	api := NewAPI(server.URL)

	fc, err := api.GetFollowCount("testaccount")
	if err != nil {
		t.Fatalf("GetFollowCount failed: %v", err)
	}
	if fc.Account != "testaccount" {
		t.Errorf("expected account testaccount, got %s", fc.Account)
	}
	if fc.FollowerCount != 42 {
		t.Errorf("expected 42 followers, got %d", fc.FollowerCount)
	}
	if fc.FollowingCount != 7 {
		t.Errorf("expected 7 following, got %d", fc.FollowingCount)
	}
}

func TestGetFollowers(t *testing.T) {
	server := mockRPCServer(t, map[string]interface{}{
		"condenser_api.get_followers": []map[string]interface{}{
			{"follower": "alice", "following": "testaccount", "what": []string{"blog"}},
			{"follower": "bob", "following": "testaccount", "what": []string{"blog"}},
		},
	})
	api := NewAPI(server.URL)

	followers, err := api.GetFollowers("testaccount", "", "blog", 100)
	if err != nil {
		t.Fatalf("GetFollowers failed: %v", err)
	}
	if len(followers) != 2 {
		t.Fatalf("expected 2 followers, got %d", len(followers))
	}
	if followers[0].Follower != "alice" {
		t.Errorf("expected first follower alice, got %s", followers[0].Follower)
	}
	if len(followers[1].What) != 1 || followers[1].What[0] != "blog" {
		t.Errorf("expected what=[blog], got %v", followers[1].What)
	}
}

func TestGetFollowing(t *testing.T) {
	server := mockRPCServer(t, map[string]interface{}{
		"condenser_api.get_following": []map[string]interface{}{
			{"follower": "testaccount", "following": "carol", "what": []string{"blog"}},
		},
	})
	api := NewAPI(server.URL)

	following, err := api.GetFollowing("testaccount", "", "blog", 100)
	if err != nil {
		t.Fatalf("GetFollowing failed: %v", err)
	}
	if len(following) != 1 {
		t.Fatalf("expected 1 following, got %d", len(following))
	}
	if following[0].Following != "carol" {
		t.Errorf("expected following carol, got %s", following[0].Following)
	}
}

func TestLookupAccounts(t *testing.T) {
	server := mockRPCServer(t, map[string]interface{}{
		"condenser_api.lookup_accounts": []string{"alice", "alice2", "alicetest"},
	})
	api := NewAPI(server.URL)

	names, err := api.LookupAccounts("alic", 10)
	if err != nil {
		t.Fatalf("LookupAccounts failed: %v", err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "alice" {
		t.Errorf("expected first name alice, got %s", names[0])
	}
}

func TestGetOrderBook(t *testing.T) {
	server := mockRPCServer(t, map[string]interface{}{
		"database_api.get_order_book": map[string]interface{}{
			"asks": []map[string]interface{}{
				{"order_price": map[string]interface{}{"base": "1.000 SBD", "quote": "2.000 STEEM"}},
			},
			"bids": []map[string]interface{}{
				{"order_price": map[string]interface{}{"base": "0.400 SBD", "quote": "1.000 STEEM"}},
			},
		},
	})
	api := NewAPI(server.URL)

	ob, err := api.GetOrderBook(1)
	if err != nil {
		t.Fatalf("GetOrderBook failed: %v", err)
	}
	if len(ob.Asks) != 1 || len(ob.Bids) != 1 {
		t.Fatalf("expected 1 ask + 1 bid, got %d asks, %d bids", len(ob.Asks), len(ob.Bids))
	}
	if ob.Asks[0].OrderPrice.Base != "1.000 SBD" {
		t.Errorf("expected ask base 1.000 SBD, got %s", ob.Asks[0].OrderPrice.Base)
	}
	if ob.Bids[0].OrderPrice.Quote != "1.000 STEEM" {
		t.Errorf("expected bid quote 1.000 STEEM, got %s", ob.Bids[0].OrderPrice.Quote)
	}
}

func TestGetFeedHistory(t *testing.T) {
	server := mockRPCServer(t, map[string]interface{}{
		"database_api.get_feed_history": map[string]interface{}{
			"price_history": []map[string]interface{}{
				{"base": "0.500 SBD", "quote": "1.000 STEEM"},
				{"base": "0.510 SBD", "quote": "1.000 STEEM"},
			},
		},
	})
	api := NewAPI(server.URL)

	fh, err := api.GetFeedHistory()
	if err != nil {
		t.Fatalf("GetFeedHistory failed: %v", err)
	}
	if len(fh.PriceHistory) != 2 {
		t.Fatalf("expected 2 price_history entries, got %d", len(fh.PriceHistory))
	}
	last := fh.PriceHistory[1]
	if last.Base != "0.510 SBD" {
		t.Errorf("expected last base 0.510 SBD, got %s", last.Base)
	}
}

func TestGetAccountHistory(t *testing.T) {
	// Real condenser_api.get_account_history wire shape: array of
	// [index, {"op":[type, payload], "timestamp": ts}] tuples. This exercises
	// AccountHistoryEntry.UnmarshalJSON (the local custom unmarshaler).
	server := mockRPCServer(t, map[string]interface{}{
		"condenser_api.get_account_history": []interface{}{
			[]interface{}{float64(10), map[string]interface{}{
				"op": []interface{}{
					"transfer",
					map[string]interface{}{
						"from":   "alice",
						"to":     "bob",
						"amount": "1.000 STEEM",
						"memo":   "hello",
					},
				},
				"timestamp": "2026-01-01T00:00:00Z",
			}},
			[]interface{}{float64(9), map[string]interface{}{
				"op": []interface{}{
					"vote",
					map[string]interface{}{
						"voter":    "alice",
						"author":   "carol",
						"permlink": "post1",
						"weight":   float64(10000),
					},
				},
				"timestamp": "2026-01-02T00:00:00Z",
			}},
		},
	})
	api := NewAPI(server.URL)

	entries, err := api.GetAccountHistory("bob", -1, 1000)
	if err != nil {
		t.Fatalf("GetAccountHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// First entry: transfer op at index 10.
	if entries[0].Index != 10 {
		t.Errorf("expected index 10, got %d", entries[0].Index)
	}
	if entries[0].Op.Type != "transfer" {
		t.Errorf("expected op type transfer, got %s", entries[0].Op.Type)
	}
	if entries[0].Op.Payload["to"] != "bob" {
		t.Errorf("expected transfer to bob, got %v", entries[0].Op.Payload["to"])
	}
	if entries[0].Timestamp != "2026-01-01T00:00:00Z" {
		t.Errorf("unexpected timestamp: %s", entries[0].Timestamp)
	}

	// Second entry: vote op at index 9.
	if entries[1].Index != 9 {
		t.Errorf("expected index 9, got %d", entries[1].Index)
	}
	if entries[1].Op.Type != "vote" {
		t.Errorf("expected op type vote, got %s", entries[1].Op.Type)
	}
	if entries[1].Op.Payload["permlink"] != "post1" {
		t.Errorf("expected permlink post1, got %v", entries[1].Op.Payload["permlink"])
	}
}

// TestAccountHistoryEntry_UnmarshalJSON_Malformed guards the custom
// unmarshaler against malformed wire forms.
func TestAccountHistoryEntry_UnmarshalJSON_Malformed(t *testing.T) {
	cases := map[string]string{
		"not an array":            `"string"`,
		"single element":          `[10]`,
		"three elements":          `[10, {}, "extra"]`,
		"bad index":               `["notnum", {}]`,
		"op missing payload":      `[10, {"op":["transfer"], "timestamp":"x"}]`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			var e AccountHistoryEntry
			if err := e.UnmarshalJSON([]byte(raw)); err == nil {
				t.Errorf("expected error for %s, got nil", name)
			}
		})
	}

	// null should leave the entry at zero value (standard json behavior).
	var e AccountHistoryEntry
	if err := e.UnmarshalJSON([]byte("null")); err != nil {
		t.Errorf("null should not error, got: %v", err)
	}
	if e.Index != 0 || e.Op.Type != "" {
		t.Errorf("null should leave zero value, got %+v", e)
	}
}
