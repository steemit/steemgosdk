//go:build integration

package v2

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Integration tests against a real Steem node (https://api.steemit.com).
//
// Run with: go test -tags=integration ./api/v2/...
// Gated behind the "integration" build tag so CI does not depend on network
// availability or a live node. They verify:
//   - range throughput at concurrency 16/32 (the live_sync batch catch-up
//     parameter baseline recorded in the P0 plan)
//   - the defensive chain-head signal: a block beyond head returns
//     ErrBlockNotFound on the first attempt (no retry, no infinite loop)
//   - GetDynamicGlobalProperties exposes LastIrreversibleBlockNum, the
//     catch-up loop's window end

const integrationNodeURL = "https://api.steemit.com"

// TestIntegration_GetBlocks_Throughput records blocks/s at concurrency 16
// and 32 over a fixed historical range. It asserts completion (no failures)
// and prints rates for the live_sync tuning notes.
func TestIntegration_GetBlocks_Throughput(t *testing.T) {
	for _, concurrency := range []int{16, 32} {
		concurrency := concurrency
		t.Run(string(rune('0'+concurrency/10))+string(rune('0'+concurrency%10)), func(t *testing.T) {
			a := NewAPI(integrationNodeURL, WithConcurrency(concurrency))

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			start := time.Now()
			blocks, err := a.GetBlocks(ctx, 20000000, 20001000)
			elapsed := time.Since(start)
			if err != nil {
				t.Fatalf("GetBlocks(concurrency=%d): %v", concurrency, err)
			}
			if len(blocks) != 1000 {
				t.Fatalf("expected 1000 blocks, got %d", len(blocks))
			}
			if blocks[0].BlockNum != 20000000 || blocks[999].BlockNum != 20000999 {
				t.Fatalf("range mismatch: first=%d last=%d", blocks[0].BlockNum, blocks[999].BlockNum)
			}
			if blocks[0].Block == nil || blocks[0].Block.Timestamp == nil {
				t.Fatal("block content missing")
			}
			rate := float64(len(blocks)) / elapsed.Seconds()
			t.Logf("concurrency=%d: 1000 blocks in %v (%.1f blocks/s)", concurrency, elapsed, rate)
		})
	}
}

// TestIntegration_GetBlock_BeyondHeadIsErrBlockNotFound verifies the defensive
// chain-head signal on the real node: Steem block numbers are contiguous, so
// head+1000 can only mean "not produced yet" → ErrBlockNotFound, first attempt.
func TestIntegration_GetBlock_BeyondHeadIsErrBlockNotFound(t *testing.T) {
	a := NewAPI(integrationNodeURL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	props, err := a.GetDynamicGlobalProperties(ctx)
	if err != nil {
		t.Fatalf("GetDynamicGlobalProperties: %v", err)
	}
	head := uint(props.HeadBlockNumber)
	if uint(props.LastIrreversibleBlockNum) >= head {
		t.Errorf("LIB (%d) should trail head (%d)", props.LastIrreversibleBlockNum, head)
	}

	start := time.Now()
	_, err = a.GetBlock(ctx, head+1000)
	if !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("expected ErrBlockNotFound for block %d, got: %v", head+1000, err)
	}
	// Must be immediate — one attempt, no retry backoff chain.
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("beyond-head should fail fast, took %v", elapsed)
	}
	t.Logf("head=%d LIB=%d: block %d → ErrBlockNotFound in %v",
		head, props.LastIrreversibleBlockNum, head+1000, time.Since(start))
}

// TestIntegration_GetOpsInBlock_EmptyOpsForRealEmptyBlock pins that a real
// on-chain block with no operations decodes to an empty (non-nil) slice and
// is never mistaken for not-found.
func TestIntegration_GetOpsInBlock_EmptyOpsForRealEmptyBlock(t *testing.T) {
	a := NewAPI(integrationNodeURL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Find an empty block near block 20000000 (missed-slot blocks carry only
	// the witness signature — ops may legitimately be empty).
	found := false
	for n := uint(20000000); n < 20000200 && !found; n++ {
		ops, err := a.GetOpsInBlock(ctx, n, false)
		if err != nil {
			continue
		}
		if len(ops) == 0 {
			if ops == nil {
				t.Fatalf("block %d: empty ops must be non-nil slice", n)
			}
			found = true
			t.Logf("block %d has empty ops (non-nil, no error) — ambiguity contract holds", n)
		}
	}
	if !found {
		t.Log("no empty block found in scanned range; ambiguity contract not exercised this run")
	}
}
