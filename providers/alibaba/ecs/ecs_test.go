// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package ecs_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/alibaba/ecs"
)

// Every path Alibaba documents under /latest/meta-data/. Values are
// illustrative rather than captured. ram/security-credentials is documented
// too and is deliberately absent: this provider never requests it. The Windows
// and spot paths are absent here on purpose, so the optional handling is
// exercised.
var values = map[string]string{
	"instance-id":            "i-bp1234567890abcdef",
	"hostname":               "iZbp1234567890Z",
	"owner-account-id":       "1234567890123456",
	"instance/instance-type": "ecs.g6.large",
	"image-id":               "ubuntu_22_04_x64_20G_alibase",
	"region-id":              "cn-hangzhou",
	"zone-id":                "cn-hangzhou-b",

	"serial-number":      "6ec1b1e1-1111-2222-3333-444444444444",
	"network-type":       "vpc",
	"vpc-id":             "vpc-bp1234567890",
	"vpc-cidr-block":     "172.16.0.0/12",
	"vswitch-id":         "vsw-bp1234567890",
	"vswitch-cidr-block": "172.16.0.0/24",
	"mac":                "00:16:3e:0a:1b:2c",
	"private-ipv4":       "172.16.0.5",
	"public-ipv4":        "47.98.0.10",
	"eipv4":              "47.98.0.11",

	"dns-conf/nameservers": "100.100.2.136\n100.100.2.138\n",
	"ntp-conf/ntp-servers": "ntp.cloud.aliyuncs.com\n",
	"source-address":       "http://mirrors.cloud.aliyuncs.com",

	"instance/max-netbw-egress":                   "1024000",
	"instance/instance-type-spec/max-netpps":      "500000",
	"instance/instance-type-spec/max-connections": "250000",
	"instance/virtualization-solution":            "ECS Virt",
	"instance/virtualization-solution-version":    "2.0",
	"image/market-place/product-code":             "cmjj000000",
	"image/market-place/charge-type":              "PrePaid",
}

// The real service refuses a metadata read without a token and answers the
// token endpoint only for PUT, so the fake insists on both: a provider that
// stopped exchanging a token would otherwise keep passing.
func handler(tb testing.TB) http.Handler {
	tb.Helper()
	const token = "session-token"

	mux := http.NewServeMux()
	mux.HandleFunc("/latest/api/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("X-aliyun-ecs-metadata-token-ttl-seconds") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(token))
	})
	mux.HandleFunc("/latest/meta-data/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-aliyun-ecs-metadata-token") != token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		v, ok := values[strings.TrimPrefix(r.URL.Path, "/latest/meta-data/")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(v))
	})
	return mux
}

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "alibaba/ecs",
		Handler: handler(t),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := ecs.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler(t))

	md, err := ecs.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, ecs.Metadata{
		ID:        "i-bp1234567890abcdef",
		Hostname:  "iZbp1234567890Z",
		AccountID: "1234567890123456",
		Type:      "ecs.g6.large",
		ImageID:   "ubuntu_22_04_x64_20G_alibase",
		Region:    "cn-hangzhou",
		Zone:      "cn-hangzhou-b",

		SerialNumber:                  "6ec1b1e1-1111-2222-3333-444444444444",
		NetworkType:                   "vpc",
		VPCID:                         "vpc-bp1234567890",
		VPCCIDRBlock:                  "172.16.0.0/12",
		VSwitchID:                     "vsw-bp1234567890",
		VSwitchCIDRBlock:              "172.16.0.0/24",
		MAC:                           "00:16:3e:0a:1b:2c",
		PrivateIP:                     "172.16.0.5",
		PublicIP:                      "47.98.0.10",
		ElasticIP:                     "47.98.0.11",
		Nameservers:                   []string{"100.100.2.136", "100.100.2.138"},
		NTPServers:                    []string{"ntp.cloud.aliyuncs.com"},
		SourceAddress:                 "http://mirrors.cloud.aliyuncs.com",
		MaxNetworkBandwidthEgress:     "1024000",
		MaxPPS:                        "500000",
		MaxConnections:                "250000",
		VirtualizationSolution:        "ECS Virt",
		VirtualizationSolutionVersion: "2.0",
		MarketplaceProductCode:        "cmjj000000",
		MarketplaceChargeType:         "PrePaid",
	}, md)
}

// The token exchange is the availability probe, so a 4xx there is what says
// "not on Alibaba".
func TestTokenRejectedIsNotDetected(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusNotFound))

	_, err := ecs.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

// Once the token exchange has succeeded the service has identified itself, so a
// missing path afterwards is a failure of that service rather than evidence of
// being on another cloud. Reporting it as not-detected would discard an
// instance already identified.
func TestMissingPathAfterTokenIsAFailureNotAbsence(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/latest/api/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("session-token"))
	})
	mux.HandleFunc("/latest/meta-data/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	ep := imdstest.Server(t, mux)

	_, err := ecs.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.Error(t, err)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}

// A token is a credential for the metadata service. An error string is the one
// place credentials most reliably escape into logs.
func TestTokenNeverAppearsInAnError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/latest/api/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("super-secret-token"))
	})
	mux.HandleFunc("/latest/meta-data/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	ep := imdstest.Server(t, mux)

	_, err := ecs.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "super-secret-token")
}

// This instance is Linux and not spot, so the Windows activation servers and
// the spot termination time 404. None of that is a failure.
func TestPathsAbsentOnThisConfigurationAreNotFailures(t *testing.T) {
	ep := imdstest.Server(t, handler(t))

	md, err := ecs.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Empty(t, md.KMSServer)
	assert.Empty(t, md.WSUSServer)
	assert.Empty(t, md.SpotTerminationTime)
}
