// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package oraclecloud_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/oraclecloud"
)

// The field set follows Oracle's /opc/v2/instance document. Values are
// illustrative rather than captured. The metadata map also carries whatever
// the launcher put there, including cloud-init user data; only the two
// descriptive keys are read.
const metadataJSON = `{
  "id": "ocid1.instance.oc1.eu-frankfurt-1.abc123",
  "displayName": "app-server-1",
  "shape": "VM.Standard.E4.Flex",
  "canonicalRegionName": "eu-frankfurt-1",
  "availabilityDomain": "Uocm:EU-FRANKFURT-1-AD-1",
  "faultDomain": "FAULT-DOMAIN-2",
  "compartmentId": "ocid1.compartment.oc1..aaaa",
  "tenantId": "ocid1.tenancy.oc1..bbbb",
  "image": "ocid1.image.oc1.eu-frankfurt-1.cccc",
  "state": "Running",
  "timeCreated": 1704164645000,
  "ociAdName": "eu-frankfurt-1-ad-1",
  "regionInfo": {
    "realmKey": "oc1",
    "realmDomainComponent": "oraclecloud.com",
    "regionKey": "FRA",
    "regionIdentifier": "eu-frankfurt-1"
  },
  "shapeConfig": {
    "ocpus": 2,
    "vcpus": 4,
    "memoryInGBs": 16,
    "networkingBandwidthInGbps": 2,
    "maxVnicAttachments": 2,
    "gpus": 0,
    "localDisks": 0,
    "baselineOcpuUtilization": "BASELINE_1_1"
  },
  "agentConfig": {
    "monitoringDisabled": false,
    "managementDisabled": false,
    "allPluginsDisabled": false,
    "pluginsConfig": [
      { "name": "Compute Instance Monitoring", "desiredState": "ENABLED" }
    ]
  },
  "definedTags": { "Operations": { "CostCenter": "42" } },
  "freeformTags": { "env": "prod" },
  "metadata": {
    "oke-cluster-display-name": "telemetry-cluster",
    "realm": "oc1"
  }
}`

// The real service answers 401 without the fixed authorization header.
func handler(body string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/opc/v2/instance/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer Oracle" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	return mux
}

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "oraclecloud",
		Handler: handler(metadataJSON),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := oraclecloud.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler(metadataJSON))

	md, err := oraclecloud.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, oraclecloud.Metadata{
		ID:                 "ocid1.instance.oc1.eu-frankfurt-1.abc123",
		Hostname:           "app-server-1",
		Type:               "VM.Standard.E4.Flex",
		Region:             "eu-frankfurt-1",
		AvailabilityDomain: "Uocm:EU-FRANKFURT-1-AD-1",
		Realm:              "oc1",
		OKEClusterName:     "telemetry-cluster",

		FaultDomain:          "FAULT-DOMAIN-2",
		CompartmentID:        "ocid1.compartment.oc1..aaaa",
		TenantID:             "ocid1.tenancy.oc1..bbbb",
		ImageID:              "ocid1.image.oc1.eu-frankfurt-1.cccc",
		State:                "Running",
		TimeCreated:          1704164645000,
		ADName:               "eu-frankfurt-1-ad-1",
		RegionIdentifier:     "eu-frankfurt-1",
		RealmDomainComponent: "oraclecloud.com",
		RegionKey:            "FRA",
		ShapeConfig: oraclecloud.ShapeConfig{
			OCPUs: 2, VCPUs: 4, MemoryInGBs: 16,
			NetworkingBandwidthInGbps: 2, MaxVnicAttachments: 2,
			BaselineOCPUUtilization: "BASELINE_1_1",
		},
		DefinedTags:  map[string]map[string]string{"Operations": {"CostCenter": "42"}},
		FreeformTags: map[string]string{"env": "prod"},
		AgentConfig: oraclecloud.AgentConfig{
			PluginsConfig: []oraclecloud.PluginConfig{
				{Name: "Compute Instance Monitoring", DesiredState: "ENABLED"},
			},
		},
	}, md)
}

func TestInstanceOutsideAClusterHasNoClusterName(t *testing.T) {
	ep := imdstest.Server(t, handler(`{"id":"ocid1.instance.oc1.phx.x","metadata":{"realm":"oc1"}}`))

	md, err := oraclecloud.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Empty(t, md.OKEClusterName)
	assert.Equal(t, "oc1", md.Realm)
}

// Another cloud's service on the shared address serves JSON that decodes
// cleanly into an empty document.
func TestDocumentWithoutIDIsForeign(t *testing.T) {
	ep := imdstest.Server(t, handler(`{"droplet_id":2756294,"hostname":"sample-droplet"}`))

	_, err := oraclecloud.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrForeign)
	require.ErrorIs(t, err, imds.ErrNotDetected)
}
