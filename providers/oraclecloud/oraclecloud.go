// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package oraclecloud

import (
	"context"
	"fmt"

	"github.com/paulojmdias/go-imds/imds"
)

var defaultEndpoints = []string{"http://169.254.169.254"}

const instancePath = "/opc/v2/instance/"

// Metadata describes the compute instance the process is running on.
type Metadata struct {
	// ID is the instance OCID, which is also its resource identifier.
	ID string
	// Hostname is the instance display name.
	Hostname string
	// Type is the instance shape, for example "VM.Standard.E4.Flex".
	Type string
	// Region is the canonical region name, for example "eu-frankfurt-1".
	Region string
	// AvailabilityDomain is the availability domain, for example "Uocm:PHX-AD-1".
	AvailabilityDomain string
	// Realm is the realm the region belongs to, for example "oc1".
	Realm string
	// OKEClusterName is the display name of the Container Engine cluster the
	// instance belongs to, empty for an instance outside one.
	OKEClusterName string

	// FaultDomain is the fault domain within the availability domain.
	FaultDomain string
	// CompartmentID and TenantID identify the instance's place in the tenancy.
	CompartmentID string
	TenantID      string
	// ImageID is the OCID of the image the instance was created from.
	ImageID string
	// State is the instance's lifecycle state, for example "Running".
	State string
	// TimeCreated is the creation time in milliseconds since the epoch, as
	// reported.
	TimeCreated int64
	// ADName is the availability domain's tenancy-independent name.
	ADName string
	// RegionIdentifier is the full region name, for example "eu-frankfurt-1".
	RegionIdentifier string
	// RealmDomainComponent and RegionKey complete the region identity.
	RealmDomainComponent string
	RegionKey            string
	// ShapeConfig is the instance's allocated resources.
	ShapeConfig ShapeConfig
	// DefinedTags and FreeformTags are the tags applied to the instance.
	DefinedTags  map[string]map[string]string
	FreeformTags map[string]string
	// AgentConfig describes the Oracle Cloud Agent's plugins.
	AgentConfig AgentConfig
}

// ShapeConfig is an instance's allocated resources.
type ShapeConfig struct {
	OCPUs                     float64 `json:"ocpus"`
	VCPUs                     int     `json:"vcpus"`
	MemoryInGBs               float64 `json:"memoryInGBs"`
	NetworkingBandwidthInGbps float64 `json:"networkingBandwidthInGbps"`
	MaxVnicAttachments        int     `json:"maxVnicAttachments"`
	GPUs                      int     `json:"gpus"`
	GPUDescription            string  `json:"gpuDescription"`
	LocalDisks                int     `json:"localDisks"`
	LocalDisksTotalSizeInGBs  float64 `json:"localDisksTotalSizeInGBs"`
	LocalDiskDescription      string  `json:"localDiskDescription"`
	BaselineOCPUUtilization   string  `json:"baselineOcpuUtilization"`
}

// AgentConfig describes the Oracle Cloud Agent's configuration.
type AgentConfig struct {
	MonitoringDisabled bool           `json:"monitoringDisabled"`
	ManagementDisabled bool           `json:"managementDisabled"`
	AllPluginsDisabled bool           `json:"allPluginsDisabled"`
	PluginsConfig      []PluginConfig `json:"pluginsConfig"`
}

// PluginConfig is one Oracle Cloud Agent plugin and its desired state.
type PluginConfig struct {
	Name         string `json:"name"`
	DesiredState string `json:"desiredState"`
}

type document struct {
	ID                 string      `json:"id"`
	DisplayName        string      `json:"displayName"`
	Shape              string      `json:"shape"`
	CanonicalRegion    string      `json:"canonicalRegionName"`
	AvailabilityDomain string      `json:"availabilityDomain"`
	FaultDomain        string      `json:"faultDomain"`
	CompartmentID      string      `json:"compartmentId"`
	TenantID           string      `json:"tenantId"`
	Image              string      `json:"image"`
	State              string      `json:"state"`
	TimeCreated        int64       `json:"timeCreated"`
	OCIAdName          string      `json:"ociAdName"`
	ShapeConfig        ShapeConfig `json:"shapeConfig"`
	AgentConfig        AgentConfig `json:"agentConfig"`
	RegionInfo         struct {
		RealmKey             string `json:"realmKey"`
		RealmDomainComponent string `json:"realmDomainComponent"`
		RegionKey            string `json:"regionKey"`
		RegionIdentifier     string `json:"regionIdentifier"`
	} `json:"regionInfo"`
	DefinedTags  map[string]map[string]string `json:"definedTags"`
	FreeformTags map[string]string            `json:"freeformTags"`

	// Only the two descriptive keys are taken from metadata. The map also
	// carries whatever the launcher put there, which on Oracle includes
	// cloud-init user data and is not this package's to hand back.
	Metadata struct {
		OKEClusterDisplayName string `json:"oke-cluster-display-name"`
		Realm                 string `json:"realm"`
	} `json:"metadata"`
}

// Fetch reads the metadata of the compute instance the process is running on.
//
// The error is [imds.ErrNotDetected] when the process is not on an OCI compute
// instance. Any other error means the metadata service was reachable and the
// lookup failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var doc document
	err := c.Lookup(ctx, func(ctx context.Context) error {
		if _, err := c.GetJSONFirst(ctx, c.Endpoints(defaultEndpoints...), instancePath, &doc,
			imds.WithHeader("Authorization", "Bearer Oracle"),
			imds.WithHeader("Accept", "application/json")); err != nil {
			return err
		}
		// The address is shared with several clouds, and unknown fields decode
		// without complaint, so a document from one of them would otherwise
		// pass as an instance with no identifier.
		if doc.ID == "" {
			return fmt.Errorf("%w: no id in the metadata document", imds.ErrForeign)
		}
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}

	return Metadata{
		ID:                 doc.ID,
		Hostname:           doc.DisplayName,
		Type:               doc.Shape,
		Region:             doc.CanonicalRegion,
		AvailabilityDomain: doc.AvailabilityDomain,
		Realm:              doc.Metadata.Realm,
		OKEClusterName:     doc.Metadata.OKEClusterDisplayName,

		FaultDomain:          doc.FaultDomain,
		CompartmentID:        doc.CompartmentID,
		TenantID:             doc.TenantID,
		ImageID:              doc.Image,
		State:                doc.State,
		TimeCreated:          doc.TimeCreated,
		ADName:               doc.OCIAdName,
		RegionIdentifier:     doc.RegionInfo.RegionIdentifier,
		RealmDomainComponent: doc.RegionInfo.RealmDomainComponent,
		RegionKey:            doc.RegionInfo.RegionKey,
		ShapeConfig:          doc.ShapeConfig,
		DefinedTags:          doc.DefinedTags,
		FreeformTags:         doc.FreeformTags,
		AgentConfig:          doc.AgentConfig,
	}, nil
}
