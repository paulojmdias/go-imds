// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package imds

// version is reported in the User-Agent of every request, so an operator
// reading metadata service logs can tell which client made them.
const version = "0.0.0-dev" // x-release-please-version

const userAgent = "go-imds/" + version
