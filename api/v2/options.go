package v2

import (
	"net/http"
	"time"
)

const (
	// defaultConcurrency bounds the worker count of range methods
	// (GetBlocks / GetOpsInBlocks). 16 is conservative: steemdb measured 32
	// concurrent requests as stable against api.steemit.com (~51 blocks/s,
	// ~153 req/s counting the ops double-query), and 30-request bursts have
	// been observed to draw connection resets, so the default leaves margin.
	defaultConcurrency = 16
	maxConcurrency     = 64
	defaultTimeout     = 15 * time.Second
	defaultMaxRetry    = 3
)

// Option configures an API instance created by NewAPI.
type Option func(*API)

// WithConcurrency sets the worker count used inside range methods.
// n <= 0 selects the default (16); values above 64 are clamped to 64.
func WithConcurrency(n int) Option {
	return func(a *API) {
		if n <= 0 {
			n = defaultConcurrency
		}
		if n > maxConcurrency {
			n = maxConcurrency
		}
		a.concurrency = n
	}
}

// WithTimeout sets the per-attempt HTTP timeout (default 15s). Note the
// worst case for one logical call is attempts × timeout plus backoff
// (default ≈ 4×15s + 0.7s ≈ 61s); set outer context deadlines accordingly.
// Ignored when WithHTTPClient supplies a client — that client's own timeout
// then applies.
func WithTimeout(d time.Duration) Option {
	return func(a *API) {
		if d <= 0 {
			d = defaultTimeout
		}
		a.timeout = d
	}
}

// WithMaxRetry sets the number of retries after the first attempt
// (default 3, i.e. up to 4 attempts total). n <= 0 selects the default.
func WithMaxRetry(n int) Option {
	return func(a *API) {
		if n <= 0 {
			n = defaultMaxRetry
		}
		a.maxRetry = n
	}
}

// WithHTTPClient replaces the HTTP client entirely, including its transport
// and timeout. Callers take over connection-pool tuning (see NewAPI for the
// defaults this bypasses).
func WithHTTPClient(c *http.Client) Option {
	return func(a *API) {
		a.httpClient = c
	}
}
