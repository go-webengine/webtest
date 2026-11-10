// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package hclscenario

import (
	"testing"

	"github.com/zclconf/go-cty/cty"
)

func callString(t *testing.T, name string, args ...cty.Value) string {
	t.Helper()
	fn, ok := Functions()[name]
	if !ok {
		t.Fatalf("no such function: %s", name)
	}
	got, err := fn.Call(args)
	if err != nil {
		t.Fatalf("%s%v: %v", name, args, err)
	}
	return got.AsString()
}

func TestUpper(t *testing.T) {
	if got := callString(t, "upper", cty.StringVal("Alice")); got != "ALICE" {
		t.Fatalf("upper(Alice) = %q, want ALICE", got)
	}
}

func TestLower(t *testing.T) {
	if got := callString(t, "lower", cty.StringVal("Alice")); got != "alice" {
		t.Fatalf("lower(Alice) = %q, want alice", got)
	}
}

func TestTrim(t *testing.T) {
	if got := callString(t, "trim", cty.StringVal("  alice  ")); got != "alice" {
		t.Fatalf("trim = %q, want %q", got, "alice")
	}
}

func TestJoin(t *testing.T) {
	if got := callString(t, "join", cty.StringVal("-"), cty.StringVal("a"), cty.StringVal("b"), cty.StringVal("c")); got != "a-b-c" {
		t.Fatalf("join = %q, want %q", got, "a-b-c")
	}
}

func TestJoinNoParts(t *testing.T) {
	if got := callString(t, "join", cty.StringVal(",")); got != "" {
		t.Fatalf("join(\",\") with no parts = %q, want empty", got)
	}
}

// TestFunctionsAllowlistIsExactlyThis guards against silently growing the
// allowlist (e.g. a future contributor importing cty's own stdlib wholesale
// instead of adding one reviewed function at a time) — go-wasmcontainers'
// own security note: the function allowlist IS the attack surface, keep it
// small and reviewed.
func TestFunctionsAllowlistIsExactlyThis(t *testing.T) {
	want := map[string]bool{"upper": true, "lower": true, "trim": true, "join": true}
	got := Functions()
	if len(got) != len(want) {
		t.Fatalf("Functions() has %d entries, want exactly %d: %v", len(got), len(want), got)
	}
	for name := range got {
		if !want[name] {
			t.Fatalf("Functions() has unexpected entry %q — review before adding to the allowlist", name)
		}
	}
}
