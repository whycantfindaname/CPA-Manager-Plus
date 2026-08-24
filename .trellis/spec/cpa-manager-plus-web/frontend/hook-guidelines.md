---
name: cpamp-web-hooks
description: Cross-feature and feature-local React hook conventions
paths:
  - apps/web/src/hooks/**
  - apps/web/src/features/**/hooks/**
---

# Hook Guidelines

## Ownership

- Put reusable browser or application behavior in `src/hooks/`, such as `useDebounce.ts`, `useInterval.ts`, `useLocalStorage.ts`, and `useUnsavedChangesGuard.ts`.
- Put workflow-specific orchestration under `features/<domain>/hooks/`. `features/accounts/hooks/useAccountsWorkspaceRefresh.ts` and `features/authFiles/hooks/useAuthFilesData.ts` are representative.
- Keep pure parsing, gating, and transformation outside hooks so it can be tested without a renderer. `usePanelFeatureAvailability.ts` exports pure helpers alongside the hook and tests both behavior layers.

## Async Effects

Effects must clean up timers, event listeners, and stale async work. When requests can overlap, preserve the current request's connection identity and prevent an older result from overwriting newer state. The scoped request support in `services/api/client.ts` exists for multi-step operations that must retain the API base and management key captured at start.

```ts
useEffect(() => {
  let cancelled = false;
  void load().then((value) => {
    if (!cancelled) setValue(value);
  });
  return () => {
    cancelled = true;
  };
}, [load]);
```

Stabilize callbacks and derived objects when they are dependencies or public hook results. Avoid broad dependency suppression; `eslint-plugin-react-hooks` is part of the required lint path.

## Data Fetching

Use typed functions from `services/api/`; do not call Axios or construct authentication headers in feature hooks. Deduplicate shared probes where multiple consumers mount together, as tested by `hooks/usePanelFeatureAvailability.test.ts`. Surface request errors in the shape expected by the calling feature rather than swallowing them.

## Testing

- Test pure exported rules directly with Vitest.
- For lifecycle-sensitive hooks, use `react-test-renderer`, `act`, controlled promises, and explicit global cleanup as in `usePanelFeatureAvailability.test.ts` and `features/accounts/hooks/useAccountsWorkspaceRefresh.test.tsx`.
- Verify unmount, stale response, concurrent consumer, and connection-change behavior when the hook owns those risks.

## Anti-patterns

- Do not duplicate the same fetch lifecycle in several components; promote it to the owning feature hook.
- Do not persist transient loading state.
- Do not treat a CPA-hosted panel as Manager Server merely because stale Manager configuration exists; the tested host/mode gates are authoritative.
