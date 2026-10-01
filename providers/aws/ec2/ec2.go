// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package ec2

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/paulojmdias/go-imds/imds"
)

// The environment variables the AWS tooling defines for the metadata service.
const (
	disabledEnv     = "AWS_EC2_METADATA_DISABLED"
	endpointEnv     = "AWS_EC2_METADATA_SERVICE_ENDPOINT"
	endpointModeEnv = "AWS_EC2_METADATA_SERVICE_ENDPOINT_MODE"
)

const (
	endpointIPv4 = "http://169.254.169.254"
	endpointIPv6 = "http://[fd00:ec2::254]"
)

const (
	tokenPath    = "/latest/api/token"
	documentPath = "/latest/dynamic/instance-identity/document"
	metaDataBase = "/latest/meta-data/"
	hostnamePath = "/latest/meta-data/hostname"

	tokenHeader    = "X-aws-ec2-metadata-token"
	tokenTTLHeader = "X-aws-ec2-metadata-token-ttl-seconds"
	tokenTTL       = 6 * time.Hour
)

// Metadata describes the EC2 instance the process is running on.
type Metadata struct {
	// ID is the instance identifier.
	ID string
	// Hostname is the instance hostname.
	Hostname string
	// AccountID is the account owning the instance.
	AccountID string
	// Type is the instance type, for example "m5.large".
	Type string
	// ImageID is the AMI the instance was launched from.
	ImageID string
	// Region is the region, for example "eu-west-1".
	Region string
	// Zone is the availability zone, for example "eu-west-1a".
	Zone string
	// Architecture is the instance's CPU architecture, as reported.
	Architecture string
	// PrivateIP is the instance's private IPv4 address.
	PrivateIP string
	// KernelID and RamdiskID are set only for the paravirtual instance types
	// that still use them.
	KernelID  string
	RamdiskID string
	// PendingTime is when the instance was launched, RFC 3339.
	PendingTime string
	// DocumentVersion is the identity document's own schema version.
	DocumentVersion string
	// BillingProducts, MarketplaceProductCodes and DevpayProductCodes record
	// how the image is licensed, for example a Marketplace or Windows charge.
	BillingProducts         []string
	MarketplaceProductCodes []string
	DevpayProductCodes      []string

	// LocalHostname and PublicHostname are the instance's DNS names.
	// PublicHostname is empty without a public address.
	LocalHostname  string
	PublicHostname string
	// PublicIP is the instance's public IPv4 address, empty without one.
	PublicIP string
	// MAC is the primary interface's hardware address.
	MAC string
	// AMILaunchIndex distinguishes instances launched together from one
	// request, and ReservationID identifies that request.
	AMILaunchIndex string
	ReservationID  string
	// InstanceLifecycle is "spot" on a spot instance and "on-demand"
	// otherwise.
	InstanceLifecycle string
	// SpotTerminationTime is set only on a spot instance due to be reclaimed.
	SpotTerminationTime string
	// AutoscalingLifecycleState is set only for an instance in an auto scaling
	// group.
	AutoscalingLifecycleState string
	// SecurityGroups are the groups on the primary interface.
	SecurityGroups []string
	// PlacementGroup, HostID, PartitionNumber and ZoneID complete the
	// placement picture. Each is empty unless the instance uses that feature.
	PlacementGroup  string
	HostID          string
	PartitionNumber string
	ZoneID          string
	// ServicesDomain and ServicesPartition name the AWS partition the instance
	// is in, for example "amazonaws.com" and "aws".
	ServicesDomain    string
	ServicesPartition string
	// Interface describes the primary network interface.
	Interface Interface
}

// Interface is the primary network interface.
type Interface struct {
	InterfaceID     string
	OwnerID         string
	SubnetID        string
	SubnetCIDRBlock string
	VPCID           string
	VPCCIDRBlock    string
	LocalHostname   string
	PublicHostname  string
	LocalIPv4s      []string
	PublicIPv4s     []string
	IPv6s           []string
	SecurityGroups  []string
}

