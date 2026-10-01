// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package imdstest

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/paulojmdias/go-imds/imds"
)

// Subject is the provider under test.
type Subject struct {
	// Name appears in subtest names.
	Name string

	// Handler serves a valid metadata document for this provider, at the paths
	// the provider requests. Take the payload from the provider's own fixture
	// rather than writing a new one.
	Handler http.Handler

	// Identify stamps whatever headers make a response recognisably this
	// metadata service. It may be nil for a provider that recognises its
	// service by address and status alone.
	//
	// The cases below that serve a synthetic failure need a response the
	// provider accepts as coming from its own service and then fails on.
	// Without this, a provider that identifies its service by a response header
	// -- which is a stronger test than any status code at an address several
	// clouds share -- would classify those responses as foreign, and the cases
	// would assert nothing.
	Identify func(http.Header)

	// Fetch performs the provider's lookup with the given options and reports
	// only whether it succeeded. Conformance checks the outcome and the
	// lifecycle; whether the decoded fields are right is the provider's own
	// test to make.
	Fetch func(ctx context.Context, opts ...imds.Option) error
}

// Conformance runs the checks every provider must pass unchanged.
//
// Do not add exemptions or skips here. A provider that cannot pass has a bug in
// the provider; each case below corresponds to a rule in the package
// documentation, and a provider that opts out of one has opted out of the
// reason this module exists.
func Conformance(t *testing.T, s Subject) {
	t.Helper()

	name := s.Name
	if name == "" {
		name = "conformance"
	}

	t.Run(name, func(t *testing.T) {
		t.Run("detected", func(t *testing.T) {
			ep := Server(t, s.Handler)
			if err := s.Fetch(t.Context(), imds.WithEndpoints(ep)); err != nil {
				t.Fatalf("lookup against a working metadata service failed: %v", err)
			}
		})

		t.Run("absent", func(t *testing.T) {
			err := s.Fetch(t.Context(), imds.WithEndpoints(Closed(t)))
			if !errors.Is(err, imds.ErrNotDetected) {
				t.Fatalf("nothing listening should report ErrNotDetected, got %v", err)
			}
			if !errors.Is(err, imds.ErrAbsent) {
				t.Fatalf("nothing listening should report ErrAbsent, got %v", err)
			}
		})

		// The rule the whole module exists for. A metadata service that is
		// reachable and failing must never be reported as absence, because the
		// caller would then record the instance as belonging to no cloud at all
		// and never find out why.
		t.Run("service failure is not absence", func(t *testing.T) {
			ep := Server(t, s.identified(Status(http.StatusInternalServerError)))
			err := s.Fetch(t.Context(), imds.WithEndpoints(ep))
			if err == nil {
				t.Fatal("a 500 from the metadata service should be an error")
			}
			if errors.Is(err, imds.ErrNotDetected) {
				t.Fatalf("a 500 from the metadata service was reported as not-detected: %v", err)
			}
		})

		// A metadata service that accepts the connection and never answers is
		// what a non-cancellable request looks like from the outside. The lookup
		// has to come back on the caller's deadline; Main separately checks that
		// it left nothing running.
		t.Run("unresponsive", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
			defer cancel()

			start := time.Now()
			err := s.Fetch(ctx, imds.WithEndpoints(Blackhole(t)))
			elapsed := time.Since(start)

			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("want context.DeadlineExceeded, got %v", err)
			}
			// Generous, because CI machines stall. The failure being guarded
			// against is a lookup that restarts its budget per address or per
			// request, which overruns by a multiple rather than by jitter.
			if limit := 5 * time.Second; elapsed > limit {
				t.Fatalf("lookup took %v, past the %v bound: the deadline is not bounding the whole operation", elapsed, limit)
			}
		})

		t.Run("caller cancelled", func(t *testing.T) {
			ep := Server(t, s.Handler)
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err := s.Fetch(ctx, imds.WithEndpoints(ep))
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("want context.Canceled, got %v", err)
			}
		})

		t.Run("oversized body", func(t *testing.T) {
			ep := Server(t, s.identified(Oversized(2<<20)))
			err := s.Fetch(t.Context(), imds.WithEndpoints(ep))
			if !errors.Is(err, imds.ErrBodyTooLarge) {
				t.Fatalf("want ErrBodyTooLarge, got %v", err)
			}
		})
	})
}

// identified wraps h so that every response it writes carries the provider's
// identifying headers. The headers are set before h runs, so they are in place
// whether h writes a status, a body, or both.
func (s Subject) identified(h http.Handler) http.Handler {
	if s.Identify == nil {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.Identify(w.Header())
		h.ServeHTTP(w, r)
	})
}
