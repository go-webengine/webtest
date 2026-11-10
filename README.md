# webtest

HCL-driven browser-automation CLI for [go-webengine/engine](https://github.com/go-webengine/engine): click/fill-form/assert test scenarios, for driving a rendered page's own `LiveDocument` (`Focus`/`Type`/`Click`/`Submit`) from a declarative scenario file instead of hand-written Go.

## Usage

```sh
webtest validate scenario.hcl        # parse + schema-check only, no render
webtest run scenario.hcl             # render + drive; every submit step is a DRY RUN (no request)
webtest run scenario.hcl --submit    # submit steps perform the real request
```

`run`'s guardrails:

- **Dry-run by default.** A `submit` step always computes and reports
  what it WOULD send (method, URL, body); only `--submit` makes it real.
- **Pinned-page check**, built into `driver.Run` itself: before the first
  step runs, the rendered page's own URL and every `pin.selectors` entry
  must already match, or the run refuses with a named diff.
- **Session persistence.** Cookies the real HTTP client actually received
  (e.g. from a prior `run --submit` that logged in) are kept in a 0600
  file under the user's own config dir (`--cookie-file` to override) —
  never in an argument, environment variable, or log line. A file found
  with any other permission is treated as corrupted, not silently
  trusted.
- **Append-only audit log** (`--audit-log` to override), one JSON line
  per submit step — timestamp, URL, method, body, and whether it was a
  dry run — 0600, rotated past 10 MiB.
- **Bounds.** `--timeout` (default 30s) covers the whole render + every
  step; `--max-steps` (default 200) refuses an oversized scenario before
  rendering anything.
- **Anti-bot detection that stops, never evades**: a small, named set of
  markers for well-known CAPTCHA providers and Cloudflare's own
  interstitial — found on the page, `run` refuses outright rather than
  attempting to continue. A heuristic, not exhaustive; check a target
  site's own terms of service before automating it regardless.

The `run` subcommand does not perform a login on its own behalf — "the
user logs in themselves" (a scenario's own `submit` step against a real
login form, run once with `--submit`); the resulting session cookie is
then reused by later `run` invocations against the same host.

## Scenario format

HCL was chosen specifically for its native function-call expressions — an
escape valve for logic beyond pure declarative description, without going
all the way to embedding real Go/script code in a scenario file.

```hcl
scenario "login" {
  url = "https://example.com/login"

  pin {
    selectors = ["#login-form", "#username"]
  }

  step "fill" {
    selector = "#username"
    value    = "alice"
  }
  step "fill" {
    selector = "#password"
    value    = "s3cr3t"
  }
  step "click" {
    selector = "#submit"
  }
  step "assert" {
    selector = "#welcome"
    text     = "Welcome, alice"
  }
  step "submit" {
    selector = "#login-form"
  }
}
```

- `pin` is the pinned-page identity check: before the first step runs, the
  rendered page's URL (after redirects) and every selector here must be
  present, or the run refuses with a named diff rather than acting against
  an unexpected page.
- `step "submit"` always computes a dry-run preview (method, URL, body) by
  default; only `run`'s own `--submit` flag makes it a real request.
- Every attribute value is an HCL expression and may call a function from
  a small, fixed, reviewed allowlist (`hclscenario.Functions`) — currently
  `upper`/`lower`/`trim`/`join`. The allowlist is deliberately NOT cty's
  own stdlib: no file/exec/network/env functions, nothing unbounded. See
  `hclscenario/functions.go`'s own doc comment for the full rationale.
