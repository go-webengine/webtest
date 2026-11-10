// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCookieJarMissingFileIsEmptyNotError(t *testing.T) {
	jar, err := loadCookieJar(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("loadCookieJar of a missing file: %v, want a fresh empty jar", err)
	}
	if got := jar.Cookies(&url.URL{Scheme: "https", Host: "example.test"}); len(got) != 0 {
		t.Fatalf("fresh jar already has cookies: %v", got)
	}
}

func TestLoadCookieJarRefusesWrongPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.json")
	if err := os.WriteFile(path, []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCookieJar(path); err == nil {
		t.Fatal("loadCookieJar of a 0644 (group/world-readable) cookie file: want a refusal, not a silent read")
	}
}

func TestSaveAndLoadCookieJarRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.json")
	jar, err := loadCookieJar(filepath.Join(t.TempDir(), "nope.json")) // fresh empty jar
	if err != nil {
		t.Fatal(err)
	}
	u := &url.URL{Scheme: "https", Host: "example.test"}
	jar.SetCookies(u, []*http.Cookie{{Name: "session", Value: "abc123", Path: "/"}})

	if err := saveCookieJar(path, jar, []string{"example.test"}); err != nil {
		t.Fatalf("saveCookieJar: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("saved cookie file mode = %#o, want 0600", mode)
	}

	reloaded, err := loadCookieJar(path)
	if err != nil {
		t.Fatalf("loadCookieJar after save: %v", err)
	}
	got := reloaded.Cookies(u)
	if len(got) != 1 || got[0].Name != "session" || got[0].Value != "abc123" {
		t.Fatalf("reloaded cookies = %v, want [session=abc123]", got)
	}
}

func TestSaveCookieJarOnlyNamedHosts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookies.json")
	jar, _ := loadCookieJar(filepath.Join(t.TempDir(), "nope.json"))
	jar.SetCookies(&url.URL{Scheme: "https", Host: "a.test"}, []*http.Cookie{{Name: "x", Value: "1", Path: "/"}})
	jar.SetCookies(&url.URL{Scheme: "https", Host: "b.test"}, []*http.Cookie{{Name: "y", Value: "2", Path: "/"}})

	if err := saveCookieJar(path, jar, []string{"a.test"}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := loadCookieJar(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Cookies(&url.URL{Scheme: "https", Host: "a.test"}); len(got) != 1 {
		t.Fatalf("a.test cookies = %v, want 1", got)
	}
	if got := reloaded.Cookies(&url.URL{Scheme: "https", Host: "b.test"}); len(got) != 0 {
		t.Fatalf("b.test cookies = %v, want 0 (not in the named hosts list)", got)
	}
}
