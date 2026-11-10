// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package hclscenario

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

// evalString evaluates expr and requires the result to be a known,
// non-null string — never calls cty.Value.AsString on a value of any
// other type, which panics (confirmed directly: FuzzParseScenario's own
// first real run found this crash immediately, e.g. a scenario writing
// `url = 123`, a bare number, where a string was expected). A malformed
// scenario gets a clean diagnostic here, never a panic.
func evalString(expr hcl.Expression, ctx *hcl.EvalContext) (string, hcl.Diagnostics) {
	v, diags := expr.Value(ctx)
	if diags.HasErrors() {
		return "", diags
	}
	if !v.IsKnown() {
		return "", hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Value must be known",
			Detail:   "this expression must evaluate to a known value at parse time.",
			Subject:  expr.Range().Ptr(),
		}}
	}
	if v.IsNull() {
		return "", hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Value must not be null",
			Subject:  expr.Range().Ptr(),
		}}
	}
	if v.Type() != cty.String {
		return "", hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Value must be a string",
			Detail:   fmt.Sprintf("got %s.", v.Type().FriendlyName()),
			Subject:  expr.Range().Ptr(),
		}}
	}
	return v.AsString(), nil
}

// Scenario is one parsed, validated automation scenario — a single file's
// worth. The grammar:
//
//	scenario "name" {
//	  url = "https://example.com/login"
//
//	  pin {
//	    url       = "https://example.com/login" # optional, defaults to url above
//	    selectors = ["#login-form", "#username"]
//	  }
//
//	  step "fill"   { selector = "#username" value = "alice" }
//	  step "click"  { selector = "#submit" }
//	  step "assert" { selector = "#welcome" text = "Welcome, alice" }
//	  step "submit" { selector = "#login-form" }
//	}
//
// A scenario's expressions (any attribute value above) may call the
// functions in Functions — nothing else: no variables are defined, so a
// bare identifier other than a registered function name is a validation
// error, not a silent null.
type Scenario struct {
	Name  string
	URL   string
	Pin   *Pin
	Steps []Step
}

// Pin is the scenario's own pinned-page identity check: before the first
// step runs, the rendered page's URL (after redirects) and every selector
// here must be present, or the run refuses with a named diff rather than
// acting against an unexpected page. Nil if the scenario declares no pin
// block (a caller may still choose to require one).
type Pin struct {
	URL       string
	Selectors []string
}

// StepKind enumerates the only step types a scenario may declare.
type StepKind string

const (
	StepClick  StepKind = "click"
	StepFill   StepKind = "fill"
	StepAssert StepKind = "assert"
	StepSubmit StepKind = "submit"
)

// Step is one scenario step, in document order. Which fields are populated
// depends on Kind: Click/Submit use only Selector; Fill uses Selector and
// Value; Assert uses Selector and, optionally, Text (empty Text means
// "assert the selector matches at least one element", not "assert empty
// text" — a scenario wanting to assert genuinely empty text is not
// distinguishable from "no text check" in this version; documented, not
// silently wrong).
type Step struct {
	Kind     StepKind
	Selector string
	Value    string
	Text     string
	HasText  bool
	Range    hcl.Range
}

var fileSchema = &hcl.BodySchema{
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "scenario", LabelNames: []string{"name"}},
	},
}

var scenarioSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "url", Required: true},
	},
	Blocks: []hcl.BlockHeaderSchema{
		{Type: "pin"},
		{Type: "step", LabelNames: []string{"kind"}},
	},
}

var pinSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "url", Required: false},
		{Name: "selectors", Required: true},
	},
}

func stepSchema(kind StepKind) *hcl.BodySchema {
	switch kind {
	case StepClick, StepSubmit:
		return &hcl.BodySchema{Attributes: []hcl.AttributeSchema{{Name: "selector", Required: true}}}
	case StepFill:
		return &hcl.BodySchema{Attributes: []hcl.AttributeSchema{
			{Name: "selector", Required: true},
			{Name: "value", Required: true},
		}}
	case StepAssert:
		return &hcl.BodySchema{Attributes: []hcl.AttributeSchema{
			{Name: "selector", Required: true},
			{Name: "text", Required: false},
		}}
	default:
		return &hcl.BodySchema{}
	}
}

