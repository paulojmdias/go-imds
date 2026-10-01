// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package vpc reads instance metadata on IBM Cloud VPC virtual server
// instances.
//
// The metadata service answers at api.metadata.cloud.ibm.com, a name rather
// than a link-local address, and identifies the caller by the source address of
// the request. That is the same reason a link-local service must not be
// proxied: a proxied request arrives from the proxy, and the service would
// describe the proxy's host rather than this one.
//
// Reads require a bearer token obtained by PUT.
//
// A lookup costs two requests: the token exchange and the instance document.

package vpc
