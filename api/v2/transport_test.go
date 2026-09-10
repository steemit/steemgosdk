package v2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// fakeNode is an httptest stand-in for a Steem RPC node. The handler is
// invoked with a 1-based hit counter so tests can script per-attempt
// behavior (fail twice, then succeed, ...).
type fakeNode struct {
	hits    int64
	handler func(hit int64, w http.ResponseWriter)
	srv     *httptest.Server
}

func (f *fakeNode) hitCount() int64 {
	return atomic.LoadInt64(&f.hits)
}

func newFakeNode(t *testing.T, handler func(hit int64, w http.ResponseWriter)) *fakeNode {
	t.Helper()
	f := &fakeNode{handler: handler}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.handler(atomic.AddInt64(&f.hits, 1), w)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func fastAPI(t *testing.T, url string, opts ...Option) *API {
	t.Helper()
	a := NewAPI(url, opts...)
	a.baseBackoff = time.Millisecond
	return a
}

func TestCall_Retries5xxThenSucceeds(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		if hit <= 2 {
			writeJSON(w, http.StatusInternalServerError, `{"id":1,"jsonrpc":"2.0","result":null}`)
			return
		}
		writeJSON(w, http.StatusOK, `{"id":1,"jsonrpc":"2.0","result":{"block_id":"00000001..."}}`)
	})
	a := fastAPI(t, node.srv.URL)

	resp, err := a.call(context.Background(), "condenser_api.get_block", []interface{}{1})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil result")
	}
	if node.hitCount() != 3 {
		t.Errorf("expected 3 attempts (2 failures + success), got %d", node.hitCount())
	}
}

func TestCall_429ExhaustedWrapsErrRateLimited(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		writeJSON(w, http.StatusTooManyRequests, `{"id":1,"jsonrpc":"2.0","result":null}`)
	})
	a := fastAPI(t, node.srv.URL)

	_, err := a.call(context.Background(), "condenser_api.get_block", []interface{}{1})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited in chain, got: %v", err)
	}
	if node.hitCount() != 4 { // default maxRetry 3 → 4 attempts
		t.Errorf("expected 4 attempts, got %d", node.hitCount())
	}
}

func TestCall_NonRetryableJSONRPCErrorSingleAttempt(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		writeJSON(w, http.StatusOK, `{"id":1,"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid parameters"}}`)
	})
	a := fastAPI(t, node.srv.URL)

	_, err := a.call(context.Background(), "condenser_api.get_block", []interface{}{1})
	if err == nil {
		t.Fatal("expected error")
	}
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected *RPCError in chain, got: %T: %v", err, err)
	}
	if rpcErr.Code != -32602 {
		t.Errorf("expected code -32602, got %d", rpcErr.Code)
	}
	if node.hitCount() != 1 {
		t.Errorf("protocol errors must not retry, got %d attempts", node.hitCount())
	}
}

func TestCall_BlockNotExistErrorFormNoRetry(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		writeJSON(w, http.StatusOK, `{"id":1,"jsonrpc":"2.0","error":{"code":-32001,"message":"Block 999999999 does not exist"}}`)
	})
	a := fastAPI(t, node.srv.URL)

	_, err := a.call(context.Background(), "condenser_api.get_block", []interface{}{999999999})
	if !errors.Is(err, ErrBlockNotFound) {
		t.Fatalf("expected ErrBlockNotFound in chain, got: %v", err)
	}
	if node.hitCount() != 1 {
		t.Errorf("block-not-found must not retry, got %d attempts", node.hitCount())
	}
}

func TestCall_PerAttemptTimeoutWrapsErrTimeout(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		time.Sleep(2 * time.Second)
		writeJSON(w, http.StatusOK, `{"id":1,"jsonrpc":"2.0","result":null}`)
	})
	a := fastAPI(t, node.srv.URL, WithTimeout(150*time.Millisecond))

	start := time.Now()
	_, err := a.call(context.Background(), "condenser_api.get_block", []interface{}{1})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout in chain, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 2*time.Second {
		t.Errorf("per-attempt timeout should bound each attempt, took %v", elapsed)
	}
	if node.hitCount() != 4 { // timeouts are retryable
		t.Errorf("expected 4 attempts, got %d", node.hitCount())
	}
}

