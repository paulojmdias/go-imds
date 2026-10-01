// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package scaleway

import (
	"context"
	"strings"

	"github.com/paulojmdias/go-imds/imds"
)

// Addresses of the Scaleway metadata service, tried in order.
var defaultEndpoints = []string{"http://169.254.42.42", "http://[fd00:42::42]"}

const metadataPath = "/conf?format=json"

// Metadata describes the Scaleway Instance the process is running on.
//
// The field set follows Scaleway's own metadata type in
// scaleway-sdk-go/api/instance/v1/instance_metadata_sdk.go. The nested types
// below carry the JSON tags and are decoded into directly, so there is no
// second copy of the shape to keep in step.
type Metadata struct {
	// ID is the instance identifier.
	ID string
	// Hostname is the instance hostname.
	Hostname string
	// Name is the instance name, which may differ from the hostname.
	Name string
	// AccountID is the owning organization.
	AccountID string
	// ProjectID is the project within the organization.
	ProjectID string
	// Type is the commercial type, for example "DEV1-S".
	Type string
	// ImageID and ImageName describe the image the instance was created from.
	ImageID   string
	ImageName string
	// Zone is the availability zone, for example "fr-par-1".
	Zone string
	// Region is derived from Zone, for example "fr-par". The metadata service
	// does not report it separately.
	Region string
	// Tags are the tags applied to the instance.
	Tags []string
	// StateDetail is the instance's reported state.
	StateDetail string
	// Timezone is the instance's configured timezone.
	Timezone string
	// PrivateIP is the instance's private address.
	PrivateIP string
	// PublicIP is the primary public address. PublicIPsV4 and PublicIPsV6 carry
	// every attached address, including the primary.
	PublicIP    IP
	PublicIPsV4 []IP
	PublicIPsV6 []IP
	// IPv6 is the legacy single-address IPv6 configuration.
	IPv6 IPv6
	// Location places the instance within Scaleway's own topology.
	Location Location
	// SSHPublicKeys are the keys installed on the instance.
	SSHPublicKeys []SSHPublicKey
	// Volumes are the attached volumes, keyed as the service keys them.
	Volumes map[string]Volume
	// PrivateNICs are the instance's private-network interfaces.
	PrivateNICs []PrivateNIC
}

// IP is a public address attached to the instance.
type IP struct {
	ID               string   `json:"id"`
	Address          string   `json:"address"`
	Dynamic          bool     `json:"dynamic"`
	Gateway          string   `json:"gateway"`
	Netmask          string   `json:"netmask"`
	Family           string   `json:"family"`
	ProvisioningMode string   `json:"provisioning_mode"`
	Tags             []string `json:"tags"`
}

// IPv6 is the legacy single-address IPv6 configuration.
type IPv6 struct {
	Address string `json:"address"`
	Gateway string `json:"gateway"`
	Netmask string `json:"netmask"`
}

// Location places the instance on Scaleway's hardware.
type Location struct {
	PlatformID   string `json:"platform_id"`
	HypervisorID string `json:"hypervisor_id"`
	NodeID       string `json:"node_id"`
	ClusterID    string `json:"cluster_id"`
	ZoneID       string `json:"zone_id"`
}

// SSHPublicKey is one key installed on the instance.
type SSHPublicKey struct {
	ID               string `json:"id"`
	Key              string `json:"key"`
	Fingerprint      string `json:"fingerprint"`
	Description      string `json:"description"`
	IP               string `json:"ip"`
	CreationDate     string `json:"creation_date"`
	ModificationDate string `json:"modification_date"`
}

// Volume is one attached volume.
type Volume struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	VolumeType       string `json:"volume_type"`
	Size             int64  `json:"size"`
	State            string `json:"state"`
	ExportURI        string `json:"export_uri"`
	Organization     string `json:"organization"`
	Project          string `json:"project"`
	CreationDate     string `json:"creation_date"`
	ModificationDate string `json:"modification_date"`
	Server           struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"server"`
}

// PrivateNIC is one private-network interface.
type PrivateNIC struct {
	ID               string `json:"id"`
	PrivateNetworkID string `json:"private_network_id"`
	ServerID         string `json:"server_id"`
	MacAddress       string `json:"mac_address"`
	Zone             string `json:"zone"`
	CreationDate     string `json:"creation_date"`
}

type document struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Hostname       string `json:"hostname"`
	Organization   string `json:"organization"`
	Project        string `json:"project"`
	CommercialType string `json:"commercial_type"`
	Image          struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"image"`
	Location      Location          `json:"location"`
	Tags          []string          `json:"tags"`
	StateDetail   string            `json:"state_detail"`
	Timezone      string            `json:"timezone"`
	PrivateIP     string            `json:"private_ip"`
	PublicIP      IP                `json:"public_ip"`
	PublicIpsV4   []IP              `json:"public_ips_v4"`
	PublicIpsV6   []IP              `json:"public_ips_v6"`
	IPv6          IPv6              `json:"ipv6"`
	SSHPublicKeys []SSHPublicKey    `json:"ssh_public_keys"`
	Volumes       map[string]Volume `json:"volumes"`
	PrivateNICs   []PrivateNIC      `json:"private_nics"`
}

// Fetch reads the metadata of the Scaleway Instance the process is running on.
//
// The error is [imds.ErrNotDetected] when the process is not on a Scaleway
// Instance. Any other error means the metadata service was reachable and the
// lookup failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var doc document
	err := c.Lookup(ctx, func(ctx context.Context) error {
		_, err := c.GetJSONFirst(ctx, c.Endpoints(defaultEndpoints...), metadataPath, &doc,
			// The addresses belong to Scaleway alone, so anything answering at
			// them is the metadata service. A 4xx is that service failing, not
			// a sign of being on a different cloud.
			imds.NeverForeign())
		return err
	})
	if err != nil {
		return Metadata{}, err
	}

	// The document carries both, and they usually agree. An instance created
	// before Scaleway reported hostname separately has only name, and this
	// provider reported that as the hostname before the field existed.
	hostname := doc.Hostname
	if hostname == "" {
		hostname = doc.Name
	}

	return Metadata{
		ID:            doc.ID,
		Hostname:      hostname,
		Name:          doc.Name,
		AccountID:     doc.Organization,
		ProjectID:     doc.Project,
		Type:          doc.CommercialType,
		ImageID:       doc.Image.ID,
		ImageName:     doc.Image.Name,
		Zone:          doc.Location.ZoneID,
		Region:        regionFromZone(doc.Location.ZoneID),
		Tags:          doc.Tags,
		StateDetail:   doc.StateDetail,
		Timezone:      doc.Timezone,
		PrivateIP:     doc.PrivateIP,
		PublicIP:      doc.PublicIP,
		PublicIPsV4:   doc.PublicIpsV4,
		PublicIPsV6:   doc.PublicIpsV6,
		IPv6:          doc.IPv6,
		Location:      doc.Location,
		SSHPublicKeys: doc.SSHPublicKeys,
		Volumes:       doc.Volumes,
		PrivateNICs:   doc.PrivateNICs,
	}, nil
}

// regionFromZone returns the region a zone belongs to, "fr-par" for "fr-par-1".
// It returns an empty string for a zone that carries no region.
func regionFromZone(zone string) string {
	if i := strings.LastIndex(zone, "-"); i > 0 {
		return zone[:i]
	}
	return ""
}
