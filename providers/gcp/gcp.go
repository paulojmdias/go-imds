// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/paulojmdias/go-imds/imds"
)

// hostEnv names the host to use instead of the defaults. Setting it is how a
// process running outside Compute Engine is pointed at a metadata service, and
// how tests redirect this provider.
const hostEnv = "GCE_METADATA_HOST"

// The name is tried first: on a host with no route to the link-local address
// but working internal DNS, it is the one that answers.
var defaultEndpoints = []string{"http://metadata.google.internal", "http://169.254.169.254"}

const (
	metadataBase = "/computeMetadata/v1/"

	// The whole instance and project trees, one request each. Reading a path
	// per field cost six, and the service will hand over the lot.
	//
	// Neither dump carries a credential: the service account access token and
	// identity document are served only from their own endpoints, which this
	// package never requests. The attributes maps are omitted for a different
	// reason -- they hold whatever the project or instance owner put there,
	// startup scripts included, which is user data rather than metadata.
	instancePath = "instance/?recursive=true"
	projectPath  = "project/?recursive=true"

	flavorHeader = "Metadata-Flavor"
	flavorValue  = "Google"
)

// Metadata describes the Compute Engine instance the process is running on.
type Metadata struct {
	// ID is the instance identifier.
	ID string
	// Hostname is the instance's fully qualified hostname.
	Hostname string
	// Name is the instance name.
	Name string
	// AccountID is the project the instance belongs to.
	AccountID string
	// Type is the machine type, for example "n2-standard-4". The metadata
	// service reports it as a full resource path; only the final segment is
	// kept.
	Type string
	// Zone is the zone, for example "us-central1-a". The metadata service
	// reports it as a full resource path; only the final segment is kept.
	Zone string
	// Region is derived from Zone, for example "us-central1". The metadata
	// service does not report it separately.
	Region string

	// Description is the instance's free-text description.
	Description string
	// CPUPlatform is the host processor, for example "Intel Broadwell".
	CPUPlatform string
	// Image is the resource path of the image the instance was created from.
	Image string
	// Tags are the network tags applied to the instance.
	Tags []string
	// NumericProjectID is the project's number, where AccountID is its name.
	NumericProjectID uint64
	// MaintenanceEvent is the host maintenance state, normally "NONE".
	MaintenanceEvent string
	// Preempted reports whether a preemptible instance has been preempted. The
	// service reports it as "TRUE" or "FALSE".
	Preempted string
	// Scheduling describes the instance's restart and preemption policy.
	Scheduling Scheduling
	// Disks are the attached disks.
	Disks []Disk
	// NetworkInterfaces are the instance's interfaces.
	NetworkInterfaces []NetworkInterface
	// Licenses are the licence identifiers attached to the boot image.
	Licenses []string
	// ServiceAccounts are the accounts the instance can obtain tokens for,
	// keyed by account name. The tokens themselves are served from a separate
	// endpoint and are not read.
	ServiceAccounts map[string]ServiceAccount
}

// Scheduling describes an instance's restart and preemption policy. The
// service reports the booleans as "TRUE" and "FALSE".
type Scheduling struct {
	AutomaticRestart  string `json:"automaticRestart"`
	OnHostMaintenance string `json:"onHostMaintenance"`
	Preemptible       string `json:"preemptible"`
}

// Disk is one attached disk.
type Disk struct {
	DeviceName string `json:"deviceName"`
	Index      int    `json:"index"`
	Mode       string `json:"mode"`
	Type       string `json:"type"`
}

// NetworkInterface is one network interface.
type NetworkInterface struct {
	IP            string         `json:"ip"`
	MAC           string         `json:"mac"`
	Gateway       string         `json:"gateway"`
	Subnetmask    string         `json:"subnetmask"`
	MTU           int            `json:"mtu"`
	Network       string         `json:"network"`
	DNSServers    []string       `json:"dnsServers"`
	IPAliases     []string       `json:"ipAliases"`
	AccessConfigs []AccessConfig `json:"accessConfigs"`
}

// AccessConfig is an interface's external address mapping.
type AccessConfig struct {
	ExternalIP string `json:"externalIp"`
	Type       string `json:"type"`
}

// ServiceAccount is one account the instance can obtain tokens for. The token
// itself is served from a separate endpoint and is deliberately not read.
type ServiceAccount struct {
	Email   string   `json:"email"`
	Aliases []string `json:"aliases"`
	Scopes  []string `json:"scopes"`
}

