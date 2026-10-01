// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package classic reads instance metadata on IBM Cloud Classic
// infrastructure, the former SoftLayer platform.
//
// It is the odd one out. The SoftLayer Resource Metadata API is a public HTTPS
// service rather than a link-local address, and it needs no token; it
// identifies the caller purely by the source address of the request. That is
// exactly why it is still not proxied: a proxied request arrives from the
// proxy, and the API would describe the proxy's host instead of this one.
//
// Each field is a separate plain-text document.
//
// A lookup costs around seventeen requests, one per documented path.
//
// getUserMetadata.txt returns whatever the launcher supplied and is not read.

package classic
