// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package imds

import (
	"net/http"
	"testing"
	"time"
)

// The first rule, checked directly rather than only through behaviour. A
// transport whose Proxy is non-nil routes link-local metadata addresses through
// whatever HTTP_PROXY names, which makes detection depend on that proxy and can
// send instance metadata to it.
//
// Go exempts loopback from proxying, so no test against a local server can
// observe this going wrong. The field is therefore asserted on directly.
func TestTransportDoesNotProxy(t *testing.T) {
	c := NewClient()
	defer c.Close()

	tr, ok := c.http.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("client transport is %T, not *http.Transport", c.http.Transport)
	}
	if tr.Proxy != nil {
		t.Fatal("transport has a Proxy function: link-local metadata requests must never be proxied")
	}
}

// A client-level timeout is not derived from the caller's context, cannot be
// cancelled by the caller, and restarts for every request, so a lookup over two
// addresses would take twice as long as the caller allowed.
func TestClientHasNoWallClockTimeout(t *testing.T) {
	c := NewClient(WithTimeout(time.Second))
	defer c.Close()

	if c.http.Timeout != 0 {
		t.Fatalf("http.Client.Timeout is %v; the context must be the only bound", c.http.Timeout)
	}
}
