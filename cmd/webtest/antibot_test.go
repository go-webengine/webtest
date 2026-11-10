// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import "testing"

func TestDetectAntiBotFindsRecaptcha(t *testing.T) {
	marker, found := detectAntiBot("Verify you are human", `<div class="g-recaptcha"></div>`)
	if !found || marker != "recaptcha" {
		t.Fatalf("detectAntiBot = (%q, %v), want (recaptcha, true)", marker, found)
	}
}

func TestDetectAntiBotFindsCloudflareInterstitial(t *testing.T) {
	marker, found := detectAntiBot("Just a moment...", `<p>Checking your browser</p>`)
	if !found || marker != "just a moment" {
		t.Fatalf("detectAntiBot = (%q, %v), want (just a moment, true)", marker, found)
	}
}

func TestDetectAntiBotOrdinaryPage(t *testing.T) {
	if marker, found := detectAntiBot("Sign in", `<form><input name="email"></form>`); found {
		t.Fatalf("detectAntiBot on an ordinary login page = (%q, true), want false", marker)
	}
}
