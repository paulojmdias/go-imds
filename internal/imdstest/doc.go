// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package imdstest provides the test fixtures and the conformance suite that
// every provider in this repository must pass.
//
// The suite exists because the failures this module guards against are not
// visible in ordinary unit tests: a lookup that leaks a goroutine still
// returns the right value, and a lookup routed through a proxy still succeeds
// on a developer machine with no proxy configured. [Conformance] makes both
// fail loudly.
package imdstest
