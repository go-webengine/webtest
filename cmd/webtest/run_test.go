// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runEnv returns isolated --cookie-file/--audit-log paths under t.TempDir()
// so a test never touches the real user's config dir.
func runEnv(t *testing.T) (cookieFile, auditFile string) {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "cookies.json"), filepath.Join(dir, "audit.log")
}

func TestRunDryRunByDefaultNeverPosts(t *testing.T) {
	var posted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
		}
		w.Write([]byte(`<html><body><form id="f" method="POST" action="/login"><input name="x" value="1"></form></body></html>`))
	}))
	defer srv.Close()

	src := `
scenario "s" {
  url = "` + srv.URL + `/"
  step "submit" {
    selector = "#f"
  }
}
`
	cookieFile, auditFile := runEnv(t)
	path := writeScenario(t, src)
	stdout, _, err := runCmd(t, "run", path, "--cookie-file", cookieFile, "--audit-log", auditFile)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if posted {
		t.Fatal("run with no --submit must never perform a real request")
	}
	if !strings.Contains(stdout, "completed (1 step") {
		t.Fatalf("stdout = %q", stdout)
	}

	// The dry run must still be audited.
	data, err := os.ReadFile(auditFile)
	if err != nil {
		t.Fatalf("audit log: %v", err)
	}
	var entry auditEntry
	if err := json.Unmarshal([]byte(firstLine(t, data)), &entry); err != nil {
		t.Fatal(err)
	}
	if !entry.DryRun {
		t.Fatal("audit entry for a dry run must have DryRun=true")
	}
}

func TestRunSubmitFlagPerformsRealRequest(t *testing.T) {
	var posted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><body>ok</body></html>`))
			return
		}
		w.Write([]byte(`<html><body><form id="f" method="POST" action="/login"><input name="x" value="1"></form></body></html>`))
	}))
	defer srv.Close()

	src := `
scenario "s" {
  url = "` + srv.URL + `/"
  step "submit" {
    selector = "#f"
  }
}
`
	cookieFile, auditFile := runEnv(t)
	path := writeScenario(t, src)
	if _, _, err := runCmd(t, "run", path, "--submit", "--cookie-file", cookieFile, "--audit-log", auditFile); err != nil {
		t.Fatalf("run --submit: %v", err)
	}
	if !posted {
		t.Fatal("run --submit must perform the real request")
	}
}

func TestRunMaxStepsExceeded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><div id="a"></div></body></html>`))
	}))
	defer srv.Close()

	src := `
scenario "s" {
  url = "` + srv.URL + `/"
  step "assert" { selector = "#a" }
}
`
	cookieFile, auditFile := runEnv(t)
	path := writeScenario(t, src)
	_, _, err := runCmd(t, "run", path, "--max-steps", "0", "--cookie-file", cookieFile, "--audit-log", auditFile)
	if err == nil {
		t.Fatal("run with --max-steps=0 against a 1-step scenario: want a refusal")
	}
}

func TestRunRefusesAntiBotPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>Just a moment...</title></head><body></body></html>`))
	}))
	defer srv.Close()

	src := `
scenario "s" {
  url = "` + srv.URL + `/"
  step "assert" { selector = "body" }
}
`
	cookieFile, auditFile := runEnv(t)
	path := writeScenario(t, src)
	_, _, err := runCmd(t, "run", path, "--cookie-file", cookieFile, "--audit-log", auditFile)
	if err == nil {
		t.Fatal("run against a Cloudflare interstitial page: want a refusal, not proceeding")
	}
}

func TestRunPersistsCookiesAcrossInvocations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("session"); err == nil && c.Value == "logged-in" {
			w.Write([]byte(`<html><body><div id="whoami">authenticated</div></body></html>`))
			return
		}
		if r.Method == http.MethodPost {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "logged-in", Path: "/"})
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><body>welcome</body></html>`))
			return
		}
		w.Write([]byte(`<html><body><form id="f" method="POST" action="/login"><input name="x" value="1"></form></body></html>`))
	}))
	defer srv.Close()

	cookieFile, auditFile := runEnv(t)

	loginSrc := `
scenario "login" {
  url = "` + srv.URL + `/"
  step "submit" {
    selector = "#f"
  }
}
`
	if _, _, err := runCmd(t, "run", writeScenario(t, loginSrc), "--submit", "--cookie-file", cookieFile, "--audit-log", auditFile); err != nil {
		t.Fatalf("login run: %v", err)
	}

	checkSrc := `
scenario "check" {
  url = "` + srv.URL + `/"
  step "assert" {
    selector = "#whoami"
    text     = "authenticated"
  }
}
`
	if _, _, err := runCmd(t, "run", writeScenario(t, checkSrc), "--cookie-file", cookieFile, "--audit-log", auditFile); err != nil {
		t.Fatalf("second run did not see the persisted session cookie: %v", err)
	}
}

func firstLine(t *testing.T, data []byte) string {
	t.Helper()
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	if !sc.Scan() {
		t.Fatal("expected at least one line")
	}
	return sc.Text()
}
