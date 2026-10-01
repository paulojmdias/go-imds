// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package vultr_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/vultr"
)

// The field set follows Vultr's v1.json document as read by cloud-init's
// helper (cloudinit/sources/helpers/vultr.py). Values are illustrative rather
// than captured. user-data and vendor-data are present on a real instance and
// are deliberately absent here: this provider does not read them.
const metadataJSON = `{
  "hostname": "guest",
  "instanceid": "42",
  "instance-v2-id": "11111111-2222-3333-4444-555555555555",
  "public-keys": ["ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@vultr"],
  "region": { "regioncode": "EWR", "countrycode": "US" },
  "interfaces": [
    {
      "mac": "56:00:03:15:c1:9e",
      "network-type": "public",
      "ipv4": {
        "address": "192.0.2.10",
        "netmask": "255.255.254.0",
        "gateway": "192.0.2.1",
        "additional": [
          { "address": "192.0.2.11", "netmask": "255.255.255.255" }
        ]
      },
      "ipv6": {
        "address": "2001:db8:1000::1",
        "network": "2001:db8:1000::",
        "prefix": "64",
        "additional": []
      }
    }
  ],
  "bgp": {
    "ipv4": {
      "my-address": "192.0.2.10",
      "my-asn": "64515",
      "peer-address": "169.254.169.254",
      "peer-asn": "64515"
    },
    "ipv6": {
      "my-address": "",
      "my-asn": "",
      "peer-address": "",
      "peer-asn": ""
    }
  }
}`

func handler() http.Handler { return imdstest.JSON("/v1.json", metadataJSON) }

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "vultr",
		Handler: handler(),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := vultr.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler())

	md, err := vultr.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, vultr.Metadata{
		ID:       "11111111-2222-3333-4444-555555555555",
		Hostname: "guest",
		// Lower-cased: the metadata service reports "EWR", but that is not the
		// form other tooling records for the same instance.
		Region:      "ewr",
		LegacyID:    "42",
		CountryCode: "US",
		PublicKeys:  []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@vultr"},
		Interfaces:  wantInterfaces(),
		BGP: vultr.BGP{IPv4: vultr.BGPSession{
			MyAddress: "192.0.2.10", MyASN: "64515",
			PeerAddress: "169.254.169.254", PeerASN: "64515",
		}},
	}, md)
}

func TestLegacyInstanceIDIsUsedWhenNoV2ID(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/v1.json",
		`{"hostname":"legacy","instanceid":"42","region":{"regioncode":"SJC"}}`))

	md, err := vultr.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Equal(t, "42", md.ID)
}

// 169.254.169.254 is shared, so a 4xx there means something other than Vultr's
// metadata service replied.
func TestClientErrorIsForeign(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusNotFound))

	_, err := vultr.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrForeign)
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

func wantInterfaces() []vultr.Interface {
	var i vultr.Interface
	i.MAC = "56:00:03:15:c1:9e"
	i.NetworkType = "public"
	i.IPv4.Address = "192.0.2.10"
	i.IPv4.Netmask = "255.255.254.0"
	i.IPv4.Gateway = "192.0.2.1"
	i.IPv4.Additional = []vultr.Additional{{Address: "192.0.2.11", Netmask: "255.255.255.255"}}
	i.IPv6.Address = "2001:db8:1000::1"
	i.IPv6.Network = "2001:db8:1000::"
	i.IPv6.Prefix = "64"
	i.IPv6.Additional = []vultr.Additional{}
	return []vultr.Interface{i}
}

// Older instances report public-keys as one newline-separated string rather
// than an array. Both have to decode, since the caller cannot tell which
// instance it is on.
func TestPublicKeysAcceptsAStringOrAnArray(t *testing.T) {
	for name, body := range map[string]string{
		"array":  `{"instanceid":"1","public-keys":["key-one","key-two"]}`,
		"string": `{"instanceid":"1","public-keys":"key-one\nkey-two\n"}`,
	} {
		t.Run(name, func(t *testing.T) {
			ep := imdstest.Server(t, imdstest.JSON("/v1.json", body))

			md, err := vultr.Fetch(t.Context(), imds.WithEndpoints(ep))
			require.NoError(t, err)
			assert.Equal(t, []string{"key-one", "key-two"}, md.PublicKeys)
		})
	}
}
