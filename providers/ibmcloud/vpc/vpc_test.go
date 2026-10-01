// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package vpc_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/ibmcloud/vpc"
)

// The field set follows IBM Cloud's VPC instance metadata document. Values are
// illustrative rather than captured from a running instance.
const metadataJSON = `{
  "id": "0717_54cd52e1-2b21-4b1f-8b4a-1d9dc47bd08b",
  "crn": "crn:v1:bluemix:public:is:us-south-1:a/4329f0c2b6ab4c8e8f6f0d05c6a03d5f::instance:0717_54cd52e1-2b21-4b1f-8b4a-1d9dc47bd08b",
  "name": "app-server-1",
  "profile": { "name": "bx2-2x8" },
  "zone": { "name": "us-south-1" },
  "image": { "id": "r006-image-id", "name": "ibm-ubuntu-22-04-minimal-amd64-1" },
  "created_at": "2024-01-02T03:04:05Z",
  "status": "running",
  "lifecycle_state": "stable",
  "memory": 8,
  "bandwidth": 4000,
  "numa_count": 1,
  "confidential_compute_mode": "disabled",
  "enable_secure_boot": true,
  "health_state": "ok",
  "vcpu": { "architecture": "amd64", "count": 2, "manufacturer": "intel" },
  "gpu": { "count": 0, "manufacturer": "", "model": "", "memory": 0 },
  "vpc": { "id": "r006-vpc-id", "crn": "crn:v1:bluemix:public:is:us-south:a/4329::vpc:r006-vpc-id", "name": "telemetry-vpc" },
  "resource_group": { "id": "rg-id-1", "name": "default" },
  "primary_network_interface": {
    "id": "nic-1",
    "name": "eth0",
    "href": "https://us-south.iaas.cloud.ibm.com/v1/instances/i/network_interfaces/nic-1",
    "primary_ip": { "address": "10.240.0.6", "id": "ip-1", "name": "primary-ip" },
    "subnet": { "id": "subnet-1", "crn": "crn:v1:bluemix:public:is:us-south-1:a/4329::subnet:subnet-1", "name": "telemetry-subnet" },
    "resource_type": "network_interface"
  },
  "network_interfaces": [
    {
      "id": "nic-1",
      "name": "eth0",
      "href": "https://us-south.iaas.cloud.ibm.com/v1/instances/i/network_interfaces/nic-1",
      "primary_ip": { "address": "10.240.0.6", "id": "ip-1", "name": "primary-ip" },
      "subnet": { "id": "subnet-1", "crn": "crn:v1:bluemix:public:is:us-south-1:a/4329::subnet:subnet-1", "name": "telemetry-subnet" },
      "resource_type": "network_interface"
    }
  ],
  "boot_volume_attachment": {
    "volume": { "id": "vol-boot", "crn": "crn:v1:bluemix:public:is:us-south-1:a/4329::volume:vol-boot", "name": "boot", "href": "https://us-south.iaas.cloud.ibm.com/v1/volumes/vol-boot" }
  },
  "volume_attachments": [
    { "volume": { "id": "vol-boot", "crn": "crn:v1:bluemix:public:is:us-south-1:a/4329::volume:vol-boot", "name": "boot", "href": "https://us-south.iaas.cloud.ibm.com/v1/volumes/vol-boot" } },
    { "volume": { "id": "vol-data", "crn": "crn:v1:bluemix:public:is:us-south-1:a/4329::volume:vol-data", "name": "data", "href": "https://us-south.iaas.cloud.ibm.com/v1/volumes/vol-data" } }
  ]
}`

