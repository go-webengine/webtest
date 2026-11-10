// Copyright (c) the go-webengine/webtest authors.
// SPDX-License-Identifier: BSD-3-Clause

package driver

import (
	"context"
	"image"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-webengine/engine"
	"github.com/go-webengine/webtest/hclscenario"
)

func openFixture(t *testing.T, e *engine.Engine, url string) *engine.LiveDocument {
	t.Helper()
	live, err := e.Open(context.Background(), url, image.Rect(0, 0, 400, 400))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(live.Close)
	return live
}

func parseScenario(t *testing.T, src string) *hclscenario.Scenario {
	t.Helper()
	sc, diags := hclscenario.Parse([]byte(src), "test.hcl")
	if diags.HasErrors() {
		t.Fatalf("Parse: %v", diags)
	}
	return sc
}

func TestRunClickFillAssert(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body>
			<input id="name" value="">
			<button id="go"></button>
			<div id="welcome"></div>
			<script>
				document.getElementById('go').addEventListener('click', function() {
					document.getElementById('welcome').textContent = 'Hello, ' + document.getElementById('name').value;
				});
			</script>
		</body></html>`))
	}))
	defer srv.Close()

	src := `
scenario "s" {
  url = "` + srv.URL + `/"
  step "fill" {
    selector = "#name"
    value    = "alice"
  }
  step "click" {
    selector = "#go"
  }
  step "assert" {
    selector = "#welcome"
    text     = "Hello, alice"
  }
}
`
	live := openFixture(t, engine.New(), srv.URL+"/")
	_, res, err := Run(context.Background(), live, parseScenario(t, src), false)
	if err != nil {
		t.Fatalf("Run: %v (steps=%+v)", err, res.Steps)
	}
	if len(res.Steps) != 3 {
		t.Fatalf("len(Steps) = %d, want 3", len(res.Steps))
	}
	if res.Steps[2].AssertGot != "Hello, alice" {
		t.Fatalf("assert step AssertGot = %q", res.Steps[2].AssertGot)
	}
}

func TestRunFillReplacesExistingValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><input id="name" value="stale"></body></html>`))
	}))
	defer srv.Close()

	src := `
scenario "s" {
  url = "` + srv.URL + `/"
  step "fill" {
    selector = "#name"
    value    = "fresh"
  }
  step "assert" {
    selector = "#name"
    text     = "fresh"
  }
}
`
	live := openFixture(t, engine.New(), srv.URL+"/")
	_, res, err := Run(context.Background(), live, parseScenario(t, src), false)
	if err != nil {
		t.Fatalf("Run: %v (steps=%+v)", err, res.Steps)
	}
	if res.Steps[1].AssertGot != "fresh" {
		t.Fatalf("fill did not replace the stale value: got %q", res.Steps[1].AssertGot)
	}
}

func TestRunAssertSelectorNotFoundFailsTheRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body></body></html>`))
	}))
	defer srv.Close()

	src := `
scenario "s" {
  url = "` + srv.URL + `/"
  step "assert" {
    selector = "#nope"
  }
}
`
	live := openFixture(t, engine.New(), srv.URL+"/")
	_, res, err := Run(context.Background(), live, parseScenario(t, src), false)
	if err == nil {
		t.Fatal("Run with an assert on a missing selector: want an error")
	}
	if len(res.Steps) != 1 {
		t.Fatalf("len(Steps) = %d, want the one failed step recorded", len(res.Steps))
	}
}

func TestRunPinMismatchStopsBeforeAnyStep(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><div id="marker-attr"></div></body></html>`))
	}))
	defer srv.Close()

	src := `
scenario "s" {
  url = "` + srv.URL + `/"
  pin {
    selectors = ["#does-not-exist"]
  }
  step "assert" {
    selector = "#marker-attr"
  }
}
`
	live := openFixture(t, engine.New(), srv.URL+"/")
	_, res, err := Run(context.Background(), live, parseScenario(t, src), false)
	if err == nil {
		t.Fatal("Run with a pin selector that doesn't match: want an error")
	}
	if len(res.Steps) != 0 {
		t.Fatalf("len(Steps) = %d, want 0 (pin must be checked before any step runs)", len(res.Steps))
	}
}

func TestRunPinURLMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body></body></html>`))
	}))
	defer srv.Close()

	src := `
scenario "s" {
  url = "` + srv.URL + `/"
  pin {
    url       = "https://not-the-real-url.test/"
    selectors = ["body"]
  }
}
`
	live := openFixture(t, engine.New(), srv.URL+"/")
	_, _, err := Run(context.Background(), live, parseScenario(t, src), false)
	if err == nil {
		t.Fatal("Run with a pin url that doesn't match the real page: want an error")
	}
}

func TestRunSubmitDryRunDoesNotTouchNetwork(t *testing.T) {
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
	live := openFixture(t, engine.New(), srv.URL+"/")
	_, res, err := Run(context.Background(), live, parseScenario(t, src), false) // doSubmit = false
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if posted {
		t.Fatal("submit step with doSubmit=false must never touch the network")
	}
	sr := res.Steps[0]
	if sr.SubmitMethod != http.MethodPost || sr.SubmitTarget != srv.URL+"/login" || sr.SubmitBody != "x=1" {
		t.Fatalf("preview = %+v", sr)
	}
}

func TestRunSubmitRealRequestNavigates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><title>Done</title></head><body>ok</body></html>`))
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
	live := openFixture(t, engine.New(), srv.URL+"/")
	final, res, err := Run(context.Background(), live, parseScenario(t, src), true) // doSubmit = true
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	t.Cleanup(final.Close)
	if !res.Steps[0].Navigated {
		t.Fatal("submit step with doSubmit=true: want Navigated=true")
	}
	if final.Document().Title != "Done" {
		t.Fatalf("final document title = %q, want Done (the real navigation's own response)", final.Document().Title)
	}
}
