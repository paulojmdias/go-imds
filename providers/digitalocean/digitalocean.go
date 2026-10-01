// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package digitalocean

import (
	"context"
	"fmt"
	"strconv"

	"github.com/paulojmdias/go-imds/imds"
)

var defaultEndpoints = []string{"http://169.254.169.254"}

const metadataPath = "/metadata/v1.json"

// Metadata describes the Droplet the process is running on.
//
// user_data and vendor_data are in the document and are deliberately not here;
// see the package documentation.
type Metadata struct {
	// ID is the droplet identifier, in decimal. The metadata service reports it
	// as a number; it is rendered as a string here so that every provider in
	// this repository names its host identifier the same way.
	ID string
	// Hostname is the droplet hostname.
	Hostname string
	// Region is the region slug, for example "nyc3".
	Region string
	// Tags are the tags applied to the droplet.
	Tags []string
	// PublicKeys are the SSH public keys installed on the droplet.
	PublicKeys []string
	// Interfaces are the droplet's network interfaces, grouped as the service
	// groups them.
	Interfaces Interfaces
	// FloatingIP and ReservedIP describe a detachable address. ReservedIP is
	// the current name for the feature; FloatingIP is the older one, and a
	// droplet may report either.
	FloatingIP DetachableIP
	ReservedIP DetachableIP
	// Nameservers are the resolvers the droplet is configured with.
	Nameservers []string
	// DHCPEnabled reports whether the droplet uses DHCP.
	DHCPEnabled bool
}

// Interfaces groups a droplet's network interfaces by reachability.
type Interfaces struct {
	Public  []Interface
	Private []Interface
}

// Interface is one network interface.
type Interface struct {
	// Type is "public" or "private".
	Type string
	// MAC is the interface's hardware address.
	MAC string
	// IPv4 and IPv6 are the addresses configured on the interface. Either may
	// be absent, which is reported as a zero value rather than an error.
	IPv4 IPv4
	IPv6 IPv6
	// AnchorIPv4 is the address a floating or reserved IP is attached through.
	// It is set on public interfaces only.
	AnchorIPv4 IPv4
}

// IPv4 is an IPv4 address and its routing.
type IPv4 struct {
	Address string
	Netmask string
	Gateway string
}

// IPv6 is an IPv6 address and its routing.
type IPv6 struct {
	Address string
	CIDR    int
	Gateway string
}

// DetachableIP describes a floating or reserved address.
type DetachableIP struct {
	// Active reports whether an address is currently attached.
	Active bool
	// Address is the attached address, empty when none is.
	Address string
}

type document struct {
	DropletID  uint64   `json:"droplet_id"`
	Hostname   string   `json:"hostname"`
	Region     string   `json:"region"`
	Tags       []string `json:"tags"`
	PublicKeys []string `json:"public_keys"`
	Interfaces struct {
		Public  []iface `json:"public"`
		Private []iface `json:"private"`
	} `json:"interfaces"`
	FloatingIP detachable `json:"floating_ip"`
	ReservedIP detachable `json:"reserved_ip"`
	DNS        struct {
		Nameservers []string `json:"nameservers"`
	} `json:"dns"`
	Features struct {
		DHCPEnabled bool `json:"dhcp_enabled"`
	} `json:"features"`
}

type iface struct {
	Type       string `json:"type"`
	MAC        string `json:"mac"`
	IPv4       ipv4   `json:"ipv4"`
	AnchorIPv4 ipv4   `json:"anchor_ipv4"`
	IPv6       struct {
		Address string `json:"ip_address"`
		CIDR    int    `json:"cidr"`
		Gateway string `json:"gateway"`
	} `json:"ipv6"`
}

type ipv4 struct {
	Address string `json:"ip_address"`
	Netmask string `json:"netmask"`
	Gateway string `json:"gateway"`
}

type detachable struct {
	IPv4 struct {
		Active  bool   `json:"active"`
		Address string `json:"ip_address"`
	} `json:"ipv4"`
}

// convert maps the decoded interfaces onto the exported types, so that the
// json tags stay an implementation detail rather than part of the API.
func convert(in []iface) []Interface {
	if len(in) == 0 {
		return nil
	}
	out := make([]Interface, 0, len(in))
	for _, i := range in {
		out = append(out, Interface{
			Type:       i.Type,
			MAC:        i.MAC,
			IPv4:       IPv4(i.IPv4),
			AnchorIPv4: IPv4(i.AnchorIPv4),
			IPv6:       IPv6{Address: i.IPv6.Address, CIDR: i.IPv6.CIDR, Gateway: i.IPv6.Gateway},
		})
	}
	return out
}

// Fetch reads the metadata of the Droplet the process is running on.
//
// The error is [imds.ErrNotDetected] when the process is not on a Droplet,
// including when the shared address is answered by a metadata service
// belonging to another cloud. Any other error means DigitalOcean's service was
// reachable and the lookup failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var doc document
	err := c.Lookup(ctx, func(ctx context.Context) error {
		if _, err := c.GetJSONFirst(ctx, c.Endpoints(defaultEndpoints...), metadataPath, &doc); err != nil {
			return err
		}
		// Unknown JSON fields decode without complaint, so a document from a
		// different cloud at the same address would otherwise pass as an empty
		// Droplet. droplet_id is the field only DigitalOcean sets.
		if doc.DropletID == 0 {
			return fmt.Errorf("%w: no droplet_id in the metadata document", imds.ErrForeign)
		}
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}

	return Metadata{
		ID:          strconv.FormatUint(doc.DropletID, 10),
		Hostname:    doc.Hostname,
		Region:      doc.Region,
		Tags:        doc.Tags,
		PublicKeys:  doc.PublicKeys,
		Interfaces:  Interfaces{Public: convert(doc.Interfaces.Public), Private: convert(doc.Interfaces.Private)},
		FloatingIP:  DetachableIP{Active: doc.FloatingIP.IPv4.Active, Address: doc.FloatingIP.IPv4.Address},
		ReservedIP:  DetachableIP{Active: doc.ReservedIP.IPv4.Active, Address: doc.ReservedIP.IPv4.Address},
		Nameservers: doc.DNS.Nameservers,
		DHCPEnabled: doc.Features.DHCPEnabled,
	}, nil
}
