// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package imds

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// TokenConfig describes a metadata service's token exchange.
//
// Services that require one issue it in exchange for a request a browser
// cannot be tricked into making -- a PUT, usually, sometimes with a required
// header -- so that a server-side request forgery cannot reach the metadata
// service through a victim's browser and read instance credentials.
type TokenConfig struct {
	// Path is the token endpoint, relative to the address. It is requested
	// with PUT, which is what makes it a request a browser will not issue
	// cross-origin.
	Path string

	// Header is sent with the token request. Services commonly require the
	// requested lifetime, or a flavour header, here.
	Header http.Header

	// Body is sent with the token request, for services that want the requested
	// lifetime as a document rather than a header.
	Body []byte

	// Parse extracts the token from the response body. The default takes the
	// body with surrounding whitespace removed, which is what a plain-text token
	// endpoint returns.
	Parse func(body []byte) (string, error)

	// HeaderName is the header the token is presented in on later requests. It
	// is required.
	HeaderName string

	// HeaderPrefix is placed before the token in that header, for example
	// "Bearer ".
	HeaderPrefix string
}

// Token performs a token exchange and returns the [RequestOption] that presents
// the result on subsequent requests.
//
// The exchange runs on the context it is given, so it is inside the caller's
// deadline rather than alongside it. Nothing about the token appears in any
// error this returns: a token is a credential for the metadata service, and an
// error string is the one place credentials most reliably escape into logs.
func (c *Client) Token(ctx context.Context, endpoint string, cfg TokenConfig) (RequestOption, error) {
	if cfg.HeaderName == "" {
		return nil, errors.New("imds: TokenConfig.HeaderName is required")
	}

	opts := []RequestOption{WithMethod(http.MethodPut)}
	if len(cfg.Body) > 0 {
		opts = append(opts, WithBody(cfg.Body))
	}
	for k, vs := range cfg.Header {
		for _, v := range vs {
			opts = append(opts, WithHeader(k, v))
		}
	}

	url := endpoint + cfg.Path
	body, err := c.Get(ctx, url, opts...)
	if err != nil {
		return nil, err
	}

	token := strings.TrimSpace(string(body))
	if cfg.Parse != nil {
		if token, err = cfg.Parse(body); err != nil {
			return nil, fmt.Errorf("%s: reading token response: %w", url, err)
		}
	}
	if token == "" {
		return nil, fmt.Errorf("%s: token response contained no token", url)
	}

	return WithHeader(cfg.HeaderName, cfg.HeaderPrefix+token), nil
}
