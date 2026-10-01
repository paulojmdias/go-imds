// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package imds_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
)

func TestTokenIsPresentedOnLaterRequests(t *testing.T) {
	var got string
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(" t-1\n"))
	})
	mux.HandleFunc("GET /meta", func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("{}"))
	})
	ep := imdstest.Server(t, mux)

	c := imds.NewClient()
	defer c.Close()

	token, err := c.Token(t.Context(), ep, imds.TokenConfig{
		Path:         "/token",
		HeaderName:   "Authorization",
		HeaderPrefix: "Bearer ",
	})
	require.NoError(t, err)
	_, err = c.Get(t.Context(), ep+"/meta", token)
	require.NoError(t, err)
	assert.Equal(t, "Bearer t-1", got)
}

// A token exchange that answers but yields no usable token is a failure of the
// metadata service, never "not on this cloud". The service has already
// identified itself by answering the PUT.
func TestTokenFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		cfg  imds.TokenConfig
	}{
		{"header name missing", "t-1", imds.TokenConfig{Path: "/token"}},
		{"empty token", " \n", imds.TokenConfig{Path: "/token", HeaderName: "X-Token"}},
		{"parse fails", "t-1", imds.TokenConfig{
			Path:       "/token",
			HeaderName: "X-Token",
			Parse:      func([]byte) (string, error) { return "", errors.New("bad document") },
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))

			c := imds.NewClient()
			defer c.Close()

			_, err := c.Token(t.Context(), ep, tc.cfg)
			require.Error(t, err)
			require.NotErrorIs(t, err, imds.ErrNotDetected)
			// The response may be the token itself; it must not reach the error.
			assert.NotContains(t, err.Error(), "t-1")
		})
	}
}
