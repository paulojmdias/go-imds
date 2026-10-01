// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package ec2 reads instance metadata on Amazon EC2 instances.
//
// Reads use IMDSv2: a token obtained by PUT, presented on every request. That
// exchange is what makes the service unreachable through a server-side request
// forgery, since a browser will not issue the PUT cross-origin.
//
// The AWS conventions callers expect are honoured: AWS_EC2_METADATA_DISABLED
// turns detection off without a request being made,
// AWS_EC2_METADATA_SERVICE_ENDPOINT replaces the address, and
// AWS_EC2_METADATA_SERVICE_ENDPOINT_MODE selects between the IPv4 and IPv6
// defaults.
//
// AWS publishes a metadata client of its own, and this package does not use
// it. It reads both the token and every response body with no limit, which is
// the thing this package exists to get right, and it brings a large dependency
// tree for what is one PUT and two GETs.
//
// A lookup costs one token exchange, the identity document, and one request per
// path under /latest/meta-data/, around thirty in total. Most are conditional
// on how the instance is configured -- no spot termination time on an on-demand
// instance, no auto-scaling state outside a group -- so a 404 is reported as an
// empty field rather than a failure.
//
// iam/security-credentials returns live role credentials and /latest/user-data
// returns whatever the launcher supplied; neither is read.

package ec2
