package v2

import (
	"context"
	"fmt"
	"sync"

	"github.com/steemit/steemutil/protocol"
	protocolapi "github.com/steemit/steemutil/protocol/api"
)

// runRange fans block numbers [from, to) out to a bounded worker pool (the
// API's concurrency setting) and returns per-index results and errors. Every
// index is guaranteed to be accounted for: it either has a result or an
// error in the returned slices.
//
// On context cancellation the sender stops dispatching and unattempted blocks
// are marked with the context error, so the caller can report exactly what
// was fetched, what failed, and what never ran.
func runRange[T any](ctx context.Context, a *API, from, to uint, fetchOne func(ctx context.Context, blockNum uint) (T, error)) ([]T, []error) {
	n := int(to - from)
	results := make([]T, n)
	errs := make([]error, n)
	done := make([]bool, n)

	workers := a.concurrency
	if workers > n {
		workers = n
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				// GetBlock and friends honor ctx themselves; a canceled
				// context makes each remaining fetch fail fast.
				res, err := fetchOne(ctx, from+uint(idx))
				if err != nil {
					errs[idx] = err
					done[idx] = true
					continue
				}
				results[idx] = res
				done[idx] = true
			}
		}()
	}

	sending := true
	for idx := 0; idx < n && sending; idx++ {
		select {
		case jobs <- idx:
		case <-ctx.Done():
			for j := idx; j < n; j++ {
				errs[j] = ctx.Err()
				done[j] = true
			}
			sending = false
		}
	}
	close(jobs)
	wg.Wait()

	// Defensive: every index must be accounted for by now.
	for idx := 0; idx < n; idx++ {
		if !done[idx] {
			errs[idx] = fmt.Errorf("steemgosdk: block %d was not fetched", from+uint(idx))
		}
	}
	return results, errs
}

// GetBlocks gets blocks in the range [from, to), returned in ascending block
// order. Fetches run on a bounded worker pool (WithConcurrency, default 16).
//
// On partial failure it returns the successfully fetched blocks AND an error
// of type *RangeError whose Failures maps the failing block numbers to their
// final errors (after transport-level retries were exhausted). If the caller
// cancels the context mid-range, unattempted blocks appear in Failures and
// the RangeError unwraps to the context error.
func (a *API) GetBlocks(ctx context.Context, from, to uint) ([]*WrapBlock, error) {
	if from >= to {
		return nil, fmt.Errorf("steemgosdk: unexpected params {from: %d}, {to: %d}", from, to)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	results, errs := runRange(ctx, a, from, to, func(ctx context.Context, blockNum uint) (*protocolapi.Block, error) {
		return a.GetBlock(ctx, blockNum)
	})

	blocks := make([]*WrapBlock, 0, len(results))
	for idx, block := range results {
		if errs[idx] != nil {
			continue
		}
		blocks = append(blocks, &WrapBlock{BlockNum: from + uint(idx), Block: block})
	}
	if err := newRangeError(from, errs, ctx); err != nil {
		return blocks, err
	}
	return blocks, nil
}

// GetOpsInBlocks gets operations for blocks in the range [from, to),
// returned as a map keyed by block number (only successfully fetched blocks
// are present). Fetches run on a bounded worker pool (WithConcurrency,
// default 16). Partial failures are reported like GetBlocks: successful
// results are still returned alongside a *RangeError whose Failures maps the
// failing block numbers to their final errors.
func (a *API) GetOpsInBlocks(ctx context.Context, from, to uint, onlyVirtual bool) (map[uint][]*protocol.OperationObject, error) {
	if from >= to {
		return nil, fmt.Errorf("steemgosdk: unexpected params {from: %d}, {to: %d}", from, to)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	results, errs := runRange(ctx, a, from, to, func(ctx context.Context, blockNum uint) ([]*protocol.OperationObject, error) {
		return a.GetOpsInBlock(ctx, blockNum, onlyVirtual)
	})

	opsMap := make(map[uint][]*protocol.OperationObject, len(results))
	for idx, ops := range results {
		if errs[idx] != nil {
			continue
		}
		opsMap[from+uint(idx)] = ops
	}
	if err := newRangeError(from, errs, ctx); err != nil {
		return opsMap, err
	}
	return opsMap, nil
}

// newRangeError aggregates per-index errors into a *RangeError, or returns
// nil when every block succeeded. When the context is done, Err carries the
// context error so errors.Is(rangeErr, context.Canceled) works.
func newRangeError(from uint, errs []error, ctx context.Context) error {
	var failures map[uint]error
	for idx, err := range errs {
		if err != nil {
			if failures == nil {
				failures = make(map[uint]error)
			}
			failures[from+uint(idx)] = err
		}
	}
	if failures == nil {
		return nil
	}
	rErr := &RangeError{Failures: failures}
	if cerr := ctx.Err(); cerr != nil {
		rErr.Err = cerr
	}
	return rErr
}
