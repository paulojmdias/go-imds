// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package upcloud reads instance metadata on UpCloud servers.
//
// UpCloud and DigitalOcean both serve a document at
// 169.254.169.254/metadata/v1.json. This provider requires cloud_name, which
// only UpCloud sets, and reports [imds.ErrForeign] otherwise.
//
// A lookup costs one request: the whole metadata document.
//
// The document also carries user_data and vendor_data, which are not read.

package upcloud
