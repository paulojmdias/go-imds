// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package hetzner reads instance metadata on Hetzner Cloud servers.
//
// The metadata service answers at 169.254.169.254 under /hetzner/v1/metadata
// and serves one plain-text value per path.
//
// Hetzner publishes a metadata client of its own, and this package does not
// use it. Its constructor assigns to the Timeout field of the http.Client it is
// given, which both replaces the caller's deadline with a wall-clock one and
// mutates an object the caller owns. The package also pulls in Prometheus. All
// of that to read four fields from one address.
//
// A lookup normally costs one request: the whole document. A server whose
// document omits region and availability-zone costs two more, since those are
// served as their own paths.
//
// The document also carries vendor_data, a cloud-config blob with a random seed
// in it, which is not read.

package hetzner
