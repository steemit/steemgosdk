# Migrating to api/v2

A checklist for moving consumers (steemdb-web, steemdb-sync, conveyor, …)
from the legacy `api` package to `api/v2`.

## 1. Thread a context through every call

```go
// before
block, err := api.GetBlock(20000000)

// after
block, err := v2api.GetBlock(ctx, 20000000)
```

Cancel the context to abort in-flight HTTP requests immediately; deadlines
bound the whole logical call.

## 2. Replace zero-value block checks with ErrBlockNotFound

`GetBlock` for a block number beyond the head now fails fast:

```go
block, err := v2api.GetBlock(ctx, blockNum)
if errors.Is(err, v2.ErrBlockNotFound) {
    // block not produced yet — not an error worth retrying
}
```

The old behavior (a zero-value `Block` with nil error for `result: null`)
was a silent data hazard, not a contract to preserve.

## 3. Delete outer timeout/retry wrappers

The transport layer now retries transient failures (default 3 retries,
100/200/400ms backoff, doubled on 429) and applies a 15s per-attempt
timeout. Worst case for one logical call under defaults ≈ 4×15s + 0.7s ≈
61s — set outer context deadlines accordingly. steemdb-web's per-RPC
`select` + `timeout` wrapper (PR #55) can be deleted outright.

## 4. Handle partial failures of range calls

`GetBlocks`/`GetOpsInBlocks` return successfully fetched blocks *and* an
error when some blocks fail:

```go
blocks, err := v2api.GetBlocks(ctx, from, to)
if err != nil {
    var rErr *v2.RangeError
    if errors.As(err, &rErr) {
        // blocks holds every block that succeeded;
        // rErr.Failures maps block numbers to their final errors
    }
    // simple path: sleep and retry the whole batch
}
```

Never advance past a batch while ignoring the error — failed blocks would be
silently skipped.

## 5. Chain-head detection: DGP window, not error probing

Recommended catch-up loop (the pattern live_sync uses): each round fetch
`GetDynamicGlobalProperties(ctx)` and use `LastIrreversibleBlockNum` as the
window end; sleep ~3s when caught up. Steem block numbers are contiguous, so
`ErrBlockNotFound` is a purely defensive signal (bad bounds, node anomaly) —
and `get_ops_in_block` returning `[]` is ambiguous (empty block vs beyond
head) and must never be used for head detection. See README for the loop
skeleton.

## 6. Configure via options

```go
v2api := apiv2.NewAPI(url,
    apiv2.WithConcurrency(32),        // range-method workers (default 16, max 64)
    apiv2.WithTimeout(15*time.Second) // per-attempt HTTP timeout
)
```

`WithHTTPClient` takes over the HTTP client entirely (transport, pooling,
timeout).

## 7. SignedCall stays single-shot

Signed requests may be non-idempotent, so `api/v2` deliberately does not
retry them. If your signed flow needs delivery confirmation, add your own
idempotency key or follow-up query.
