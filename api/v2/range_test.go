package v2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// steemdFake answers one condenser_api block method (get_block or
// get_ops_in_block) with scripted per-block behavior, tracking in-flight
// requests (to assert the concurrency ceiling) and per-block attempt counts.
type steemdFake struct {
	mu          sync.Mutex
	srv         *httptest.Server
	failBlocks  map[uint]func(attempt int64) (status int, body string)
	attempts    map[uint]int64
	inFlight    int64
	maxInFlight int64
}

func newSteemdFake(t *testing.T, method string, failBlocks map[uint]func(attempt int64) (int, string), success func(blockNum uint) string) *steemdFake {
	t.Helper()
	b := &steemdFake{
		failBlocks: failBlocks,
		attempts:   make(map[uint]int64),
	}
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Method string        `json:"method"`
			Params []interface{} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		if !strings.Contains(req.Method, method) {
			writeJSON(w, http.StatusNotFound, `{"id":1,"jsonrpc":"2.0","result":null}`)
			return
		}

		var blockNum uint
		if len(req.Params) > 0 {
			if f, ok := req.Params[0].(float64); ok {
				blockNum = uint(f)
			}
		}

		cur := atomic.AddInt64(&b.inFlight, 1)
		for {
			max := atomic.LoadInt64(&b.maxInFlight)
			if cur <= max || atomic.CompareAndSwapInt64(&b.maxInFlight, max, cur) {
				break
			}
		}
		defer atomic.AddInt64(&b.inFlight, -1)

		b.mu.Lock()
		b.attempts[blockNum]++
		attempt := b.attempts[blockNum]
		b.mu.Unlock()

		if fail, ok := failBlocks[blockNum]; ok && fail != nil {
			if status, respBody := fail(attempt); status != http.StatusOK {
				writeJSON(w, status, respBody)
				return
			}
		}
		writeJSON(w, http.StatusOK, success(blockNum))
	}))
	t.Cleanup(b.srv.Close)
	return b
}

func (b *steemdFake) maxConcurrency() int64 { return atomic.LoadInt64(&b.maxInFlight) }

func (b *steemdFake) attemptsFor(n uint) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.attempts[n]
}

func blockJSON(n uint) string {
	return fmt.Sprintf(`{"id":1,"jsonrpc":"2.0","result":{"block_id":"%08x0000","previous":"%08x0000","timestamp":"2026-09-09T00:00:00","witness":"ety001"}}`, n, n-1)
}

func alwaysFail(attempt int64) (int, string) {
	return http.StatusInternalServerError, `{"id":1,"jsonrpc":"2.0","result":null}`
}

func newBlockServer(t *testing.T, failBlocks map[uint]func(attempt int64) (int, string)) *steemdFake {
	t.Helper()
	return newSteemdFake(t, "get_block", failBlocks, blockJSON)
}

func newOpsServer(t *testing.T, failBlocks map[uint]func(attempt int64) (int, string)) *steemdFake {
	t.Helper()
	return newSteemdFake(t, "get_ops_in_block", failBlocks, func(n uint) string {
		return fmt.Sprintf(`{"id":1,"jsonrpc":"2.0","result":[{"trx_id":"t%[1]d","block":%[1]d,"trx_in_block":0,"op_in_trx":0,"virtual_op":0,"timestamp":"2026-09-09T00:00:00","op":["vote",{"voter":"ety001"}]}]}`, n)
	})
}

func TestGetBlocks_BoundedConcurrency(t *testing.T) {
	const from, to = 1000, 1050 // 50 blocks
	srv := newBlockServer(t, nil)

	a := NewAPI(srv.srv.URL, WithConcurrency(4))
	a.baseBackoff = time.Millisecond

	blocks, err := a.GetBlocks(context.Background(), from, to)
	if err != nil {
		t.Fatalf("GetBlocks: %v", err)
	}
	if len(blocks) != int(to-from) {
		t.Fatalf("expected %d blocks, got %d", to-from, len(blocks))
	}
	for i, wb := range blocks {
		if wb.BlockNum != from+uint(i) {
			t.Fatalf("blocks out of order: index %d has block %d", i, wb.BlockNum)
		}
	}
	if max := srv.maxConcurrency(); max > 4 {
		t.Errorf("concurrency ceiling violated: max in-flight %d > 4", max)
	}
}

func TestGetBlocks_PartialFailureRangeError(t *testing.T) {
	const from, to = 2000, 2010
	srv := newBlockServer(t, map[uint]func(attempt int64) (int, string){
		2003: alwaysFail,
	})

	a := NewAPI(srv.srv.URL, WithConcurrency(4))
	a.baseBackoff = time.Millisecond

	blocks, err := a.GetBlocks(context.Background(), from, to)
	if err == nil {
		t.Fatal("expected RangeError")
	}
	var rErr *RangeError
	if !errors.As(err, &rErr) {
		t.Fatalf("expected *RangeError, got %T: %v", err, err)
	}
	if len(rErr.Failures) != 1 {
		t.Fatalf("expected exactly 1 failure, got %d (%v)", len(rErr.Failures), rErr.Failures)
	}
	if _, ok := rErr.Failures[2003]; !ok {
		t.Fatalf("expected block 2003 in Failures, got %v", rErr.Failures)
	}
	if len(blocks) != int(to-from)-1 {
		t.Errorf("expected %d successful blocks, got %d", int(to-from)-1, len(blocks))
	}
	for _, wb := range blocks {
		if wb.BlockNum == 2003 {
			t.Error("failed block must not appear in results")
		}
	}
	if got := srv.attemptsFor(2003); got != 4 { // transport retried 3 times
		t.Errorf("block 2003 attempts = %d, want 4", got)
	}
}

