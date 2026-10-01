// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package upcloud

import (
	"context"
	"fmt"

	"github.com/paulojmdias/go-imds/imds"
)

var defaultEndpoints = []string{"http://169.254.169.254"}

const metadataPath = "/metadata/v1.json"

// Metadata describes the UpCloud server the process is running on.
type Metadata struct {
	// ID is the server UUID.
	ID string
	// Hostname is the server hostname.
	Hostname string
	// Region is the zone the server runs in, for example "de-fra1".
	Region string
	// CloudName is the platform name the metadata service reports, for example
	// "upcloud". It is what distinguishes this document from DigitalOcean's at
	// the same address.
	CloudName string
	// Platform and Subplatform describe how the server is being run, for
	// example "servers" and "metadata (http://169.254.169.254)".
	Platform    string
	Subplatform string
	// Tags are the tags applied to the server.
	Tags []string
	// PublicKeys are the SSH public keys installed on the server.
	PublicKeys []string
	// Interfaces are the server's network interfaces.
	Interfaces []Interface
	// Nameservers are the resolvers the server is configured with.
	Nameservers []string
	// Disks are the attached storage devices.
	Disks []Disk
}

// Interface is one network interface. Type is "public", "utility" or
// "private", which is UpCloud's own division.
type Interface struct {
	Index       int         `json:"index"`
	MAC         string      `json:"mac"`
	NetworkID   string      `json:"network_id"`
	Type        string      `json:"type"`
	IPAddresses []IPAddress `json:"ip_addresses"`
}

// IPAddress is one address configured on an interface. Family is "IPv4" or
// "IPv6", as reported.
type IPAddress struct {
	Address  string   `json:"address"`
	Family   string   `json:"family"`
	Network  string   `json:"network"`
	Gateway  string   `json:"gateway"`
	DHCP     bool     `json:"dhcp"`
	Floating bool     `json:"floating"`
	DNS      []string `json:"dns"`
}

// Disk is one attached storage device. Size is in mebibytes.
type Disk struct {
	ID     string `json:"id"`
	Serial string `json:"serial"`
	Size   int64  `json:"size"`
	Type   string `json:"type"`
	Tier   string `json:"tier"`
}

type document struct {
	CloudName   string   `json:"cloud_name"`
	Hostname    string   `json:"hostname"`
	InstanceID  string   `json:"instance_id"`
	Region      string   `json:"region"`
	Platform    string   `json:"platform"`
	Subplatform string   `json:"subplatform"`
	Tags        []string `json:"tags"`
	PublicKeys  []string `json:"public_keys"`
	Network     struct {
		Interfaces []Interface `json:"interfaces"`
		DNS        []string    `json:"dns"`
	} `json:"network"`
	Storage struct {
		Disks []Disk `json:"disks"`
	} `json:"storage"`
}

// Fetch reads the metadata of the UpCloud server the process is running on.
//
// The error is [imds.ErrNotDetected] when the process is not on an UpCloud
// server, including when the shared address is answered by a metadata service
// belonging to another cloud. Any other error means UpCloud's service was
// reachable and the lookup failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var doc document
	err := c.Lookup(ctx, func(ctx context.Context) error {
		if _, err := c.GetJSONFirst(ctx, c.Endpoints(defaultEndpoints...), metadataPath, &doc); err != nil {
			return err
		}
		// See the package documentation: DigitalOcean serves a different
		// document at this exact path, and unknown fields decode silently.
		if doc.CloudName == "" {
			return fmt.Errorf("%w: no cloud_name in the metadata document", imds.ErrForeign)
		}
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}

	return Metadata{
		ID:          doc.InstanceID,
		Hostname:    doc.Hostname,
		Region:      doc.Region,
		CloudName:   doc.CloudName,
		Platform:    doc.Platform,
		Subplatform: doc.Subplatform,
		Tags:        doc.Tags,
		PublicKeys:  doc.PublicKeys,
		Interfaces:  doc.Network.Interfaces,
		Nameservers: doc.Network.DNS,
		Disks:       doc.Storage.Disks,
	}, nil
}
