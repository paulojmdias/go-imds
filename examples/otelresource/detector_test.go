// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package otelresource_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"

	"github.com/paulojmdias/go-imds/examples/otelresource"
	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
)

const metadataJSON = `{
  "id": "11111111-2222-3333-4444-555555555555",
  "name": "scw-vigilant-shannon",
  "organization": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "commercial_type": "DEV1-S",
  "image": { "id": "99999999-8888-7777-6666-555555555555", "name": "Ubuntu 22.04" },
  "location": { "zone_id": "fr-par-1" }
}`

func TestMain(m *testing.M) { imdstest.Main(m) }

func TestDetect(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/conf", metadataJSON))
	d := &otelresource.ScalewayDetector{Options: []imds.Option{imds.WithEndpoints(ep)}}

	res, err := d.Detect(t.Context())
	require.NoError(t, err)

	got := map[attribute.Key]string{}
	for _, kv := range res.Attributes() {
		got[kv.Key] = kv.Value.AsString()
	}
	assert.Equal(t, map[attribute.Key]string{
		"cloud.provider":          "scaleway_cloud",
		"cloud.platform":          "scaleway_cloud_compute",
		"cloud.account.id":        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"cloud.region":            "fr-par",
		"cloud.availability_zone": "fr-par-1",
		"host.id":                 "11111111-2222-3333-4444-555555555555",
		"host.name":               "scw-vigilant-shannon",
		"host.type":               "DEV1-S",
		"host.image.id":           "99999999-8888-7777-6666-555555555555",
		"host.image.name":         "Ubuntu 22.04",
	}, got)
}

// The ordinary case on every host that is not a Scaleway Instance.
func TestNotOnScalewayIsAnEmptyResource(t *testing.T) {
	d := &otelresource.ScalewayDetector{Options: []imds.Option{imds.WithEndpoints(imdstest.Closed(t))}}

	res, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Empty(t, res.Attributes())
}

// The case a detector must not swallow: the service was reached and failed.
func TestReachableButFailingIsAnError(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusInternalServerError))
	d := &otelresource.ScalewayDetector{Options: []imds.Option{imds.WithEndpoints(ep)}}

	_, err := d.Detect(t.Context())
	require.Error(t, err)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}

// Fields the metadata service did not report are absent, not empty.
func TestAbsentFieldsAreOmitted(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/conf", `{"id":"i-1","location":{"zone_id":"fr-par-1"}}`))
	d := &otelresource.ScalewayDetector{Options: []imds.Option{imds.WithEndpoints(ep)}}

	res, err := d.Detect(t.Context())
	require.NoError(t, err)
	for _, kv := range res.Attributes() {
		assert.NotEmpty(t, kv.Value.AsString(), "attribute %s is present but empty", kv.Key)
	}
}
