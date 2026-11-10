// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package hclscenario parses and validates an HCL scenario file: a
// declarative sequence of browser-automation steps (click/fill/assert/
// submit) for go-webengine/engine's LiveDocument driver.
//
// HCL was chosen specifically for its native function-call expressions — an
// escape valve for logic beyond pure declarative description, without going
// all the way to embedding real Go/script code in a scenario file. That
// escape valve is also the real attack surface a scenario author's own
// expressions can reach: Functions (this file) is therefore a small, FIXED,
// reviewed allowlist, not cty's own stdlib wholesale. Every function here is
// pure (same output for the same input, no I/O, no observable side effect —
// cty's own function.Spec.Impl doc comment requires this), and explicitly
// excludes anything that could read a file, run a process, touch the
// network, or read an environment variable. None of them can make a
// scenario's own evaluation cost unbounded either: every one operates on
// its own arguments only, with no recursion or looping construct of its own
// (HCL itself gives a scenario author no way to DEFINE a new function, only
// call one of these, so the whole risk lives in exactly this list).
package hclscenario

import (
	"strings"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// Functions is the function allowlist passed to every scenario's own
// hcl.EvalContext — the complete set a scenario's expressions may call.
func Functions() map[string]function.Function {
	return map[string]function.Function{
		"upper": unaryStringFn("upper", "Returns a copy of the given string with all letters converted to upper case.", strings.ToUpper),
		"lower": unaryStringFn("lower", "Returns a copy of the given string with all letters converted to lower case.", strings.ToLower),
		"trim":  unaryStringFn("trim", "Returns a copy of the given string with leading and trailing whitespace removed.", strings.TrimSpace),
		"join":  joinFn,
	}
}

// unaryStringFn builds a function.Function taking one string argument and
// returning one string, for the common case (upper/lower/trim above) — a
// single reviewed shape every new string helper reuses instead of each
// hand-rolling its own Spec.
func unaryStringFn(name, description string, impl func(string) string) function.Function {
	return function.New(&function.Spec{
		Description: description,
		Params: []function.Parameter{
			{Name: "s", Type: cty.String},
		},
		Type: function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal(impl(args[0].AsString())), nil
		},
	})
}

// joinFn is "join(separator, list_of_strings)" — the one function here
// taking more than a single scalar, needed to build a composite value (a
// full name, a delimited path) from parts a scenario's own attributes
// already carry as separate strings.
var joinFn = function.New(&function.Spec{
	Description: "Joins the given list of strings with the given separator.",
	Params: []function.Parameter{
		{Name: "separator", Type: cty.String},
	},
	VarParam: &function.Parameter{Name: "strings", Type: cty.String},
	Type:     function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
		sep := args[0].AsString()
		parts := make([]string, 0, len(args)-1)
		for _, a := range args[1:] {
			parts = append(parts, a.AsString())
		}
		return cty.StringVal(strings.Join(parts, sep)), nil
	},
})
