// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package driver executes a parsed hclscenario.Scenario against a real
// go-webengine/engine LiveDocument. It adds NO new engine-level
// capability of its own — every step is a thin call into LiveDocument's
// already-existing, already-tested primitives (Click/Type/Backspace/
// Submit/PreviewSubmit/QuerySelector), confirmed directly before writing
// this package (see FIDELITY.md round 34 / PR #420 in go-webengine/engine).
//
// Cookie persistence, the CLI's own dry-run-by-default gating, the
// append-only audit log, resource bounds, and anti-bot detection are
// NOT this package's job — they are the `run` subcommand's own
// guardrails, layered on top of Run's result, not inside it.
package driver

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-webengine/engine"
	"github.com/go-webengine/engine/dom"
	"github.com/go-webengine/webtest/hclscenario"
)

// StepResult is one executed step's outcome.
type StepResult struct {
	Step hclscenario.Step

	// DefaultPrevented is set for a click or submit step: whether a page
	// script's own listener called preventDefault() on the synthetic
	// event — the same signal LiveDocument.Click/Submit already report.
	DefaultPrevented bool

	// AssertGot is the text/value assertStep actually found, for a failed
	// assert step's own error message and for a caller that wants to log
	// what was seen even on success.
	AssertGot string

	// SubmitMethod/SubmitTarget/SubmitBody are a submit step's own
	// PreviewSubmit computation — always populated for a submit step,
	// Submit or not (see Run's own doSubmit parameter).
	SubmitMethod, SubmitTarget, SubmitBody string
	// Navigated is set once Run actually called LiveDocument.Submit (only
	// when doSubmit is true) and it performed a real navigation.
	Navigated bool
}

// Result is a scenario run's own full record, in step order. A failed
// step stops the run (Run itself returns the error); Result still holds
// every step that DID complete, including the one that failed, so a
// caller can report partial progress.
type Result struct {
	Steps []StepResult
}

// Run executes sc's steps in order against live, in-place: fill clears
// and retypes a field's existing value (LiveDocument's own Backspace/Type,
// not a bulk assignment, so a page's own keystroke-driven listeners see
// it exactly as they would a real user's), click dispatches a real
// mousedown/mouseup/click, assert resolves a selector and optionally
// compares its text/value, and submit always computes a dry-run preview
// (PreviewSubmit) and additionally performs the real request only when
// doSubmit is true.
//
// If sc has a Pin, it is checked FIRST, before any step runs: the
// rendered page's own URL (when Pin.URL is set) and every Pin.Selector
// must already be present, or Run refuses with a named diff rather than
// acting against a page the scenario did not expect.
//
// Run returns the LiveDocument the caller should keep using afterward —
// a submit step that navigates replaces it (Submit's own contract: the
// previous one is Closed, matching a real browser navigation), so the
// ORIGINAL live argument must not be used again once Run returns; use
// the returned one instead, even on error (whichever document was
// current when the error occurred).
func Run(ctx context.Context, live *engine.LiveDocument, sc *hclscenario.Scenario, doSubmit bool) (*engine.LiveDocument, *Result, error) {
	if sc.Pin != nil {
		if err := checkPin(live, sc.Pin); err != nil {
			return live, &Result{}, err
		}
	}

	res := &Result{}
	for i, step := range sc.Steps {
		sr := StepResult{Step: step}
		var err error
		live, err = runStep(ctx, live, step, doSubmit, &sr)
		res.Steps = append(res.Steps, sr)
		if err != nil {
			return live, res, fmt.Errorf("step %d (%s %q): %w", i, step.Kind, step.Selector, err)
		}
	}
	return live, res, nil
}

