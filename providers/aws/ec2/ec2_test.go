// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package ec2_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/aws/ec2"
)

// The paths under /latest/meta-data/ that this instance has. It is on-demand
// and not in an auto scaling group, so spot/termination-time and
// autoscaling/target-lifecycle-state are absent on purpose.
// iam/security-credentials is served on an instance with a role and is
// deliberately absent: this provider never requests it.
var metaData = map[string]string{
	"hostname":                       "ip-10-0-1-25.eu-west-1.compute.internal",
	"local-hostname":                 "ip-10-0-1-25.eu-west-1.compute.internal",
	"public-hostname":                "ec2-192-0-2-25.eu-west-1.compute.amazonaws.com",
	"public-ipv4":                    "192.0.2.25",
	"mac":                            "0a:1b:2c:3d:4e:5f",
	"ami-launch-index":               "0",
	"reservation-id":                 "r-0abcdef1234567890",
	"instance-life-cycle":            "on-demand",
	"placement/availability-zone-id": "euw1-az2",
	"services/domain":                "amazonaws.com",
	"services/partition":             "aws",
	"security-groups":                "default\ntelemetry\n",

	"network/interfaces/macs/0a:1b:2c:3d:4e:5f/interface-id":           "eni-0abcdef1234567890",
	"network/interfaces/macs/0a:1b:2c:3d:4e:5f/owner-id":               "111122223333",
	"network/interfaces/macs/0a:1b:2c:3d:4e:5f/subnet-id":              "subnet-0abcdef123",
	"network/interfaces/macs/0a:1b:2c:3d:4e:5f/subnet-ipv4-cidr-block": "10.0.1.0/24",
	"network/interfaces/macs/0a:1b:2c:3d:4e:5f/vpc-id":                 "vpc-0abcdef123",
	"network/interfaces/macs/0a:1b:2c:3d:4e:5f/vpc-ipv4-cidr-block":    "10.0.0.0/16",
	"network/interfaces/macs/0a:1b:2c:3d:4e:5f/local-hostname":         "ip-10-0-1-25.eu-west-1.compute.internal",
	"network/interfaces/macs/0a:1b:2c:3d:4e:5f/local-ipv4s":            "10.0.1.25\n",
	"network/interfaces/macs/0a:1b:2c:3d:4e:5f/public-ipv4s":           "192.0.2.25\n",
	"network/interfaces/macs/0a:1b:2c:3d:4e:5f/security-group-ids":     "sg-0abcdef123\n",
}

// The field set is the EC2 instance identity document in full. Values are
// illustrative rather than captured.
const documentJSON = `{
  "accountId": "111122223333",
  "architecture": "x86_64",
  "availabilityZone": "eu-west-1a",
  "imageId": "ami-0abcdef1234567890",
  "instanceId": "i-0abcdef1234567890",
  "instanceType": "m5.large",
  "privateIp": "10.0.1.25",
  "region": "eu-west-1",
  "kernelId": null,
  "ramdiskId": null,
  "pendingTime": "2024-01-02T03:04:05Z",
  "version": "2017-09-30",
  "billingProducts": ["bp-6ba54002"],
  "marketplaceProductCodes": null,
  "devpayProductCodes": null
}`

