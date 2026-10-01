// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package imdstest

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// Server starts a fake metadata service serving h and returns the address to
// pass to [imds.WithEndpoints]. It is stopped when the test ends.
func Server(tb testing.TB, h http.Handler) string {
	tb.Helper()
	srv := httptest.NewServer(h)
	tb.Cleanup(srv.Close)
	return srv.URL
}

// JSON returns a handler serving body at path and 404 elsewhere, which is the
// shape of most metadata services.
func JSON(path, body string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	return mux
}

// Status returns a handler answering every request with the given status.
func Status(code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	})
}

// Oversized returns a handler that writes more than n bytes, to check that a
// metadata service which streams without end cannot exhaust the caller.
func Oversized(n int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		chunk := make([]byte, 32*1024)
		for i := range chunk {
			chunk[i] = 'a'
		}
		for written := int64(0); written <= n; written += int64(len(chunk)) {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	})
}

// Blackhole returns the address of a listener that accepts connections and
// never answers.
//
// This is the failure that ordinary tests miss. A lookup against a blackhole
// must still return when its deadline expires, and must leave no goroutine or
// connection behind -- which is exactly what a metadata request wrapped in a
// goroutine to escape a non-cancellable SDK call does not do.
func Blackhole(tb testing.TB) string {
	tb.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("blackhole listen: %v", err)
	}

	var (
		mu     sync.Mutex
		conns  []net.Conn
		accept sync.WaitGroup
	)
	accept.Go(func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()
		}
	})

	tb.Cleanup(func() {
		_ = ln.Close()
		accept.Wait()
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			_ = c.Close()
		}
	})

	return "http://" + ln.Addr().String()
}

// Closed returns an address where nothing is listening, so a connection is
// refused. This is what "not running on this cloud" looks like on a host whose
// link-local addresses are unclaimed.
func Closed(tb testing.TB) string {
	tb.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("closed listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		tb.Fatalf("closed listener: %v", err)
	}
	return "http://" + addr
}
