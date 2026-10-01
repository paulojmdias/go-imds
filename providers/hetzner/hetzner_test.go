// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package hetzner_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/hetzner"
)

// The document shape follows the capture published at
// gist.github.com/paulc/127434c3466e4d4e9676268108b4efb0, with region and
// availability-zone added inline. It also carries vendor_data on a real
// server, which this provider deliberately does not read.
const metadataYAML = `
hostname: app-server-1
instance-id: 42763331
public-ipv4: 65.21.0.10
local-ipv4: 10.0.0.3
availability-zone: fsn1-dc14
region: eu-central
public-keys:
- ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@hetzner
network-config:
  version: 1
  config:
  - name: eth0
    type: physical
    mac_address: 96:00:02:d1:9e:01
    subnets:
    - type: dhcp
      ipv4: true
    - type: static6
      address: 2a01:4f8:1c1c::1/64
      gateway: fe80::1
      dns_nameservers:
      - 2a01:4ff:ff00::add:1
`

func handler(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/hetzner/v1/metadata") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	})
}

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "hetzner",
		Handler: handler(metadataYAML),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := hetzner.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler(metadataYAML))

	md, err := hetzner.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	// The first four are what this provider returned when it read one path per
	// field, and must keep returning; the rest is what that could not reach.
	assert.Equal(t, hetzner.Metadata{
		ID:                   "42763331",
		Hostname:             "app-server-1",
		Region:               "eu-central",
		Zone:                 "fsn1-dc14",
		PublicIPv4:           "65.21.0.10",
		PrivateIPv4:          "10.0.0.3",
		PublicKeys:           []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@hetzner"},
		NetworkConfigVersion: 1,
		Interfaces: []hetzner.Interface{{
			Name:       "eth0",
			Type:       "physical",
			MACAddress: "96:00:02:d1:9e:01",
			Subnets: []hetzner.Subnet{
				{Type: "dhcp", IPv4: true},
				{
					Type:           "static6",
					Address:        "2a01:4f8:1c1c::1/64",
					Gateway:        "fe80::1",
					DNSNameservers: []string{"2a01:4ff:ff00::add:1"},
				},
			},
		}},
	}, md)
}

// One request when the document carries region and availability-zone inline.
func TestCompleteDocumentCostsOneRequest(t *testing.T) {
	var n int
	ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n++
		_, _ = w.Write([]byte(metadataYAML))
	}))

	_, err := hetzner.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Equal(t, 1, n, "the whole document is one request")
}

// A server whose document omits them falls back to the paths that serve them,
// so no field is lost. Still fewer requests than one per field.
func TestMissingRegionAndZoneAreReadFromTheirOwnPaths(t *testing.T) {
	var n int
	ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		switch r.URL.Path {
		case "/hetzner/v1/metadata/region":
			_, _ = w.Write([]byte("eu-central"))
		case "/hetzner/v1/metadata/availability-zone":
			_, _ = w.Write([]byte("fsn1-dc14"))
		default:
			_, _ = w.Write([]byte("hostname: h\ninstance-id: 42763331\n"))
		}
	}))

	md, err := hetzner.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Equal(t, "eu-central", md.Region)
	assert.Equal(t, "fsn1-dc14", md.Zone)
	assert.Equal(t, 3, n)
}

// 169.254.169.254 is shared, so a document without an instance-id came from
// something other than Hetzner's service.
func TestDocumentWithoutInstanceIDIsForeign(t *testing.T) {
	ep := imdstest.Server(t, handler("hostname: someone-else\n"))

	_, err := hetzner.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrForeign)
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

// A body that is not YAML at all is the service malfunctioning, not evidence
// of being on another cloud.
func TestUndecodableDocumentIsAFailureNotAbsence(t *testing.T) {
	ep := imdstest.Server(t, handler("\tthis: is: not: yaml\n"))

	_, err := hetzner.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.Error(t, err)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}
