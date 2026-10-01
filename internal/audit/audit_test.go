// Copyright The go-imds Authors
// SPDX-License-Identifier: Apache-2.0

// Package audit holds repository-wide checks that no single package can make
// about itself.
package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbidden are path fragments a provider must never request.
//
// This is a source scan rather than a linter rule on purpose. These are string
// literals inside URL paths, not identifiers, so forbidigo cannot see them; and
// a blanket ban on "token" would be wrong, because the IMDSv2-style token
// exchange is legitimate and four providers depend on it.
var forbidden = []struct {
	fragment string
	why      string
}{
	{"security-credentials", "EC2 and Alibaba serve live credentials under this path"},
	{"service-accounts/", "GCP serves OAuth access tokens and identity documents under this path"},
	{"identity/oauth2/token", "Azure serves managed-identity access tokens here"},
	{"user-data", "user-data routinely carries secrets placed there by whoever launched the instance"},
	// Both spellings, because the JSON key differs by cloud: DigitalOcean uses
	// user_data and vendor_data, Vultr uses user-data and vendor-data.
	{"user_data", "user-data routinely carries secrets placed there by whoever launched the instance"},
	{"vendor-data", "vendor-data is supplied by the platform and is not descriptive metadata"},
	{"vendor_data", "Hetzner's vendor_data is a cloud-config blob including a random seed"},
	// Azure spells both in camelCase, and customData is its user-data by
	// another name.
	{"userData", "Azure's userData is user-supplied and routinely carries secrets"},
	{"customData", "Azure's customData is user-supplied and routinely carries secrets"},
	// OpenStack puts the generated administrator password in its metadata
	// document, and entropy alongside it.
	{"admin_pass", "OpenStack reports the generated administrator password here"},
	{"random_seed", "entropy seeded into the instance, not descriptive metadata"},
}

// TestProvidersRequestNothingSensitive fails if a provider names a path that
// returns credentials, or user-supplied data likely to contain them.
//
// Metadata structs get logged and turned into resource attributes, so
// everything in one has to be safe to print. Decoding into typed structs means
// an unexpected field in a bulk response is dropped rather than retained, but
// that is a second line of defence: the rule is not to ask for it in the first
// place, and this is what holds the line once nobody remembers why.
func TestProvidersRequestNothingSensitive(t *testing.T) {
	root := filepath.Join("..", "..", "providers")

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		src, err := os.ReadFile(path) //nolint:gosec // walking a fixed directory in the repository
		if err != nil {
			return err
		}
		// The exclusion is documented in each provider, and audit_test.go names
		// the fragments itself, so only non-comment lines are considered.
		for i, line := range strings.Split(string(src), "\n") {
			if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//") {
				continue
			}
			for _, f := range forbidden {
				if strings.Contains(line, f.fragment) {
					t.Errorf("%s:%d requests a forbidden path containing %q: %s",
						path, i+1, f.fragment, f.why)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
}
