// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package imds

import (
	"errors"
	"fmt"
)

// The three outcomes a metadata lookup can have, kept distinct because
// collapsing them is the bug this package exists to prevent. A metadata service
// that is reachable but failing, reported as "not running on this cloud", makes
// detection silently miss an instance it should have found.
//
// Callers who only need to know whether they are on the cloud in question test
// against [ErrNotDetected]; anything else is a real failure and must not be
// swallowed:
//
//	md, err := scaleway.Fetch(ctx)
//	switch {
//	case errors.Is(err, imds.ErrNotDetected):
//		// Not running on Scaleway.
//	case err != nil:
//		return err
//	}
var (
	// ErrNotDetected reports that the process is not running on the cloud the
	// provider is for. Both [ErrAbsent] and [ErrForeign] satisfy it.
	ErrNotDetected = errors.New("metadata service not detected")

	// ErrAbsent reports that nothing answered at any of the addresses tried.
	ErrAbsent = fmt.Errorf("%w: no response from any address", ErrNotDetected)

	// ErrForeign reports that an address answered but the responder is not the
	// metadata service being looked for. Link-local addresses are shared
	// between clouds and can be claimed by unrelated software, so an answer is
	// not by itself evidence of the platform.
	ErrForeign = fmt.Errorf("%w: address answered but is not this metadata service", ErrNotDetected)

	// ErrBodyTooLarge reports a response larger than the configured limit. The
	// body is never buffered past the limit, so a metadata service that streams
	// indefinitely cannot exhaust memory.
	ErrBodyTooLarge = errors.New("metadata response exceeds the maximum body size")

	// ErrNotFound reports that the metadata service answered 404 for a path
	// the caller said was not foreign.
	//
	// Many services document paths that exist only in some configurations: a
	// termination time on a spot instance, an activation server on Windows, a
	// tag tree only when tags are exposed. A provider reading one of those has
	// to tell "this instance has no such field" from "this service is broken",
	// and the status code is the only thing that carries the difference.
	//
	// It is deliberately not [ErrNotDetected]. The service answered, and it is
	// the service being looked for; one of its paths is simply not populated.
	ErrNotFound = errors.New("metadata path not found")

	// ErrTimeout reports that a lookup ran past the client's own deadline,
	// which applies only when the caller supplied none.
	//
	// It is deliberately not [context.DeadlineExceeded]: that would tell the
	// caller their context expired when it did not, and a caller who sees their
	// own cancellation reflected back has no way to tell "you gave up" from "we
	// gave up on your behalf".
	//
	// It is also deliberately not [ErrNotDetected]. An address that accepted the
	// connection and then went silent has said nothing about whether the
	// instance is on that cloud, and guessing "absent" is how a reachable
	// metadata service gets recorded as no cloud at all.
	ErrTimeout = errors.New("metadata lookup timed out")
)
