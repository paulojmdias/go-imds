// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package outscale reads instance metadata on 3DS OUTSCALE VMs.
//
// The metadata service answers at 169.254.169.254 under /latest/meta-data and
// serves one plain-text value per path. It is EC2-compatible in shape but
// IMDSv1 only: there is no token exchange, and a PUT to /latest/api/token is
// not served.
//
// That compatibility is why this package cannot simply be
// providers/aws/ec2 pointed at a different address, in either direction. The
// EC2 provider makes its token exchange the availability probe, so it reports
// not-detected on an OUTSCALE VM, which is correct. Going the other way is the
// harder problem: on an EC2 instance with IMDSv1 enabled, every path this
// package reads would answer, so reading them alone would claim an EC2
// instance as OUTSCALE. See the probe in Fetch.
//
// A lookup costs around twenty-five requests, one per documented path. Many are
// optional: a VM with no public address has no public hostname, and that is
// reported as an empty field rather than a failure.

package outscale
