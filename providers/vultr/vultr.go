// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package vultr

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/paulojmdias/go-imds/imds"
)

var defaultEndpoints = []string{"http://169.254.169.254"}

const metadataPath = "/v1.json"

// Metadata describes the Vultr instance the process is running on.
type Metadata struct {
	// ID is the instance identifier. The v2 UUID is preferred when the metadata
	// service reports one, falling back to the legacy numeric identifier.
	ID string
	// Hostname is the instance hostname.
	Hostname string
	// Region is the region code, lower-cased. The metadata service reports it
	// upper-cased ("EWR"), which does not match what other tooling records for
	// the same instance.
	Region string
	// LegacyID is the numeric identifier the service has always reported. It is
	// kept alongside ID because existing tooling records instances by it.
	LegacyID string
	// CountryCode is the region's country, for example "US".
	CountryCode string
	// PublicKeys are the SSH public keys installed on the instance.
	PublicKeys []string
	// Interfaces are the instance's network interfaces.
	Interfaces []Interface
	// BGP is the instance's BGP session configuration, zero when BGP is not
	// enabled.
	BGP BGP
}

// Interface is one network interface. NetworkType is "public" or "private".
type Interface struct {
	MAC         string `json:"mac"`
	NetworkType string `json:"network-type"`
	IPv4        struct {
		Address    string       `json:"address"`
		Netmask    string       `json:"netmask"`
		Gateway    string       `json:"gateway"`
		Additional []Additional `json:"additional"`
	} `json:"ipv4"`
	IPv6 struct {
		Address    string       `json:"address"`
		Network    string       `json:"network"`
		Prefix     string       `json:"prefix"`
		Additional []Additional `json:"additional"`
	} `json:"ipv6"`
}

// Additional is a secondary address on an interface. Netmask is set for IPv4,
// Network and Prefix for IPv6.
type Additional struct {
	Address string `json:"address"`
	Netmask string `json:"netmask"`
	Network string `json:"network"`
	Prefix  string `json:"prefix"`
}

// BGP is the instance's BGP session configuration.
type BGP struct {
	IPv4 BGPSession `json:"ipv4"`
	IPv6 BGPSession `json:"ipv6"`
}

// BGPSession is one address family's BGP session. The ASNs are reported as
// strings by the service and are kept that way.
type BGPSession struct {
	MyAddress   string `json:"my-address"`
	MyASN       string `json:"my-asn"`
	PeerAddress string `json:"peer-address"`
	PeerASN     string `json:"peer-asn"`
}

type document struct {
	Hostname     string      `json:"hostname"`
	InstanceID   string      `json:"instanceid"`
	InstanceV2ID string      `json:"instance-v2-id"`
	PublicKeys   publicKeys  `json:"public-keys"`
	Interfaces   []Interface `json:"interfaces"`
	BGP          BGP         `json:"bgp"`
	Region       struct {
		RegionCode  string `json:"regioncode"`
		CountryCode string `json:"countrycode"`
	} `json:"region"`
}

// publicKeys decodes either shape the service uses for this key: a JSON array,
// or a single newline-separated string. Decoding straight into []string fails
// on the second, which is what older instances return.
type publicKeys []string

func (k *publicKeys) UnmarshalJSON(b []byte) error {
	var list []string
	if err := json.Unmarshal(b, &list); err == nil {
		*k = list
		return nil
	}

	var blob string
	if err := json.Unmarshal(b, &blob); err != nil {
		return err
	}
	*k = nil
	for line := range strings.SplitSeq(blob, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			*k = append(*k, line)
		}
	}
	return nil
}

// Fetch reads the metadata of the Vultr instance the process is running on.
//
// The error is [imds.ErrNotDetected] when the process is not on a Vultr
// instance. Any other error means the metadata service was reachable and the
// lookup failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var doc document
	err := c.Lookup(ctx, func(ctx context.Context) error {
		_, err := c.GetJSONFirst(ctx, c.Endpoints(defaultEndpoints...), metadataPath, &doc)
		return err
	})
	if err != nil {
		return Metadata{}, err
	}

	id := doc.InstanceV2ID
	if id == "" {
		id = doc.InstanceID
	}

	return Metadata{
		ID:          id,
		Hostname:    doc.Hostname,
		Region:      strings.ToLower(doc.Region.RegionCode),
		LegacyID:    doc.InstanceID,
		CountryCode: doc.Region.CountryCode,
		PublicKeys:  doc.PublicKeys,
		Interfaces:  doc.Interfaces,
		BGP:         doc.BGP,
	}, nil
}
