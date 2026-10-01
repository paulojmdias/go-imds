// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package imds

import (
	"net/http"
	"time"
)

type config struct {
	timeout   time.Duration
	maxBody   int64
	endpoints []string
}

// Option configures a [Client]. Providers accept Options and pass them through,
// so the same knobs work everywhere.
//
// There is deliberately no option to supply an [http.Client] or an
// [http.Transport]. The request boundary is the point of this package; handing
// it to the caller would make every guarantee conditional.
type Option interface {
	apply(*config)
}

type optionFunc func(*config)

func (f optionFunc) apply(c *config) { f(c) }

// WithTimeout sets how long a lookup may take when the caller's context carries
// no deadline of its own. A caller's deadline always wins; this is only a
// backstop so a lookup from [context.Background] terminates.
func WithTimeout(d time.Duration) Option {
	return optionFunc(func(c *config) { c.timeout = d })
}

// WithMaxBodySize sets the largest response body that will be read. Responses
// past the limit fail with [ErrBodyTooLarge] rather than being truncated, since
// a truncated metadata document decodes into a plausible but wrong result.
func WithMaxBodySize(n int64) Option {
	return optionFunc(func(c *config) { c.maxBody = n })
}

// WithEndpoints replaces the addresses a provider would otherwise use.
//
// This exists so tests have one uniform way to point any provider at a fake
// metadata service: a test that reached the real link-local address would
// either hang or, worse, pass only on the machine it was written on. Each
// address is a scheme and authority, without a path -- "http://127.0.0.1:8080".
func WithEndpoints(endpoints ...string) Option {
	return optionFunc(func(c *config) { c.endpoints = endpoints })
}

type requestConfig struct {
	method     string
	header     http.Header
	body       []byte
	notForeign bool
	check      func(*http.Response) error
}

// RequestOption configures a single request.
type RequestOption interface {
	applyRequest(*requestConfig)
}

type requestOptionFunc func(*requestConfig)

func (f requestOptionFunc) applyRequest(c *requestConfig) { f(c) }

// WithMethod sets the request method. The default is GET.
func WithMethod(method string) RequestOption {
	return requestOptionFunc(func(c *requestConfig) { c.method = method })
}

// WithHeader adds a header to the request. Metadata services commonly require
// one as a proof that the request was not made by a browser following a
// cross-origin redirect, for example Metadata-Flavor or Metadata: True.
func WithHeader(key, value string) RequestOption {
	return requestOptionFunc(func(c *requestConfig) {
		if c.header == nil {
			c.header = make(http.Header)
		}
		c.header.Add(key, value)
	})
}

// WithBody sets the request body. Token exchanges use it; metadata reads do
// not.
func WithBody(body []byte) RequestOption {
	return requestOptionFunc(func(c *requestConfig) { c.body = body })
}

// NeverForeign reports no response status as foreign.
//
// A 4xx is foreign by default, which suits a shared link-local address where
// an unrelated service may be listening. Providers whose address is theirs
// alone, so that any answer at all identifies the platform, pass this: for
// them a 4xx is a failure of the metadata service, not evidence of being
// somewhere else.
func NeverForeign() RequestOption {
	return requestOptionFunc(func(c *requestConfig) { c.notForeign = true })
}

// WithResponseCheck registers a check run against the response before its body
// is accepted. Returning an error rejects the response; return one wrapping
// [ErrForeign] to report that something other than this metadata service
// answered.
//
// Services that identify themselves in a response header use this. Requiring
// that header is a far better test than the status code at an address several
// clouds share, because anything can answer 200.
func WithResponseCheck(f func(*http.Response) error) RequestOption {
	return requestOptionFunc(func(c *requestConfig) { c.check = f })
}
