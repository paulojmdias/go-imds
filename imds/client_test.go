// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

package imds_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/paulojmdias/go-imds/imds"
	"github.com/paulojmdias/go-imds/internal/imdstest"
)

func TestMain(m *testing.M) { imdstest.Main(m) }

func TestGetSuccess(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/meta", `{"id":"i-1"}`))

	c := imds.NewClient()
	defer c.Close()

	body, err := c.Get(t.Context(), ep+"/meta")
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"i-1"}`, string(body))
}

func TestGetSendsHeadersAndUserAgent(t *testing.T) {
	var got http.Header
	ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte("{}"))
	}))

	c := imds.NewClient()
	defer c.Close()

	_, err := c.Get(t.Context(), ep+"/meta", imds.WithHeader("Metadata-Flavor", "Google"))
	require.NoError(t, err)

	assert.Equal(t, "Google", got.Get("Metadata-Flavor"))
	// Identifying the client to the metadata service is the module's job, not
	// the caller's, so the agent is fixed rather than an option.
	assert.True(t, strings.HasPrefix(got.Get("User-Agent"), "go-imds/"),
		"User-Agent is %q, want a go-imds/ prefix", got.Get("User-Agent"))
}

func TestGetMethod(t *testing.T) {
	var method string
	ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		_, _ = w.Write([]byte("token"))
	}))

	c := imds.NewClient()
	defer c.Close()

	_, err := c.Get(t.Context(), ep+"/token", imds.WithMethod(http.MethodPut))
	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, method)
}

func TestGetNothingListeningIsAbsent(t *testing.T) {
	c := imds.NewClient()
	defer c.Close()

	_, err := c.Get(t.Context(), imdstest.Closed(t)+"/meta")
	require.ErrorIs(t, err, imds.ErrAbsent)
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

func TestGetClientErrorIsForeign(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusNotFound))

	c := imds.NewClient()
	defer c.Close()

	_, err := c.Get(t.Context(), ep+"/meta")
	require.ErrorIs(t, err, imds.ErrForeign)
	require.ErrorIs(t, err, imds.ErrNotDetected)
}

// A provider whose address is its own alone treats any answer as identifying
// the platform, so a 4xx is a failure of the metadata service rather than
// evidence of being somewhere else.
func TestGetNeverForeign(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusNotFound))

	c := imds.NewClient()
	defer c.Close()

	_, err := c.Get(t.Context(), ep+"/meta", imds.NeverForeign())
	require.Error(t, err)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}

func TestGetServerErrorIsNotAbsence(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusInternalServerError))

	c := imds.NewClient()
	defer c.Close()

	_, err := c.Get(t.Context(), ep+"/meta")
	require.Error(t, err)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}

func TestGetBodyIsBounded(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Oversized(1<<20))

	c := imds.NewClient(imds.WithMaxBodySize(4096))
	defer c.Close()

	_, err := c.Get(t.Context(), ep+"/meta")
	require.ErrorIs(t, err, imds.ErrBodyTooLarge)
}

// A body at exactly the limit is valid: the limit is a maximum, not a
// threshold that rejects the largest legitimate document.
func TestGetBodyAtLimitIsAccepted(t *testing.T) {
	const size = 4096
	ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, size))
	}))

	c := imds.NewClient(imds.WithMaxBodySize(size))
	defer c.Close()

	body, err := c.Get(t.Context(), ep+"/meta")
	require.NoError(t, err)
	assert.Len(t, body, size)
}

func TestGetFirstFallsBackPastAbsent(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/meta", `{"id":"i-2"}`))

	c := imds.NewClient()
	defer c.Close()

	body, used, err := c.GetFirst(t.Context(), []string{imdstest.Closed(t), ep}, "/meta")
	require.NoError(t, err)
	assert.Equal(t, ep, used)
	assert.JSONEq(t, `{"id":"i-2"}`, string(body))
}

// A link-local address can be claimed by unrelated software while the real
// metadata service answers on another, so a foreign answer is not the end of
// the walk.
func TestGetFirstFallsBackPastForeign(t *testing.T) {
	foreign := imdstest.Server(t, imdstest.Status(http.StatusNotFound))
	serving := imdstest.Server(t, imdstest.JSON("/meta", `{"id":"i-3"}`))

	c := imds.NewClient()
	defer c.Close()

	body, used, err := c.GetFirst(t.Context(), []string{foreign, serving}, "/meta")
	require.NoError(t, err)
	assert.Equal(t, serving, used)
	assert.JSONEq(t, `{"id":"i-3"}`, string(body))
}

// The regression this guards against: errors.Is reports true when any joined
// error matches, so joining a real failure with "nothing at the other address"
// would make the failure read as absence.
func TestGetFirstDoesNotJoinRealFailureWithAbsence(t *testing.T) {
	failing := imdstest.Server(t, imdstest.Status(http.StatusInternalServerError))

	c := imds.NewClient()
	defer c.Close()

	_, _, err := c.GetFirst(t.Context(), []string{imdstest.Closed(t), failing, "http://unused"}, "/meta")
	require.Error(t, err)
	require.NotErrorIs(t, err, imds.ErrNotDetected,
		"a metadata service that answered and failed must not read as not-detected")
}

func TestGetFirstAllAbsent(t *testing.T) {
	c := imds.NewClient()
	defer c.Close()

	_, _, err := c.GetFirst(t.Context(), []string{imdstest.Closed(t), imdstest.Closed(t)}, "/meta")
	require.ErrorIs(t, err, imds.ErrAbsent)
}

func TestGetFirstNoEndpoints(t *testing.T) {
	c := imds.NewClient()
	defer c.Close()

	_, _, err := c.GetFirst(t.Context(), nil, "/meta")
	require.Error(t, err)
}

func TestEndpointsOverrideReplacesDefaults(t *testing.T) {
	c := imds.NewClient()
	defer c.Close()
	assert.Equal(t, []string{"http://default"}, c.Endpoints("http://default"))

	c2 := imds.NewClient(imds.WithEndpoints("http://override"))
	defer c2.Close()
	assert.Equal(t, []string{"http://override"}, c2.Endpoints("http://default"))
}

// One deadline covers every address, rather than restarting per request. With a
// per-request timeout the two blackholes below would take twice as long.
func TestLookupBoundsEveryAddressWithOneDeadline(t *testing.T) {
	c := imds.NewClient()
	defer c.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := c.Lookup(ctx, func(ctx context.Context) error {
		_, _, err := c.GetFirst(ctx, []string{imdstest.Blackhole(t), imdstest.Blackhole(t)}, "/meta")
		return err
	})
	elapsed := time.Since(start)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, elapsed, 3*time.Second, "the deadline is not bounding the whole operation")
}

func TestLookupAppliesDefaultTimeoutWithoutCallerDeadline(t *testing.T) {
	c := imds.NewClient(imds.WithTimeout(150 * time.Millisecond))
	defer c.Close()

	start := time.Now()
	err := c.Lookup(context.Background(), func(ctx context.Context) error {
		_, err := c.Get(ctx, imdstest.Blackhole(t)+"/meta")
		return err
	})

	// The client's own deadline expiring is not the caller giving up: the caller
	// never set one. Reporting DeadlineExceeded here would tell the caller their
	// context ended when it did not.
	require.ErrorIs(t, err, imds.ErrTimeout)
	require.NotErrorIs(t, err, context.DeadlineExceeded)
	// Nor is it a determination that nothing is there. Something accepted the
	// connection; it simply never answered.
	require.NotErrorIs(t, err, imds.ErrNotDetected)
	assert.Less(t, time.Since(start), 3*time.Second)
}

func TestLookupReportsCallerCancellationAsItself(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/meta", `{}`))

	c := imds.NewClient()
	defer c.Close()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := c.Lookup(ctx, func(ctx context.Context) error {
		_, err := c.Get(ctx, ep+"/meta")
		return err
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestLookupSuccess(t *testing.T) {
	ep := imdstest.Server(t, imdstest.JSON("/meta", `{"id":"i-4"}`))

	c := imds.NewClient()
	defer c.Close()

	var md struct {
		ID string `json:"id"`
	}
	err := c.Lookup(t.Context(), func(ctx context.Context) error {
		body, _, err := c.GetFirst(ctx, []string{ep}, "/meta")
		if err != nil {
			return err
		}
		return json.Unmarshal(body, &md)
	})
	require.NoError(t, err)
	assert.Equal(t, "i-4", md.ID)
}

func TestErrAbsentAndForeignAreNotDetected(t *testing.T) {
	require.ErrorIs(t, imds.ErrAbsent, imds.ErrNotDetected)
	require.ErrorIs(t, imds.ErrForeign, imds.ErrNotDetected)
	require.NotErrorIs(t, imds.ErrAbsent, imds.ErrForeign)
	// The relationship is one-way: every ErrAbsent is a not-detected result,
	// but not every not-detected result is an ErrAbsent.
	require.NotErrorIs(t, imds.ErrNotDetected, imds.ErrAbsent)
}

// A path that does not exist is distinct from a service that is failing and
// from a service that is not the one being looked for. Providers read
// configuration-dependent paths -- a spot termination time, a Windows
// activation server -- and have to tell an unpopulated field from a fault.
func TestNotFoundAfterTheProbeIsItsOwnOutcome(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusNotFound))

	c := imds.NewClient()
	defer c.Close()

	_, err := c.Get(t.Context(), ep+"/meta", imds.NeverForeign())
	require.ErrorIs(t, err, imds.ErrNotFound)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
}

// Without NeverForeign a 404 still means the responder is not this service,
// which is what a shared link-local address requires.
func TestNotFoundBeforeTheProbeIsForeign(t *testing.T) {
	ep := imdstest.Server(t, imdstest.Status(http.StatusNotFound))

	c := imds.NewClient()
	defer c.Close()

	_, err := c.Get(t.Context(), ep+"/meta")
	require.ErrorIs(t, err, imds.ErrForeign)
	require.NotErrorIs(t, err, imds.ErrNotFound)
}

// A body that is not JSON at all is a malfunction. Calling it "not this cloud"
// would hide a broken metadata service behind a missed detection.
func TestGetJSONFirstUndecodableBodyIsAFailure(t *testing.T) {
	ep := imdstest.Server(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>not json</html>"))
	}))

	c := imds.NewClient()
	defer c.Close()

	var v struct{}
	used, err := c.GetJSONFirst(t.Context(), []string{ep}, "/meta", &v)
	require.Error(t, err)
	require.NotErrorIs(t, err, imds.ErrNotDetected)
	assert.Equal(t, ep, used)
}
