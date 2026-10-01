// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package scaleway_test

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/providers/scaleway"
)

// Not detected and failed are different outcomes. Only the first means the
// process is not on Scaleway.
func ExampleFetch() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	md, err := scaleway.Fetch(ctx)
	switch {
	case errors.Is(err, imds.ErrNotDetected):
		log.Print("not running on Scaleway")
	case err != nil:
		// On Scaleway, but the lookup failed. Not the same as being elsewhere.
		log.Printf("metadata lookup failed: %v", err)
	default:
		log.Printf("instance %s in %s", md.ID, md.Zone)
	}
}
