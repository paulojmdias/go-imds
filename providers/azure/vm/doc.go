// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package vm reads instance metadata on Azure virtual machines.
//
// The Azure Instance Metadata Service requires the header "Metadata: True",
// which is what stops a browser following a cross-origin redirect from reading
// it. A request without the header is answered with 400, so on Azure the
// header is also what distinguishes this service from anything else listening
// on the shared 169.254.169.254 address.
//
// A lookup costs one request: the whole instance document, compute and network
// together.
//
// The document also carries osProfile.adminPassword, customData and userData.
// None is read.

package vm
