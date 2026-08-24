---
name: cpamp-web-state
description: Local, Zustand, persistent, and connection-scoped state rules
paths:
  - apps/web/src/stores/**
  - apps/web/src/services/storage/**
  - apps/web/src/features/**/model/**
---

# State Management

## Choose The Narrowest Owner

- Use component state for open/closed controls, draft input, selection, and state used by one rendered subtree.
- Use a feature hook or feature model for a workflow shared within one domain.
- Use Zustand in `src/stores/` only for cross-route or application-lifecycle state such as authentication, configuration, language, theme, notifications, quota cache, and usage-service connection.

Stores expose typed actions and selectors; callers should not reproduce persistence or normalization rules. `useAuthStore.ts` owns the active API base and management key. `useQuotaStore.ts` owns provider quota maps and deliberately persists only stable success/error entries with verified credential identity.

## Persistence And Scope

Browser persistence is a compatibility and UX mechanism, not a security boundary. `services/storage/secureStorage.ts` explicitly provides reversible obfuscation. Never describe it as encryption or use it to justify storing new secrets.

Persist only state that is safe and useful after reload. Include a scope discriminator when cached data depends on the active CPA/Manager connection or credential. The quota-store tests prove that legacy, unverified, observed-only, and loading entries must not survive persistence.

```ts
partialize: (state) => ({
  cacheScope: state.cacheScope,
  // Keep stable, identity-verified results; omit transient loading state.
})
```

Use explicit reset actions when authentication or API target changes. A response captured for one `apiBase`/management key pair must not update state for another pair.

## Server And Derived State

This project does not use a general server-state cache library. API modules return typed data; hooks coordinate requests; Zustand stores only data that must span consumers. Compute display models in pure feature/model helpers instead of persisting duplicated derived values.

## Anti-patterns

- Do not put every fetched response in a global store.
- Do not persist loading flags, stale observed quota headers, or unverified credential-keyed data.
- Do not mutate nested store objects outside store actions.
- Do not call reversible obfuscation cryptographic protection.
