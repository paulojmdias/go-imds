# go-imds

[![Go Reference](https://pkg.go.dev/badge/github.com/paulojmdias/go-imds.svg)](https://pkg.go.dev/github.com/paulojmdias/go-imds)
[![CI](https://github.com/paulojmdias/go-imds/actions/workflows/ci.yml/badge.svg)](https://github.com/paulojmdias/go-imds/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/paulojmdias/go-imds)](https://goreportcard.com/report/github.com/paulojmdias/go-imds)

Read cloud instance metadata, from one place, without getting the request
boundary wrong.

> **Status: pre-release.** The API described here is being built and is not yet
> tagged. Expect it to move until `v0.1.0`.

## The problem

Every cloud exposes instance metadata over a link-local HTTP endpoint, and
every project ends up writing its own client for it. Those clients are short,
which is why the same five mistakes keep getting made:

- The request goes through `HTTP_PROXY`, because `http.DefaultTransport` and
  every client built without an explicit transport use `ProxyFromEnvironment`.
  A proxy that answers for `169.254.169.254` makes detection depend on the
  proxy, and can send instance metadata to it.
- The lookup is bounded by `http.Client.Timeout` instead of the caller's
  context. That timeout is not cancellable by the caller, and it restarts for
  each request, so a two-endpoint lookup can take twice as long as the caller
  allowed.
- A vendor SDK with no context-aware call gets wrapped in a goroutine, which
  outlives the lookup along with its connection. Repeated detection accumulates
  them.
- A failure of the metadata service is reported as "not running on this cloud",
  so detection silently misses an instance it should have found.
- The response body is read unbounded.

## The contract

The whole point of this module is that these five rules hold everywhere, and
that a provider cannot opt out of them.

1. **Proxies are off.** The transport is built once with `Proxy` set to `nil`.
   `http.DefaultClient`, `http.DefaultTransport` and `http.Get` are rejected by
   the linter, repository-wide.
2. **One context bounds the whole operation.** The token exchange, every
   endpoint in a fallback list and every path in a multi-request lookup share a
   single deadline. `http.Client.Timeout` is never used. When the caller's
   context has no deadline an internal default applies, and the caller's own
   cancellation stays distinguishable from it.
3. **Nothing outlives the call.** No goroutine, request or connection started by
   a lookup survives its return. Verified with `goleak` against a server that
   accepts a connection and never answers.
4. **The outcome is never collapsed.** "Nothing is there", "something else
   answered" and "the metadata service failed" are distinct results.
5. **Bodies are bounded.** Every response, token responses included, is read
   through an `io.LimitReader`. This is the rule every vendor SDK assessed for
   this module broke.

## Install

```
go get github.com/paulojmdias/go-imds
```

One module, no cloud SDK dependencies. The only runtime dependency is a YAML
parser, imported by `providers/hetzner` alone.

## Usage

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

md, err := scaleway.Fetch(ctx)
switch {
case errors.Is(err, imds.ErrNotDetected):
	// Not running on Scaleway. Nothing answered, or something answered that
	// was not the Scaleway metadata service.
case err != nil:
	// Running on Scaleway, but the lookup failed. Do not treat this as
	// "not on Scaleway" -- that is the silent-miss bug.
	return err
default:
	log.Printf("instance %s in %s", md.ID, md.Zone)
}
```

`imds.ErrAbsent` and `imds.ErrForeign` are available when the finer distinction
matters; both satisfy `errors.Is(err, imds.ErrNotDetected)`.

## Providers

| Provider | Package |
| --- | --- |
| Akamai (Linode) | `providers/akamai` |
| Alibaba Cloud ECS | `providers/alibaba/ecs` |
| Amazon EC2 | `providers/aws/ec2` |
| Amazon ECS | `providers/aws/ecs` |
| Azure VM | `providers/azure/vm` |
| DigitalOcean | `providers/digitalocean` |
| Google Compute Engine | `providers/gcp` |
| Hetzner Cloud | `providers/hetzner` |
| IBM Cloud Classic | `providers/ibmcloud/classic` |
| IBM Cloud VPC | `providers/ibmcloud/vpc` |
| OpenStack Nova | `providers/openstack/nova` |
| Oracle Cloud | `providers/oraclecloud` |
| OUTSCALE | `providers/outscale` |
| Scaleway | `providers/scaleway` |
| Tencent Cloud CVM | `providers/tencent/cvm` |
| UpCloud | `providers/upcloud` |
| Vultr | `providers/vultr` |

Providers return plain Go structs carrying **everything the service reports**,
not the handful of fields one consumer happens to need. `providers/digitalocean`
returns the interfaces, DNS and features alongside the droplet id;
`providers/scaleway` returns the volumes, private NICs and public addresses.
Where a service offers the whole tree in one request, it is read that way:
`providers/gcp` uses `?recursive=true` for two requests rather than six, and
`providers/hetzner` reads its whole document in one rather than four.

Two things are deliberately left out, and every provider's `doc.go` says so:

- **Credentials.** No provider requests `iam/security-credentials`,
  `service-accounts/*/token`, `ram/security-credentials` or their equivalents.
  A `Metadata` struct gets logged and turned into resource attributes, so
  everything in one should be safe to print. `internal/audit` fails the build
  if such a path appears in a provider.
- **User data.** `user-data`, `vendor-data`, Azure's `customData` and Hetzner's
  `vendor_data` carry whatever the person who launched the instance put there,
  which is frequently a secret. That is the caller's to fetch, knowingly, if
  they want it.

There is no OpenTelemetry dependency anywhere in this module, which is what
makes it usable outside observability tooling. The one non-test dependency is a
YAML parser, used by `providers/hetzner` alone because Hetzner alone serves
YAML; a lint rule keeps it there.

Some clouds are covered without a package of their own. OVHcloud Public Cloud
and STACKIT are OpenStack, and are read through `providers/openstack/nova`.

What is out of scope is worth stating too, because the boundary is not obvious.
This module answers "what am I running on" from inside an instance, over HTTP,
with no credentials. That is a different question from service discovery, which
runs elsewhere and calls a cloud's management API with credentials to enumerate
*other* machines -- so a cloud having a Prometheus service-discovery integration
says nothing about whether it has a metadata service. Two consequences: Joyent
Triton is excluded because its metadata API runs over a serial port and a unix
socket rather than HTTP, and IONOS because no link-local metadata service is
documented for it.

## Why not vendor SDKs

Most clouds publish a metadata client, and every one that was assessed for this
module was rejected, on evidence rather than preference. A vendor client is
usable here only if it accepts an injected `*http.Client` without mutating it,
honours the caller's context throughout, leaves nothing running, and passes the
conformance suite unmodified.

- `hetznercloud/hcloud-go` assigns to the `Timeout` field of the `http.Client`
  you give it, which replaces the caller's deadline with a wall-clock one and
  mutates an object you own. It also pulls in Prometheus, to read four fields.
- `cloud.google.com/go/compute/metadata` reads response bodies with
  `io.ReadAll` and no limit. Its `Get` also returns only the body, so a caller
  cannot check the `Metadata-Flavor: Google` header the service sends back --
  which is the strongest available test of whether the thing answering on the
  shared address is really Google's metadata service. This package checks it.
- `aws-sdk-go-v2/feature/ec2/imds` reads the token and every response body with
  no limit, and brings a large dependency tree for what is one PUT and two
  GETs.

An unbounded read is the interesting one. It matters precisely because
`169.254.169.254` is shared and can be claimed by anything, which is the same
reason the outcome is never collapsed. A client that cannot be told to stop
reading cannot make the guarantee.

What was worth inheriting from those SDKs was the conventions, not the code,
and those are short: `GCE_METADATA_HOST`, `AWS_EC2_METADATA_DISABLED`,
`AWS_EC2_METADATA_SERVICE_ENDPOINT` and its endpoint mode are all honoured
here.

The full criteria are in [CONTRIBUTING.md](CONTRIBUTING.md), so the next SDK
gets judged rather than assumed.

## With OpenTelemetry

This module deliberately ships no semantic-convention adapter: which semconv
version to pin, and how to map fields onto it, is your policy, not this
module's. A shipped adapter would pin a version for you and would need a
release every time upstream cut one.

The mapping is short. [`examples/otelresource`](examples/otelresource) has a
complete, tested `resource.Detector` to copy -- kept in its own module, so that
depending on go-imds never pulls OpenTelemetry into your build.

```go
md, err := scaleway.Fetch(ctx)
switch {
case errors.Is(err, imds.ErrNotDetected):
	return resource.Empty(), nil
case err != nil:
	return nil, err
}

attrs := []attribute.KeyValue{
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
} {
	// Absent fields are left out rather than emitted empty: an empty
	// attribute is worse than a missing one, because it looks like data.
	if a.value != "" {
		attrs = append(attrs, a.attr(a.value))
	}
}

return resource.NewWithAttributes(semconv.SchemaURL, attrs...), nil
```

## Testing

Every provider is tested against `internal/imdstest`, because the failures that
matter here do not show up in ordinary unit tests: a lookup that leaks a
goroutine still returns the right value, and a lookup routed through a proxy
still succeeds on a machine with no proxy set.

- `imdstest.Server` serves recorded per-provider fixtures, with knobs for status
  codes and oversized bodies.
- `imdstest.Blackhole` accepts a connection and never answers, so a test can
  assert that the lookup returns *and* leaves nothing behind.
- `imdstest.Main` runs a package's tests under a goroutine-leak check, with
  `HTTP_PROXY` pointed at a trap that fails the run if anything reaches it.
- `imdstest.Conformance` is the suite every provider must pass unmodified.

The kit is internal: it exists to hold this repository's providers to the
contract, not as API to keep stable.

It has teeth: a provider that collapses a failing metadata service into "not
running on this cloud" fails five of its six cases.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). If you are an automated agent, read
[AGENTS.md](AGENTS.md) first.

## License

Apache-2.0. See [LICENSE](LICENSE).
