// Package v2 is the context-aware Steem RPC client: every method takes a
// context, retries happen once at the transport layer (bounded, with
// backoff and error classification), and range methods run with bounded
// concurrency.
//
// It replaces the deprecated top-level api package, which now delegates here.
package v2

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Sentinel errors returned (wrapped) by this package. Use errors.Is to test
// for them; the original error remains in the chain where it carries
// additional information.
var (
	// ErrBlockNotFound means the requested block number is beyond the current
	// chain head (a get_block response of `null`, or a JSON-RPC "does not
	// exist" error on node implementations that report it that way). Steem
	// block numbers are strictly contiguous — missed slots skip time, not
	// numbers — so this is an unambiguous "not produced yet" signal, not
	// "maybe a gap". Chain-head detection should normally be done with
	// GetDynamicGlobalProperties instead of probing for this error; treat
	// ErrBlockNotFound as a defensive signal (bad range bounds, node anomaly).
	ErrBlockNotFound = errors.New("steemgosdk: block not found")

	// ErrRateLimited means the node answered 429. Retries back off at double
	// the normal rate; when retries are exhausted the final error wraps this.
	ErrRateLimited = errors.New("steemgosdk: rate limited")

	// ErrTimeout means a single attempt exceeded the per-attempt HTTP timeout
	// (default 15s, see WithTimeout). This is not the same as the caller's
	// context deadline, which is returned as-is (context.DeadlineExceeded).
	ErrTimeout = errors.New("steemgosdk: request timed out")
)

// RPCError describes a failed RPC at the protocol level: either a non-200
// HTTP response (HTTPStatus set, Code zero) or a JSON-RPC error object in an
// otherwise successful HTTP response (Code and Message set, HTTPStatus zero).
type RPCError struct {
	HTTPStatus int
	Code       int
	Message    string
}

func (e *RPCError) Error() string {
	if e.HTTPStatus != 0 {
		return fmt.Sprintf("steemgosdk: http %d: %s", e.HTTPStatus, e.Message)
	}
	return fmt.Sprintf("steemgosdk: rpc error %d: %s", e.Code, e.Message)
}

// RangeError reports per-block failures of a range call (GetBlocks /
// GetOpsInBlocks). Blocks that succeeded are still returned in the result
// slice; Failures maps the block numbers that did not. When the whole range
// was aborted (context canceled or deadline exceeded), Err holds the cause
// and Failures lists the blocks that were never attempted.
type RangeError struct {
	Failures map[uint]error
	Err      error
}

func (e *RangeError) Error() string {
	if e.Err != nil && len(e.Failures) == 0 {
		return fmt.Sprintf("steemgosdk: range aborted: %v", e.Err)
	}
	parts := make([]string, 0, len(e.Failures))
	for n, err := range e.Failures {
		parts = append(parts, fmt.Sprintf("block %d: %v", n, err))
	}
	msg := "steemgosdk: range has failures: " + strings.Join(parts, "; ")
	if e.Err != nil {
		msg += fmt.Sprintf(" (aborted: %v)", e.Err)
	}
	return msg
}

func (e *RangeError) Unwrap() error { return e.Err }

// httpCodeRe matches the transport error steemutil's jsonrpc2 returns for
// non-200 responses ("failed to response(http code): 500"). Both repos are
// versioned together, so the format is pinned by transport_test.go.
var httpCodeRe = regexp.MustCompile(`failed to response\(http code\): (\d+)`)

// httpStatusFromTransportError extracts the HTTP status code from a
// transport error, if there is one.
func httpStatusFromTransportError(err error) (int, bool) {
	m := httpCodeRe.FindStringSubmatch(err.Error())
	if m == nil {
		return 0, false
	}
	status, convErr := strconv.Atoi(m[1])
	if convErr != nil {
		return 0, false
	}
	return status, true
}

// asTimeoutError reports whether err is a per-attempt transport timeout
// (net.Error / url.Error with Timeout() true, e.g. http.Client.Timeout).
func asTimeoutError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Timeout() {
		return true
	}
	return false
}

// isBlockNotExistMessage reports whether a JSON-RPC error message is the
// "block does not exist" form used by some steemd builds. api.steemit.com
// instead answers `result: null` for the same condition (handled in
// GetBlock); this path exists for node implementations that report an error.
func isBlockNotExistMessage(msg string) bool {
	return strings.Contains(strings.ToLower(msg), "does not exist")
}

// rpcErrorFromObject converts the raw decoded JSON-RPC error object into an
// RPCError. The wire form is {"code":..., "message":"...", "data":...}.
func rpcErrorFromObject(raw interface{}) *RPCError {
	e := &RPCError{Message: fmt.Sprintf("%v", raw)}
	if m, ok := raw.(map[string]interface{}); ok {
		if code, ok := m["code"].(float64); ok {
			e.Code = int(code)
		}
		if msg, ok := m["message"].(string); ok {
			e.Message = msg
		}
	}
	return e
}

// isRetryable reports whether a failed attempt is transient and worth
// retrying: network errors, timeouts, HTTP 5xx, and 429 qualify. Caller-side
// context errors, "block does not exist", and other protocol-level errors
// (bad params, unknown method) do not.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, ErrRateLimited) || errors.Is(err, ErrTimeout) {
		return true
	}
	var rpcErr *RPCError
	if errors.As(err, &rpcErr) {
		return rpcErr.HTTPStatus == http.StatusTooManyRequests || rpcErr.HTTPStatus >= 500
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		// client.Do failures (DNS, connection refused, TLS, resets) surface
		// as *url.Error; treat as transient network trouble.
		return true
	}
	return false
}
