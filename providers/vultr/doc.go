// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package vultr reads instance metadata on Vultr Cloud Compute instances.
//
// The metadata service answers at 169.254.169.254, which several clouds share,
// so a 4xx there means something other than Vultr's service replied and is
// reported as [imds.ErrForeign].
//
// A lookup costs one request: the whole metadata document.
//
// The document also carries user-data and vendor-data, which are not read.

package vultr
