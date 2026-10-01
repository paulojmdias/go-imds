// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package gcp reads instance metadata on Google Compute Engine.
//
// The metadata service answers at metadata.google.internal, with
// 169.254.169.254 as the address it resolves to, and GCE_METADATA_HOST
// replaces both when set. Every request must carry "Metadata-Flavor: Google",
// and every genuine response carries it back.
//
// That response header is the whole availability test here, and it is a
// stronger one than a status code at an address several clouds share: anything
// can answer 200, but only the Google metadata service echoes the flavour.
//
// Google publishes a metadata client of its own, and this package does not use
// it. Its Get reads the response body with no limit, and returns only the body
// string, so a caller cannot check the flavour header the service sends back.
// Both of those are the things this package exists to get right.
//
// A lookup costs two requests: the instance tree and the project tree, each
// fetched recursively. Reading a path per field cost six.
//
// The service account access token and identity document are served from their
// own endpoints and are not requested. The attributes maps are not read either:
// they hold whatever the project or instance owner put there, startup scripts
// included.

package gcp
