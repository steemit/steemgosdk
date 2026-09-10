# Changelog

## v0.0.31 (2026-09-10)

### Added

- `api/v2`: context-aware RPC client. Every method takes a `context.Context`;
  transient failures retry once at the transport layer (bounded, exponential
  backoff); range methods fetch on a bounded worker pool; per-attempt HTTP
  timeout defaults to 15s.
- Options: `WithConcurrency` (default 16, clamp 64), `WithTimeout` (default
  15s), `WithMaxRetry` (default 3), `WithHTTPClient` (full client takeover).
- Typed errors: `ErrBlockNotFound`, `ErrRateLimited`, `ErrTimeout`,
  `*RPCError`, and `*RangeError{Failures map[uint]error}` for partial-success
  range results.

### Changed (behavior — read before upgrading)

- **`GetBlock` beyond the chain head now returns `v2.ErrBlockNotFound` on the
  first attempt.** Previously a `result: null` response was silently
  unmarshaled into a zero-value `Block` and returned with a nil error
  (`json.Unmarshal("null", ...)` is a no-op). Steem block numbers are
  contiguous, so null unambiguously means "not produced yet". Migration:
  replace zero-value checks with `errors.Is(err, v2.ErrBlockNotFound)`.
- Legacy `api` methods (and `client.GetAPI()`) now go through the
  transport-level bounded retry and 15s per-attempt timeout. **Remove outer
  retry loops when upgrading** — they would multiply attempts.
- `GetBlocks`/`GetOpsInBlocks` fetch with a bounded worker pool instead of
  one goroutine per block, and report partial failures as `*v2.RangeError`
  (successful blocks are still returned) instead of aborting with a generic
  error.
- `SetMaxRetry` now takes effect (it was dead configuration before); values
  `<= 0` normalize to the default of 3.

### Fixed

- Removed the `wrapGetBlock`/`wrapGetOpsInBlock` infinite retry loops and
  their `fmt.Printf` error swallowing — a permanently failing block no longer
  hangs the whole range call forever.
- `SignedCall` request IDs are now allocated atomically (was a data race
  under concurrent use).

### Unchanged / compatibility

- All legacy signatures compile and work unchanged apart from the behavior
  notes above; `WrapBlock`, `WrapOpsInBlock`, `AccountHistoryEntry`,
  `AccountHistoryOp` are re-exported as type aliases.
- `SignedCall` remains single-shot by design (signed requests may be
  non-idempotent); timeout and context cancellation still apply.
