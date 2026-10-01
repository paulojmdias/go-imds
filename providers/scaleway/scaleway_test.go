// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package scaleway_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/scaleway"
)

// Trimmed to the fields this provider reads, from the document the Scaleway
// metadata service serves at /conf?format=json.
// The field set follows Scaleway's own metadata type in
// scaleway-sdk-go/api/instance/v1/instance_metadata_sdk.go, which is the
// documented shape of /conf?format=json. Values are illustrative; the document
// has not been captured from a running Instance.
const metadataJSON = `{
  "id": "11111111-2222-3333-4444-555555555555",
  "name": "scw-vigilant-shannon",
  "hostname": "scw-vigilant-shannon",
  "organization": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "project": "cccccccc-dddd-eeee-ffff-000000000000",
  "commercial_type": "DEV1-S",
  "state_detail": "booted",
  "timezone": "Europe/Paris",
  "private_ip": "10.66.0.4",
  "tags": ["web", "prod"],
  "image": {
    "id": "99999999-8888-7777-6666-555555555555",
    "name": "Ubuntu 22.04 Jammy Jellyfish"
  },
  "location": {
    "platform_id": "14",
    "hypervisor_id": "201",
    "node_id": "5",
    "cluster_id": "6",
    "zone_id": "fr-par-1"
  },
  "public_ip": {
    "id": "77777777-6666-5555-4444-333333333333",
    "address": "51.15.0.10",
    "dynamic": false,
    "gateway": "62.210.0.1",
    "netmask": "32",
    "family": "inet",
    "provisioning_mode": "dhcp"
  },
  "public_ips_v4": [
    {
      "id": "77777777-6666-5555-4444-333333333333",
      "address": "51.15.0.10",
      "family": "inet",
      "netmask": "32",
      "gateway": "62.210.0.1",
      "provisioning_mode": "dhcp",
      "tags": ["primary"]
    }
  ],
  "public_ips_v6": [
    {
      "id": "88888888-7777-6666-5555-444444444444",
      "address": "2001:bc8::1",
      "family": "inet6",
      "netmask": "64",
      "gateway": "fe80::1",
      "provisioning_mode": "slaac"
    }
  ],
  "ipv6": {
    "address": "2001:bc8::1",
    "gateway": "fe80::1",
    "netmask": "64"
  },
  "ssh_public_keys": [
    {
      "id": "22222222-3333-4444-5555-666666666666",
      "key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@scw",
      "fingerprint": "2048 06:ae:ee:c6 test@scw",
      "description": "laptop",
      "ip": "",
      "creation_date": "2024-01-02T03:04:05Z",
      "modification_date": "2024-01-02T03:04:05Z"
    }
  ],
  "volumes": {
    "0": {
      "id": "33333333-4444-5555-6666-777777777777",
      "name": "root",
      "volume_type": "b_ssd",
      "size": 20000000000,
      "state": "available",
      "export_uri": "",
      "organization": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "project": "cccccccc-dddd-eeee-ffff-000000000000",
      "creation_date": "2024-01-02T03:04:05Z",
      "modification_date": "2024-01-02T03:04:05Z",
      "server": { "id": "11111111-2222-3333-4444-555555555555", "name": "scw-vigilant-shannon" }
    }
  },
  "private_nics": [
    {
      "id": "44444444-5555-6666-7777-888888888888",
      "private_network_id": "55555555-6666-7777-8888-999999999999",
      "server_id": "11111111-2222-3333-4444-555555555555",
      "mac_address": "02:00:00:00:00:01",
      "zone": "fr-par-1",
      "creation_date": "2024-01-02T03:04:05Z"
    }
  ]
}`

