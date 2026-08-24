---
name: cpamp-web-type-safety
description: TypeScript contracts and runtime boundary validation
paths:
  - apps/web/src/**/*.ts
  - apps/web/src/**/*.tsx
---

# Type Safety

## Contract Placement

- Shared product types live in `src/types/` and are re-exported through `types/index.ts` where broadly consumed.
- Endpoint-specific request and response contracts live beside their API module in `services/api/`. `services/api/usageService.ts` owns Manager Server contracts; provider modules own CPA endpoint contracts.
- Feature-only view models and discriminated states live under `features/<domain>/model/`.
- Component props stay beside the component unless multiple modules genuinely share them.

Prefer narrow unions for finite domain states while retaining a documented string fallback when the backend can add values independently. `ManagerCodexInspectionScheduleMode` and related contracts in `usageService.ts` show this compatibility pattern.

## Runtime Boundaries

TypeScript types do not validate network, localStorage, YAML, or pasted JSON. Parse unknown values at the boundary and narrow before use. `services/api/client.ts` treats response error data as `unknown`, checks record/string shapes, and then creates a typed `ApiError`; `features/authFiles/sessionAuthConverter.ts` validates imported auth data before conversion.

```ts
const isRecord = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object';
```

Preserve optional fields for capabilities introduced across CPA versions. Consumers must be absent-safe and should centralize fallback behavior in a transformer or feature model rather than scatter assertions through JSX.

## Rules

- Avoid `any`; use `unknown` plus a type guard at external boundaries.
- Prefer `import type` for type-only imports.
- Keep API wire names and units explicit; do not silently reinterpret milliseconds, percentages, Credits, costs, or quota window kinds.
- When changing an API field, trace backend JSON, frontend interface, transformer/model, state, rendering, demo fixture, and tests.

## Anti-patterns

- Do not cast raw JSON directly in components.
- Do not make optional backend fields required only because one fixture currently includes them.
- Do not duplicate similar-but-different payload interfaces in multiple consumers; the owning service module is the contract authority.
- Do not suppress a compiler error with a broad assertion before verifying whether it reveals a real mode or version mismatch.
