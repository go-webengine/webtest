// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"os"

	"github.com/go-webengine/webtest/hclscenario"
	"github.com/spf13/cobra"
)

// newValidateCmd parses and schema-checks a scenario file WITHOUT
// rendering anything — the fast, no-network check a scenario author (or
// CI) runs before ever pointing it at a real page.
func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <scenario.hcl>",
		Short: "Parse and validate a scenario file (no render, no network)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(cmd, args[0])
		},
	}
	return cmd
}

func runValidate(cmd *cobra.Command, path string) error {
	src, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	sc, diags := hclscenario.Parse(src, path)
	if diags.HasErrors() {
		fmt.Fprint(cmd.ErrOrStderr(), diags)
		return fmt.Errorf("%s: invalid scenario", path)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: scenario %q is valid (%d step(s))\n", path, sc.Name, len(sc.Steps))
	return nil
}