func TestGetBlocks_TransientFailureRetriedPerBlock(t *testing.T) {
	const from, to = 3000, 3003
	srv := newBlockServer(t, map[uint]func(attempt int64) (int, string){
		3001: func(attempt int64) (int, string) {
			if attempt < 3 {
				return http.StatusInternalServerError, `{"id":1,"jsonrpc":"2.0","result":null}`
			}
			return http.StatusOK, blockJSON(3001)
		},
	})

	a := NewAPI(srv.srv.URL, WithConcurrency(2))
	a.baseBackoff = time.Millisecond

	blocks, err := a.GetBlocks(context.Background(), from, to)
	if err != nil {
		t.Fatalf("GetBlocks: %v", err)
	}
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}
	if got := srv.attemptsFor(3001); got != 3 {
		t.Errorf("block 3001 attempts = %d, want 3", got)
	}
}

func TestGetBlocks_CtxCancelMidRange(t *testing.T) {
	const from, to uint = 4000, 4400 // 400 blocks, all failing
	failAll := make(map[uint]func(attempt int64) (int, string), to-from)
	for n := from; n < to; n++ {
		failAll[n] = alwaysFail
	}
	srv := newBlockServer(t, failAll)

	a := NewAPI(srv.srv.URL, WithConcurrency(4))
	a.baseBackoff = 50 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := a.GetBlocks(ctx, from, to)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error after cancel")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled in chain, got: %v", err)
	}
	var rErr *RangeError
	if !errors.As(err, &rErr) {
		t.Fatalf("expected *RangeError, got %T", err)
	}
	if len(rErr.Failures) != int(to-from) {
		t.Errorf("all blocks should be accounted in Failures, got %d", len(rErr.Failures))
	}
	if elapsed >= 3*time.Second {
		t.Errorf("cancel should unwind the range promptly, took %v", elapsed)
	}
}

func TestGetBlocks_BadRangeParams(t *testing.T) {
	a := NewAPI("http://unused")
	if _, err := a.GetBlocks(context.Background(), 10, 10); err == nil {
		t.Error("from == to must error")
	}
	if _, err := a.GetBlocks(context.Background(), 11, 10); err == nil {
		t.Error("from > to must error")
	}
	if _, err := a.GetOpsInBlocks(context.Background(), 11, 10, false); err == nil {
		t.Error("GetOpsInBlocks from > to must error")
	}
}

func TestGetBlocks_CtxCanceledBeforeStart(t *testing.T) {
	srv := newBlockServer(t, nil)
	a := NewAPI(srv.srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := a.GetBlocks(ctx, 1, 2); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestGetOpsInBlocks_PartialFailure(t *testing.T) {
	const from, to = 5000, 5010
	srv := newOpsServer(t, map[uint]func(attempt int64) (int, string){
		5005: alwaysFail,
	})

	a := NewAPI(srv.srv.URL, WithConcurrency(4))
	a.baseBackoff = time.Millisecond

	opsMap, err := a.GetOpsInBlocks(context.Background(), from, to, false)
	if err == nil {
		t.Fatal("expected RangeError")
	}
	var rErr *RangeError
	if !errors.As(err, &rErr) {
		t.Fatalf("expected *RangeError, got %T", err)
	}
	if _, ok := rErr.Failures[5005]; !ok {
		t.Errorf("expected 5005 in Failures, got %v", rErr.Failures)
	}
	if len(opsMap) != int(to-from)-1 {
		t.Errorf("expected %d entries, got %d", int(to-from)-1, len(opsMap))
	}
	if _, ok := opsMap[5005]; ok {
		t.Error("failed block must not appear in ops map")
	}
	if len(opsMap[5000]) != 1 {
		t.Errorf("block 5000 ops = %d, want 1", len(opsMap[5000]))
	}
}

// TestGetBlocks_CancelStorm hammers the cancellation path under -race: a mix
// of instantly-succeeding and forever-failing blocks, with the context
// canceled mid-range. Every block must end up either in the results or in
// Failures — no silent holes — and the returned error must unwrap to
// context.Canceled.
func TestGetBlocks_CancelStorm(t *testing.T) {
	const from, to uint = 6000, 6200
	failEveryThird := make(map[uint]func(attempt int64) (int, string), 40)
	for n := from; n < to; n++ {
		if n%3 == 0 {
			failEveryThird[n] = alwaysFail
		}
	}
	srv := newBlockServer(t, failEveryThird)

	for round := 0; round < 5; round++ {
		a := NewAPI(srv.srv.URL, WithConcurrency(8))
		a.baseBackoff = time.Millisecond

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(25 * time.Millisecond)
			cancel()
		}()

		blocks, err := a.GetBlocks(ctx, from, to)
		if err == nil {
			t.Fatalf("round %d: expected error after cancel", round)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("round %d: expected context.Canceled in chain, got: %v", round, err)
		}
		var rErr *RangeError
		if !errors.As(err, &rErr) {
			t.Fatalf("round %d: expected *RangeError, got %T", round, err)
		}
		if got := len(rErr.Failures) + len(blocks); got != int(to-from) {
			t.Fatalf("round %d: %d blocks unaccounted (results %d + failures %d != %d)",
				round, int(to-from)-got, len(blocks), len(rErr.Failures), to-from)
		}
		for _, wb := range blocks {
			if wb.BlockNum%3 == 0 {
				t.Fatalf("round %d: scripted-failing block %d must not be in results", round, wb.BlockNum)
			}
		}
		cancel()
	}
}
