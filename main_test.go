package steemgosdk

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/steemit/steemgosdk/api/v2"
)

// TestGetBlocks is a live smoke test against the public endpoint.
//
// The endpoint rate-limits per client IP and answers HTTP 403 to a burst of
// single-block requests. GitHub-hosted runners hit that intermittently (two
// CI runs in a row on 2026-11-25, while the same test passed from other
// networks), so a failure made up entirely of 403s is skipped: it reports
// the endpoint's limit, not the SDK. Any other failure still fails.
func TestGetBlocks(t *testing.T) {
	client := GetClient("https://api.steemit.com")
	api := client.GetAPI()
	// dgp, err := api.GetDynamicGlobalProperties()
	// t.Errorf("test: %+v, err: %+v", dgp, err)
	// block, err := api.GetBlock(20221123)
	// for _, tr := range block.Transactions {
	// 	for _, op := range tr.Operations {
	// 		fmt.Printf("op: %+v\n", op.Type() == "vote")
	// 	}
	// }
	// t.Errorf("test: %+v, err: %+v", block, err)
	blocks, err := api.GetBlocks(10000000, 10000100)
	if allFailuresAreHTTP403(err) {
		t.Skipf("public RPC rate-limited the request: %v", err)
	}
	if len(blocks) != 100 {
		t.Errorf("GetBlocks unexpected length, err: %+v", err)
	}
}

// allFailuresAreHTTP403 reports whether err is a *v2.RangeError with at
// least one failure and every failure an HTTP 403 response.
func allFailuresAreHTTP403(err error) bool {
	var rangeErr *v2.RangeError
	if !errors.As(err, &rangeErr) || len(rangeErr.Failures) == 0 {
		return false
	}
	for _, failure := range rangeErr.Failures {
		if !strings.Contains(failure.Error(), "http 403:") {
			return false
		}
	}
	return true
}

// The skip path cannot be exercised reliably offline (the 403 depends on
// the endpoint), so pin the classification with synthetic errors.
func TestAllFailuresAreHTTP403(t *testing.T) {
	limited := errors.New("steemgosdk: http 403: failed to response(http code): 403")
	server := errors.New("steemgosdk: http 500: failed to response(http code): 500")
	cases := map[string]struct {
		err  error
		want bool
	}{
		"nil":            {nil, false},
		"plain 403":      {limited, false},
		"no failures":    {&v2.RangeError{}, false},
		"all 403":        {&v2.RangeError{Failures: map[uint]error{1: limited, 2: limited}}, true},
		"mixed statuses": {&v2.RangeError{Failures: map[uint]error{1: limited, 2: server}}, false},
		"wrapped range":  {fmt.Errorf("wrapped: %w", &v2.RangeError{Failures: map[uint]error{1: limited}}), true},
	}
	for name, tc := range cases {
		if got := allFailuresAreHTTP403(tc.err); got != tc.want {
			t.Errorf("%s: allFailuresAreHTTP403 = %v, want %v", name, got, tc.want)
		}
	}
}
