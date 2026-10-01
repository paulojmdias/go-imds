// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package vpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/paulojmdias/go-imds/imds"
)

var defaultEndpoints = []string{"http://api.metadata.cloud.ibm.com"}

const (
	// apiVersion is pinned rather than tracking the newest: the service dates
	// its schema, and a version pinned here is one every field below exists in.
	apiVersion = "2026-01-30"

	tokenPath    = "/instance_identity/v1/token?version=" + apiVersion
	instancePath = "/metadata/v1/instance?version=" + apiVersion

	// tokenLifetime is the lifetime requested for the session token, in seconds,
	// as the service takes it in the request body rather than a header.
	tokenLifetime = 300
)

// Metadata describes the VPC virtual server instance the process is running on.
type Metadata struct {
	// ID is the instance identifier.
	ID string
	// Hostname is the instance name.
	Hostname string
	// CRN is the IBM Cloud Resource Name of the instance.
	CRN string
	// AccountID is taken from the CRN, which is the only place the metadata
	// document carries it.
	AccountID string
	// Type is the instance profile, for example "bx2-2x8".
	Type string
	// ImageID and ImageName describe the image the instance was created from.
	ImageID   string
	ImageName string
	// Zone is the zone name, for example "us-south-1".
	Zone string
	// Region is derived from Zone, for example "us-south". The metadata document
	// does not report it separately.
	Region string

	// CreatedAt is the instance's creation time, RFC 3339.
	CreatedAt string
	// Status and LifecycleState are the instance's reported states.
	Status         string
	LifecycleState string
	// Memory is in gibibytes, Bandwidth in megabits per second.
	Memory    int64
	Bandwidth int64
	// VCPU describes the instance's processors.
	VCPU VCPU
	// GPU describes attached accelerators, zero when there are none.
	GPU GPU
	// VPCID, VPCName and VPCCRN identify the VPC the instance runs in.
	VPCID   string
	VPCName string
	VPCCRN  string
	// ResourceGroupID and ResourceGroupName identify the resource group.
	ResourceGroupID   string
	ResourceGroupName string
	// PrimaryIP is the address on the primary network interface.
	PrimaryIP string
	// NetworkInterfaces are the instance's interfaces, the primary included.
	NetworkInterfaces []NetworkInterface
	// BootVolume is the volume the instance booted from.
	BootVolume Volume
	// Volumes are every attached volume, the boot volume included.
	Volumes []Volume
	// NUMACount is the number of NUMA nodes.
	NUMACount int
	// ConfidentialComputeMode and EnableSecureBoot describe the instance's
	// hardware protections.
	ConfidentialComputeMode string
	EnableSecureBoot        bool
	// HealthState is the instance's reported health.
	HealthState string
}

// VCPU describes an instance's processors.
type VCPU struct {
	Architecture string `json:"architecture"`
	Count        int    `json:"count"`
	Manufacturer string `json:"manufacturer"`
}