// IMDSv2: the service issues a token only for a PUT carrying the TTL header,
// and refuses every read without it.
func handler() http.Handler {
	const token = "imdsv2-token"

	mux := http.NewServeMux()
	mux.HandleFunc("/latest/api/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.Header.Get("X-aws-ec2-metadata-token-ttl-seconds") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(token))
	})
	guard := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-aws-ec2-metadata-token") != token {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next(w, r)
		}
	}
	mux.HandleFunc("/latest/dynamic/instance-identity/document", guard(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(documentJSON))
	}))
	mux.HandleFunc("/latest/meta-data/", guard(func(w http.ResponseWriter, r *http.Request) {
		v, ok := metaData[strings.TrimPrefix(r.URL.Path, "/latest/meta-data/")]
		if !ok {
			// Everything under here is conditional on how the instance is
			// configured, and the real service 404s the paths that do not
			// apply -- no spot termination time on an on-demand instance, no
			// autoscaling state outside a group.
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(v))
	}))
	return mux
}

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "aws/ec2",
		Handler: handler(),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := ec2.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler())

	md, err := ec2.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, ec2.Metadata{
		ID:              "i-0abcdef1234567890",
		Hostname:        "ip-10-0-1-25.eu-west-1.compute.internal",
		AccountID:       "111122223333",
		Type:            "m5.large",
		ImageID:         "ami-0abcdef1234567890",
		Region:          "eu-west-1",
		Zone:            "eu-west-1a",
		Architecture:    "x86_64",
		PrivateIP:       "10.0.1.25",
		PendingTime:     "2024-01-02T03:04:05Z",
		DocumentVersion: "2017-09-30",
		BillingProducts: []string{"bp-6ba54002"},

		LocalHostname:     "ip-10-0-1-25.eu-west-1.compute.internal",
		PublicHostname:    "ec2-192-0-2-25.eu-west-1.compute.amazonaws.com",
		PublicIP:          "192.0.2.25",
		MAC:               "0a:1b:2c:3d:4e:5f",
		AMILaunchIndex:    "0",
		ReservationID:     "r-0abcdef1234567890",
		InstanceLifecycle: "on-demand",
		SecurityGroups:    []string{"default", "telemetry"},
		ZoneID:            "euw1-az2",
		ServicesDomain:    "amazonaws.com",
		ServicesPartition: "aws",
		Interface: ec2.Interface{
			InterfaceID:     "eni-0abcdef1234567890",
			OwnerID:         "111122223333",
			SubnetID:        "subnet-0abcdef123",
			SubnetCIDRBlock: "10.0.1.0/24",
			VPCID:           "vpc-0abcdef123",
			VPCCIDRBlock:    "10.0.0.0/16",
			LocalHostname:   "ip-10-0-1-25.eu-west-1.compute.internal",
			LocalIPv4s:      []string{"10.0.1.25"},
			PublicIPv4s:     []string{"192.0.2.25"},
			SecurityGroups:  []string{"sg-0abcdef123"},
		},
	}, md)
}

// Turning the service off is an operator's decision, not a failure, and no
// request should be made to discover it.
func TestDisabledEnvironmentVariableIsAbsence(t *testing.T) {
	ep := imdstest.Server(t, handler())
	t.Setenv("AWS_EC2_METADATA_DISABLED", "TRUE")

	_, err := ec2.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrAbsent)
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

func TestEndpointEnvironmentVariableReplacesTheDefault(t *testing.T) {
	ep := imdstest.Server(t, handler())
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT", ep)

	md, err := ec2.Fetch(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "i-0abcdef1234567890", md.ID)
}

// An explicit endpoint wins over the mode, which is the precedence the AWS
// tooling defines.
func TestExplicitEndpointWinsOverMode(t *testing.T) {
	ep := imdstest.Server(t, handler())
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT", ep)
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT_MODE", "IPv6")

	md, err := ec2.Fetch(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "i-0abcdef1234567890", md.ID)
}

func TestTokenRejectedIsNotDetected(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusNotFound))

	_, err := ec2.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

func TestTokenNeverAppearsInAnError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/latest/api/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("super-secret-token"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	ep := imdstest.Server(t, mux)

	_, err := ec2.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "super-secret-token")
}

// An EC2-compatible service that serves no token is not EC2.
//
// OUTSCALE serves an EC2-shaped tree at the address EC2 uses, but IMDSv1 only:
// there is no /latest/api/token. The token exchange being the availability
// probe is what keeps this provider from claiming those VMs, and
// providers/outscale from having to fight it for them.
func TestIMDSv1OnlyServiceIsNotDetected(t *testing.T) {
	ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte("i-be1dcf1a"))
	}))

	_, err := ec2.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

// An on-demand instance outside an auto scaling group has neither path, and
// the real service 404s both. Neither is a failure.
func TestConditionalPathsAbsentIsNotAFailure(t *testing.T) {
	ep := imdstest.Server(t, handler())

	md, err := ec2.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Empty(t, md.SpotTerminationTime)
	assert.Empty(t, md.AutoscalingLifecycleState)
}
