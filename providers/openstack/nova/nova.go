// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package nova

import (
	"context"

	"github.com/paulojmdias/go-imds/imds"
)

var defaultEndpoints = []string{"http://169.254.169.254"}

const (
	documentPath     = "/openstack/latest/meta_data.json"
	instanceTypePath = "/latest/meta-data/instance-type"
)

// Metadata describes the Nova instance the process is running on.
type Metadata struct {
	// ID is the instance UUID.
	ID string
	// Hostname is the instance hostname.
	Hostname string
	// Name is the instance name, which a deployment may set independently of
	// the hostname.
	Name string
	// AccountID is the project the instance belongs to.
	AccountID string
	// Zone is the availability zone.
	Zone string
	// Type is the instance flavour, read from the EC2-compatible tree. It is
	// empty on deployments that do not serve that tree, which is not an error.
	Type string
	// Meta carries the deployment-defined key/value pairs attached to the
	// instance.
	Meta map[string]string
	// LaunchIndex distinguishes instances launched together from one request.
	LaunchIndex int
	// PublicKeys are the SSH public keys installed on the instance, keyed by
	// the name the deployment gave each one.
	PublicKeys map[string]string
	// Keys is the newer form of the same thing, carrying the key type.
	Keys []Key
	// Devices are the devices attached to the instance.
	Devices []Device
	// DedicatedCPUs are the host CPUs the instance is pinned to, empty when it
	// is not pinned.
	DedicatedCPUs []int
}

// Key is one SSH public key installed on the instance.
type Key struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Data string `json:"data"`
}

// Device is one device attached to the instance.
type Device struct {
	Type    string   `json:"type"`
	Bus     string   `json:"bus"`
	Address string   `json:"address"`
	Serial  string   `json:"serial"`
	Path    string   `json:"path"`
	Tags    []string `json:"tags"`
}

type document struct {
	AvailabilityZone string            `json:"availability_zone"`
	Hostname         string            `json:"hostname"`
	Name             string            `json:"name"`
	Meta             map[string]string `json:"meta"`
	ProjectID        string            `json:"project_id"`
	UUID             string            `json:"uuid"`
	LaunchIndex      int               `json:"launch_index"`
	PublicKeys       map[string]string `json:"public_keys"`
	Keys             []Key             `json:"keys"`
	Devices          []Device          `json:"devices"`
	DedicatedCPUs    []int             `json:"dedicated_cpus"`
}

// Fetch reads the metadata of the Nova instance the process is running on.
//
// The error is [imds.ErrNotDetected] when the process is not on a Nova
// instance. Any other error means the metadata service was reachable and the
// lookup failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var (
		doc          document
		instanceType string
	)
	err := c.Lookup(ctx, func(ctx context.Context) error {
		endpoint, err := c.GetJSONFirst(ctx, c.Endpoints(defaultEndpoints...), documentPath, &doc)
		if err != nil {
			return err
		}

		// Best effort, and deliberately not part of the result's success. Many
		// deployments do not serve the EC2-compatible tree at all, so a failure
		// here says nothing about the instance and must not fail a lookup that
		// has already read the metadata document.
		if v, err := c.GetText(ctx, endpoint+instanceTypePath); err == nil {
			instanceType = v
		}
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}

	return Metadata{
		ID:        doc.UUID,
		Hostname:  doc.Hostname,
		Name:      doc.Name,
		AccountID: doc.ProjectID,
		Zone:      doc.AvailabilityZone,
		Type:      instanceType,
		Meta:      doc.Meta,

		LaunchIndex:   doc.LaunchIndex,
		PublicKeys:    doc.PublicKeys,
		Keys:          doc.Keys,
		Devices:       doc.Devices,
		DedicatedCPUs: doc.DedicatedCPUs,
	}, nil
}
