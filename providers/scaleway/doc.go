// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package scaleway reads instance metadata on Scaleway Instances.
//
// The metadata service is reached at 169.254.42.42, with fd00:42::42 as an
// IPv6 alternative, and the addresses are Scaleway's own rather than the
// 169.254.169.254 several clouds share. Any answer at them therefore
// identifies the platform, so this provider treats a 4xx as a failure of the
// metadata service rather than as evidence of running somewhere else.
//
// A lookup costs one request: the whole configuration document.

package scaleway
