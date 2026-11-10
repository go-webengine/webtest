// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// runCmd drives RootCmd with the given args, capturing stdout and stderr.
func runCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	c := RootCmd()
	var out, errb bytes.Buffer
	c.SetArgs(args)
	c.SetOut(&out)
	c.SetErr(&errb)
	err = c.Execute()
	return out.String(), errb.String(), err
}

func writeScenario(t *testing.T, src string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scenario.hcl")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const validScenario = `
scenario "login" {
  url = "https://example.com/login"
  step "click" {
    selector = "#submit"
  }
}
`

func TestValidateValidScenario(t *testing.T) {
	path := writeScenario(t, validScenario)
	stdout, _, err := runCmd(t, "validate", path)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !bytes.Contains([]byte(stdout), []byte(`"login" is valid (1 step(s))`)) {
		t.Fatalf("stdout = %q, want it to report the scenario name and step count", stdout)
	}
}

func TestValidateInvalidScenario(t *testing.T) {
	path := writeScenario(t, `scenario "s" { }`) // missing required url
	_, stderr, err := runCmd(t, "validate", path)
	if err == nil {
		t.Fatal("validate of a scenario missing the required url attribute: want an error")
	}
	if stderr == "" {
		t.Fatal("validate of an invalid scenario: want diagnostics on stderr")
	}
}

func TestValidateMissingFile(t *testing.T) {
	_, _, err := runCmd(t, "validate", filepath.Join(t.TempDir(), "nope.hcl"))
	if err == nil {
		t.Fatal("validate of a nonexistent file: want an error")
	}
}

func TestValidateRequiresExactlyOneArg(t *testing.T) {
	if _, _, err := runCmd(t, "validate"); err == nil {
		t.Fatal("validate with no path argument: want an error")
	}
	if _, _, err := runCmd(t, "validate", "a.hcl", "b.hcl"); err == nil {
		t.Fatal("validate with two path arguments: want an error")
	}
}
