// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package hetzner

import (
	"context"
	"fmt"

	"go.yaml.in/yaml/v3"

	"github.com/paulojmdias/go-imds/imds"
)

var defaultEndpoints = []string{"http://169.254.169.254"}

// metadataPath serves the whole document. Hetzner also serves one path per
// field, which is what this package used to read: four requests for four
// scalars, and no way to reach the nested network configuration at all.
const metadataPath = "/hetzner/v1/metadata"

// Metadata describes the Hetzner Cloud server the process is running on.
//
// The document also carries vendor_data, a cloud-config blob with a random
// seed in it. It is deliberately not here; see the package documentation.
type Metadata struct {
	// ID is the server identifier, in decimal. The metadata service reports it
	// as a number; it is rendered as a string here so that every provider in
	// this repository names its host identifier the same way.
	ID string
	// Hostname is the server hostname.
	Hostname string
	// Region is the network zone, for example "eu-central".
	Region string
	// Zone is the data centre, for example "fsn1-dc14".
	Zone string
	// PublicIPv4 and PrivateIPv4 are the server's addresses.
	PublicIPv4  string
	PrivateIPv4 string
	// PublicKeys are the SSH public keys installed on the server.
	PublicKeys []string
	// Interfaces are the network interfaces, as cloud-init network-config v1
	// describes them.
	Interfaces []Interface
	// NetworkConfigVersion is the network-config schema version, normally 1.
	NetworkConfigVersion int
}

// Interface is one network interface.
type Interface struct {
	Name       string   `yaml:"name"`
	Type       string   `yaml:"type"`
	MACAddress string   `yaml:"mac_address"`
	Subnets    []Subnet `yaml:"subnets"`
}

// Subnet is one address configured on an interface. Type is "dhcp", "static"
// or "static6", as cloud-init names them.
type Subnet struct {
	Type           string   `yaml:"type"`
	Address        string   `yaml:"address"`
	Gateway        string   `yaml:"gateway"`
	Netmask        string   `yaml:"netmask"`
	IPv4           bool     `yaml:"ipv4"`
	IPv6           bool     `yaml:"ipv6"`
	DNSNameservers []string `yaml:"dns_nameservers"`
	DNSSearch      []string `yaml:"dns_search"`
}

type document struct {
	Hostname      string   `yaml:"hostname"`
	InstanceID    int64    `yaml:"instance-id"`
	PublicIPv4    string   `yaml:"public-ipv4"`
	PrivateIPv4   string   `yaml:"local-ipv4"`
	AvailZone     string   `yaml:"availability-zone"`
	Region        string   `yaml:"region"`
	PublicKeys    []string `yaml:"public-keys"`
	NetworkConfig struct {
		Version int         `yaml:"version"`
		Config  []Interface `yaml:"config"`
	} `yaml:"network-config"`
}

// Fetch reads the metadata of the Hetzner Cloud server the process is running
// on.
//
// The error is [imds.ErrNotDetected] when the process is not on a Hetzner
// Cloud server. Any other error means the metadata service was reachable and
// the lookup failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var doc document
	err := c.Lookup(ctx, func(ctx context.Context) error {
		// 169.254.169.254 is shared, so a 4xx here means something other than
		// Hetzner's service answered.
		body, endpoint, err := c.GetFirst(ctx, c.Endpoints(defaultEndpoints...), metadataPath)
		if err != nil {
			return err
		}
		// The document is YAML, not JSON, which is why this provider carries
		// the one non-test dependency in the repository.
		if err := yaml.Unmarshal(body, &doc); err != nil {
			return fmt.Errorf("%s: decoding metadata document: %w", metadataPath, err)
		}
		// Every Hetzner server reports an instance-id. Its absence means the
		// document came from something else answering on the shared address.
		if doc.InstanceID == 0 {
			return fmt.Errorf("%w: no instance-id in the metadata document", imds.ErrForeign)
		}

		// region and availability-zone are served as their own paths, and are
		// not reliably in the document. They are read only when the document
		// did not supply them, so a server that reports them inline costs one
		// request rather than three -- and neither loses a field the
		// path-per-field version of this provider used to return.
		//
		// The service has identified itself by now, so a 4xx here is that
		// service failing rather than a sign of being on another cloud.
		for _, f := range []struct {
			path string
			dst  *string
		}{
			{"/region", &doc.Region},
			{"/availability-zone", &doc.AvailZone},
		} {
			if *f.dst != "" {
				continue
			}
			v, err := c.GetText(ctx, endpoint+metadataPath+f.path, imds.NeverForeign())
			if err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
			*f.dst = v
		}
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}

	return Metadata{
		ID:                   fmt.Sprint(doc.InstanceID),
		Hostname:             doc.Hostname,
		Region:               doc.Region,
		Zone:                 doc.AvailZone,
		PublicIPv4:           doc.PublicIPv4,
		PrivateIPv4:          doc.PrivateIPv4,
		PublicKeys:           doc.PublicKeys,
		Interfaces:           doc.NetworkConfig.Config,
		NetworkConfigVersion: doc.NetworkConfig.Version,
	}, nil
}
