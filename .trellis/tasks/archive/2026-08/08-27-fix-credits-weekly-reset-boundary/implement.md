# Implementation Plan: Credits weekly reset boundary pairing

## 1. Backend regression coverage

- Replace the stale-reset success fixture with the real 2026-08-25 shape:
  pre-reset 38%, post-reset 0%, and a seven-day Credits interval ending about
  22 hours before the quota reset.
- Assert that this fixture produces no `cycle_complete` Credits estimate and
  never returns US$6,739.
- Preserve an exact-alignment fixture proving a legitimate complete-cycle
  estimate still works.
- Add persisted-baseline coverage proving a mismatched historical
  `cycle_complete` record is omitted while compatible rolling/current records
  remain available.
- Retain multi-account isolation assertions.

## 2. Backend implementation

- Remove the mismatched reset-transition fallback from complete-cycle Credits
  construction.
- Centralize the complete-cycle boundary compatibility check and use it both
  when constructing and when reading persisted Credits estimates.
- Allow promotion of the previous reset window's compatible rolling estimate
  when an incompatible formal record already exists.
- Keep inspection history and persisted rows unchanged.
- Run `gofmt` on changed Go files.

## 3. Frontend state verification

- Verify the estimate component receives no rejected historical value.
- Add or update component tests for current-cycle collecting plus a compatible
  previous-cycle reference.
- If an explicit mismatch reason is exposed, add localized English and Chinese
  copy and its component assertion.

## 4. Automated validation

- Run focused Manager Server weekly-estimate tests.
- Run `npm run manager-server:test`.
- Run focused frontend component tests.
- Run `npm run type-check`.
- Run `npm run lint`.
- Run `npm run test`.
- Run `npm run build` and verify the single-file panel output.

## 5. Live acceptance

- Build the native Manager Server and panel using the repository packaging
  path.
- Preserve rollback copies before replacing the local service artifacts.
- Restart the LaunchAgent and verify `127.0.0.1:18317` health.
- Query the real account through the running service and confirm the stored
  US$6,739 mismatched estimate is absent.
- Confirm current-cycle collection and any valid previous-cycle reference are
  shown separately in the live UI.
- Use ego-browser for the final visual review with the user.

## 6. Completion gates

- Perform an adversarial check that exact alignment, mismatch, old persisted
  data, reset transition, and account isolation are all covered.
- Update the relevant Trellis cross-layer and Manager Server guidance with the
  interval-alignment invariant.
- Do not commit, push, merge, or publish until separately authorized.
