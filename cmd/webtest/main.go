// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

// webtest drives a go-webengine/engine-rendered page through a declarative
// HCL scenario (click/fill/assert/submit), for browser-automation test
// suites. See the repository's own README for the scenario format.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// osExit allows tests to override os.Exit.
var osExit = os.Exit

func main() {
	if err := RootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		osExit(1)
	}
}

// RootCmd returns the top-level cobra command. Exported so tests in the
// same package can exercise the wiring without spawning a child process.
func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "webtest",
		Short:        "Drive a go-webengine/engine page through a declarative HCL scenario",
		SilenceUsage: true,
	}
	cmd.AddCommand(newValidateCmd())
	cmd.AddCommand(newRunCmd())
	return cmd
}
