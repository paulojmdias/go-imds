// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package cvm

import (
	"context"
	"fmt"
	"net/url"

	"github.com/paulojmdias/go-imds/imds"
)

var defaultEndpoints = []string{"http://metadata.tencentyun.com"}

const metadataBase = "/latest/meta-data/"

// Metadata describes the CVM instance the process is running on.
type Metadata struct {
	// ID is the instance identifier.
	ID string
	// Hostname is the instance name.
	Hostname string
	// AccountID is the application identifier owning the instance.
	AccountID string
	// Type is the instance type, for example "S5.MEDIUM4".
	Type string
	// ImageID is the image the instance was created from.
	ImageID string
	// Region is the region identifier, for example "ap-guangzhou".
	Region string
	// Zone is the zone identifier, for example "ap-guangzhou-3".
	Zone string

	// UUID is the instance identifier in its other form.
	UUID string
	// PrivateIP and PublicIP are the primary interface's addresses. PublicIP
	// is empty on an instance with no public address.
	PrivateIP string
	PublicIP  string
	// MAC is the primary interface's hardware address.
	MAC string
	// ChargeType is the billing plan, CreateTime and TerminationTime bracket
	// the instance's paid life. TerminationTime is empty for a pay-as-you-go
	// instance.
	ChargeType      string
	CreateTime      string
	TerminationTime string
	// SpotTerminationTime is set only on a spot instance due to be reclaimed.
	SpotTerminationTime string
	// ASGroupID is the auto scaling group, empty when the instance is not in
	// one.
	ASGroupID string
	// SecurityGroup lists the groups bound to the instance, as reported.
	SecurityGroup string
	// BandwidthLimitEgress and BandwidthLimitIngress are private-network
	// limits in Kbit/s.
	BandwidthLimitEgress  string
	BandwidthLimitIngress string
	// Volumes describes the instance's storage, as reported.
	Volumes string
	// Interface describes the primary network interface.
	Interface Interface
}

// Interface is the primary network interface.
type Interface struct {
	MAC            string
	PrimaryLocalIP string
	PublicIPs      string
	VPCID          string
	SubnetID       string
	Gateway        string
	SubnetMask     string
}

// Fetch reads the metadata of the CVM instance the process is running on.
//
// The error is [imds.ErrNotDetected] when the process is not on a CVM
// instance. Any other error means the metadata service was reachable and the
// lookup failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var md Metadata
	err := c.Lookup(ctx, func(ctx context.Context) error {
		endpoint := c.Endpoints(defaultEndpoints...)[0]

		// Every documented scalar path. The first is the availability probe: a
		// 4xx there means this is not Tencent's metadata service. Once it has
		// answered, a later 4xx is that service failing, and calling it "not on
		// Tencent" would discard an instance already identified.
		required := []struct {
			path string
			dst  *string
		}{
			{"instance-id", &md.ID},
			{"instance-name", &md.Hostname},
			{"app-id", &md.AccountID},
			{"instance/instance-type", &md.Type},
			{"instance/image-id", &md.ImageID},
			{"placement/region", &md.Region},
			{"placement/zone", &md.Zone},
			{"uuid", &md.UUID},
			{"local-ipv4", &md.PrivateIP},
			{"mac", &md.MAC},
		}
		for i, f := range required {
			var read []imds.RequestOption
			if i > 0 {
				read = []imds.RequestOption{imds.NeverForeign()}
			}
			v, err := c.GetText(ctx, endpoint+metadataBase+f.path, read...)
			if err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
			*f.dst = v
		}

		// Paths that exist only in some configurations. A 404 means the
		// instance has no such field, which is not a failure -- an instance
		// that is not spot has no termination time and never will.
		optional := []struct {
			path string
			dst  *string
		}{
			{"public-ipv4", &md.PublicIP},
			{"payment/charge-type", &md.ChargeType},
			{"payment/create-time", &md.CreateTime},
			{"payment/termination-time", &md.TerminationTime},
			{"spot/termination-time", &md.SpotTerminationTime},
			{"as-group-id", &md.ASGroupID},
			{"instance/security-group", &md.SecurityGroup},
			{"instance/bandwidth-limit-egress", &md.BandwidthLimitEgress},
			{"instance/bandwidth-limit-ingress", &md.BandwidthLimitIngress},
			{"volumes", &md.Volumes},
		}
		for _, f := range optional {
			v, err := c.GetTextOptional(ctx, endpoint+metadataBase+f.path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
			*f.dst = v
		}

		// The interface tree is keyed by the primary MAC, which the read above
		// supplied.
		if md.MAC != "" {
			base := endpoint + metadataBase + "network/interfaces/macs/" + url.PathEscape(md.MAC) + "/"
			md.Interface.MAC = md.MAC
			for _, f := range []struct {
				path string
				dst  *string
			}{
				{"primary-local-ipv4", &md.Interface.PrimaryLocalIP},
				{"public-ipv4s", &md.Interface.PublicIPs},
				{"vpc-id", &md.Interface.VPCID},
				{"subnet-id", &md.Interface.SubnetID},
			} {
				v, err := c.GetTextOptional(ctx, base+f.path)
				if err != nil {
					return fmt.Errorf("reading %s: %w", f.path, err)
				}
				*f.dst = v
			}
			if ip := md.Interface.PrimaryLocalIP; ip != "" {
				addr := base + "local-ipv4s/" + url.PathEscape(ip) + "/"
				for _, f := range []struct {
					path string
					dst  *string
				}{
					{"gateway", &md.Interface.Gateway},
					{"subnet-mask", &md.Interface.SubnetMask},
				} {
					v, err := c.GetTextOptional(ctx, addr+f.path)
					if err != nil {
						return fmt.Errorf("reading %s: %w", f.path, err)
					}
					*f.dst = v
				}
			}
		}
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}
	return md, nil
}