type document struct {
	InstanceID       string `json:"instanceId"`
	AccountID        string `json:"accountId"`
	InstanceType     string `json:"instanceType"`
	ImageID          string `json:"imageId"`
	Region           string `json:"region"`
	AvailabilityZone string `json:"availabilityZone"`
	Architecture     string `json:"architecture"`
	PrivateIP        string `json:"privateIp"`
	KernelID         string `json:"kernelId"`
	RamdiskID        string `json:"ramdiskId"`
	PendingTime      string `json:"pendingTime"`
	Version          string `json:"version"`
	// Null rather than absent when the image carries no such code, which
	// decodes to a nil slice either way.
	BillingProducts         []string `json:"billingProducts"`
	MarketplaceProductCodes []string `json:"marketplaceProductCodes"`
	DevpayProductCodes      []string `json:"devpayProductCodes"`
}

// Fetch reads the metadata of the EC2 instance the process is running on.
//
// The error is [imds.ErrNotDetected] when the process is not on an EC2
// instance, which includes AWS_EC2_METADATA_DISABLED being set. Any other
// error means the metadata service was reachable and the lookup failed, and
// must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	if disabled() {
		// Turning the service off is an operator's decision, not a failure, and
		// the answer is the same one absence gives: there is nothing to report.
		return Metadata{}, fmt.Errorf("%w: %s is set", imds.ErrAbsent, disabledEnv)
	}

	c := imds.NewClient(opts...)

	var (
		doc      document
		hostname string
		tree     Metadata
	)
	err := c.Lookup(ctx, func(ctx context.Context) error {
		endpoint := c.Endpoints(fromEnvironment()...)[0]

		// The token exchange is the availability probe: its outcome decides
		// whether the process is on EC2.
		token, err := c.Token(ctx, endpoint, imds.TokenConfig{
			Path:       tokenPath,
			Header:     http.Header{tokenTTLHeader: {strconv.Itoa(int(tokenTTL.Seconds()))}},
			HeaderName: tokenHeader,
		})
		if err != nil {
			return err
		}

		// Past the token exchange the service has identified itself, so a 4xx
		// is its failure rather than a sign of being on another cloud.
		read := []imds.RequestOption{token, imds.NeverForeign()}

		if _, err := c.GetJSONFirst(ctx, []string{endpoint}, documentPath, &doc, read...); err != nil {
			return err
		}
		hostname, err = c.GetText(ctx, endpoint+hostnamePath, read...)
		if err != nil {
			return fmt.Errorf("reading hostname: %w", err)
		}

		// The rest of /latest/meta-data/. Every one of these is conditional on
		// how the instance is configured -- there is no public hostname without
		// a public address, no autoscaling state outside a group, no spot
		// termination time on an on-demand instance -- so a 404 means the
		// instance has no such field rather than that the service is broken.
		base := endpoint + metaDataBase
		var securityGroups string
		for _, f := range []struct {
			path string
			dst  *string
		}{
			{"local-hostname", &tree.LocalHostname},
			{"public-hostname", &tree.PublicHostname},
			{"public-ipv4", &tree.PublicIP},
			{"mac", &tree.MAC},
			{"ami-launch-index", &tree.AMILaunchIndex},
			{"reservation-id", &tree.ReservationID},
			{"instance-life-cycle", &tree.InstanceLifecycle},
			{"spot/termination-time", &tree.SpotTerminationTime},
			{"autoscaling/target-lifecycle-state", &tree.AutoscalingLifecycleState},
			{"placement/group-name", &tree.PlacementGroup},
			{"placement/host-id", &tree.HostID},
			{"placement/partition-number", &tree.PartitionNumber},
			{"placement/availability-zone-id", &tree.ZoneID},
			{"services/domain", &tree.ServicesDomain},
			{"services/partition", &tree.ServicesPartition},
			{"security-groups", &securityGroups},
		} {
			v, err := c.GetTextOptional(ctx, base+f.path, token)
			if err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
			*f.dst = v
		}
		tree.SecurityGroups = lines(securityGroups)

		if tree.MAC == "" {
			return nil
		}
		// The interface tree is keyed by the primary MAC. The MAC is a value
		// the service chose and is being placed into a path, so it is escaped;
		// colons are legal in a path segment and survive it unchanged.
		iface := base + "network/interfaces/macs/" + url.PathEscape(tree.MAC) + "/"
		var localIPs, publicIPs, ipv6s, ifaceGroups string
		for _, f := range []struct {
			path string
			dst  *string
		}{
			{"interface-id", &tree.Interface.InterfaceID},
			{"owner-id", &tree.Interface.OwnerID},
			{"subnet-id", &tree.Interface.SubnetID},
			{"subnet-ipv4-cidr-block", &tree.Interface.SubnetCIDRBlock},
			{"vpc-id", &tree.Interface.VPCID},
			{"vpc-ipv4-cidr-block", &tree.Interface.VPCCIDRBlock},
			{"local-hostname", &tree.Interface.LocalHostname},
			{"public-hostname", &tree.Interface.PublicHostname},
			{"local-ipv4s", &localIPs},
			{"public-ipv4s", &publicIPs},
			{"ipv6s", &ipv6s},
			{"security-group-ids", &ifaceGroups},
		} {
			v, err := c.GetTextOptional(ctx, iface+f.path, token)
			if err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
			*f.dst = v
		}
		tree.Interface.LocalIPv4s = lines(localIPs)
		tree.Interface.PublicIPv4s = lines(publicIPs)
		tree.Interface.IPv6s = lines(ipv6s)
		tree.Interface.SecurityGroups = lines(ifaceGroups)
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}

	return Metadata{
		ID:                        doc.InstanceID,
		Hostname:                  hostname,
		AccountID:                 doc.AccountID,
		Type:                      doc.InstanceType,
		ImageID:                   doc.ImageID,
		Region:                    doc.Region,
		Zone:                      doc.AvailabilityZone,
		Architecture:              doc.Architecture,
		PrivateIP:                 doc.PrivateIP,
		KernelID:                  doc.KernelID,
		RamdiskID:                 doc.RamdiskID,
		PendingTime:               doc.PendingTime,
		DocumentVersion:           doc.Version,
		BillingProducts:           doc.BillingProducts,
		MarketplaceProductCodes:   doc.MarketplaceProductCodes,
		DevpayProductCodes:        doc.DevpayProductCodes,
		LocalHostname:             tree.LocalHostname,
		PublicHostname:            tree.PublicHostname,
		PublicIP:                  tree.PublicIP,
		MAC:                       tree.MAC,
		AMILaunchIndex:            tree.AMILaunchIndex,
		ReservationID:             tree.ReservationID,
		InstanceLifecycle:         tree.InstanceLifecycle,
		SpotTerminationTime:       tree.SpotTerminationTime,
		AutoscalingLifecycleState: tree.AutoscalingLifecycleState,
		SecurityGroups:            tree.SecurityGroups,
		PlacementGroup:            tree.PlacementGroup,
		HostID:                    tree.HostID,
		PartitionNumber:           tree.PartitionNumber,
		ZoneID:                    tree.ZoneID,
		ServicesDomain:            tree.ServicesDomain,
		ServicesPartition:         tree.ServicesPartition,
		Interface:                 tree.Interface,
	}, nil
}

// disabled reports whether the operator has turned metadata lookups off. AWS
// treats any casing of "true" as set.
func disabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(disabledEnv)), "true")
}

// fromEnvironment returns the configured address, an explicit endpoint winning
// over the mode, and the IPv4 default when neither is set.
func fromEnvironment() []string {
	if ep := strings.TrimSpace(os.Getenv(endpointEnv)); ep != "" {
		return []string{ep}
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv(endpointModeEnv)), "ipv6") {
		return []string{endpointIPv6}
	}
	return []string{endpointIPv4}
}

// lines splits a response that carries one value per line, which is how this
// service reports its list-valued paths.
func lines(s string) []string {
	var out []string
	for line := range strings.SplitSeq(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}
