# Design: Credits weekly reset boundary pairing

## Problem Boundary

The Manager Server currently has two independent observations:

1. Codex Analytics provides Credits aggregated on closed natural-day
   boundaries in an Analytics timezone.
2. The official quota endpoint provides weekly used percentage and a reset
   timestamp that may occur at any time of day.

A reset transition proves that the quota window changed. It does not prove
that a seven-natural-day Credits bucket covers that quota window.

## Invariants

- Every estimate is scoped to one `auth_index` and `account_id`.
- A `cycle_complete` Credits estimate requires its interval start and end to
  match the previous weekly reset window, within the existing boundary
  tolerance.
- A reset-transition sample may identify the previous quota endpoint but may
  not bypass interval alignment.
- Closed-boundary rolling estimates keep their existing same-reset-window
  pairing rules.
- Persisted estimates are displayable only when their recorded provenance still
  satisfies the current compatibility rules.

## Backend Changes

### Strict complete-cycle construction

Keep the exact-alignment path in `previousCycleCompleteCreditsEstimate`.
Remove the transition fallback that constructs `cycle_complete` from a
mismatched natural-day interval.

The exact path remains valid when:

- `interval_end` matches `previous_reset_at` within tolerance;
- `interval_start` matches `previous_reset_at - 7 days` within tolerance; and
- a pre-reset quota sample exists near the interval end.

### Persisted baseline compatibility

Extend Credits estimate compatibility so a persisted `cycle_complete` record
is rejected when its interval boundaries do not match its recorded weekly
reset window. This hides the existing US$6,739 record without deleting SQLite
rows or authoritative inspection history.

Approximate and partial estimates remain governed by their existing
calculation-version and reset-window rules; they are not reclassified as
complete.

### Previous-cycle reference

When the reset changes, retain the most recent compatible rolling Credits
estimate from the old reset window as `formal_baseline` with
`cycle_approximate`. This is a previous-cycle reference, not a complete-cycle
claim. If no compatible estimate exists, omit the dollar value.

## Frontend Behavior

No invalid estimate should reach the estimate list. The frontend therefore:

- continues to render current-cycle collection separately;
- renders a compatible learned previous-cycle reference when available; and
- does not render a card, range, or formula for rejected historical estimates.

Add or adjust copy only if the backend needs to expose an explicit
interval-mismatch reason for the collecting state. Do not show the rejected
dollar amount in explanatory text.

## Compatibility And Data Handling

- Do not delete or rewrite `codex_inspection_results`, usage events, or
  credentials.
- Existing valid baselines remain readable.
- Existing invalid complete-cycle baselines remain stored for auditability but
  fail compatibility filtering.
- No schema migration is required.

## Rollback

The change is code-only. Rollback restores the previous binary/panel; SQLite
is unchanged. Deployment must preserve the current binary and panel rollback
copies according to the existing local service procedure.
