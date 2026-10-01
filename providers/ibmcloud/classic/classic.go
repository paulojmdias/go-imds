// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package classic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/paulojmdias/go-imds/imds"
)

var defaultEndpoints = []string{"https://api.service.softlayer.com/rest/v3.1/SoftLayer_Resource_Metadata"}

// Metadata describes the Classic infrastructure instance the process is
// running on.
type Metadata struct {
	// ID is the instance identifier.
	ID string
	// Hostname is the instance hostname.
	Hostname string
	// AccountID is the account owning the instance.
	AccountID string
	// Datacenter is the data centre the instance runs in, for example "par01".
	Datacenter string
	// GlobalIdentifier is the instance's UUID.
	GlobalIdentifier string

	// Domain and FullyQualifiedDomainName complete the instance's naming.
	Domain                   string
	FullyQualifiedDomainName string
	// PrimaryIP is the public address and PrimaryBackendIP the private one.
	PrimaryIP        string
	PrimaryBackendIP string
	// DatacenterID is the data centre's numeric identifier.
	DatacenterID string
	// Router is the backend router the instance is attached to.
	Router string
	// ProvisionState is the instance's provisioning state.
	ProvisionState string
	// Tags are the tags applied to the instance.
	Tags []string
	// PrimaryMACAddresses and BackendMACAddresses are the instance's hardware
	// addresses, front and back.
	PrimaryMACAddresses []string
	BackendMACAddresses []string
	// FrontendVLANIDs and BackendVLANIDs are the VLANs the instance is on.
	FrontendVLANIDs []int64
	BackendVLANIDs  []int64
}

// Fetch reads the metadata of the Classic infrastructure instance the process
// is running on.
//
// The error is [imds.ErrNotDetected] when the process is not on Classic
// infrastructure. Any other error means the API was reachable and the lookup
// failed, and must not be read as absence.
func Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error) {
	c := imds.NewClient(opts...)

	var md Metadata
	err := c.Lookup(ctx, func(ctx context.Context) error {
		endpoint := c.Endpoints(defaultEndpoints...)[0]

		fields := []struct {
			path string
			dst  *string
		}{
			{"/getId.txt", &md.ID},
			{"/getHostname.txt", &md.Hostname},
			{"/getAccountId.txt", &md.AccountID},
			{"/getDatacenter.txt", &md.Datacenter},
			{"/getGlobalIdentifier.txt", &md.GlobalIdentifier},
		}

		for i, f := range fields {
			// The first read is the availability probe: a 4xx there means this
			// is not the SoftLayer API. Once it has answered, a later 4xx is the
			// API failing, and reporting that as "not on Classic" would discard
			// an instance already identified.
			var read []imds.RequestOption
			if i > 0 {
				read = []imds.RequestOption{imds.NeverForeign()}
			}
			v, err := c.GetText(ctx, endpoint+f.path, read...)
			if err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
			*f.dst = v
		}

		// The rest of the descriptive surface. getUserMetadata.txt is served
		// too and is deliberately not read; see the package documentation.
		for _, f := range []struct {
			path string
			dst  *string
		}{
			{"/getDomain.txt", &md.Domain},
			{"/getFullyQualifiedDomainName.txt", &md.FullyQualifiedDomainName},
			{"/getPrimaryIpAddress.txt", &md.PrimaryIP},
			{"/getPrimaryBackendIpAddress.txt", &md.PrimaryBackendIP},
			{"/getDatacenterId.txt", &md.DatacenterID},
			{"/getRouter.txt", &md.Router},
			{"/getProvisionState.txt", &md.ProvisionState},
		} {
			v, err := c.GetTextOptional(ctx, endpoint+f.path)
			if err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
			*f.dst = v
		}

		// These are served as JSON arrays rather than text, which is why they
		// are read separately.
		for _, f := range []struct {
			path string
			dst  *[]string
		}{
			{"/getTags.json", &md.Tags},
			{"/getPrimaryMacAddresses.json", &md.PrimaryMACAddresses},
			{"/getBackendMacAddresses.json", &md.BackendMACAddresses},
		} {
			if err := readJSON(ctx, c, endpoint+f.path, f.dst); err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
		}
		for _, f := range []struct {
			path string
			dst  *[]int64
		}{
			{"/getFrontendVlanIds.json", &md.FrontendVLANIDs},
			{"/getBackendVlanIds.json", &md.BackendVLANIDs},
		} {
			if err := readJSON(ctx, c, endpoint+f.path, f.dst); err != nil {
				return fmt.Errorf("reading %s: %w", f.path, err)
			}
		}
		return nil
	})
	if err != nil {
		return Metadata{}, err
	}
	return md, nil
}

// readJSON reads a path that serves a JSON array, tolerating its absence in the
// way [imds.Client.GetTextOptional] does for text.
func readJSON[T any](ctx context.Context, c *imds.Client, url string, dst *[]T) error {
	body, err := c.Get(ctx, url, imds.NeverForeign())
	switch {
	case errors.Is(err, imds.ErrNotFound):
		return nil
	case err != nil:
		return err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	return json.Unmarshal(body, dst)
}