// The real service issues a token only for a PUT carrying the ibm flavour
// header, and refuses metadata reads without the bearer token.
func handler(body string) http.Handler {
	const token = "instance-identity-token"

	mux := http.NewServeMux()
	mux.HandleFunc("/instance_identity/v1/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.Header.Get("Metadata-Flavor") != "ibm" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"` + token + `","expires_in":300}`))
	})
	mux.HandleFunc("/metadata/v1/instance", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
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
		Name:    "ibmcloud/vpc",
		Handler: handler(metadataJSON),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := vpc.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler(metadataJSON))

	md, err := vpc.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, vpc.Metadata{
		ID:        "0717_54cd52e1-2b21-4b1f-8b4a-1d9dc47bd08b",
		Hostname:  "app-server-1",
		CRN:       "crn:v1:bluemix:public:is:us-south-1:a/4329f0c2b6ab4c8e8f6f0d05c6a03d5f::instance:0717_54cd52e1-2b21-4b1f-8b4a-1d9dc47bd08b",
		AccountID: "4329f0c2b6ab4c8e8f6f0d05c6a03d5f",
		Type:      "bx2-2x8",
		ImageID:   "r006-image-id",
		ImageName: "ibm-ubuntu-22-04-minimal-amd64-1",
		Zone:      "us-south-1",
		Region:    "us-south",

		CreatedAt:               "2024-01-02T03:04:05Z",
		Status:                  "running",
		LifecycleState:          "stable",
		Memory:                  8,
		Bandwidth:               4000,
		VCPU:                    vpc.VCPU{Architecture: "amd64", Count: 2, Manufacturer: "intel"},
		VPCID:                   "r006-vpc-id",
		VPCName:                 "telemetry-vpc",
		VPCCRN:                  "crn:v1:bluemix:public:is:us-south:a/4329::vpc:r006-vpc-id",
		ResourceGroupID:         "rg-id-1",
		ResourceGroupName:       "default",
		PrimaryIP:               "10.240.0.6",
		NetworkInterfaces:       []vpc.NetworkInterface{wantNIC()},
		BootVolume:              wantVolume("vol-boot", "boot"),
		Volumes:                 []vpc.Volume{wantVolume("vol-boot", "boot"), wantVolume("vol-data", "data")},
		NUMACount:               1,
		ConfidentialComputeMode: "disabled",
		EnableSecureBoot:        true,
		HealthState:             "ok",
	}, md)
}

// The account appears only inside the Resource Name, so a document without one
// yields no account rather than a wrong one.
func TestNoCRNYieldsNoAccountID(t *testing.T) {
	ep := imdstest.Server(t, handler(`{"id":"i-1","name":"n","zone":{"name":"eu-de-2"}}`))

	md, err := vpc.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Empty(t, md.AccountID)
	assert.Equal(t, "eu-de", md.Region)
}

func TestTokenRejectedIsNotDetected(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusNotFound))

	_, err := vpc.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

func TestTokenNeverAppearsInAnError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/instance_identity/v1/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"super-secret-token","expires_in":300}`))
	})
	mux.HandleFunc("/metadata/v1/instance", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	ep := imdstest.Server(t, mux)

	_, err := vpc.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "super-secret-token")
}

func wantNIC() vpc.NetworkInterface {
	var n vpc.NetworkInterface
	n.ID = "nic-1"
	n.Name = "eth0"
	n.Href = "https://us-south.iaas.cloud.ibm.com/v1/instances/i/network_interfaces/nic-1"
	n.PrimaryIP.Address = "10.240.0.6"
	n.PrimaryIP.ID = "ip-1"
	n.PrimaryIP.Name = "primary-ip"
	n.Subnet.ID = "subnet-1"
	n.Subnet.CRN = "crn:v1:bluemix:public:is:us-south-1:a/4329::subnet:subnet-1"
	n.Subnet.Name = "telemetry-subnet"
	n.ResourceType = "network_interface"
	return n
}

func wantVolume(id, name string) vpc.Volume {
	return vpc.Volume{
		ID:   id,
		CRN:  "crn:v1:bluemix:public:is:us-south-1:a/4329::volume:" + id,
		Name: name,
		Href: "https://us-south.iaas.cloud.ibm.com/v1/volumes/" + id,
	}
}
