// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package outscale_test

import (
	"context"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/outscale"
)

// These values follow the category table in OUTSCALE's own documentation,
// "Accessing the Metadata and User Data of a VM"; instance-id is the example the documentation gives
// verbatim, and the rest are plausible values in the documented shape. This
// fixture has not been captured from a running VM.
var values = map[string]string{
	"placement/cluster":           "cluster-1",
	"placement/availability-zone": "eu-west-2a",
	"instance-id":                 "i-be1dcf1a",
	"local-hostname":              "ip-10-0-0-4.eu-west-2.compute.internal",
	"instance-type":               "tinav5.c2r4p1",
	"ami-id":                      "ami-0b7df01c",
	"local-ipv4":                  "10.0.0.4",
	"mac":                         "02:1a:2b:3c:4d:5e",
	"public-hostname":             "ows-192-0-2-20.eu-west-2.compute.outscale.com",
	"placement/server":            "server-17",
	"ami-launch-index":            "0",
	"reservation-id":              "r-12345678",
	"public-keys/0/openssh-key":   "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@outscale",
	"block-device-mapping/root":   "/dev/sda1",
	"security-groups":             "default\nweb\n",

	"network/interfaces/macs/02:1a:2b:3c:4d:5e/owner-id":               "123456789012",
	"network/interfaces/macs/02:1a:2b:3c:4d:5e/interface-id":           "eni-12345678",
	"network/interfaces/macs/02:1a:2b:3c:4d:5e/subnet-id":              "subnet-12345678",
	"network/interfaces/macs/02:1a:2b:3c:4d:5e/subnet-ipv4-cidr-block": "10.0.0.0/24",
	"network/interfaces/macs/02:1a:2b:3c:4d:5e/vpc-id":                 "vpc-12345678",
	"network/interfaces/macs/02:1a:2b:3c:4d:5e/vpc-ipv4-cidr-block":    "10.0.0.0/16",
	"network/interfaces/macs/02:1a:2b:3c:4d:5e/local-hostname":         "ip-10-0-0-4.eu-west-2.compute.internal",
	"network/interfaces/macs/02:1a:2b:3c:4d:5e/gateway-ipv4":           "10.0.0.1",
	"network/interfaces/macs/02:1a:2b:3c:4d:5e/local-ip4s":             "10.0.0.4\n",
	"network/interfaces/macs/02:1a:2b:3c:4d:5e/public-ipv4s":           "192.0.2.20\n",
	"network/interfaces/macs/02:1a:2b:3c:4d:5e/security-groups":        "default\nweb\n",
}

func handlerFor(v map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value, ok := v[strings.TrimPrefix(r.URL.Path, "/latest/meta-data/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(value))
	})
}

func handler() http.Handler { return handlerFor(values) }

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "outscale",
		Handler: handler(),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := outscale.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler())

	md, err := outscale.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, outscale.Metadata{
		ID:        "i-be1dcf1a",
		Hostname:  "ip-10-0-0-4.eu-west-2.compute.internal",
		AccountID: "123456789012",
		Type:      "tinav5.c2r4p1",
		ImageID:   "ami-0b7df01c",
		Zone:      "eu-west-2a",
		Region:    "eu-west-2",
		PrivateIP: "10.0.0.4",

		PublicHostname: "ows-192-0-2-20.eu-west-2.compute.outscale.com",
		MAC:            "02:1a:2b:3c:4d:5e",
		Cluster:        "cluster-1",
		Server:         "server-17",
		AMILaunchIndex: "0",
		ReservationID:  "r-12345678",
		SecurityGroups: []string{"default", "web"},
		PublicKey:      "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@outscale",
		RootDeviceName: "/dev/sda1",
		Interface: outscale.Interface{
			InterfaceID:     "eni-12345678",
			OwnerID:         "123456789012",
			SubnetID:        "subnet-12345678",
			SubnetCIDRBlock: "10.0.0.0/24",
			VPCID:           "vpc-12345678",
			VPCCIDRBlock:    "10.0.0.0/16",
			LocalHostname:   "ip-10-0-0-4.eu-west-2.compute.internal",
			GatewayIPv4:     "10.0.0.1",
			LocalIPs:        []string{"10.0.0.4"},
			PublicIPs:       []string{"192.0.2.20"},
			SecurityGroups:  []string{"default", "web"},
		},
	}, md)
}

// The case this provider exists to get right. OUTSCALE's tree is
// EC2-compatible and lives at the address EC2 uses, so every path below answers
// on an EC2 instance with IMDSv1 enabled. Only placement/cluster does not, and
// requiring it is what stops an EC2 instance being reported as OUTSCALE.
//
// If this test passes with the probe changed to instance-id, the provider is
// wrong.
func TestEC2InstanceIsForeign(t *testing.T) {
	ec2 := maps.Clone(values)
	delete(ec2, "placement/cluster")
	ep := imdstest.Server(t, handlerFor(ec2))

	_, err := outscale.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrForeign)
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

func TestNothingListeningIsAbsent(t *testing.T) {
	_, err := outscale.Fetch(t.Context(), imds.WithEndpoints(imdstest.Closed(t)))
	require.ErrorIs(t, err, imds.ErrAbsent)
}

// Once the service has answered the probe, a later 4xx is that service failing.
// Reporting it as absence would discard a VM already identified.
func TestLaterReadRejectedIsAFailureNotAbsence(t *testing.T) {
	probeOnly := map[string]string{"placement/cluster": "cluster-1"}
	ep := imdstest.Server(t, handlerFor(probeOnly))

	_, err := outscale.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.Error(t, err)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}

// The account is served per network interface, so it takes the MAC to address
// it. A service that answers the probe but serves no MAC is failing, not
// absent.
func TestMissingMACIsAFailureNotAbsence(t *testing.T) {
	noMAC := maps.Clone(values)
	noMAC["mac"] = ""
	ep := imdstest.Server(t, handlerFor(noMAC))

	_, err := outscale.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorContains(t, err, "mac is empty")
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}

func TestRegionFromZone(t *testing.T) {
	for _, tc := range []struct{ zone, want string }{
		{"eu-west-2a", "eu-west-2"},
		{"cloudgouv-eu-west-1a", "cloudgouv-eu-west-1"},
		{"us-east-2", "us-east-2"}, // already a region: nothing to strip
		{"", ""},
	} {
		ep := imdstest.Server(t, handlerFor(withZone(tc.zone)))
		md, err := outscale.Fetch(t.Context(), imds.WithEndpoints(ep))
		require.NoError(t, err)
		assert.Equal(t, tc.want, md.Region, "zone %q", tc.zone)
	}
}

func withZone(zone string) map[string]string {
	v := maps.Clone(values)
	v["placement/availability-zone"] = zone
	return v
}
