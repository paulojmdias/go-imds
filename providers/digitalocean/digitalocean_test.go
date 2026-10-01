// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package digitalocean_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/digitalocean"
)

// The payload shape follows the fixture in cloud-init's own DigitalOcean
// datasource tests (tests/unittests/sources/test_digitalocean.py), which is a
// capture from a real Droplet, extended with the tags, reserved_ip and
// features keys the metadata API documents. user_data and vendor_data are
// present on a real Droplet and are deliberately absent here: this provider
// does not read them.
const metadataJSON = `{
  "droplet_id": 2756294,
  "hostname": "sample-droplet",
  "region": "nyc3",
  "tags": ["web", "prod"],
  "public_keys": ["ssh-rsa AAAAB3NzaC1yc2EAAAA test@do.co"],
  "interfaces": {
    "public": [
      {
        "type": "public",
        "mac": "04:01:57:d1:9e:01",
        "ipv4": {
          "ip_address": "192.0.2.20",
          "netmask": "255.255.255.0",
          "gateway": "192.0.2.1"
        },
        "ipv6": {
          "ip_address": "2604:a880:800::1000:0",
          "cidr": 64,
          "gateway": "2604:a880:800::1"
        },
        "anchor_ipv4": {
          "ip_address": "10.0.0.5",
          "netmask": "255.255.0.0",
          "gateway": "10.0.0.1"
        }
      }
    ],
    "private": [
      {
        "type": "private",
        "mac": "04:01:57:d1:9e:02",
        "ipv4": {
          "ip_address": "10.132.6.205",
          "netmask": "255.255.0.0",
          "gateway": "10.132.0.1"
        }
      }
    ]
  },
  "floating_ip": { "ipv4": { "active": false } },
  "reserved_ip": { "ipv4": { "active": true, "ip_address": "198.51.100.7" } },
  "dns": { "nameservers": ["2001:4860:4860::8844", "8.8.8.8"] },
  "features": { "dhcp_enabled": true }
}`

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "digitalocean",
		Handler: imdstest.JSON("/metadata/v1.json", metadataJSON),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := digitalocean.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/metadata/v1.json", metadataJSON))

	md, err := digitalocean.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	// The first three are what this provider returned before it read the whole
	// document, and must keep returning; the rest is what was being thrown away.
	assert.Equal(t, digitalocean.Metadata{
		ID:         "2756294",
		Hostname:   "sample-droplet",
		Region:     "nyc3",
		Tags:       []string{"web", "prod"},
		PublicKeys: []string{"ssh-rsa AAAAB3NzaC1yc2EAAAA test@do.co"},
		Interfaces: digitalocean.Interfaces{
			Public: []digitalocean.Interface{{
				Type:       "public",
				MAC:        "04:01:57:d1:9e:01",
				IPv4:       digitalocean.IPv4{Address: "192.0.2.20", Netmask: "255.255.255.0", Gateway: "192.0.2.1"},
				IPv6:       digitalocean.IPv6{Address: "2604:a880:800::1000:0", CIDR: 64, Gateway: "2604:a880:800::1"},
				AnchorIPv4: digitalocean.IPv4{Address: "10.0.0.5", Netmask: "255.255.0.0", Gateway: "10.0.0.1"},
			}},
			Private: []digitalocean.Interface{{
				Type: "private",
				MAC:  "04:01:57:d1:9e:02",
				IPv4: digitalocean.IPv4{Address: "10.132.6.205", Netmask: "255.255.0.0", Gateway: "10.132.0.1"},
			}},
		},
		FloatingIP:  digitalocean.DetachableIP{Active: false},
		ReservedIP:  digitalocean.DetachableIP{Active: true, Address: "198.51.100.7"},
		Nameservers: []string{"2001:4860:4860::8844", "8.8.8.8"},
		DHCPEnabled: true,
	}, md)
}

// UpCloud serves a different document at this exact path. Unknown fields
// decode without complaint, so without the droplet_id check an UpCloud server
// would be reported as a Droplet with an empty identifier.
func TestUpCloudDocumentIsForeign(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/metadata/v1.json",
		`{"cloud_name":"upcloud","hostname":"srv","instance_id":"uuid","region":"de-fra1"}`))

	_, err := digitalocean.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrForeign)
	require.ErrorIs(t, err, imds.ErrNotDetected)
}
