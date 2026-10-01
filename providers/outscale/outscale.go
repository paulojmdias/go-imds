// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package outscale

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/paulojmdias/go-imds/imds"
)

var defaultEndpoints = []string{"http://169.254.169.254"}

const metadataBase = "/latest/meta-data/"

// clusterPath is both the availability probe and the identity test.
//
// OUTSCALE serves an EC2-compatible tree at the address EC2 itself uses, so
// instance-id, instance-type, ami-id and placement/availability-zone all answer
// on an EC2 instance with IMDSv1 enabled. Reading those first would claim that
// instance as OUTSCALE. placement/cluster is an OUTSCALE key: neither EC2 nor
// OpenStack's EC2-compatible tree serves it, so requiring it is what
// distinguishes the two. It is read under the default rule, so a 4xx here is
// ErrForeign -- something answered, but it is not this service.
//
// This is the same technique as the droplet_id check in providers/digitalocean
// and the Metadata-Flavor check in providers/gcp: require a field only this
// service sets, rather than trusting that an answer at a shared address
// identifies the platform.
const clusterPath = "placement/cluster"

// Metadata describes the OUTSCALE VM the process is running on.
type Metadata struct {
	// ID is the VM identifier.
	ID string
	// Hostname is the VM's primary private DNS name.
	Hostname string
	// AccountID owns the VM. The service reports it per network interface
	// rather than at the top level, so it is read through the primary MAC.
	AccountID string
	// Type is the VM type, for example "tinav5.c2r4p1".
	Type string
	// ImageID is the OUTSCALE machine image (OMI) the VM was created from.
	ImageID string
	// Zone is the Subregion, for example "eu-west-2a".
	Zone string
	// Region is derived from Zone, for example "eu-west-2". The metadata
	// service does not report it separately.
	Region string
	// PrivateIP is the VM's primary private IP.
	PrivateIP string

	// PublicHostname is the VM's public DNS name, empty without a public
	// address.
	PublicHostname string
	// MAC is the primary interface's hardware address.
	MAC string
	// Cluster and Server place the VM on OUTSCALE's own hardware. Cluster is
	// what identifies this service; see clusterPath.
	Cluster string
	Server  string
	// AMILaunchIndex distinguishes VMs launched together from one request.
	AMILaunchIndex string
	// ReservationID identifies the reservation the VM belongs to.
	ReservationID string
	// SecurityGroups are the groups on the primary interface, one per line as
	// reported.
	SecurityGroups []string
	// PublicKey is the public half of the VM's keypair.
	PublicKey string
	// RootDeviceName is the root volume's device name.
	RootDeviceName string
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
	LocalIPs        []string
	PublicIPs       []string
	GatewayIPv4     string
	SecurityGroups  []string
}

// Fetch reads the metadata of the OUTSCALE VM the process is running on.
//
// The error is [imds.ErrNotDetected] when the process is not on an OUTSCALE VM,
// which includes running on EC2, since that is the case worth getting right.
// Any other error means the metadata service was reachable and the lookup
// failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var md Metadata
	err := c.Lookup(ctx, func(ctx context.Context) error {
		base := c.Endpoints(defaultEndpoints...)[0] + metadataBase

		// The probe. Its value is not kept; what matters is that this service
		// serves the path at all. See clusterPath.
		if _, err := c.GetText(ctx, base+clusterPath); err != nil {
			return err
		}

		// Past the probe the service has identified itself, so a 4xx is that
		// service failing rather than a sign of being on another cloud.
		for _, f := range []struct {
			path string
			dst  *string
		}{
			{"instance-id", &md.ID},
			{"local-hostname", &md.Hostname},
			{"instance-type", &md.Type},
			{"ami-id", &md.ImageID},
			{"placement/availability-zone", &md.Zone},
			{"local-ipv4", &md.PrivateIP},
		} {
			v, err := c.GetText(ctx, base+f.path, imds.NeverForeign())
			if err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
			*f.dst = v
		}

		// Paths that exist only in some configurations: no public DNS name
		// without a public address, no keypair on a VM created without one.
		for _, f := range []struct {
			path string
			dst  *string
		}{
			{"public-hostname", &md.PublicHostname},
			{"mac", &md.MAC},
			{"placement/cluster", &md.Cluster},
			{"placement/server", &md.Server},
			{"ami-launch-index", &md.AMILaunchIndex},
			{"reservation-id", &md.ReservationID},
			{"public-keys/0/openssh-key", &md.PublicKey},
			{"block-device-mapping/root", &md.RootDeviceName},
		} {
			v, err := c.GetTextOptional(ctx, base+f.path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
			*f.dst = v
		}

		groups, err := c.GetTextOptional(ctx, base+"security-groups")
		if err != nil {
			return fmt.Errorf("reading security-groups: %w", err)
		}
		md.SecurityGroups = lines(groups)

		// The interface tree is keyed by the primary MAC. The account is
		// reported there rather than at the top level, which is why the MAC is
		// read before it.
		if md.MAC == "" {
			return fmt.Errorf("%s: mac is empty, so the account cannot be read", base)
		}
		// The MAC is a value the service chose, and it is being placed into a
		// path. Escaping keeps a surprising value from reaching somewhere else
		// in the tree; colons are legal in a path segment and survive it.
		iface := base + "network/interfaces/macs/" + url.PathEscape(md.MAC) + "/"
		md.Interface.OwnerID, err = c.GetText(ctx, iface+"owner-id", imds.NeverForeign())
		if err != nil {
			return fmt.Errorf("reading owner-id: %w", err)
		}
		md.AccountID = md.Interface.OwnerID

		var localIPs, publicIPs, ifaceGroups string
		for _, f := range []struct {
			path string
			dst  *string
		}{
			{"interface-id", &md.Interface.InterfaceID},
			{"subnet-id", &md.Interface.SubnetID},
			{"subnet-ipv4-cidr-block", &md.Interface.SubnetCIDRBlock},
			{"vpc-id", &md.Interface.VPCID},
			{"vpc-ipv4-cidr-block", &md.Interface.VPCCIDRBlock},
			{"local-hostname", &md.Interface.LocalHostname},
			{"public-hostname", &md.Interface.PublicHostname},
			{"gateway-ipv4", &md.Interface.GatewayIPv4},
			{"local-ip4s", &localIPs},
			{"public-ipv4s", &publicIPs},
			{"security-groups", &ifaceGroups},
		} {
			v, err := c.GetTextOptional(ctx, iface+f.path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
			*f.dst = v
		}
		md.Interface.LocalIPs = lines(localIPs)
		md.Interface.PublicIPs = lines(publicIPs)
		md.Interface.SecurityGroups = lines(ifaceGroups)
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}

	md.Region = regionFromZone(md.Zone)
	return md, nil
}

// regionFromZone returns the Region a Subregion belongs to, "eu-west-2" for
// "eu-west-2a". Subregions are the Region with one letter appended, so the
// letter is what comes off -- unlike Scaleway, where the zone carries a numeric
// suffix after a separator.
func regionFromZone(zone string) string {
	if zone == "" {
		return ""
	}
	if last := zone[len(zone)-1]; last >= 'a' && last <= 'z' {
		return zone[:len(zone)-1]
	}
	return zone
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
