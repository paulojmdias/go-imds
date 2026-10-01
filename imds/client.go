// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package imds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	// defaultTimeout bounds a lookup whose caller supplied no deadline. Metadata
	// services answer in single-digit milliseconds when they are present, so this
	// is generous; its purpose is only to stop a lookup from context.Background()
	// running forever.
	defaultTimeout = 2 * time.Second

	// defaultDialTimeout bounds establishing the connection, not the operation.
	//
	// This is not a wall-clock substitute for the caller's deadline: DialContext
	// still honours the context, whichever expires first. It exists so that a
	// blackholed link-local address -- the normal case when the process is not on
	// that cloud -- is reported as absent quickly instead of consuming the whole
	// lookup budget and starving the next address in a fallback list.
	defaultDialTimeout = time.Second

	// defaultMaxBodySize bounds every response read, so a metadata service that
	// streams without end cannot exhaust memory.
	defaultMaxBodySize = 1 << 20
)

// Client issues metadata requests. It owns the request boundary described in
// the package documentation; providers describe endpoints and payloads, and
// never construct an [http.Client] of their own.
//
// A Client is safe for concurrent use. Close it, or run work through [Client.Lookup],
// to release its connections.
type Client struct {
	http      *http.Client
	timeout   time.Duration
	maxBody   int64
	endpoints []string
}

// NewClient returns a Client configured for metadata lookups.
func NewClient(opts ...Option) *Client {
	cfg := config{
		timeout: defaultTimeout,
		maxBody: defaultMaxBodySize,
	}
	for _, o := range opts {
		o.apply(&cfg)
	}

	// Proxy is nil, never http.ProxyFromEnvironment.
	//
	// Metadata services live on link-local addresses that must never be reached
	// through an HTTP proxy. A proxy configured for ordinary outbound traffic
	// would answer for them, which makes detection depend on that proxy and can
	// send instance metadata to it. This is the single most repeated mistake in
	// hand-rolled metadata clients, so it is set explicitly and commented rather
	// than left to a default.
	transport := &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout: defaultDialTimeout,
			// A lookup is a handful of requests at startup. TCP keepalive probes
			// would only matter for a connection held open across minutes, and
			// none is.
			KeepAlive: -1,
		}).DialContext,
		MaxIdleConns:          2,
		IdleConnTimeout:       10 * time.Second,
		ExpectContinueTimeout: time.Second,
		// Metadata services speak HTTP/1.1. Attempting HTTP/2 costs a round trip
		// and buys nothing.
		ForceAttemptHTTP2: false,
	}

	return &Client{
		// Timeout is deliberately unset. It is not derived from the caller's
		// context, cannot be cancelled by the caller, and restarts for each
		// request, so a two-address lookup would take twice as long as the
		// caller allowed. The context is the only bound.
		http:      &http.Client{Transport: transport},
		timeout:   cfg.timeout,
		maxBody:   cfg.maxBody,
		endpoints: cfg.endpoints,
	}
}

// Endpoints returns the addresses the caller substituted with
// [WithEndpoints], or defaults unchanged when none were given. Providers pass
// their own defaults so that tests, which cannot reach a real metadata service,
// have a single uniform way to redirect them.
func (c *Client) Endpoints(defaults ...string) []string {
	if len(c.endpoints) > 0 {
		return c.endpoints
	}
	return defaults
}

// Close releases the client's connections. It is safe to call more than once.
func (c *Client) Close() {
	c.http.CloseIdleConnections()
}

// Lookup runs fn as a single metadata operation.
//
// It is the one place where two of this package's rules are implemented, so
// that a provider cannot get either wrong:
//
// Every request fn makes shares one deadline. When ctx carries a deadline of
// its own it is used unchanged -- the caller decides how long detection may
// take. Otherwise the client's default applies, so a lookup started from
// [context.Background] still terminates.
//
// Every connection opened during fn is closed before Lookup returns, so nothing
// started by the lookup outlives it.
func (c *Client) Lookup(ctx context.Context, fn func(context.Context) error) error {
	lookupCtx, cancel := c.bound(ctx)
	defer cancel()
	defer c.Close()

	if err := fn(lookupCtx); err != nil {
		// The caller's own context ending is reported as itself: a caller who
		// gave up has told us nothing about the metadata service.
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		// Our deadline, not theirs. The underlying error is folded in with %v
		// rather than %w on purpose: it wraps context.DeadlineExceeded, and
		// leaving that in the chain would let the caller conclude their own
		// context expired when it never did.
		if lookupCtx.Err() != nil {
			return fmt.Errorf("%w after %s: %v", ErrTimeout, c.timeout, err) //nolint:errorlint // see above
		}
		return err
	}
	return nil
}

func (c *Client) bound(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.timeout)
}

