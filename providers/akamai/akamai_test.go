// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package akamai_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
	"github.com/paulojmdias/go-imds/providers/akamai"
)

// The field set follows linode/go-metadata's InstanceData, which is Akamai's
// own client for this endpoint. Values are illustrative rather than captured.
const metadataJSON = `{
  "id": 12345678,
  "label": "app-server-1",
  "region": "us-east",
  "type": "g6-standard-2",
  "host_uuid": "3d4a1f8c-1111-2222-3333-444444444444",
  "tags": ["telemetry", "prod"],
  "image": { "id": "linode/ubuntu22.04", "label": "Ubuntu 22.04 LTS" },
  "account_euuid": "A1B2C3D4-1111-2222-3333-444444444444",
  "specs": { "vcpus": 2, "memory": 4096, "gpus": 0, "transfer": 4000, "disk": 81920 },
  "backups": { "enabled": true, "status": "pending" }
}`

func handler(body string) http.Handler {
	const token = "metadata-token"

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.Header.Get("Metadata-Token-Expiry-Seconds") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(token))
	})
	mux.HandleFunc("/v1/instance", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata-Token") != token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})
	return mux
}

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "akamai",
		Handler: handler(metadataJSON),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := akamai.Fetch(ctx, opts...)
			return err
		},
	})
}

func TestFetch(t *testing.T) {
	ep := imdstest.Server(t, handler(metadataJSON))

	md, err := akamai.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)

	assert.Equal(t, akamai.Metadata{
		ID:        "12345678",
		Hostname:  "app-server-1",
		AccountID: "A1B2C3D4-1111-2222-3333-444444444444",
		Type:      "g6-standard-2",
		ImageID:   "linode/ubuntu22.04",
		ImageName: "Ubuntu 22.04 LTS",
		Region:    "us-east",
		HostUUID:  "3d4a1f8c-1111-2222-3333-444444444444",
		Tags:      []string{"telemetry", "prod"},
		Specs:     akamai.Specs{VCPUs: 2, Memory: 4096, Transfer: 4000, Disk: 81920},
		Backups:   akamai.Backups{Enabled: true, Status: "pending"},
	}, md)
}

func TestTokenRejectedIsNotDetected(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusNotFound))

	_, err := akamai.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

func TestTokenNeverAppearsInAnError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("super-secret-token"))
	})
	mux.HandleFunc("/v1/instance", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	ep := imdstest.Server(t, mux)

	_, err := akamai.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "super-secret-token")
}

// backups.status is null until a backup has run, which is not the same as the
// key being absent. Decoding it into a string directly would fail.
func TestNullBackupStatusDecodes(t *testing.T) {
	ep := imdstest.Server(t, handler(`{"id":1,"backups":{"enabled":false,"status":null}}`))

	md, err := akamai.Fetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Equal(t, akamai.Backups{Enabled: false, Status: ""}, md.Backups)
}
