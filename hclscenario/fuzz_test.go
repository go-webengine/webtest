// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package hclscenario

import "testing"

// FuzzParseScenario feeds arbitrary bytes as scenario source through Parse
// — untrusted input (a scenario file is, in practice, handed around and
// edited by whoever runs these tests), exactly the kind of parser/
// validator CLAUDE.md's own "fuzz every parser/decoder" rule targets, and
// specifically what go-wasmcontainers asked for once HCL was chosen (the
// function allowlist is the attack surface; this is the OTHER half, the
// parser/validator itself never panicking on malformed input). The only
// invariant held here is "never panics" — a malformed scenario is expected
// to be a clean, non-empty hcl.Diagnostics, not success.
func FuzzParseScenario(f *testing.F) {
	seeds := []string{
		validScenario,
		``,
		`scenario`,
		`scenario "s" { url = "https://x/" step "fill" { selector = "#a" value = reverse("x") } }`,
		`scenario "s" { url = ((((((((((1))))))))))`,
		`scenario "s" "t" { url = "x" }`,
		`scenario "s" { url = "x" pin { selectors = [1, 2, 3] } }`,
		`scenario "s" { url = "x" step "fill" { selector = "#a" value = join(",", "a", "b") } }`,
		string([]byte{0, 1, 2, 0xff, 0xfe}),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		_, _ = Parse([]byte(src), "fuzz.hcl")
	})
}
