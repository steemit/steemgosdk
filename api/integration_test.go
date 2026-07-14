//go:build integration

package api

import (
	"testing"

	protocolapi "github.com/steemit/steemutil/protocol/api"
)

// Integration tests against a real Steem node (https://api.steemit.com).
//
// Run with: go test -tags=integration ./api/...
// These are gated behind the "integration" build tag so CI does not depend on
// network availability or a live node. They verify that the wire shapes this
// SDK emits are actually accepted by a real steemd, and that the returned
// fields align with steemutil's structs.
//
// What these tests verify (the non-blocking follow-up from the PR audit):
//   - condenser_api.get_order_book accepts our positional-array [limit] param
//     and returns OrderBook fields (order_price.base/quote as string-assets)
//     that unmarshal into protocolapi.OrderBook.
//   - condenser_api.get_feed_history accepts our empty positional-array param
//     and returns price_history entries that unmarshal into protocolapi.FeedHistory.
//   - condenser_api.get_dynamic_global_properties returns the vesting fields
//     needed by ComputePrices.
//   - steemutil's ComputePrices runs end-to-end on real data without error.

const integrationNodeURL = "https://api.steemit.com"

func TestIntegration_GetOrderBook(t *testing.T) {
	api := NewAPI(integrationNodeURL)

	ob, err := api.GetOrderBook(5)
	if err != nil {
		t.Fatalf("GetOrderBook against real node failed: %v", err)
	}
	if len(ob.Asks) == 0 || len(ob.Bids) == 0 {
		t.Fatalf("expected non-empty order book, got %d asks / %d bids", len(ob.Asks), len(ob.Bids))
	}
	// order_price.base/quote must be string-assets like "400.000 STEEM".
	first := ob.Asks[0].OrderPrice
	if first.Base == "" || first.Quote == "" {
		t.Errorf("expected non-empty order_price base/quote, got base=%q quote=%q", first.Base, first.Quote)
	}
	// Each side must carry a symbol (STEEM or SBD) — a missing symbol means
	// the wire shape drifted from the struct.
	for i, o := range ob.Asks {
		bp, e := protocolapi.ParseAsset(o.OrderPrice.Base)
		if e != nil {
			t.Errorf("ask[%d].order_price.base not a parseable asset: %v (%q)", i, e, o.OrderPrice.Base)
		}
		qp, e := protocolapi.ParseAsset(o.OrderPrice.Quote)
		if e != nil {
			t.Errorf("ask[%d].order_price.quote not a parseable asset: %v (%q)", i, e, o.OrderPrice.Quote)
		}
		if bp.Symbol == qp.Symbol {
			t.Errorf("ask[%d] base/quote have same symbol %s", i, bp.Symbol)
		}
	}
}

func TestIntegration_GetFeedHistory(t *testing.T) {
	api := NewAPI(integrationNodeURL)

	fh, err := api.GetFeedHistory()
	if err != nil {
		t.Fatalf("GetFeedHistory against real node failed: %v", err)
	}
	if len(fh.PriceHistory) == 0 {
		t.Fatal("expected non-empty price_history")
	}
	// Last entry is what ComputePrices(SteemUsd) uses; verify it parses.
	last := fh.PriceHistory[len(fh.PriceHistory)-1]
	if _, err := protocolapi.ParsePrice(last.Base, last.Quote); err != nil {
		t.Fatalf("last price_history entry not a parseable price: %v (base=%q quote=%q)", err, last.Base, last.Quote)
	}
}

func TestIntegration_ComputePrices_EndToEnd(t *testing.T) {
	api := NewAPI(integrationNodeURL)

	ob, err := api.GetOrderBook(5)
	if err != nil {
		t.Fatalf("GetOrderBook failed: %v", err)
	}
	fh, err := api.GetFeedHistory()
	if err != nil {
		t.Fatalf("GetFeedHistory failed: %v", err)
	}
	dgp, err := api.GetDynamicGlobalProperties()
	if err != nil {
		t.Fatalf("GetDynamicGlobalProperties failed: %v", err)
	}
	// Verify the vesting fields are present (string-asset form).
	if _, err := protocolapi.ParseAsset(dgp.TotalVestingFundSteem); err != nil {
		t.Fatalf("total_vesting_fund_steem not a parseable asset: %v (%q)", err, dgp.TotalVestingFundSteem)
	}
	if _, err := protocolapi.ParseAsset(dgp.TotalVestingShares); err != nil {
		t.Fatalf("total_vesting_shares not a parseable asset: %v (%q)", err, dgp.TotalVestingShares)
	}

	res, err := protocolapi.ComputePrices(ob, fh, dgp)
	if err != nil {
		t.Fatalf("ComputePrices on real data failed: %v", err)
	}

	// Sanity: 1.000 STEEM converted via SteemVest should yield a positive
	// VESTS amount (the real ratio is ~1 STEEM -> thousands of VESTS).
	oneSteem, _ := protocolapi.ParseAsset("1.000 STEEM")
	vest, err := res.SteemVest.Convert(oneSteem)
	if err != nil {
		t.Fatalf("Convert(1 STEEM) via SteemVest failed: %v", err)
	}
	if vest.Amount <= 0 {
		t.Errorf("expected positive VESTS for 1 STEEM, got %d", vest.Amount)
	}
	if vest.Symbol != "VESTS" {
		t.Errorf("expected VESTS symbol, got %s", vest.Symbol)
	}
	t.Logf("1.000 STEEM -> %s (SteemSbd=%s)", vest.String(), res.SteemSbd.String())
}
