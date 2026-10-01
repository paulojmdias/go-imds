# Contributing

## Getting set up

```
git clone https://github.com/paulojmdias/go-imds
cd go-imds
make precommit
```

`make precommit` runs formatting, tidy, lint, tests and the build. It is what
CI runs, so if it passes locally it passes there. Individual targets:
`make gotest`, `make golint`, `make gofmt`, `make gotidy`, `make gobuild`,
`make gogovulncheck`.

Developer tools are pinned in `internal/tools/go.mod` and fetched on demand, so
there is nothing to install beyond Go 1.26 or later, which every module in the
repository requires.

## The contract

The five rules in the [README](README.md#the-contract) are invariants, not
guidelines. A change that weakens one is a change to the point of this module,
so raise it in an issue rather than working around it.

Two of them are enforced mechanically:

- `forbidigo` rejects `http.DefaultClient`, `http.DefaultTransport` and
  `http.Get`/`Head`/`Post` anywhere in the repository, because each uses
  `ProxyFromEnvironment`.
- `depguard` rejects every vendor cloud SDK import, and rejects any
  OpenTelemetry import in `imds`, `imdstest` and `providers`.

The rest are enforced by `imdstest.Conformance`. Do not add exemptions to it. A
provider that cannot pass the suite has a bug in the provider.

## Adding a provider

1. Work out the service's full documented surface, in this order of
   preference: a capture from a real instance, the vendor's own SDK type, the
   vendor's published schema. `providers/scaleway` follows Scaleway's own
   metadata type; `providers/digitalocean` and `providers/upcloud` follow
   captures published in cloud-init's datasource tests.

   Whatever the source, the test must name it in a comment and say whether the
   payload was captured or derived, so a doc-derived fixture is never mistaken
   for a real one.

   Providers expose everything the service exposes, not the subset one
   consumer happens to need. When you widen an existing provider, the fields it
   already returned must keep decoding to the same values -- that is the
   regression guard, and its assertions belong in the same test as the new
   ones.

2. **Descriptive fields only.** Never request a path that returns credentials:
   `iam/security-credentials` on EC2, `service-accounts/*/token` on GCP,
   `ram/security-credentials` on Alibaba, `cam/security-credentials` on
   Tencent. Never read `user-data`, `vendor-data`, `customData` or an
   equivalent -- those carry whatever the person who launched the instance put
   there, and a `Metadata` struct gets logged. `internal/audit` fails the build
   if one of those paths appears in a provider.

   Paths that exist only in some configurations -- a spot termination time, a
   Windows activation server, an auto-scaling state -- go through
   `imds.Client.GetTextOptional`, which tolerates a 404 and nothing else.
3. Record, from the source and not from any issue description: every endpoint
   and its order, whether a token exchange is involved, the required headers,
   and how the detector decides it is not running on that cloud.
4. Add `providers/<cloud>/` with a `doc.go`, a typed `Metadata` struct and
   `Fetch(ctx context.Context, opts ...imds.Option) (Metadata, error)`.
   Return plain Go values: no `attribute.KeyValue`, no semconv, no OTel. The
   `doc.go` states how many requests a lookup costs, and names what the service
   serves that this provider deliberately does not read.
5. Wire `imdstest.Conformance` into the package's tests.
6. Add the row to the provider table in the README.
7. Run `make precommit`.

### Clouds already assessed

Answered once so the question does not get reopened from a list of cloud names.

- **Covered without a package of their own.** OVHcloud Public Cloud and STACKIT
  are OpenStack; `providers/openstack/nova` reads them. Adding a package per
  deployment would be the same code under a different name.
- **Joyent Triton: no.** Its metadata API runs over the second serial port for
  hardware-virtualised guests and a unix socket for zones, and Triton's own
  documentation says it does not rely on networking. It is not IMDS over HTTP.
- **IONOS: no.** No link-local metadata service is documented, and there is no
  cloud-init datasource for it. Show one and this changes.
- **A service-discovery integration is not evidence of a metadata service.**
  Prometheus's `discovery/` packages call management APIs with credentials, from
  somewhere else, to enumerate other machines. That is the opposite direction
  from this module. Check for a documented link-local endpoint instead.

### Before reaching for a vendor SDK

A vendor metadata client may only be used when **all** of these hold. Record
the assessment in the pull request.

- **(a)** It accepts an injected `*http.Client` and does not mutate it.
- **(b)** Every call takes a `context.Context` and honours it, with no internal
  wall-clock timer standing in for the caller's deadline.
- **(c)** Nothing it starts outlives the call.
- **(d)** It passes `imdstest.Conformance` unmodified.
- **(e)** The protocol has real complexity or user-visible conventions worth
  inheriting, rather than being one GET returning JSON.

No SDK assessed so far passes, and the reasons are recorded so the next one is
judged rather than assumed:

- `hetznercloud/hcloud-go` fails (a) and (b): its metadata constructor assigns
  to the `Timeout` field of the client you give it.
- `cloud.google.com/go/compute/metadata` fails (d): it reads response bodies
  with `io.ReadAll` and no limit, so it cannot pass the oversized-body case. It
  also cannot check the `Metadata-Flavor` response header, because `Get`
  returns only the body.
- `aws-sdk-go-v2/feature/ec2/imds` fails (d) for the same reason, in both its
  token exchange and its reads.

If you find one that passes all five, say so in the pull request with the
evidence, and change the `depguard` rule in the same change.

## Commits and releases

Commit subjects follow [Conventional Commits](https://www.conventionalcommits.org),
because release-please derives versions and changelog entries from them. Pull
requests are squash-merged, so the **pull request title** is what ends up in the
history and is checked by CI.

```
feat(scaleway): add the Scaleway Instances provider
fix(imds): bound the token exchange by the caller's deadline
```

Every commit needs a `Signed-off-by` trailer (`git commit -s`).

`CHANGELOG.md` and `.release-please-manifest.json` are machine-managed. Editing
them by hand conflicts with the release pull request.

The library is one module and one tag. `examples/` and `internal/tools` are
separate modules and are never released: the first so that depending on go-imds
never pulls OpenTelemetry into your build, the second so tool versions stay out
of the library's graph entirely.

## Disclosing AI assistance

If a significant part of a commit came from an AI tool, say so with an
`Assisted-by:` trailer:

```
Assisted-by: Claude Opus 5
```

Do not use `Co-authored-by:` for this. It attributes authorship to an account
that did not write the code, and CI rejects it.
