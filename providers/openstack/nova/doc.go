// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package nova reads instance metadata on OpenStack Nova instances.
//
// Nova serves a JSON document at /openstack/latest/meta_data.json and, on
// deployments configured for it, an EC2-compatible tree at
// /latest/meta-data/. Only the JSON document is required: the EC2-compatible
// tree is not served by every deployment, so the instance type read from it is
// best effort and its absence is not an error.
//
// A lookup costs two requests: the OpenStack metadata document and the instance
// type from the EC2-compatible tree.
//
// The document also carries admin_pass, the generated administrator password,
// and random_seed. Neither is read.

package nova
