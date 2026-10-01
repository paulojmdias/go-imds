// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package upcloud_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/upcloud"
)

// The payload shape follows the fixture in cloud-init's own UpCloud datasource
// tests (tests/unittests/sources/test_upcloud.py), which is a capture from a
// real server. user_data and vendor_data are present on a real server and are
// deliberately absent here: this provider does not read them.
const metadataJSON = `{
  "cloud_name": "upcloud",
  "instance_id": "00c7c8a1-1111-2222-3333-444444444444",
  "hostname": "srv-fra-1",
  "region": "de-fra1",
  "platform": "servers",
  "subplatform": "metadata (http://169.254.169.254)",
  "tags": ["prod"],
  "public_keys": ["ssh-rsa AAAAB test1@example.com"],
  "network": {
    "interfaces": [
      {
        "index": 1,
        "mac": "3a:d6:ba:4a:36:e7",
        "network_id": "031457f4-0f8c-483c-96f2-eccede02909c",
        "type": "public",
        "ip_addresses": [
          {
            "address": "94.237.105.53",
            "family": "IPv4",
            "network": "94.237.104.0/22",
            "gateway": "94.237.104.1",
            "dhcp": true,
            "floating": false,
            "dns": ["94.237.127.9", "94.237.40.9"]
          }
        ]
      },
      {
        "index": 2,
        "mac": "3a:d6:ba:4a:84:cc",
        "network_id": "03d82553-5bea-4132-b29a-e1cf67ec2dd1",
        "type": "utility",
        "ip_addresses": [
          {
            "address": "10.6.3.27",
            "family": "IPv4",
            "network": "10.6.0.0/22",
            "gateway": "10.6.0.1",
            "dhcp": true,
            "floating": false,
            "dns": null
          }
        ]
      }
    ],
    "dns": ["94.237.127.9", "94.237.40.9"]
  },
  "storage": {
    "disks": [
      {
        "id": "014efb65-223b-4d44-8f0a-c29535b88dcf",
        "serial": "014efb65223b4d448f0a",
        "size": 10240,
        "type": "disk",
        "tier": "maxiops"
      }
    ]
  }
}`

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "upcloud",
		Handler: imdstest.JSON("/metadata/v1.json", metadataJSON),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := upcloud.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/metadata/v1.json", metadataJSON))

	md, err := upcloud.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, upcloud.Metadata{
		ID:          "00c7c8a1-1111-2222-3333-444444444444",
		Hostname:    "srv-fra-1",
		Region:      "de-fra1",
		CloudName:   "upcloud",
		Platform:    "servers",
		Subplatform: "metadata (http://169.254.169.254)",
		Tags:        []string{"prod"},
		PublicKeys:  []string{"ssh-rsa AAAAB test1@example.com"},
		Interfaces: []upcloud.Interface{
			{
				Index: 1, MAC: "3a:d6:ba:4a:36:e7",
				NetworkID: "031457f4-0f8c-483c-96f2-eccede02909c", Type: "public",
				IPAddresses: []upcloud.IPAddress{{
					Address: "94.237.105.53", Family: "IPv4", Network: "94.237.104.0/22",
					Gateway: "94.237.104.1", DHCP: true,
					DNS: []string{"94.237.127.9", "94.237.40.9"},
				}},
			},
			{
				Index: 2, MAC: "3a:d6:ba:4a:84:cc",
				NetworkID: "03d82553-5bea-4132-b29a-e1cf67ec2dd1", Type: "utility",
				IPAddresses: []upcloud.IPAddress{{
					Address: "10.6.3.27", Family: "IPv4", Network: "10.6.0.0/22",
					Gateway: "10.6.0.1", DHCP: true,
				}},
			},
		},
		Nameservers: []string{"94.237.127.9", "94.237.40.9"},
		Disks: []upcloud.Disk{{
			ID: "014efb65-223b-4d44-8f0a-c29535b88dcf", Serial: "014efb65223b4d448f0a",
			Size: 10240, Type: "disk", Tier: "maxiops",
		}},
	}, md)
}

// The mirror of the DigitalOcean case: same address, same path, different
// document.
func TestDigitalOceanDocumentIsForeign(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/metadata/v1.json",
		`{"droplet_id":2756294,"hostname":"sample-droplet","region":"nyc3"}`))

	_, err := upcloud.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrForeign)
	require.ErrorIs(t, err, imds.ErrNotDetected)
}