// Get issues one request and returns the response body.
//
// The error is [ErrAbsent] when nothing answered at the address and [ErrForeign]
// when the responder is not the metadata service, both of which satisfy
// [ErrNotDetected]. Any other error means the metadata service answered and the
// request failed, and callers must not treat it as absence.
func (c *Client) Get(ctx context.Context, url string, opts ...RequestOption) ([]byte, error) {
	cfg := requestConfig{method: http.MethodGet}
	for _, o := range opts {
		o.applyRequest(&cfg)
	}

	reqBody := io.Reader(http.NoBody)
	if len(cfg.body) > 0 {
		reqBody = bytes.NewReader(cfg.body)
	}

	req, err := http.NewRequestWithContext(ctx, cfg.method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	for k, vs := range cfg.header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			// Our own expiry, or the caller's. Either way this is not evidence
			// that the metadata service is absent.
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s: %w", ErrAbsent, url, err)
	}
	defer resp.Body.Close()

	if cfg.check != nil {
		if err := cfg.check(resp); err != nil {
			return nil, fmt.Errorf("%s: %w", url, err)
		}
	}

	// Read before classifying the status: the body is bounded either way, and
	// draining it lets the connection be reused for the next path in a
	// multi-request lookup.
	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("%s: reading metadata response: %w", url, err)
	}
	if int64(len(body)) > c.maxBody {
		return nil, fmt.Errorf("%s: %w (%d bytes)", url, ErrBodyTooLarge, c.maxBody)
	}

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return body, nil
	case !cfg.notForeign && resp.StatusCode >= 400 && resp.StatusCode < 500:
		return nil, fmt.Errorf("%w: %s: status %d", ErrForeign, url, resp.StatusCode)
	case resp.StatusCode == http.StatusNotFound:
		// Only reachable once the caller has said no status is foreign, which
		// is to say once the service has identified itself. Before that a 404
		// is the case above.
		return nil, fmt.Errorf("%w: %s", ErrNotFound, url)
	default:
		return nil, fmt.Errorf("%s: metadata service returned status %d", url, resp.StatusCode)
	}
}

// GetFirst tries each address in order, under the one context, and returns the
// body from the first that serves the path along with the address that did.
//
// Addresses are tried past a not-detected result, since a link-local address
// can be claimed by unrelated software while the real metadata service answers
// on another. A metadata service that answers and fails stops the walk
// immediately, and its error is returned alone.
func (c *Client) GetFirst(ctx context.Context, endpoints []string, path string, opts ...RequestOption) (body []byte, endpoint string, err error) {
	if len(endpoints) == 0 {
		return nil, "", errors.New("imds: no endpoints configured")
	}

	var errs []error
	for _, ep := range endpoints {
		body, err := c.Get(ctx, ep+path, opts...)
		if err == nil {
			return body, ep, nil
		}
		if ctx.Err() != nil {
			return nil, "", err
		}
		if !errors.Is(err, ErrNotDetected) {
			// Returned alone rather than joined with the other addresses'
			// results. errors.Is reports true if any joined error matches, so
			// joining a real failure with "nothing at the other address" would
			// make the failure read as absence -- the exact collapse this
			// package exists to prevent.
			return nil, "", err
		}
		errs = append(errs, err)
	}
	return nil, "", errors.Join(errs...)
}

// GetJSONFirst walks endpoints as [Client.GetFirst] does and decodes the first
// successful response into v.
//
// A body that will not decode is reported as a plain error, never as
// [ErrNotDetected]. Another cloud's metadata service answering on a shared
// link-local address produces valid JSON of the wrong shape rather than a
// decode failure, and that case belongs to the provider, which can tell the
// difference by checking a field only its own service sets. A body that is not
// JSON at all is a malfunction, and calling it "not this cloud" would hide it.
func (c *Client) GetJSONFirst(ctx context.Context, endpoints []string, path string, v any, opts ...RequestOption) (endpoint string, err error) {
	body, endpoint, err := c.GetFirst(ctx, endpoints, path, opts...)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return endpoint, fmt.Errorf("%s%s: decoding metadata response: %w", endpoint, path, err)
	}
	return endpoint, nil
}

// GetTextOptional is [Client.GetText] for a path the instance may legitimately
// not have, returning the empty string when the service answers 404.
//
// Services document paths that exist only in some configurations: a
// termination time on a spot instance, an activation server on Windows, an
// auto-scaling group only for an instance in one. Treating those as failures
// would make a lookup depend on how the instance happens to be configured.
//
// Only 404 is tolerated. Any other failure is returned, so a service answering
// badly is still distinguishable from a field being unset. This implies
// [NeverForeign]: a 404 can only mean "no such path" once the service has
// already identified itself.
func (c *Client) GetTextOptional(ctx context.Context, url string, opts ...RequestOption) (string, error) {
	v, err := c.GetText(ctx, url, append(opts, NeverForeign())...)
	if errors.Is(err, ErrNotFound) {
		return "", nil
	}
	return v, err
}

// GetText fetches a path and returns the body with surrounding whitespace
// removed, which is the form plain-text metadata endpoints serve.
func (c *Client) GetText(ctx context.Context, url string, opts ...RequestOption) (string, error) {
	body, err := c.Get(ctx, url, opts...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}
