// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package imdstest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
)

func TestMain(m *testing.M) { imdstest.Main(m) }

// referenceMetadata and referenceFetch are the shape every provider in this
// repository takes, kept here so the conformance suite is exercised against a
// provider that is known to be correct, and so there is one worked example of
// what a provider is allowed to do.
//
// Note what is absent: no http.Client, no timeout, no goroutine, and no
// decision about what an error means. The provider describes addresses and a
// payload; imds owns everything else.
type referenceMetadata struct {
	ID string `json:"id"`
}

func referenceFetch(ctx context.Context, opts ...imds.Option) (referenceMetadata, error) {
	c := imds.NewClient(opts...)

	var md referenceMetadata
	err := c.Lookup(ctx, func(ctx context.Context) error {
		body, _, err := c.GetFirst(ctx, c.Endpoints("http://169.254.169.254"), "/meta")
		if err != nil {
			return err
		}
		return json.Unmarshal(body, &md)
	})
	return md, err
}

func TestConformance(t *testing.T) {
	imdstest.Conformance(t, imdstest.Subject{
		Name:    "reference",
		Handler: imdstest.JSON("/meta", `{"id":"i-reference"}`),
		Fetch: func(ctx context.Context, opts ...imds.Option) error {
			_, err := referenceFetch(ctx, opts...)
			return err
		},
	})
}

func TestReferenceProviderDecodes(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/meta", `{"id":"i-reference"}`))

	md, err := referenceFetch(t.Context(), imds.WithEndpoints(ep))
	require.NoError(t, err)
	assert.Equal(t, "i-reference", md.ID)
}

func TestBlackholeAcceptsAndNeverAnswers(t *testing.T) {
	ep := imdstest.Blackhole(t)

	c := imds.NewClient(imds.WithTimeout(100 * time.Millisecond))
	defer c.Close()

	err := c.Lookup(context.Background(), func(ctx context.Context) error {
		_, err := c.Get(ctx, ep+"/meta")
		return err
	})
	// The connection was established, so this is not absence: something is
	// there, it simply never answered.
	require.ErrorIs(t, err, imds.ErrTimeout)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}

func TestClosedRefusesConnections(t *testing.T) {
	c := imds.NewClient()
	defer c.Close()

	_, err := c.Get(t.Context(), imdstest.Closed(t)+"/meta")
	require.ErrorIs(t, err, imds.ErrAbsent)
}

func TestOversizedExceedsTheLimit(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Oversized(64<<10))

	c := imds.NewClient(imds.WithMaxBodySize(32 << 10))
	defer c.Close()

	_, err := c.Get(t.Context(), ep+"/meta")
	require.ErrorIs(t, err, imds.ErrBodyTooLarge)
}

func TestStatusServesTheGivenCode(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusTeapot))

	c := imds.NewClient()
	defer c.Close()

	_, err := c.Get(t.Context(), ep+"/meta")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "418")
}
