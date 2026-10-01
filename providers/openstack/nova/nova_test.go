// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package nova_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/openstack/nova"
)

const metadataJSON = `{
  "uuid": "d8b3f8a8-1111-2222-3333-444444444444",
  "hostname": "app-server-1.novalocal",
  "name": "app-server-1",
  "availability_zone": "nova",
  "project_id": "5b3c1e2f4a7d4e2fa1b2c3d4e5f60718",
  "meta": { "role": "telemetry" },
  "launch_index": 0,
  "public_keys": { "mykey": "ssh-rsa AAAAB test@openstack" },
  "keys": [ { "name": "mykey", "type": "ssh", "data": "ssh-rsa AAAAB test@openstack" } ],
  "devices": [
    {
      "type": "disk",
      "bus": "scsi",
      "address": "0:0:0:0",
      "serial": "disk-serial-1",
      "path": "/dev/sda",
      "tags": ["root"]
    }
  ],
  "dedicated_cpus": [0, 1]
}`

// Deployments that serve the EC2-compatible tree as well as the JSON document.
func handlerWithEC2Tree() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/openstack/latest/meta_data.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(metadataJSON))
	})
	mux.HandleFunc("/latest/meta-data/instance-type", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("m1.small"))
	})
	return mux
}

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "openstack/nova",
		Handler: handlerWithEC2Tree(),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := nova.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handlerWithEC2Tree())

	md, err := nova.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, nova.Metadata{
		ID:        "d8b3f8a8-1111-2222-3333-444444444444",
		Hostname:  "app-server-1.novalocal",
		Name:      "app-server-1",
		AccountID: "5b3c1e2f4a7d4e2fa1b2c3d4e5f60718",
		Zone:      "nova",
		Type:      "m1.small",
		Meta:      map[string]string{"role": "telemetry"},

		LaunchIndex: 0,
		PublicKeys:  map[string]string{"mykey": "ssh-rsa AAAAB test@openstack"},
		Keys: []nova.Key{
			{Name: "mykey", Type: "ssh", Data: "ssh-rsa AAAAB test@openstack"},
		},
		Devices: []nova.Device{{
			Type: "disk", Bus: "scsi", Address: "0:0:0:0",
			Serial: "disk-serial-1", Path: "/dev/sda", Tags: []string{"root"},
		}},
		DedicatedCPUs: []int{0, 1},
	}, md)
}

// Many deployments do not serve the EC2-compatible tree at all. Its absence
// says nothing about the instance, so the lookup succeeds without the type
// rather than failing after it has already read the metadata document.
func TestMissingEC2TreeIsNotAFailure(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/openstack/latest/meta_data.json", metadataJSON))

	md, err := nova.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Empty(t, md.Type)
	assert.Equal(t, "d8b3f8a8-1111-2222-3333-444444444444", md.ID)
}

func TestMissingDocumentIsNotDetected(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusNotFound))

	_, err := nova.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrNotDetected)
}
