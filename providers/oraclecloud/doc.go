// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package oraclecloud reads instance metadata on Oracle Cloud Infrastructure
// compute instances.
//
// The metadata service answers at 169.254.169.254 and requires the header
// "Authorization: Bearer Oracle" -- a fixed string rather than a credential,
// serving the same purpose as the header other services require: a request a
// browser cannot be made to issue.
//
// Unlike the implementation this was derived from, no HEAD request precedes
// the read. The read already distinguishes "nothing there" from "something
// else answered" from "the service failed", so a separate probe would double
// the round trips to learn what the read reports anyway.
//
// A lookup costs one request: the whole instance document.
//
// The document's metadata map carries whatever the launcher put there,
// including cloud-init user data. Only the two descriptive keys are taken from
// it.

package oraclecloud
