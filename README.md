# webtest

HCL-driven browser-automation CLI for [go-webengine/engine](https://github.com/go-webengine/engine): click/fill-form/assert test scenarios, for driving a rendered page's own `LiveDocument` (`Focus`/`Type`/`Click`/`Submit`) from a declarative scenario file instead of hand-written Go.

Status: in progress. This repository currently holds only the scenario
format (`hclscenario`) — parsing, validation, and the function allowlist.
The driver/CLI wiring and the dry-run/session/pin/audit-log/anti-bot
guardrails are tracked separately and not implemented yet.

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
  default; only the CLI's own `--submit` flag (once the CLI exists) makes
  it a real request.
- Every attribute value is an HCL expression and may call a function from
  a small, fixed, reviewed allowlist (`hclscenario.Functions`) — currently
  `upper`/`lower`/`trim`/`join`. The allowlist is deliberately NOT cty's
  own stdlib: no file/exec/network/env functions, nothing unbounded. See
  `hclscenario/functions.go`'s own doc comment for the full rationale.