// checkPin verifies pin against live's CURRENT state, returning a single
// error naming every mismatch found (not just the first) — a scenario
// author fixing a stale pin wants the whole diff, not one round trip per
// missing selector.
func checkPin(live *engine.LiveDocument, pin *hclscenario.Pin) error {
	var problems []string
	if pin.URL != "" {
		if got := live.Document().URL; got != pin.URL {
			problems = append(problems, fmt.Sprintf("page URL = %q, want %q", got, pin.URL))
		}
	}
	var missing []string
	for _, sel := range pin.Selectors {
		if _, ok := live.QuerySelector(sel); !ok {
			missing = append(missing, sel)
		}
	}
	if len(missing) > 0 {
		problems = append(problems, fmt.Sprintf("selector(s) not found: %s", strings.Join(missing, ", ")))
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("pin mismatch: %s", strings.Join(problems, "; "))
}

func runStep(ctx context.Context, live *engine.LiveDocument, step hclscenario.Step, doSubmit bool, sr *StepResult) (*engine.LiveDocument, error) {
	switch step.Kind {
	case hclscenario.StepClick:
		return clickStep(ctx, live, step, sr)
	case hclscenario.StepFill:
		return live, fillStep(ctx, live, step)
	case hclscenario.StepAssert:
		return live, assertStep(live, step, sr)
	case hclscenario.StepSubmit:
		return submitStep(ctx, live, step, doSubmit, sr)
	default:
		return live, fmt.Errorf("internal: unhandled step kind %q (hclscenario.Parse should have rejected this)", step.Kind)
	}
}

func resolveOne(live *engine.LiveDocument, selector string) (*dom.Node, error) {
	n, ok := live.QuerySelector(selector)
	if !ok {
		return nil, fmt.Errorf("selector %q: no matching element", selector)
	}
	return n, nil
}

func clickStep(ctx context.Context, live *engine.LiveDocument, step hclscenario.Step, sr *StepResult) (*engine.LiveDocument, error) {
	n, err := resolveOne(live, step.Selector)
	if err != nil {
		return live, err
	}
	prevented, _, _, err := live.Click(ctx, n)
	sr.DefaultPrevented = prevented
	return live, err
}

// fillStep clears n's CURRENT value via Backspace (one real keydown/
// input/keyup per character, same as a user deleting it) before Type-ing
// the new value — never a bulk value assignment, so a controlled
// React/Vue input sees exactly the sequence a real user clearing and
// retyping the field would produce.
func fillStep(ctx context.Context, live *engine.LiveDocument, step hclscenario.Step) error {
	n, err := resolveOne(live, step.Selector)
	if err != nil {
		return err
	}
	current := []rune(n.Attr["value"])
	for range current {
		if _, _, err := live.Backspace(ctx, n); err != nil {
			return err
		}
	}
	_, _, err = live.Type(ctx, n, step.Value)
	return err
}

// assertStep resolves step.Selector, failing if nothing matches. With no
// Text attribute (step.HasText false) that's the whole check — element
// presence. With Text set, compares against the element's own value (an
// input/textarea's reflected value, matching what a real user would see
// typed into it) or its text content for anything else (a message/label
// element's own displayed text).
func assertStep(live *engine.LiveDocument, step hclscenario.Step, sr *StepResult) error {
	n, err := resolveOne(live, step.Selector)
	if err != nil {
		return err
	}
	if !step.HasText {
		return nil
	}
	var got string
	switch n.Tag {
	case "input", "textarea":
		got = n.Attr["value"]
	default:
		got = dom.TextContent(n)
	}
	sr.AssertGot = got
	if got != step.Text {
		return fmt.Errorf("text = %q, want %q", got, step.Text)
	}
	return nil
}

// submitStep always computes PreviewSubmit's own method/target/body
// first — a submit step's dry-run record, logged regardless of doSubmit
// — and performs the real request (LiveDocument.Submit) only when
// doSubmit is true, swapping in the resulting LiveDocument on a real
// navigation.
func submitStep(ctx context.Context, live *engine.LiveDocument, step hclscenario.Step, doSubmit bool, sr *StepResult) (*engine.LiveDocument, error) {
	n, err := resolveOne(live, step.Selector)
	if err != nil {
		return live, err
	}
	sr.SubmitMethod, sr.SubmitTarget, sr.SubmitBody = live.PreviewSubmit(n)
	if !doSubmit {
		return live, nil
	}
	next, navigated, err := live.Submit(ctx, n)
	if err != nil {
		return live, err
	}
	sr.Navigated = navigated
	if navigated {
		return next, nil
	}
	return live, nil
}
