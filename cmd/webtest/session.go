// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// defaultCookiePath/defaultAuditPath are under the user's own config dir —
// never an argument, environment variable, or log line (the one directly
// asked for in go-wasmcontainers' own guardrail list): a cookie is
// effectively a credential, and an argument/env var is visible in a
// process listing or a shell history the way a file with the RIGHT
// permissions is not.
func defaultCookiePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "webtest", "cookies.json"), nil
}

func defaultAuditPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "webtest", "audit.log"), nil
}

// storedCookie is the cookie file's own on-disk shape — deliberately NOT
// *http.Cookie directly (whose JSON tags are not stable API and which
// carries fields, like Raw, this tool never needs to persist).
type storedCookie struct {
	Name    string    `json:"name"`
	Value   string    `json:"value"`
	Domain  string    `json:"domain"`
	Path    string    `json:"path"`
	Expires time.Time `json:"expires,omitempty"`
}

// loadCookieJar reads path (the persisted session from a PRIOR run — e.g.
// one that logged in — this tool never performs a login or writes to this
// file on its own BEHALF, only what a prior run's own real request
// actually received) into a fresh cookiejar.Jar. A missing file is not an
// error: a fresh jar (no session) is a legitimate starting state. Refuses
// to read a file whose permissions are not exactly 0600 — a
// group/world-readable cookie file is exactly the exposure this
// guardrail exists to prevent, so this tool treats it as corrupted
// rather than silently trusting it.
func loadCookieJar(path string) (*cookiejar.Jar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return jar, nil
	}
	if err != nil {
		return nil, err
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		return nil, fmt.Errorf("%s has mode %#o, want exactly 0600 — refusing to read a cookie file others may be able to read", path, mode)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var stored []storedCookie
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	byHost := map[string][]*http.Cookie{}
	for _, c := range stored {
		byHost[c.Domain] = append(byHost[c.Domain], &http.Cookie{
			Name: c.Name, Value: c.Value, Path: c.Path, Expires: c.Expires,
		})
	}
	for host, cookies := range byHost {
		jar.SetCookies(&url.URL{Scheme: "https", Host: host}, cookies)
	}
	return jar, nil
}

// saveCookieJar writes jar's CURRENT cookies for exactly the given hosts
// (the scenario's own starting host and whatever host it may have
// navigated to — cookiejar.Jar exposes no "list every cookie it holds"
// method, only Cookies(url) for one host at a time, so the caller must
// name which hosts to ask about) back to path, 0600. A host's entry fully
// replaces whatever was stored for it before (naturally dropping an
// expired or cleared cookie too — this reflects the jar's CURRENT state,
// not an ever-growing accumulation).
func saveCookieJar(path string, jar *cookiejar.Jar, hosts []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var out []storedCookie
	for _, host := range hosts {
		// "https" alone is enough: cookiejar stores by domain, not by
		// scheme — scheme only GATES a Secure-flagged cookie (hidden from
		// an http:// query), never partitions storage, so an "https"
		// query already returns every cookie (secure or not) a real
		// request to this host would carry. Querying "http" too double-
		// counted every non-secure cookie (confirmed directly: a live
		// smoke-test run against a plain-http test server produced two
		// identical entries for the one cookie it set).
		for _, c := range jar.Cookies(&url.URL{Scheme: "https", Host: host}) {
			out = append(out, storedCookie{Name: c.Name, Value: c.Value, Domain: host, Path: c.Path, Expires: c.Expires})
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
