// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"context"
	"fmt"
	"image"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-webengine/engine"
	"github.com/go-webengine/webtest/driver"
	"github.com/go-webengine/webtest/hclscenario"
	"github.com/spf13/cobra"
)

const defaultMaxSteps = 200

func newRunCmd() *cobra.Command {
	var (
		submit     bool
		timeout    time.Duration
		maxSteps   int
		auditPath  string
		cookiePath string
	)
	cmd := &cobra.Command{
		Use:   "run <scenario.hcl>",
		Short: "Render the scenario's page and drive it (dry-run by default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScenario(cmd, runOptions{
				path: args[0], submit: submit, timeout: timeout,
				maxSteps: maxSteps, auditPath: auditPath, cookiePath: cookiePath,
			})
		},
	}
	cmd.Flags().BoolVar(&submit, "submit", false,
		"Actually perform submit steps for real (default: a dry-run preview only — method/URL/body, no request)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Overall time budget for the render and every step")
	cmd.Flags().IntVar(&maxSteps, "max-steps", defaultMaxSteps, "Refuse a scenario with more than this many steps")
	cmd.Flags().StringVar(&auditPath, "audit-log", "", "Path to the append-only audit log (default: a file under the user config dir)")
	cmd.Flags().StringVar(&cookiePath, "cookie-file", "", "Path to the persisted cookie jar (default: a file under the user config dir)")
	return cmd
}

type runOptions struct {
	path                  string
	submit                bool
	timeout               time.Duration
	maxSteps              int
	auditPath, cookiePath string
}

func runScenario(cmd *cobra.Command, opt runOptions) error {
	src, err := os.ReadFile(opt.path)
	if err != nil {
		return fmt.Errorf("read %s: %w", opt.path, err)
	}
	sc, diags := hclscenario.Parse(src, opt.path)
	if diags.HasErrors() {
		fmt.Fprint(cmd.ErrOrStderr(), diags)
		return fmt.Errorf("%s: invalid scenario", opt.path)
	}
	if len(sc.Steps) > opt.maxSteps {
		return fmt.Errorf("%s: %d steps exceeds --max-steps=%d", opt.path, len(sc.Steps), opt.maxSteps)
	}

	cookiePath := opt.cookiePath
	if cookiePath == "" {
		if cookiePath, err = defaultCookiePath(); err != nil {
			return fmt.Errorf("resolve default cookie path: %w", err)
		}
	}
	auditPath := opt.auditPath
	if auditPath == "" {
		if auditPath, err = defaultAuditPath(); err != nil {
			return fmt.Errorf("resolve default audit log path: %w", err)
		}
	}

	jar, err := loadCookieJar(cookiePath)
	if err != nil {
		return fmt.Errorf("cookie file: %w", err)
	}

	e := engine.New()
	e.Client = &http.Client{Jar: jar}

	ctx, cancel := context.WithTimeout(cmd.Context(), opt.timeout)
	defer cancel()

	live, err := e.Open(ctx, sc.URL, image.Rect(0, 0, 1280, 1024))
	if err != nil {
		return fmt.Errorf("open %s: %w", sc.URL, err)
	}
	defer func() {
		if live != nil {
			live.Close()
		}
	}()

	hosts := map[string]bool{}
	addHost := func(rawurl string) {
		if u, err := url.Parse(rawurl); err == nil && u.Host != "" {
			hosts[u.Host] = true
		}
	}
	addHost(sc.URL)

	doc := live.Document()
	if marker, found := detectAntiBot(doc.Title, doc.HTML); found {
		return fmt.Errorf("refusing to proceed: anti-bot challenge detected (%q) on %s — never evaded, only reported", marker, doc.URL)
	}

	final, result, runErr := driver.Run(ctx, live, sc, opt.submit)
	live = final
	addHost(final.Document().URL)

	var hostList []string
	for h := range hosts {
		hostList = append(hostList, h)
	}
	if err := saveCookieJar(cookiePath, jar, hostList); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not save cookie jar to %s: %v\n", cookiePath, err)
	}

	for _, sr := range result.Steps {
		if sr.Step.Kind != hclscenario.StepSubmit {
			continue
		}
		entry := auditEntry{
			Time: time.Now(), URL: sr.SubmitTarget, Method: sr.SubmitMethod,
			Body: sr.SubmitBody, DryRun: !opt.submit,
		}
		if err := appendAuditLog(auditPath, entry); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not write audit log to %s: %v\n", auditPath, err)
		}
	}

	if runErr != nil {
		return runErr
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: scenario %q completed (%d step(s))\n", opt.path, sc.Name, len(result.Steps))
	return nil
}
