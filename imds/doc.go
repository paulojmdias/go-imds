// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package imds owns the request boundary for instance metadata service (IMDS)
// lookups.
//
// Cloud metadata services are reached over link-local addresses that behave
// unlike ordinary HTTP endpoints: they may be absent entirely, they may be
// answered by something else, and they must never be contacted through a
// proxy. This package encodes the rules that follow from that, so a provider
// only has to describe its endpoints and decode its payload.
//
// The rules, in full:
//
//  1. Proxies are always off. The transport is built once with Proxy set to
//     nil. An outbound proxy that answers for a link-local address makes
//     detection depend on that proxy, and can leak instance metadata to it.
//
//  2. One context bounds the whole operation. The token fetch, every endpoint
//     in a fallback list and every path in a multi-request lookup share a
//     single deadline. [http.Client.Timeout] is never used: it is not derived
//     from the caller's context and it restarts for each request, so a
//     two-endpoint lookup takes twice as long as the caller allowed.
//
//  3. Nothing outlives the call. No goroutine, request or connection started
//     by a lookup survives its return.
//
//  4. The outcome is never collapsed. "Nothing is there", "something else
//     answered" and "the metadata service failed" are distinct results. A
//     failure reported as absence silently misdetects the instance.
//
//  5. Bodies are bounded. Every response, token responses included, is read
//     through an [io.LimitReader].
package imds
