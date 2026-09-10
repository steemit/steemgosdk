package v2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/steemit/steemutil/jsonrpc2"
	protocolapi "github.com/steemit/steemutil/protocol/api"
)

// API provides context-aware methods to call Steem RPC APIs. Create one with
// NewAPI; all methods are safe for concurrent use.
type API struct {
	url         string
	concurrency int
	timeout     time.Duration
	httpClient  *http.Client
	maxRetry    int

	// baseBackoff is the base of the exponential backoff between retry
	// attempts (100ms, 200ms, 400ms by default). Tests lower it to keep
	// retry-path tests fast; production callers have no reason to touch it.
	baseBackoff time.Duration

	// seqNo sequences signed-call request IDs (atomic; SignedCall is safe
	// for concurrent use).
	seqNo int64
}

// NewAPI creates an API instance for the given RPC endpoint (HTTP/HTTPS).
//
// The HTTP client is built here and reused for the lifetime of the instance:
// a clone of http.DefaultTransport with MaxIdleConnsPerHost raised to the
// concurrency setting (the default of 2 churns connections — and TLS
// handshakes — at range-method concurrency), and a per-attempt timeout of
// 15s. Supply WithHTTPClient to take over these knobs entirely.
func NewAPI(url string, opts ...Option) *API {
	a := &API{
		url:         url,
		concurrency: defaultConcurrency,
		timeout:     defaultTimeout,
		maxRetry:    defaultMaxRetry,
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.httpClient == nil {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.MaxIdleConnsPerHost = a.concurrency
		// The floor of 100 matches http.DefaultTransport's own default, so a
		// small-concurrency instance is no more eager than the stdlib; idle
		// connections are opened on demand, the cap allocates nothing up front.
		tr.MaxIdleConns = 2 * a.concurrency
		if tr.MaxIdleConns < 100 {
			tr.MaxIdleConns = 100
		}
		a.httpClient = &http.Client{Timeout: a.timeout, Transport: tr}
	}
	return a
}

// SetMaxRetry sets the number of retries after the first attempt (n <= 0
// selects the default, 3). Configure before first use; it is not
// synchronized against in-flight calls.
func (a *API) SetMaxRetry(maxRetry int) {
	if maxRetry <= 0 {
		maxRetry = defaultMaxRetry
	}
	a.maxRetry = maxRetry
}

// call performs one logical RPC: a single attempt via callOnce, retried up
// to maxRetry times while the error is transient, with exponential backoff
// between attempts (doubled on rate limiting). It returns the successful
// response (Error field guaranteed nil) or the last failure.
func (a *API) call(ctx context.Context, fullMethod string, params []interface{}) (*protocolapi.RpcResultData, error) {
	var lastErr error
	for attempt := 0; attempt <= a.maxRetry; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		resp, err := a.callOnce(ctx, fullMethod, params)
		if err == nil {
			return resp, nil
		}
		if !isRetryable(err) {
			return nil, err
		}
		lastErr = err

		if attempt == a.maxRetry {
			break
		}
		timer := time.NewTimer(a.backoffFor(attempt, err))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("steemgosdk: %s failed after %d attempts: %w", fullMethod, a.maxRetry+1, lastErr)
}

// callOnce sends a single JSON-RPC request and classifies the failure, if
// any, into this package's error taxonomy (sentinels + RPCError).
func (a *API) callOnce(ctx context.Context, fullMethod string, params []interface{}) (*protocolapi.RpcResultData, error) {
	rpc := jsonrpc2.NewClientWithOptions(a.url, jsonrpc2.WithHTTPClient(a.httpClient))
	if err := rpc.BuildSendData(fullMethod, params); err != nil {
		return nil, fmt.Errorf("steemgosdk: failed to build RPC data for %s: %w", fullMethod, err)
	}
	resp, err := rpc.SendWithContext(ctx)
	if err != nil {
		if status, ok := httpStatusFromTransportError(err); ok {
			rpcErr := &RPCError{HTTPStatus: status, Message: err.Error()}
			if status == http.StatusTooManyRequests {
				return nil, fmt.Errorf("%w: %v", ErrRateLimited, rpcErr)
			}
			return nil, rpcErr
		}
		if asTimeoutError(err) {
			return nil, fmt.Errorf("%w: %v", ErrTimeout, err)
		}
		return nil, err
	}
	if resp.Error != nil {
		rpcErr := rpcErrorFromObject(resp.Error)
		if isBlockNotExistMessage(rpcErr.Message) {
			return nil, fmt.Errorf("%w: %s", ErrBlockNotFound, rpcErr.Message)
		}
		return nil, fmt.Errorf("steemgosdk: rpc error for %s: %w", fullMethod, rpcErr)
	}
	return resp, nil
}

// backoffFor returns the sleep before the next attempt: base << attempt,
// doubled when the failure was rate limiting.
func (a *API) backoffFor(attempt int, err error) time.Duration {
	base := a.baseBackoff
	if base <= 0 {
		base = 100 * time.Millisecond
	}
	d := base << attempt
	if errors.Is(err, ErrRateLimited) {
		d *= 2
	}
	return d
}
