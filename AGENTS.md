# AGENTS.md

Instructions for AI agents working in this repository. Read this file and
[CONTRIBUTING.md](CONTRIBUTING.md) before starting, and read the contract
section of [README.md](README.md) before touching anything under `imds/` or
`providers/`.

## What this module is

A client for cloud instance metadata services whose product is *the request
boundary*, not the JSON decoding. The per-provider payloads are trivial. The
value is that five rules hold identically everywhere, which is the thing that
every hand-rolled metadata client in the ecosystem gets wrong.

Keep that in mind when judging a change: a diff that makes a provider shorter
but moves the request boundary out of `imds` is a bad trade.

## Invariants you may not weaken

These are not style preferences.

- **Never** use `http.DefaultClient`, `http.DefaultTransport`, `http.Get`,
  `http.Head` or `http.Post`. They use `ProxyFromEnvironment`. The linter
  rejects them; do not add a `//nolint` to get past it.
- **Never** use `http.Client.Timeout` as a substitute for a context deadline.
  It is not cancellable by the caller and it restarts per request.
- **Never** let a goroutine, request or connection outlive a `Fetch`. If an
  upstream API cannot be cancelled, that is a reason not to use it, not a
  reason to wrap it in a goroutine.
- **Never** report a metadata service failure as "not running on this cloud".
  Absent, foreign and failed are three different outcomes.
- **Never** add a skip, exemption or special case to `imdstest.Conformance`. If
  a provider cannot pass it, fix the provider.

If a task appears to require weakening one of these, stop and ask rather than
finding a way around it.

## Workflow

1. Read the package you are changing and its tests first. Match the existing
   naming, option types, error handling and comment style rather than
   introducing a new pattern.
2. Add the failing test first, as a conformance case where one fits.
3. Make the smallest change that passes it. Avoid drive-by cleanup in the same
   diff.
4. Run `make precommit`. Do not report work as done without it.

For a new provider, follow the checklist in
[CONTRIBUTING.md](CONTRIBUTING.md#adding-a-provider). The first step matters
most: do not invent a payload. Prefer a capture from a real instance, then the
vendor's own SDK type, then the vendor's published schema, and say in the test
which of those it was. A fixture whose provenance is unrecorded cannot be
checked later, and vendor prose is frequently wrong about what a service
actually serves -- prefer a type or a capture over a description of one.

## Vendor SDKs

Before adding any cloud vendor dependency, work through criteria (a) to (e) in
[CONTRIBUTING.md](CONTRIBUTING.md#before-reaching-for-a-vendor-sdk) and put the
verdict in the pull request. Every SDK assessed so far fails, most of them
because they read response bodies with no limit. `depguard` rejects the import
regardless; the point is to reason about it rather than route around the
linter.

## Dependencies

The module has one runtime dependency, a YAML parser used by `providers/hetzner`
alone, and that is a property worth keeping. `testify` and `goleak` are used
only by tests and `internal/imdstest`. If a change appears to need another
runtime dependency, raise it rather than adding one.

## Commits

- Conventional-commit subjects, since release-please parses them. Pull requests
  are squash-merged, so the PR title is what matters and CI checks it.
- Sign off every commit (`git commit -s`).
- Disclose AI assistance with an `Assisted-by:` trailer, never
  `Co-authored-by:`. CI rejects the latter.
- **Do not commit, tag or push.** Write the files, run `make precommit`, report
  what changed, and leave the commit to a human.
- Do not hand-edit `CHANGELOG.md` or `.release-please-manifest.json`. They are
  machine-managed and manual edits conflict with the release pull request.

## Comments

Write comments for intent, invariants and non-obvious constraints. A comment
that restates the code is noise. Where a rule above is implemented, say *why*
in the code, so the next reader does not "simplify" it away -- the existing
comments on the transport and the deadline handling are the model.
