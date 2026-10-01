// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/paulojmdias/go-imds/imds"
)

var defaultEndpoints = []string{"http://100.100.100.200"}

const (
	tokenPath    = "/latest/api/token"
	metadataBase = "/latest/meta-data/"

	tokenTTLHeader = "X-aliyun-ecs-metadata-token-ttl-seconds"
	tokenHeader    = "X-aliyun-ecs-metadata-token"

	// tokenTTL is the lifetime requested for the session token. A lookup uses
	// its token for a handful of reads and then discards it, so the value only
	// has to comfortably outlast one lookup.
	tokenTTL = 6 * time.Hour
)

// Metadata describes the ECS instance the process is running on.
type Metadata struct {
	// ID is the instance identifier.
	ID string
	// Hostname is the instance hostname.
	Hostname string
	// AccountID is the account owning the instance.
	AccountID string
	// Type is the instance type, for example "ecs.g6.large".
	Type string
	// ImageID is the image the instance was created from.
	ImageID string
	// Region is the region identifier, for example "cn-hangzhou".
	Region string
	// Zone is the zone identifier, for example "cn-hangzhou-b".
	Zone string

	// SerialNumber is the instance's serial number.
	SerialNumber string
	// NetworkType is the network the instance is on, normally "vpc".
	NetworkType string
	// VPCID, VPCCIDRBlock, VSwitchID and VSwitchCIDRBlock place the instance
	// in its network.
	VPCID            string
	VPCCIDRBlock     string
	VSwitchID        string
	VSwitchCIDRBlock string
	// MAC, PrivateIP, PublicIP and ElasticIP are the primary interface's
	// address details. Any may be empty.
	MAC       string
	PrivateIP string
	PublicIP  string
	ElasticIP string
	// Nameservers and NTPServers are the configured resolvers and clocks, as
	// reported: one entry per line.
	Nameservers []string
	NTPServers  []string
	// SourceAddress is the package-manager mirror the instance is pointed at.
	SourceAddress string
	// MaxNetworkBandwidthEgress is the outbound limit, MaxPPS and
	// MaxConnections the instance type's packet and connection ceilings.
	MaxNetworkBandwidthEgress string
	MaxPPS                    string
	MaxConnections            string
	// VirtualizationSolution and its version describe the hypervisor.
	VirtualizationSolution        string
	VirtualizationSolutionVersion string
	// SpotTerminationTime is set only on a spot instance due to be reclaimed.
	SpotTerminationTime string
	// MarketplaceProductCode and MarketplaceChargeType are set only for an
	// instance created from a Marketplace image.
	MarketplaceProductCode string
	MarketplaceChargeType  string
	// KMSServer, WSUSServer and WSUSStatusServer are reported on Windows only.
	KMSServer        string
	WSUSServer       string
	WSUSStatusServer string
}

// Fetch reads the metadata of the ECS instance the process is running on.
//
// The error is [imds.ErrNotDetected] when the process is not on an ECS
// instance. Any other error means the metadata service was reachable and the
// lookup failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var md Metadata
	err := c.Lookup(ctx, func(ctx context.Context) error {
		endpoint := c.Endpoints(defaultEndpoints...)[0]

		// The token exchange doubles as the availability probe: nothing else is
		// requested until it succeeds, so its outcome is what decides whether
		// the process is on ECS.
		token, err := c.Token(ctx, endpoint, imds.TokenConfig{
			Path:       tokenPath,
			Header:     http.Header{tokenTTLHeader: {strconv.Itoa(int(tokenTTL.Seconds()))}},
			HeaderName: tokenHeader,
		})
		if err != nil {
			return err
		}

		// Past this point the metadata service has identified itself, so a 4xx
		// is that service failing rather than a sign of being on another cloud.
		// Leaving the default in place would report a missing path as "not on
		// Alibaba" after the service had already answered.
		read := []imds.RequestOption{token, imds.NeverForeign()}

		// The remaining required scalars. The token exchange above was the
		// probe, so a 4xx here is the service failing rather than a sign of
		// being on another cloud.
		for _, f := range []struct {
			path string
			dst  *string
		}{
			{"instance-id", &md.ID},
			{"hostname", &md.Hostname},
			{"owner-account-id", &md.AccountID},
			{"instance/instance-type", &md.Type},
			{"image-id", &md.ImageID},
			{"region-id", &md.Region},
			{"zone-id", &md.Zone},
		} {
			v, err := c.GetText(ctx, endpoint+metadataBase+f.path, read...)
			if err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
			*f.dst = v
		}

		// Paths that exist only in some configurations: no elastic address on
		// an instance without one, no activation server on Linux, no
		// termination time unless the instance is spot. A 404 means the
		// instance has no such field, which is not a failure.
		var nameservers, ntpServers string
		for _, f := range []struct {
			path string
			dst  *string
		}{
			{"serial-number", &md.SerialNumber},
			{"network-type", &md.NetworkType},
			{"vpc-id", &md.VPCID},
			{"vpc-cidr-block", &md.VPCCIDRBlock},
			{"vswitch-id", &md.VSwitchID},
			{"vswitch-cidr-block", &md.VSwitchCIDRBlock},
			{"mac", &md.MAC},
			{"private-ipv4", &md.PrivateIP},
			{"public-ipv4", &md.PublicIP},
			{"eipv4", &md.ElasticIP},
			{"dns-conf/nameservers", &nameservers},
			{"ntp-conf/ntp-servers", &ntpServers},
			{"source-address", &md.SourceAddress},
			{"instance/max-netbw-egress", &md.MaxNetworkBandwidthEgress},
			{"instance/instance-type-spec/max-netpps", &md.MaxPPS},
			{"instance/instance-type-spec/max-connections", &md.MaxConnections},
			{"instance/virtualization-solution", &md.VirtualizationSolution},
			{"instance/virtualization-solution-version", &md.VirtualizationSolutionVersion},
			{"instance/spot/termination-time", &md.SpotTerminationTime},
			{"image/market-place/product-code", &md.MarketplaceProductCode},
			{"image/market-place/charge-type", &md.MarketplaceChargeType},
			{"kms-server", &md.KMSServer},
			{"wsus-server/wu-server", &md.WSUSServer},
			{"wsus-server/wu-status-server", &md.WSUSStatusServer},
		} {
			v, err := c.GetTextOptional(ctx, endpoint+metadataBase+f.path, token)
			if err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
			*f.dst = v
		}
		md.Nameservers = lines(nameservers)
		md.NTPServers = lines(ntpServers)
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}
	return md, nil
}

// lines splits a response that carries one value per line, which is how this
// service reports the resolver and clock lists.
func lines(s string) []string {
	var out []string
	for line := range strings.SplitSeq(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}
