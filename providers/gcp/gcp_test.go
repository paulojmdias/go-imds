// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package gcp_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/gcp"
)

// The field set follows Compute Engine's recursive instance and project
// documents. Values are illustrative rather than captured. Both documents also
// carry an attributes map, and the instance one a serviceAccounts entry whose
// token is served elsewhere; the attributes are user data and are not read.
var values = map[string]string{
	"instance/": `{
  "id": 4772033139659366819,
  "hostname": "app-server-1.c.example.internal",
  "name": "app-server-1",
  "machineType": "projects/123456789/machineTypes/n2-standard-4",
  "zone": "projects/123456789/zones/us-central1-a",
  "description": "telemetry pipeline",
  "cpuPlatform": "Intel Broadwell",
  "image": "projects/debian-cloud/global/images/debian-11-bullseye-v20240110",
  "tags": ["telemetry", "prod"],
  "maintenanceEvent": "NONE",
  "preempted": "FALSE",
  "scheduling": {
    "automaticRestart": "TRUE",
    "onHostMaintenance": "MIGRATE",
    "preemptible": "FALSE"
  },
  "disks": [
    { "deviceName": "persistent-disk-0", "index": 0, "mode": "READ_WRITE", "type": "PERSISTENT" }
  ],
  "networkInterfaces": [
    {
      "ip": "10.128.0.2",
      "mac": "42:01:0a:80:00:02",
      "gateway": "10.128.0.1",
      "subnetmask": "255.255.240.0",
      "mtu": 1460,
      "network": "projects/123456789/networks/default",
      "dnsServers": ["169.254.169.254"],
      "ipAliases": [],
      "accessConfigs": [ { "externalIp": "34.1.2.3", "type": "ONE_TO_ONE_NAT" } ]
    }
  ],
  "licenses": [ { "id": "1000205" } ],
  "serviceAccounts": {
    "default": {
      "email": "sa@example-project.iam.gserviceaccount.com",
      "aliases": ["default"],
      "scopes": ["https://www.googleapis.com/auth/cloud-platform"]
    }
  }
}`,
	"project/": `{
  "projectId": "example-project",
  "numericProjectId": 123456789
}`,
}

// The real service requires the flavour header on the request and echoes it on
// every response.
func handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		v, ok := values[strings.TrimPrefix(r.URL.Path, "/computeMetadata/v1/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Metadata-Flavor", "Google")
		_, _ = w.Write([]byte(v))
	})
}

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:     "gcp",
		Handler:  handler(),
		Identify: func(h http.Header) { h.Set("Metadata-Flavor", "Google") },
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := gcp.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler())

	md, err := gcp.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, gcp.Metadata{
		ID:        "4772033139659366819",
		Hostname:  "app-server-1.c.example.internal",
		Name:      "app-server-1",
		AccountID: "example-project",
		// Both are reported as resource paths; only the final segment is kept.
		Type:   "n2-standard-4",
		Zone:   "us-central1-a",
		Region: "us-central1",

		Description:      "telemetry pipeline",
		CPUPlatform:      "Intel Broadwell",
		Image:            "projects/debian-cloud/global/images/debian-11-bullseye-v20240110",
		Tags:             []string{"telemetry", "prod"},
		NumericProjectID: 123456789,
		MaintenanceEvent: "NONE",
		Preempted:        "FALSE",
		Scheduling: gcp.Scheduling{
			AutomaticRestart: "TRUE", OnHostMaintenance: "MIGRATE", Preemptible: "FALSE",
		},
		Disks: []gcp.Disk{{
			DeviceName: "persistent-disk-0", Mode: "READ_WRITE", Type: "PERSISTENT",
		}},
		NetworkInterfaces: []gcp.NetworkInterface{{
			IP: "10.128.0.2", MAC: "42:01:0a:80:00:02", Gateway: "10.128.0.1",
			Subnetmask: "255.255.240.0", MTU: 1460,
			Network:    "projects/123456789/networks/default",
			DNSServers: []string{"169.254.169.254"},
			IPAliases:  []string{},
			AccessConfigs: []gcp.AccessConfig{
				{ExternalIP: "34.1.2.3", Type: "ONE_TO_ONE_NAT"},
			},
		}},
		Licenses: []string{"1000205"},
		ServiceAccounts: map[string]gcp.ServiceAccount{"default": {
			Email:   "sa@example-project.iam.gserviceaccount.com",
			Aliases: []string{"default"},
			Scopes:  []string{"https://www.googleapis.com/auth/cloud-platform"},
		}},
	}, md)
}

// The whole availability test. Software that merely happens to be listening on
// the shared address will answer 200, but it does not echo the flavour header,
// and treating its answer as GCE would misreport the instance entirely.
func TestResponseWithoutFlavorHeaderIsForeign(t *testing.T) {
	ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":4772033139659366819}`))
	}))

	_, err := gcp.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrForeign)
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

func TestHostEnvironmentVariableReplacesTheDefaults(t *testing.T) {
	ep := imdstest.Server(t, handler())
	// The variable holds a host and port with no scheme, as Google's tooling
	// reads it.
	t.Setenv("GCE_METADATA_HOST", strings.TrimPrefix(ep, "http://"))

	md, err := gcp.Fetch(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "app-server-1", md.Name)
}

// The name is tried before the link-local address, so a host with working
// internal DNS but no route to 169.254.169.254 still resolves.
func TestSecondAddressIsTriedWhenTheFirstIsSilent(t *testing.T) {
	ep := imdstest.Server(t, handler())

	md, err := gcp.Fetch(t.Context(), imds.WithEndpoints(imdstest.Closed(t), ep))
	require.NoError(t, err)
	assert.Equal(t, "4772033139659366819", md.ID)
}

// The whole tree in one request per document, rather than one per field. A
// change that goes back to reading paths individually would pass every other
// test in this file.
func TestFetchMakesTwoRequests(t *testing.T) {
	var paths []string
	ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Metadata-Flavor", "Google")
		_, _ = w.Write([]byte(values[strings.TrimPrefix(r.URL.Path, "/computeMetadata/v1/")]))
	}))

	_, err := gcp.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Equal(t, []string{"/computeMetadata/v1/instance/", "/computeMetadata/v1/project/"}, paths)
}
