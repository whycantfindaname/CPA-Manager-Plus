---
name: cpamp-manager-server
description: Go Manager Server layering, persistence, migration, and verification rules
paths:
  - apps/manager-server/**
---

# Manager Server Development

## Layer Ownership

Manager Server is a Go 1.24 application under `apps/manager-server`; it uses `modernc.org/sqlite` without CGO. Preserve this direction:

```text
model -> repository -> service -> controller -> router -> httpapi -> cmd/cpa-manager-plus
```

- Domain data belongs in `internal/model/`.
- SQLite and domain persistence belong in `internal/repository/<domain>/`.
- Business behavior belongs in `internal/service/<domain>/`.
- HTTP parsing and response mapping belong in `internal/http/controller/<domain>/`; controllers stay thin.
- Route registration belongs in `internal/http/router/router.go`.
- Background loops and bounded asynchronous work belong in `internal/worker/`.

`internal/service/codexinspection/weekly_estimate.go` with its focused test demonstrates service-owned calculation. `internal/repository/usageevent/` demonstrates splitting a large persistence domain by query responsibility. `internal/http/controller/usage/` demonstrates keeping HTTP concerns separate from service logic.

## Data And Runtime Constraints

- `usage_events` is authoritative input for derived usage data; rebuilds must not delete or rewrite it.
- Startup must expose the HTTP listener, management page, and core health endpoints without waiting for work proportional to stored rows.
- Large scans, backfills, rebuilds, index creation, FTS rebuilds, clears, and `VACUUM` run after listener startup or in explicit maintenance mode.
- Online migrations use bounded batches and short transactions, are idempotent and checkpointed, resume after interruption, log progress/failure/completion, and retain a correctness-preserving read fallback until derived data is complete.
- Only one Manager Server consumes a CPA usage queue because RESP pop is destructive. RESP connects directly to the CPA API port; an HTTP proxy is not a RESP transport.

The migration worker and repository tests under `internal/worker/*migration*_test.go` and `internal/repository/sqlite/*test.go` are the evidence base for migration changes.

## Errors, Security, And Generated Output

Return stable API error codes from the owning controller/service path and preserve frontend compatibility with `apps/web/src/services/api/usageService.ts`. Do not expose `fail_body`, raw provider payloads, admin keys, CPA Management Keys, SQLite files, or `data.key` through logs, APIs, fixtures, or exports.

Do not hand-edit `internal/httpapi/web/management.html`; packaging embeds the generated single-file frontend there.

## Verification

```bash
npm run manager-server:test
```

Add `cd apps/manager-server && go test -race ./...` for concurrency, worker, collector, or shared-state changes. A data-migration release also requires evidence for 100k+ rows, listener availability during rebuild, interruption, and restart/resume.

## Anti-patterns

- Do not place business calculations in controllers.
- Do not run unbounded data work before the listener starts.
- Do not use CGO-dependent SQLite behavior.
- Do not interpret a local test or build as proof that the running service was restarted or deployed.
