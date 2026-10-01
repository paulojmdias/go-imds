// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package ecs reads instance metadata on Alibaba Cloud ECS instances.
//
// The metadata service answers at 100.100.100.200, Alibaba's own address
// rather than the 169.254.169.254 several clouds share, and serves one
// plain-text value per path. A token obtained by PUT is required on every read.
//
// A lookup costs one token exchange and one request per documented path,
// around thirty in total. Most are optional: a path that does not apply to this
// instance answers 404 and is reported as an empty field, not a failure.
//
// ram/security-credentials returns live credentials and user data is served
// alongside the rest; neither is read.

package ecs
