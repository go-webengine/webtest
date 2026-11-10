// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package hclscenario

import "testing"

const validScenario = `
scenario "login" {
  url = "https://example.com/login"

  pin {
    selectors = ["#login-form", "#username"]
  }

  step "fill" {
    selector = "#username"
    value    = upper("alice")
  }
  step "fill" {
    selector = "#password"
    value    = "s3cr3t"
  }
  step "click" {
    selector = "#submit"
  }
  step "assert" {
    selector = "#welcome"
    text     = "Welcome, ALICE"
  }
  step "submit" {
    selector = "#login-form"
  }
}
`

func TestParseValidScenario(t *testing.T) {
	sc, diags := Parse([]byte(validScenario), "test.hcl")
	if diags.HasErrors() {
		t.Fatalf("Parse: %v", diags)
	}
	if sc.Name != "login" {
		t.Fatalf("Name = %q, want login", sc.Name)
	}
	if sc.URL != "https://example.com/login" {
		t.Fatalf("URL = %q", sc.URL)
	}
	if sc.Pin == nil || len(sc.Pin.Selectors) != 2 {
		t.Fatalf("Pin = %+v, want 2 selectors", sc.Pin)
	}
	if len(sc.Steps) != 5 {
		t.Fatalf("len(Steps) = %d, want 5", len(sc.Steps))
	}
	if sc.Steps[0].Kind != StepFill || sc.Steps[0].Value != "ALICE" {
		t.Fatalf("Steps[0] = %+v, want fill value=ALICE (via upper())", sc.Steps[0])
	}
	if sc.Steps[3].Kind != StepAssert || !sc.Steps[3].HasText || sc.Steps[3].Text != "Welcome, ALICE" {
		t.Fatalf("Steps[3] = %+v", sc.Steps[3])
	}
	if sc.Steps[4].Kind != StepSubmit || sc.Steps[4].Selector != "#login-form" {
		t.Fatalf("Steps[4] = %+v", sc.Steps[4])
	}
}

func TestParseAssertWithoutText(t *testing.T) {
	src := `
scenario "s" {
  url = "https://x/"
  step "assert" {
    selector = "#ok"
  }
}
`
	sc, diags := Parse([]byte(src), "test.hcl")
	if diags.HasErrors() {
		t.Fatalf("Parse: %v", diags)
	}
	if sc.Steps[0].HasText {
		t.Fatalf("HasText = true for an assert step with no text attribute, want false")
	}
}

func TestParseNoScenarioBlock(t *testing.T) {
	_, diags := Parse([]byte(`not_a_scenario "x" {}`), "test.hcl")
	if !diags.HasErrors() {
		t.Fatal("Parse with no scenario block: want an error")
	}
}

func TestParseTwoScenarioBlocks(t *testing.T) {
	src := `
scenario "a" {
  url = "https://x/"
}
scenario "b" {
  url = "https://y/"
}
`
	_, diags := Parse([]byte(src), "test.hcl")
	if !diags.HasErrors() {
		t.Fatal("Parse with two scenario blocks: want an error (exactly one is required)")
	}
}

func TestParseMissingRequiredURL(t *testing.T) {
	_, diags := Parse([]byte(`scenario "s" { }`), "test.hcl")
	if !diags.HasErrors() {
		t.Fatal("Parse with no url attribute: want an error")
	}
}

func TestParseUnrecognizedStepKind(t *testing.T) {
	src := `
scenario "s" {
  url = "https://x/"
  step "teleport" {
    selector = "#a"
  }
}
`
	_, diags := Parse([]byte(src), "test.hcl")
	if !diags.HasErrors() {
		t.Fatal(`Parse with step "teleport": want an error`)
	}
}

func TestParseUnregisteredFunctionCall(t *testing.T) {
	src := `
scenario "s" {
  url = "https://x/"
  step "fill" {
    selector = "#a"
    value    = reverse("x")
  }
}
`
	_, diags := Parse([]byte(src), "test.hcl")
	if !diags.HasErrors() {
		t.Fatal(`Parse calling an unregistered function "reverse": want an error, not a silent null`)
	}
}

func TestParseFillMissingValue(t *testing.T) {
	src := `
scenario "s" {
  url = "https://x/"
  step "fill" {
    selector = "#a"
  }
}
`
	_, diags := Parse([]byte(src), "test.hcl")
	if !diags.HasErrors() {
		t.Fatal(`Parse fill step with no value: want an error`)
	}
}

func TestParsePinNoSelectors(t *testing.T) {
	src := `
scenario "s" {
  url = "https://x/"
  pin {}
}
`
	_, diags := Parse([]byte(src), "test.hcl")
	if !diags.HasErrors() {
		t.Fatal("Parse pin block with no selectors attribute: want an error")
	}
}

func TestParseEmptySource(t *testing.T) {
	_, diags := Parse([]byte(``), "test.hcl")
	if !diags.HasErrors() {
		t.Fatal("Parse of an empty file: want an error (no scenario block)")
	}
}

func TestParseSyntaxError(t *testing.T) {
	_, diags := Parse([]byte(`scenario "s" { url = `), "test.hcl")
	if !diags.HasErrors() {
		t.Fatal("Parse of truncated/invalid syntax: want an error")
	}
}
