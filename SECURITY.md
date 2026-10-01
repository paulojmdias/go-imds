# Security policy

## Reporting a vulnerability

Report it privately through GitHub's
[private vulnerability reporting](https://github.com/paulojmdias/go-imds/security/advisories/new),
not in a public issue.

In scope is anything that breaks the [contract](README.md#the-contract) or the
credential rule, for example:

- a request reaching a proxy, or any address other than the configured one;
- a response body read without a bound;
- a goroutine, request or connection outliving a lookup;
- a provider requesting credentials or user data, or one leaking into a
  `Metadata` value or an error string.

## Supported versions

Fixes go to the latest release only. Until `v1.0.0`, upgrading may also take
an API change.
