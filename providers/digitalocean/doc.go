// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package digitalocean reads instance metadata on DigitalOcean Droplets.
//
// DigitalOcean and UpCloud both serve a document at
// 169.254.169.254/metadata/v1.json, and neither is a subset of the other, so
// reaching the address is not enough to tell them apart. This provider
// requires droplet_id, which only DigitalOcean sets, and reports
// [imds.ErrForeign] otherwise.
//
// A lookup costs one request: the whole metadata document.
//
// The document also carries user_data and vendor_data, which are not read.

package digitalocean
