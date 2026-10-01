// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package cvm reads instance metadata on Tencent Cloud CVM instances.
//
// The metadata service answers at metadata.tencentyun.com, a name resolvable
// only from inside Tencent Cloud, and serves one plain-text value per path.
//
// A lookup costs around twenty-five requests, one per documented path. Many are
// optional, so a path that does not apply to this instance is reported as an
// empty field rather than a failure.
//
// cam/security-credentials returns live role credentials and is not read.

package cvm
