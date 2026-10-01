// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package imdstest

import (
	"fmt"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"go.uber.org/goleak"
)

// Main runs a package's tests with the two checks that catch what assertions on
// return values cannot: a goroutine leak, and a request that escaped to the
// network.
//
// One line per package:
//
//	func TestMain(m *testing.M) { imdstest.Main(m) }
func Main(m *testing.M) {
	trap := newProxyTrap()

	code := m.Run()

	// The trap is torn down before the leak check, since its own accept
	// goroutine would otherwise be reported as the leak.
	hits := trap.tripped()
	trap.close()

	// Only report these when the tests themselves passed. A failing test often
	// abandons work in flight, and a leak report stacked on the real failure
	// buries it.
	if code == 0 {
		if err := goleak.Find(); err != nil {
			fmt.Fprintf(os.Stderr, "goroutine leaked after tests completed: %v\n", err)
			code = 1
		}
		if hits > 0 {
			fmt.Fprintf(os.Stderr,
				"%d request(s) reached the proxy trap: a test contacted an address outside the "+
					"test server, which means some client is not the one imds built\n", hits)
			code = 1
		}
	}

	os.Exit(code)
}

// proxyTrap points the proxy environment variables at a listener that records
// any connection made to it.
//
// What this catches, and what it does not, is worth being precise about. Go
// exempts loopback addresses from proxying unconditionally, so a request to the
// test server is never proxied and the trap cannot see it. What the trap does
// see is a request to any other address: a provider that ignored
// [imds.WithEndpoints] and went to the real link-local service, or -- the case
// that matters most -- a delegated vendor SDK that quietly used its own default
// transport instead of the client it was handed. The clients this repository
// builds set Proxy to nil and would not be caught here, which is why that rule
// is enforced by the linter instead.
type proxyTrap struct {
	ln    net.Listener
	hits  atomic.Int64
	saved map[string]string
	wg    sync.WaitGroup
}

var proxyVars = []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"}

func newProxyTrap() *proxyTrap {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		// Not fatal: the trap is a safety net, and a package whose tests cannot
		// bind a listener has larger problems that its own tests will report.
		fmt.Fprintf(os.Stderr, "proxy trap disabled: %v\n", err)
		return &proxyTrap{}
	}

	t := &proxyTrap{ln: ln, saved: make(map[string]string, len(proxyVars))}

	// NO_PROXY is cleared as well: an inherited value would exempt exactly the
	// addresses the trap is meant to notice.
	for _, v := range append(proxyVars, "NO_PROXY", "no_proxy") {
		t.saved[v] = os.Getenv(v)
	}
	for _, v := range proxyVars {
		_ = os.Setenv(v, "http://"+ln.Addr().String())
	}
	_ = os.Unsetenv("NO_PROXY")
	_ = os.Unsetenv("no_proxy")

	t.wg.Go(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			t.hits.Add(1)
			_ = conn.Close()
		}
	})

	return t
}

func (t *proxyTrap) tripped() int64 { return t.hits.Load() }

func (t *proxyTrap) close() {
	if t.ln == nil {
		return
	}
	_ = t.ln.Close()
	t.wg.Wait()
	for k, v := range t.saved {
		if v == "" {
			_ = os.Unsetenv(k)
			continue
		}
		_ = os.Setenv(k, v)
	}
}