func TestCall_CtxCancelDuringBackoff(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		writeJSON(w, http.StatusInternalServerError, `{"id":1,"jsonrpc":"2.0","result":null}`)
	})
	a := NewAPI(node.srv.URL)
	a.baseBackoff = 300 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := a.call(ctx, "condenser_api.get_block", []interface{}{1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Errorf("cancel during backoff should return promptly, took %v", elapsed)
	}
}

func TestCall_CtxCancelAbortsInFlightRequest(t *testing.T) {
	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		time.Sleep(5 * time.Second)
		writeJSON(w, http.StatusOK, `{"id":1,"jsonrpc":"2.0","result":null}`)
	})
	a := NewAPI(node.srv.URL) // default 15s timeout must NOT be what saves us here

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := a.call(ctx, "condenser_api.get_block", []interface{}{1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Errorf("cancel should abort the in-flight request, took %v", elapsed)
	}
}

func TestSetMaxRetry_NormalizesNonPositive(t *testing.T) {
	for _, in := range []int{0, -2} {
		a := NewAPI("http://unused")
		a.SetMaxRetry(in)
		if a.maxRetry != defaultMaxRetry {
			t.Errorf("SetMaxRetry(%d): expected normalization to %d, got %d", in, defaultMaxRetry, a.maxRetry)
		}
	}

	node := newFakeNode(t, func(hit int64, w http.ResponseWriter) {
		writeJSON(w, http.StatusInternalServerError, `{"id":1,"jsonrpc":"2.0","result":null}`)
	})
	a := NewAPI(node.srv.URL)
	a.SetMaxRetry(0)
	a.baseBackoff = time.Millisecond
	_, err := a.call(context.Background(), "condenser_api.get_block", []interface{}{1})
	if err == nil {
		t.Fatal("expected error")
	}
	if node.hitCount() != defaultMaxRetry+1 {
		t.Errorf("SetMaxRetry(0) must behave as default %d retries, attempts=%d", defaultMaxRetry, node.hitCount())
	}
}

func TestBackoffFor(t *testing.T) {
	a := NewAPI("http://unused")
	a.baseBackoff = 100 * time.Millisecond
	plain := errors.New("plain network error")
	rateLimited := fmt.Errorf("%w: too many requests", ErrRateLimited)

	cases := []struct {
		attempt int
		err     error
		want    time.Duration
	}{
		{0, plain, 100 * time.Millisecond},
		{1, plain, 200 * time.Millisecond},
		{2, plain, 400 * time.Millisecond},
		{0, rateLimited, 200 * time.Millisecond},
		{1, rateLimited, 400 * time.Millisecond},
		{2, rateLimited, 800 * time.Millisecond},
	}
	for _, c := range cases {
		if got := a.backoffFor(c.attempt, c.err); got != c.want {
			t.Errorf("backoffFor(%d, %v) = %v, want %v", c.attempt, c.err, got, c.want)
		}
	}

	// Zero baseBackoff (production default) falls back to 100ms.
	b := NewAPI("http://unused")
	if got := b.backoffFor(0, plain); got != 100*time.Millisecond {
		t.Errorf("default base backoff = %v, want 100ms", got)
	}
}

func TestNewAPI_DefaultTransportPooling(t *testing.T) {
	a := NewAPI("http://unused")
	tr, ok := a.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", a.httpClient.Transport)
	}
	if tr.MaxIdleConnsPerHost != defaultConcurrency {
		t.Errorf("MaxIdleConnsPerHost = %d, want %d", tr.MaxIdleConnsPerHost, defaultConcurrency)
	}

	b := NewAPI("http://unused", WithConcurrency(32))
	tr32, _ := b.httpClient.Transport.(*http.Transport)
	if tr32.MaxIdleConnsPerHost != 32 {
		t.Errorf("WithConcurrency(32): MaxIdleConnsPerHost = %d, want 32", tr32.MaxIdleConnsPerHost)
	}

	injected := &http.Client{}
	c := NewAPI("http://unused", WithHTTPClient(injected))
	if c.httpClient != injected {
		t.Error("WithHTTPClient client was not used verbatim")
	}
}

func TestWithConcurrencyClamps(t *testing.T) {
	if a := NewAPI("http://unused", WithConcurrency(0)); a.concurrency != defaultConcurrency {
		t.Errorf("WithConcurrency(0) = %d, want default %d", a.concurrency, defaultConcurrency)
	}
	if a := NewAPI("http://unused", WithConcurrency(1000)); a.concurrency != maxConcurrency {
		t.Errorf("WithConcurrency(1000) = %d, want clamp %d", a.concurrency, maxConcurrency)
	}
}
