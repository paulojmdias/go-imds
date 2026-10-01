// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package akamai reads instance metadata on Akamai Cloud Compute, the platform
// formerly known as Linode.
//
// The metadata service answers at 169.254.169.254 and requires a token
// obtained by PUT, presented in the Metadata-Token header on the read.
//
// A lookup costs two requests: the token exchange and the instance document.

package akamai