type instanceDocument struct {
	ID                uint64                    `json:"id"`
	Hostname          string                    `json:"hostname"`
	Name              string                    `json:"name"`
	MachineType       string                    `json:"machineType"`
	Zone              string                    `json:"zone"`
	Description       string                    `json:"description"`
	CPUPlatform       string                    `json:"cpuPlatform"`
	Image             string                    `json:"image"`
	Tags              []string                  `json:"tags"`
	MaintenanceEvent  string                    `json:"maintenanceEvent"`
	Preempted         string                    `json:"preempted"`
	Scheduling        Scheduling                `json:"scheduling"`
	Disks             []Disk                    `json:"disks"`
	NetworkInterfaces []NetworkInterface        `json:"networkInterfaces"`
	Licenses          []license                 `json:"licenses"`
	ServiceAccounts   map[string]ServiceAccount `json:"serviceAccounts"`
}

type license struct {
	ID string `json:"id"`
}

type projectDocument struct {
	ProjectID        string `json:"projectId"`
	NumericProjectID uint64 `json:"numericProjectId"`
}

// Fetch reads the metadata of the Compute Engine instance the process is
// running on.
//
// The error is [imds.ErrNotDetected] when the process is not on a Compute
// Engine instance. Any other error means the metadata service was reachable
// and the lookup failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var (
		inst instanceDocument
		proj projectDocument
	)
	err := c.Lookup(ctx, func(ctx context.Context) error {
		endpoints := c.Endpoints(fromEnvironment()...)

		// The instance tree doubles as the availability probe: it is read
		// across the endpoint list, and the flavour check on its response is
		// what says this is really Google's metadata service.
		endpoint, err := c.GetJSONFirst(ctx, endpoints, metadataBase+instancePath, &inst,
			imds.WithHeader(flavorHeader, flavorValue),
			imds.WithResponseCheck(requireFlavor))
		if err != nil {
			return err
		}

		// Past the probe the service has identified itself, so a 4xx is that
		// service failing rather than a sign of being elsewhere.
		if _, err := c.GetJSONFirst(ctx, []string{endpoint}, metadataBase+projectPath, &proj,
			imds.WithHeader(flavorHeader, flavorValue),
			imds.WithResponseCheck(requireFlavor),
			imds.NeverForeign()); err != nil {
			return fmt.Errorf("reading %s: %w", projectPath, err)
		}
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}

	// machineType and zone are reported as resource paths,
	// "projects/123/zones/us-central1-a".
	zone := path.Base(inst.Zone)
	md := Metadata{
		ID:        strconv.FormatUint(inst.ID, 10),
		Hostname:  inst.Hostname,
		Name:      inst.Name,
		AccountID: proj.ProjectID,
		Type:      path.Base(inst.MachineType),
		Zone:      zone,
		Region:    regionFromZone(zone),

		Description:       inst.Description,
		CPUPlatform:       inst.CPUPlatform,
		Image:             inst.Image,
		Tags:              inst.Tags,
		NumericProjectID:  proj.NumericProjectID,
		MaintenanceEvent:  inst.MaintenanceEvent,
		Preempted:         inst.Preempted,
		Scheduling:        inst.Scheduling,
		Disks:             inst.Disks,
		NetworkInterfaces: inst.NetworkInterfaces,
		ServiceAccounts:   inst.ServiceAccounts,
	}
	for _, l := range inst.Licenses {
		md.Licenses = append(md.Licenses, l.ID)
	}
	return md, nil
}

// requireFlavor rejects a response that does not echo the flavour header.
// Google's metadata service always sends it; software that merely happens to
// be listening on the same address does not.
func requireFlavor(resp *http.Response) error {
	if resp.Header.Get(flavorHeader) != flavorValue {
		return fmt.Errorf("%w: response carries no %s: %s header", imds.ErrForeign, flavorHeader, flavorValue)
	}
	return nil
}

// fromEnvironment returns the configured host, or the defaults. The variable
// holds a host and optional port with no scheme, matching what Google's own
// tooling reads.
func fromEnvironment() []string {
	host := os.Getenv(hostEnv)
	if host == "" {
		return defaultEndpoints
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	return []string{host}
}

// regionFromZone returns the region a zone belongs to, "us-central1" for
// "us-central1-a".
func regionFromZone(zone string) string {
	if i := strings.LastIndex(zone, "-"); i > 0 {
		return zone[:i]
	}
	return ""
}
