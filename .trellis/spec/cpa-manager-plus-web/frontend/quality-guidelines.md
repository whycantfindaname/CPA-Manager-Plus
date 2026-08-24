---
name: cpamp-web-quality
description: Frontend verification, test style, and single-file release constraints
paths:
  - apps/web/**
  - tests/frontendArchitectureBoundaries.test.mjs
  - tests/repoSourceIntegrity.test.mjs
---

# Quality Guidelines

## Required Local Patterns

- TypeScript uses 2 spaces, single quotes, semicolons, Prettier, ESLint, React Hooks rules, and React Refresh rules.
- Keep business rules as pure functions with colocated Vitest tests. The dense `features/accounts/model/*.test.ts` suite is the preferred pattern for account and quota behavior.
- Test rendered interaction where component state, effects, accessibility, or event ordering matters. Examples include `components/common/ConfirmationModal.test.tsx` and `components/providers/ProviderDetailDrawer/ProviderDetailDrawer.test.tsx`.
- Preserve both CPA Lightweight Panel and Manager Server Full Mode semantics for auth, setup, feature gates, and API routing.

## Single-file Panel Contract

`apps/web/vite.config.ts` uses `vite-plugin-singlefile`. Production JavaScript, CSS, and assets must be inlined into `apps/web/dist/index.html` so CPA and Manager Server can each serve one `management.html`.

Forbidden patterns include dynamic `import()`, emitted worker files, and `new URL('./asset', import.meta.url)`. Do not hand-edit generated `dist/index.html` or embedded `management.html`.

## Verification Matrix

From the repository root:

| Change | Commands |
| --- | --- |
| Pure model/helper or focused component | `npm --workspace apps/web run test -- <test-path>` |
| Frontend behavior | `npm run type-check && npm run lint && npm run test` |
| Entry point, routing, styling, asset, bundling | Add `npm run build` |
| API contract spanning Manager Server | Add `npm run manager-server:test` |

`tests/frontendArchitectureBoundaries.test.mjs` verifies import direction. `tests/repoSourceIntegrity.test.mjs` checks changed text files for hidden Unicode and requires a valid PR diff base; set `CPA_MANAGER_CHANGED_FILES_BASE` only when `origin/main...HEAD` is unavailable and use a real intended base.

## Anti-patterns

- Do not replace behavior tests with snapshots of large rendered trees.
- Do not add a test that still passes after deleting the behavior it claims to cover.
- Do not call a successful local build deployment proof.
- Do not broaden a focused fix into a new compatibility layer, feature flag, or persistence scheme without a current requirement.
