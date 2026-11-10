// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendAuditLogCreatesAndAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	e1 := auditEntry{Time: time.Now(), URL: "https://x/a", Method: "POST", Body: "a=1", DryRun: true}
	e2 := auditEntry{Time: time.Now(), URL: "https://x/b", Method: "GET", DryRun: false}
	if err := appendAuditLog(path, e1); err != nil {
		t.Fatalf("append 1: %v", err)
	}
	if err := appendAuditLog(path, e2); err != nil {
		t.Fatalf("append 2: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("audit log mode = %#o, want 0600", mode)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if len(lines) != 2 {
		t.Fatalf("audit log has %d lines, want 2", len(lines))
	}
	var got auditEntry
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("line 1 is not valid JSON: %v", err)
	}
	if got.URL != "https://x/a" || !got.DryRun {
		t.Fatalf("line 1 = %+v", got)
	}
}

func TestAppendAuditLogRotatesWhenTooLarge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	big := strings.Repeat("x", maxAuditLogBytes+1)
	if err := os.WriteFile(path, []byte(big), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := appendAuditLog(path, auditEntry{URL: "https://x/after-rotation"}); err != nil {
		t.Fatalf("append after exceeding the size bound: %v", err)
	}
	rotated := path + ".1"
	if _, err := os.Stat(rotated); err != nil {
		t.Fatalf("rotated file %s: %v, want it to exist", rotated, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "after-rotation") {
		t.Fatalf("new log file = %q, want it to contain the post-rotation entry, not the old giant content", data)
	}
	if len(data) > len(big) {
		t.Fatalf("new log file is %d bytes, want it SMALLER than the pre-rotation file (%d) — rotation must have moved the old content out", len(data), len(big))
	}
}