// GPU describes attached accelerators. Memory is in gibibytes.
type GPU struct {
	Count        int    `json:"count"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Memory       int    `json:"memory"`
}

// NetworkInterface is one interface attached to the instance.
type NetworkInterface struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Href      string `json:"href"`
	PrimaryIP struct {
		Address string `json:"address"`
		ID      string `json:"id"`
		Name    string `json:"name"`
	} `json:"primary_ip"`
	Subnet struct {
		ID   string `json:"id"`
		CRN  string `json:"crn"`
		Name string `json:"name"`
	} `json:"subnet"`
	ResourceType string `json:"resource_type"`
}

// Volume is one attached volume.
type Volume struct {
	ID   string `json:"id"`
	CRN  string `json:"crn"`
	Name string `json:"name"`
	Href string `json:"href"`
}

type document struct {
	ID      string `json:"id"`
	CRN     string `json:"crn"`
	Name    string `json:"name"`
	Profile struct {
		Name string `json:"name"`
	} `json:"profile"`
	Zone struct {
		Name string `json:"name"`
	} `json:"zone"`
	Image struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"image"`
	CreatedAt      string `json:"created_at"`
	Status         string `json:"status"`
	LifecycleState string `json:"lifecycle_state"`
	Memory         int64  `json:"memory"`
	Bandwidth      int64  `json:"bandwidth"`
	VCPU           VCPU   `json:"vcpu"`
	GPU            GPU    `json:"gpu"`
	VPC            struct {
		ID   string `json:"id"`
		CRN  string `json:"crn"`
		Name string `json:"name"`
	} `json:"vpc"`
	ResourceGroup struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"resource_group"`
	PrimaryNetworkInterface NetworkInterface   `json:"primary_network_interface"`
	NetworkInterfaces       []NetworkInterface `json:"network_interfaces"`
	BootVolumeAttachment    struct {
		Volume Volume `json:"volume"`
	} `json:"boot_volume_attachment"`
	VolumeAttachments []struct {
		Volume Volume `json:"volume"`
	} `json:"volume_attachments"`
	NUMACount               int    `json:"numa_count"`
	ConfidentialComputeMode string `json:"confidential_compute_mode"`
	EnableSecureBoot        bool   `json:"enable_secure_boot"`
	HealthState             string `json:"health_state"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

// Fetch reads the metadata of the VPC instance the process is running on.
//
// The error is [imds.ErrNotDetected] when the process is not on a VPC
// instance. Any other error means the metadata service was reachable and the
// lookup failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var doc document
	err := c.Lookup(ctx, func(ctx context.Context) error {
		endpoint := c.Endpoints(defaultEndpoints...)[0]

		// The token exchange is the availability probe: its outcome decides
		// whether the process is on IBM Cloud VPC.
		token, err := c.Token(ctx, endpoint, imds.TokenConfig{
			Path: tokenPath,
			Header: http.Header{
				"Metadata-Flavor": {"ibm"},
				"Accept":          {"application/json"},
				"Content-Type":    {"application/json"},
			},
			Body:         fmt.Appendf(nil, `{"expires_in":%d}`, tokenLifetime),
			Parse:        parseToken,
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
		})
		if err != nil {
			return err
		}

		// Past the token exchange the service has identified itself, so a 4xx is
		// its failure rather than a sign of being on another cloud.
		_, err = c.GetJSONFirst(ctx, []string{endpoint}, instancePath, &doc,
			token,
			imds.WithHeader("Accept", "application/json"),
			imds.NeverForeign())
		return err
	})
	if err != nil {
		return Metadata{}, err
	}

	return Metadata{
		ID:        doc.ID,
		Hostname:  doc.Name,
		CRN:       doc.CRN,
		AccountID: accountIDFromCRN(doc.CRN),
		Type:      doc.Profile.Name,
		ImageID:   doc.Image.ID,
		ImageName: doc.Image.Name,
		Zone:      doc.Zone.Name,
		Region:    regionFromZone(doc.Zone.Name),

		CreatedAt:               doc.CreatedAt,
		Status:                  doc.Status,
		LifecycleState:          doc.LifecycleState,
		Memory:                  doc.Memory,
		Bandwidth:               doc.Bandwidth,
		VCPU:                    doc.VCPU,
		GPU:                     doc.GPU,
		VPCID:                   doc.VPC.ID,
		VPCName:                 doc.VPC.Name,
		VPCCRN:                  doc.VPC.CRN,
		ResourceGroupID:         doc.ResourceGroup.ID,
		ResourceGroupName:       doc.ResourceGroup.Name,
		PrimaryIP:               doc.PrimaryNetworkInterface.PrimaryIP.Address,
		NetworkInterfaces:       doc.NetworkInterfaces,
		BootVolume:              doc.BootVolumeAttachment.Volume,
		Volumes:                 volumes(doc.VolumeAttachments),
		NUMACount:               doc.NUMACount,
		ConfidentialComputeMode: doc.ConfidentialComputeMode,
		EnableSecureBoot:        doc.EnableSecureBoot,
		HealthState:             doc.HealthState,
	}, nil
}

func parseToken(body []byte) (string, error) {
	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", err
	}
	return tr.AccessToken, nil
}

// accountIDFromCRN pulls the account out of a Resource Name, whose seventh
// colon-separated field is the account scope written as "a/<account>".
func accountIDFromCRN(crn string) string {
	parts := strings.Split(crn, ":")
	if len(parts) >= 7 {
		return strings.TrimPrefix(parts[6], "a/")
	}
	return ""
}

// regionFromZone returns the region a zone belongs to, "us-south" for
// "us-south-1". A zone carrying no region is returned unchanged, since it is
// then already the region.
func regionFromZone(zone string) string {
	if i := strings.LastIndex(zone, "-"); i > 0 {
		return zone[:i]
	}
	return zone
}

// volumes flattens the attachment wrappers, which carry nothing this package
// reports beyond the volume itself.
func volumes(in []struct {
	Volume Volume `json:"volume"`
},
) []Volume {
	if len(in) == 0 {
		return nil
	}
	out := make([]Volume, 0, len(in))
	for _, a := range in {
		out = append(out, a.Volume)
	}
	return out
}
