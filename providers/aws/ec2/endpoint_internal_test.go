// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package ec2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Asserted on directly: a test cannot reach fd00:ec2::254, so the IPv6 mode is
// only observable as the address it selects.
func TestEndpointModeIPv6(t *testing.T) {
	t.Setenv(endpointModeEnv, "IPv6")
	assert.Equal(t, []string{endpointIPv6}, fromEnvironment())
}