// Parse parses and fully validates src as a scenario file. Diagnostics is
// non-empty (HasErrors true) for anything from a syntax error to an
// unregistered function call to a step of an unrecognized kind — Parse
// never panics on malformed input (see FuzzParseScenario) and never
// guesses a missing/ambiguous value.
func Parse(src []byte, filename string) (*Scenario, hcl.Diagnostics) {
	f, diags := hclsyntax.ParseConfig(src, filename, hcl.InitialPos)
	if diags.HasErrors() {
		return nil, diags
	}

	fc, fdiags := f.Body.Content(fileSchema)
	diags = diags.Extend(fdiags)
	if diags.HasErrors() {
		return nil, diags
	}
	scenarios := fc.Blocks.OfType("scenario")
	if len(scenarios) != 1 {
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Exactly one scenario block is required",
			Detail:   fmt.Sprintf("found %d scenario blocks; a scenario file holds exactly one.", len(scenarios)),
			Subject:  &f.Body.(*hclsyntax.Body).SrcRange,
		})
		return nil, diags
	}
	sb := scenarios[0]

	sc := &Scenario{Name: sb.Labels[0]}
	ctx := &hcl.EvalContext{Functions: Functions()}

	content, cdiags := sb.Body.Content(scenarioSchema)
	diags = diags.Extend(cdiags)
	if diags.HasErrors() {
		return nil, diags
	}

	if attr, ok := content.Attributes["url"]; ok {
		v, vdiags := evalString(attr.Expr, ctx)
		diags = diags.Extend(vdiags)
		sc.URL = v
	}

	for _, b := range content.Blocks.OfType("pin") {
		pin, pdiags := parsePin(b, ctx)
		diags = diags.Extend(pdiags)
		sc.Pin = pin
	}
	if len(content.Blocks.OfType("pin")) > 1 {
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Only one pin block is allowed",
			Subject:  &content.Blocks.OfType("pin")[1].DefRange,
		})
	}

	for _, b := range content.Blocks.OfType("step") {
		step, sdiags := parseStep(b, ctx)
		diags = diags.Extend(sdiags)
		if !sdiags.HasErrors() {
			sc.Steps = append(sc.Steps, step)
		}
	}

	if diags.HasErrors() {
		return nil, diags
	}
	return sc, diags
}

func parsePin(b *hcl.Block, ctx *hcl.EvalContext) (*Pin, hcl.Diagnostics) {
	content, diags := b.Body.Content(pinSchema)
	if diags.HasErrors() {
		return nil, diags
	}
	pin := &Pin{}
	if attr, ok := content.Attributes["url"]; ok {
		v, vdiags := evalString(attr.Expr, ctx)
		diags = diags.Extend(vdiags)
		pin.URL = v
	}
	if attr, ok := content.Attributes["selectors"]; ok {
		v, vdiags := attr.Expr.Value(ctx)
		diags = diags.Extend(vdiags)
		if !vdiags.HasErrors() {
			if v.IsNull() || !v.IsKnown() || !v.CanIterateElements() {
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "selectors must be a list of strings",
					Subject:  attr.Expr.Range().Ptr(),
				})
			} else {
				for it := v.ElementIterator(); it.Next(); {
					_, ev := it.Element()
					if ev.IsNull() || !ev.IsKnown() || ev.Type() != cty.String {
						diags = diags.Append(&hcl.Diagnostic{
							Severity: hcl.DiagError,
							Summary:  "selectors must be a list of strings",
							Detail:   "every element must be a known, non-null string.",
							Subject:  attr.Expr.Range().Ptr(),
						})
						continue
					}
					pin.Selectors = append(pin.Selectors, ev.AsString())
				}
			}
		}
	}
	if len(pin.Selectors) == 0 && !diags.HasErrors() {
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "pin.selectors must have at least one selector",
			Subject:  &b.DefRange,
		})
	}
	return pin, diags
}

func parseStep(b *hcl.Block, ctx *hcl.EvalContext) (Step, hcl.Diagnostics) {
	kind := StepKind(b.Labels[0])
	step := Step{Kind: kind, Range: b.DefRange}
	switch kind {
	case StepClick, StepFill, StepAssert, StepSubmit:
	default:
		return step, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Unrecognized step kind",
			Detail:   fmt.Sprintf("step %q: must be one of click, fill, assert, submit.", kind),
			Subject:  &b.LabelRanges[0],
		}}
	}

	content, diags := b.Body.Content(stepSchema(kind))
	if diags.HasErrors() {
		return step, diags
	}

	str := func(name string) (string, bool) {
		attr, ok := content.Attributes[name]
		if !ok {
			return "", false
		}
		v, vdiags := evalString(attr.Expr, ctx)
		diags = diags.Extend(vdiags)
		if vdiags.HasErrors() {
			return "", false
		}
		return v, true
	}

	step.Selector, _ = str("selector")
	switch kind {
	case StepFill:
		step.Value, _ = str("value")
	case StepAssert:
		step.Text, step.HasText = str("text")
	}
	return step, diags
}
