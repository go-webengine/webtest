// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import "strings"

// antiBotMarkers is a small, named set of strings that show up on a real
// CAPTCHA/anti-bot challenge page from a well-known provider, or on
// Cloudflare's own interstitial — deliberately a HEURISTIC, not an
// exhaustive detector: go-wasmcontainers' own guardrail is "detect and
// STOP, never evade," so a false negative (a challenge this doesn't
// recognize) just means the scenario proceeds and then fails its own
// pin/assert steps normally — safe either way. A false POSITIVE (stopping
// on an ordinary page that merely mentions one of these words, e.g. in a
// footer) is the safer failure direction for a tool whose whole point is
// never to route around a real challenge.
var antiBotMarkers = []string{
	"recaptcha",
	"hcaptcha",
	"are you a robot",
	"just a moment",      // Cloudflare's own interstitial title
	"attention required", // Cloudflare's own block-page title
	"cf-challenge",
	"distil_r_captcha",
	"perimeterx",
	"datadome",
}

// detectAntiBot scans title+html (the rendered page's own <title> and raw
// decoded source) for any marker, case-insensitively, returning the first
// one found.
func detectAntiBot(title, html string) (marker string, found bool) {
	haystack := strings.ToLower(title + " " + html)
	for _, m := range antiBotMarkers {
		if strings.Contains(haystack, m) {
			return m, true
		}
	}
	return "", false
}