func handler() http.Handler { return imdstest.JSON("/conf", metadataJSON) }

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "scaleway",
		Handler: handler(),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := scaleway.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler())

	md, err := scaleway.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	// The first eight are what this provider returned before it read the whole
	// document, and must keep returning; the rest is what was being discarded.
	assert.Equal(t, scaleway.Metadata{
		ID:          "11111111-2222-3333-4444-555555555555",
		Hostname:    "scw-vigilant-shannon",
		Name:        "scw-vigilant-shannon",
		AccountID:   "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		ProjectID:   "cccccccc-dddd-eeee-ffff-000000000000",
		Type:        "DEV1-S",
		ImageID:     "99999999-8888-7777-6666-555555555555",
		ImageName:   "Ubuntu 22.04 Jammy Jellyfish",
		Zone:        "fr-par-1",
		Region:      "fr-par",
		Tags:        []string{"web", "prod"},
		StateDetail: "booted",
		Timezone:    "Europe/Paris",
		PrivateIP:   "10.66.0.4",
		PublicIP: scaleway.IP{
			ID: "77777777-6666-5555-4444-333333333333", Address: "51.15.0.10",
			Gateway: "62.210.0.1", Netmask: "32", Family: "inet", ProvisioningMode: "dhcp",
		},
		PublicIPsV4: []scaleway.IP{{
			ID: "77777777-6666-5555-4444-333333333333", Address: "51.15.0.10",
			Gateway: "62.210.0.1", Netmask: "32", Family: "inet",
			ProvisioningMode: "dhcp", Tags: []string{"primary"},
		}},
		PublicIPsV6: []scaleway.IP{{
			ID: "88888888-7777-6666-5555-444444444444", Address: "2001:bc8::1",
			Gateway: "fe80::1", Netmask: "64", Family: "inet6", ProvisioningMode: "slaac",
		}},
		IPv6: scaleway.IPv6{Address: "2001:bc8::1", Gateway: "fe80::1", Netmask: "64"},
		Location: scaleway.Location{
			PlatformID: "14", HypervisorID: "201", NodeID: "5", ClusterID: "6", ZoneID: "fr-par-1",
		},
		SSHPublicKeys: []scaleway.SSHPublicKey{{
			ID:           "22222222-3333-4444-5555-666666666666",
			Key:          "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@scw",
			Fingerprint:  "2048 06:ae:ee:c6 test@scw",
			Description:  "laptop",
			CreationDate: "2024-01-02T03:04:05Z", ModificationDate: "2024-01-02T03:04:05Z",
		}},
		Volumes: wantVolumes(),
		PrivateNICs: []scaleway.PrivateNIC{{
			ID:               "44444444-5555-6666-7777-888888888888",
			PrivateNetworkID: "55555555-6666-7777-8888-999999999999",
			ServerID:         "11111111-2222-3333-4444-555555555555",
			MacAddress:       "02:00:00:00:00:01",
			Zone:             "fr-par-1",
			CreationDate:     "2024-01-02T03:04:05Z",
		}},
	}, md)
}

// 169.254.42.42 is Scaleway's own address rather than one several clouds
// share, so anything answering there is the metadata service. A 404 is that
// service failing, and reporting it as "not on Scaleway" would make detection
// miss an instance it had already reached.
func TestClientErrorIsAFailureNotAbsence(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusNotFound))

	_, err := scaleway.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.Error(t, err)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}

func TestSecondAddressIsTriedWhenTheFirstIsSilent(t *testing.T) {
	ep := imdstest.Server(t, handler())

	md, err := scaleway.Fetch(t.Context(), imds.WithEndpoints(imdstest.Closed(t), ep))
	require.NoError(t, err)
	assert.Equal(t, "fr-par-1", md.Zone)
}

func TestZoneWithoutRegionYieldsNoRegion(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/conf", `{"id":"i-1","location":{"zone_id":"nl"}}`))

	md, err := scaleway.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Empty(t, md.Region)
	assert.Equal(t, "nl", md.Zone)
}

func wantVolumes() map[string]scaleway.Volume {
	v := scaleway.Volume{
		ID:               "33333333-4444-5555-6666-777777777777",
		Name:             "root",
		VolumeType:       "b_ssd",
		Size:             20000000000,
		State:            "available",
		Organization:     "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		Project:          "cccccccc-dddd-eeee-ffff-000000000000",
		CreationDate:     "2024-01-02T03:04:05Z",
		ModificationDate: "2024-01-02T03:04:05Z",
	}
	v.Server.ID = "11111111-2222-3333-4444-555555555555"
	v.Server.Name = "scw-vigilant-shannon"
	return map[string]scaleway.Volume{"0": v}
}

// An instance whose document has no hostname reports its name as the hostname,
// which is what this provider did before the field was read separately.
func TestHostnameFallsBackToName(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/conf",
		`{"id":"i-1","name":"scw-old-instance","location":{"zone_id":"fr-par-1"}}`))

	md, err := scaleway.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Equal(t, "scw-old-instance", md.Hostname)
	assert.Equal(t, "scw-old-instance", md.Name)
}
