// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package cvm_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/tencent/cvm"
)

// Every path Tencent documents under /latest/meta-data/. Values are
// illustrative rather than captured. cam/security-credentials is documented
// too and is deliberately absent: this provider never requests it.
var values = map[string]string{
	"instance-id":            "ins-abcd1234",
	"instance-name":          "app-server-1",
	"app-id":                 "1250000000",
	"instance/instance-type": "S5.MEDIUM4",
	"instance/image-id":      "img-9qrfy1xt",
	"placement/region":       "ap-guangzhou",
	"placement/zone":         "ap-guangzhou-3",
	"uuid":                   "3a0d1b0e-1111-2222-3333-444444444444",
	"local-ipv4":             "10.0.0.7",
	"public-ipv4":            "119.29.0.10",
	"mac":                    "52:54:00:1a:2b:3c",

	"payment/charge-type":              "PREPAID",
	"payment/create-time":              "2024-01-02T03:04:05Z",
	"payment/termination-time":         "2025-01-02T03:04:05Z",
	"as-group-id":                      "asg-abc123",
	"instance/security-group":          "sg-abc123",
	"instance/bandwidth-limit-egress":  "102400",
	"instance/bandwidth-limit-ingress": "102400",
	"volumes":                          "disk-abc123",

	"network/interfaces/macs/52:54:00:1a:2b:3c/primary-local-ipv4":               "10.0.0.7",
	"network/interfaces/macs/52:54:00:1a:2b:3c/public-ipv4s":                     "119.29.0.10",
	"network/interfaces/macs/52:54:00:1a:2b:3c/vpc-id":                           "vpc-abc123",
	"network/interfaces/macs/52:54:00:1a:2b:3c/subnet-id":                        "subnet-abc123",
	"network/interfaces/macs/52:54:00:1a:2b:3c/local-ipv4s/10.0.0.7/gateway":     "10.0.0.1",
	"network/interfaces/macs/52:54:00:1a:2b:3c/local-ipv4s/10.0.0.7/subnet-mask": "255.255.255.0",
}

func handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, ok := values[strings.TrimPrefix(r.URL.Path, "/latest/meta-data/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(v))
	})
}

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "tencent/cvm",
		Handler: handler(),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := cvm.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler())

	md, err := cvm.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, cvm.Metadata{
		ID:        "ins-abcd1234",
		Hostname:  "app-server-1",
		AccountID: "1250000000",
		Type:      "S5.MEDIUM4",
		ImageID:   "img-9qrfy1xt",
		Region:    "ap-guangzhou",
		Zone:      "ap-guangzhou-3",

		UUID:                  "3a0d1b0e-1111-2222-3333-444444444444",
		PrivateIP:             "10.0.0.7",
		PublicIP:              "119.29.0.10",
		MAC:                   "52:54:00:1a:2b:3c",
		ChargeType:            "PREPAID",
		CreateTime:            "2024-01-02T03:04:05Z",
		TerminationTime:       "2025-01-02T03:04:05Z",
		ASGroupID:             "asg-abc123",
		SecurityGroup:         "sg-abc123",
		BandwidthLimitEgress:  "102400",
		BandwidthLimitIngress: "102400",
		Volumes:               "disk-abc123",
		Interface: cvm.Interface{
			MAC:            "52:54:00:1a:2b:3c",
			PrimaryLocalIP: "10.0.0.7",
			PublicIPs:      "119.29.0.10",
			VPCID:          "vpc-abc123",
			SubnetID:       "subnet-abc123",
			Gateway:        "10.0.0.1",
			SubnetMask:     "255.255.255.0",
		},
	}, md)
}

func TestFirstReadRejectedIsNotDetected(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusNotFound))

	_, err := cvm.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

// Once the service has answered once, a later 4xx is that service failing.
func TestLaterReadRejectedIsAFailureNotAbsence(t *testing.T) {
	ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "instance-id") {
			_, _ = w.Write([]byte("ins-abcd1234"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	_, err := cvm.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.Error(t, err)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}

// A path that does not exist on this instance is not a failure. Nothing is
// spot here, so spot/termination-time 404s, and the lookup has to survive it.
func TestOptionalPathsMayBeAbsent(t *testing.T) {
	ep := imdstest.Server(t, handler())

	md, err := cvm.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Empty(t, md.SpotTerminationTime)
}

// A required path 404ing is still a failure, so the tolerance above cannot
// quietly swallow a broken service.
func TestMissingRequiredPathIsAFailure(t *testing.T) {
	ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/placement/zone") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		v, ok := values[strings.TrimPrefix(r.URL.Path, "/latest/meta-data/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(v))
	}))

	_, err := cvm.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrNotFound)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}
