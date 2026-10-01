// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package ecs reads task and container metadata inside an Amazon ECS task.
//
// ECS differs from the other providers here in how the service is found: the
// agent injects its address into the task's environment as
// ECS_CONTAINER_METADATA_URI_V4, rather than the address being a constant. The
// variable being unset is how a process learns it is not in an ECS task, so
// that case is reported as [imds.ErrAbsent] without any request being made.
//
// Everything else is unchanged. The injected address is link-local, so it must
// not be proxied, and both documents are read under the caller's one deadline.
//
// A lookup costs two requests: the container document and the task document.

package ecs
