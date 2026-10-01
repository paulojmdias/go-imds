// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package akamai

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/paulojmdias/go-imds/imds"
)

var defaultEndpoints = []string{"http://169.254.169.254"}

const (
	tokenPath    = "/v1/token"
	instancePath = "/v1/instance"

	tokenHeader       = "Metadata-Token"
	tokenExpiryHeader = "Metadata-Token-Expiry-Seconds"

	// tokenTTL is the lifetime requested for the session token. A lookup uses
	// its token for one read and discards it.
	tokenTTL = 5 * time.Minute
)

// Metadata describes the Akamai Cloud Compute instance the process is running
// on.
type Metadata struct {
	// ID is the instance identifier, in decimal. The metadata service reports
	// it as a number; it is rendered as a string here so that every provider in
	// this repository names its host identifier the same way.
	ID string
	// Hostname is the instance label.
	Hostname string
	// AccountID is the account owning the instance.
	AccountID string
	// Type is the instance type, for example "g6-standard-2".
	Type string
	// ImageID and ImageName describe the image the instance was created from.
	ImageID   string
	ImageName string
	// Region is the region, for example "us-east".
	Region string
	// HostUUID identifies the physical host the instance runs on.
	HostUUID string
	// Tags are the instance's tags.
	Tags []string
	// Specs are the instance's allocated resources.
	Specs Specs
	// Backups describes the instance's backup service.
	Backups Backups
}

// Specs are an instance's allocated resources. Memory and Disk are in
// mebibytes, Transfer in gibibytes, as the service reports them.
type Specs struct {
	VCPUs    int `json:"vcpus"`
	Memory   int `json:"memory"`
	GPUs     int `json:"gpus"`
	Transfer int `json:"transfer"`
	Disk     int `json:"disk"`
}

// Backups describes the instance's backup service.
type Backups struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"`
}

type document struct {
	ID       int      `json:"id"`
	Label    string   `json:"label"`
	Region   string   `json:"region"`
	Type     string   `json:"type"`
	HostUUID string   `json:"host_uuid"`
	Tags     []string `json:"tags"`
	Image    struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	} `json:"image"`
	AccountEUUID string  `json:"account_euuid"`
	Specs        Specs   `json:"specs"`
	Backups      backups `json:"backups"`
}

// Status is null rather than absent when no backup has run, so it is decoded
// through a pointer and flattened to the empty string.
type backups struct {
	Enabled bool    `json:"enabled"`
	Status  *string `json:"status"`
}

// Fetch reads the metadata of the Akamai Cloud Compute instance the process is
// running on.
//
// The error is [imds.ErrNotDetected] when the process is not on such an
// instance. Any other error means the metadata service was reachable and the
// lookup failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var doc document
	err := c.Lookup(ctx, func(ctx context.Context) error {
		endpoint := c.Endpoints(defaultEndpoints...)[0]

		// The token exchange is the availability probe: its outcome decides
		// whether the process is on Akamai Cloud Compute.
		token, err := c.Token(ctx, endpoint, imds.TokenConfig{
			Path:       tokenPath,
			Header:     http.Header{tokenExpiryHeader: {strconv.Itoa(int(tokenTTL.Seconds()))}},
			HeaderName: tokenHeader,
		})
		if err != nil {
			return err
		}

		// Past the token exchange the service has identified itself, so a 4xx
		// is its failure rather than a sign of being on another cloud.
		_, err = c.GetJSONFirst(ctx, []string{endpoint}, instancePath, &doc,
			token,
			imds.WithHeader("Accept", "application/json"),
			imds.NeverForeign())
		return err
	})
	if err != nil {
		return Metadata{}, err
	}

	var status string
	if doc.Backups.Status != nil {
		status = *doc.Backups.Status
	}

	return Metadata{
		ID:        strconv.Itoa(doc.ID),
		Hostname:  doc.Label,
		AccountID: doc.AccountEUUID,
		Type:      doc.Type,
		ImageID:   doc.Image.ID,
		ImageName: doc.Image.Label,
		Region:    doc.Region,
		HostUUID:  doc.HostUUID,
		Tags:      doc.Tags,
		Specs:     doc.Specs,
		Backups:   Backups{Enabled: doc.Backups.Enabled, Status: status},
	}, nil
}
