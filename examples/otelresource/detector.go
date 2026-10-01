// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package otelresource shows how to turn a go-imds provider into an
// OpenTelemetry [resource.Detector].
//
// go-imds ships no adapter for this on purpose. The mapping below is twenty
// lines, and every decision in it belongs to you rather than to a library:
// which semantic-convention version to pin, whether a zone implies a region,
// what host.name should fall back to. A shipped adapter would pin a semconv
// version for you and would need a release every time upstream cut one.
//
// Copy this file, change the provider, and keep it.
package otelresource

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/providers/scaleway"
)

// ScalewayDetector reports the Scaleway Instance the process is running on.
type ScalewayDetector struct {
	// Options are passed to the provider. Leave empty outside tests.
	Options []imds.Option
}

var _ resource.Detector = (*ScalewayDetector)(nil)

// Detect implements [resource.Detector].
//
// The three-way split is the whole point. Not running on Scaleway is an empty
// resource and no error, because it is the ordinary case on every other host.
// A metadata service that answered and then failed is an error, because
// reporting it as "not on Scaleway" would silently drop an instance that had
// already been reached -- and no one would ever find out.
func (d *ScalewayDetector) Detect(ctx context.Context) (*resource.Resource, error) {
	md, err := scaleway.Fetch(ctx, d.Options...)
	switch {
	case errors.Is(err, imds.ErrNotDetected):
		return resource.Empty(), nil
	case err != nil:
		return nil, err
	}

	attrs := []attribute.KeyValue{
		// Scaleway has no generated constant in this semconv version yet.
		semconv.CloudProviderKey.String("scaleway_cloud"),
		semconv.CloudPlatformKey.String("scaleway_cloud_compute"),
	}
	for _, a := range []struct {
		value string
		attr  func(string) attribute.KeyValue
	}{
		{md.AccountID, semconv.CloudAccountID},
		{md.Region, semconv.CloudRegion},
		{md.Zone, semconv.CloudAvailabilityZone},
		{md.ID, semconv.HostID},
		{md.Hostname, semconv.HostName},
		{md.Type, semconv.HostType},
		{md.ImageID, semconv.HostImageID},
		{md.ImageName, semconv.HostImageName},
	} {
		// Absent fields are left out rather than emitted empty: an empty
		// attribute is worse than a missing one, because it looks like data.
		if a.value != "" {
			attrs = append(attrs, a.attr(a.value))
		}
	}

	return resource.NewWithAttributes(semconv.SchemaURL, attrs...), nil
}
