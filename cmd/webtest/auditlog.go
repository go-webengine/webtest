// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// maxAuditLogBytes bounds the audit log file size: past this, the
// current file is rotated (renamed to a .1 suffix, overwriting any
// previous one) rather than left to grow without bound — this tool's
// own "every new capability needs its own bound" convention, inherited
// from go-webengine/engine's.
const maxAuditLogBytes = 10 << 20

// auditEntry is one line of the append-only audit log — go-wasmcontainers'
// own guardrail: "an append-only log of what was sent (time, URL, fields,
// response status)". DryRun is recorded explicitly so the log itself
// shows which entries were ever a REAL request, not just a preview.
type auditEntry struct {
	Time   time.Time `json:"time"`
	URL    string    `json:"url"`
	Method string    `json:"method"`
	Body   string    `json:"body,omitempty"`
	DryRun bool      `json:"dry_run"`
}

// appendAuditLog appends entry as one JSON line to path (0600, created if
// missing), rotating first if the file has already grown past
// maxAuditLogBytes.
func appendAuditLog(path string, entry auditEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if info, err := os.Stat(path); err == nil && info.Size() > maxAuditLogBytes {
		if err := os.Rename(path, path+".1"); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}
