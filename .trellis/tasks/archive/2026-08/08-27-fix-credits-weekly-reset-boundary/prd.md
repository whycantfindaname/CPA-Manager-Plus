# Fix Credits weekly reset boundary pairing

## Goal

Prevent CPAMP from presenting a Credits-based weekly pool estimate as a
complete cycle when the Credits interval and the official Codex weekly reset
window do not cover the same time range. Preserve separate current-cycle and
previous-cycle displays without inventing a dollar value from mismatched data.

## Background

- Real account samples observed on 2026-08-25 show the official weekly quota
  moving from 38% used at 22:13:06 to 0% used at 22:18:41, confirming that the
  reset transition was detected.
- Before the reset, Analytics exposed `55784.22` previous-cycle Credits for
  `2026-08-17 00:00` through `2026-08-24 00:00` in UTC+08:00.
- After the reset, Analytics exposed `64023.45` previous-cycle Credits for
  `2026-08-18 00:00` through `2026-08-25 00:00` in UTC+08:00.
- The official quota window ended at approximately `2026-08-25 22:13`, so the
  latter Credits interval ended about 22 hours before the quota window.
- `previousCycleCompleteCreditsEstimate` currently falls back to
  `previousWeeklySampleAtResetTransition` when exact boundary matching fails,
  then labels the result `cycle_complete`. This produced the misleading
  `64023.45 Credits / 38% * US$0.04 = US$6739/week` result.

## Requirements

- A `cycle_complete` Credits estimate must use the same account, quota kind,
  weekly reset window, and time interval for both Credits and quota movement.
- Detecting a reset transition must not, by itself, authorize pairing a
  natural-day Credits interval with the pre-reset quota percentage.
- An invalid historical complete-cycle estimate must not remain visible merely
  because it was persisted before this fix.
- The current-cycle rolling estimate and previous-cycle reference remain
  separate UI concepts.
- When no matched interval is available, show a collecting or interval-mismatch
  state instead of a dollar estimate.
- Do not delete or disable CPA credentials, and do not rewrite authoritative
  usage or inspection history.
- Preserve per-account isolation and require the same account identity for all
  paired samples.

## Acceptance Criteria

- [ ] The real 2026-08-25 transition fixture (`38% -> 0%`, Credits interval
      ending about 22 hours before reset) does not produce `US$6739/week` or a
      `cycle_complete` estimate.
- [ ] A strictly aligned seven-day Credits interval and weekly reset window can
      still produce a complete-cycle estimate.
- [ ] A valid matched rolling estimate from the previous reset window can be
      retained as a clearly labelled previous-cycle reference.
- [ ] Persisted complete-cycle estimates whose interval end does not match
      their recorded weekly reset are hidden entirely without deleting
      inspection history or relabelling the value as an approximation.
- [ ] Current-cycle collection continues after reset and updates only from
      closed Credits boundaries paired with quota samples from the same reset
      window.
- [ ] Focused backend regression tests, the complete Manager Server test suite,
      frontend tests affected by the state/label change, type-check, lint, and
      production build pass.
- [ ] Live acceptance uses the existing SQLite history and running local
      service; synthetic fixtures are regression coverage, not the sole
      acceptance evidence.

## Out of Scope

- Changing the `US$0.04/Credit` conversion rate.
- Estimating an exact quota value from intervals that cannot be matched.
- Deleting credentials, inspection history, or usage events.
- Publishing, merging, or pushing without separate authorization.

## Key Product Decision

- Historical estimates with mismatched Credits and quota boundaries are hidden
  entirely. They are not retained under a low-confidence label.
