// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package classic_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/ibmcloud/classic"
)

// Values as the SoftLayer Resource Metadata API serves them: one plain-text
// document per field. Illustrative rather than captured.
// The SoftLayer Resource Metadata paths this provider reads.
// getUserMetadata.txt is served too and is deliberately absent: it carries
// whatever the launcher put there.
var values = map[string]string{
	"/getId.txt":               "156800198",
	"/getHostname.txt":         "app-server-1",
	"/getDatacenter.txt":       "par01",
	"/getAccountId.txt":        "3186058",
	"/getGlobalIdentifier.txt": "06220b70-9072-4f83-ba16-d62f03106c1c",

	"/getDomain.txt":                   "example.com",
	"/getFullyQualifiedDomainName.txt": "app-server-1.example.com",
	"/getPrimaryIpAddress.txt":         "159.8.0.10",
	"/getPrimaryBackendIpAddress.txt":  "10.30.0.5",
	"/getDatacenterId.txt":             "449506",
	"/getRouter.txt":                   "bcr01a.par01",
	"/getProvisionState.txt":           "COMPLETE",

	"/getTags.json":                "[\"telemetry\",\"prod\"]",
	"/getPrimaryMacAddresses.json": "[\"06:8e:5c:1a:2b:3c\"]",
	"/getBackendMacAddresses.json": "[\"06:8e:5c:1a:2b:3d\"]",
	"/getFrontendVlanIds.json":     "[1234]",
	"/getBackendVlanIds.json":      "[5678]",
}

func handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, ok := values[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(v))
	})
}

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "ibmcloud/classic",
		Handler: handler(),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := classic.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler())

	md, err := classic.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, classic.Metadata{
		ID:               "156800198",
		Hostname:         "app-server-1",
		AccountID:        "3186058",
		Datacenter:       "par01",
		GlobalIdentifier: "06220b70-9072-4f83-ba16-d62f03106c1c",

		Domain:                   "example.com",
		FullyQualifiedDomainName: "app-server-1.example.com",
		PrimaryIP:                "159.8.0.10",
		PrimaryBackendIP:         "10.30.0.5",
		DatacenterID:             "449506",
		Router:                   "bcr01a.par01",
		ProvisionState:           "COMPLETE",
		Tags:                     []string{"telemetry", "prod"},
		PrimaryMACAddresses:      []string{"06:8e:5c:1a:2b:3c"},
		BackendMACAddresses:      []string{"06:8e:5c:1a:2b:3d"},
		FrontendVLANIDs:          []int64{1234},
		BackendVLANIDs:           []int64{5678},
	}, md)
}

// The API trims nothing itself; a trailing newline in a .txt document must not
// end up inside the value.
func TestValuesAreTrimmed(t *testing.T) {
	ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v, ok := values[r.URL.Path]; ok {
			_, _ = w.Write([]byte(v + "\n"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	md, err := classic.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Equal(t, "156800198", md.ID)
}

// The first read is the probe. A 4xx there means this is not the SoftLayer API.
func TestFirstReadRejectedIsNotDetected(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusNotFound))

	_, err := classic.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

// Once it has answered once, a later 4xx is the API failing.
func TestLaterReadRejectedIsAFailureNotAbsence(t *testing.T) {
	ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/getId.txt" {
			_, _ = w.Write([]byte("156800198"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	_, err := classic.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.Error(t, err)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}
