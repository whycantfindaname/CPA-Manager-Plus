---
name: cpamp-web-directory-structure
description: React module placement and dependency direction
paths:
  - apps/web/src/**
---

# Directory Structure

## Runtime Shape

`apps/web/src/main.tsx` mounts the application. `app/RootShell.tsx` owns top-level lifecycle and shell composition, while `app/AppRouter.tsx` and `app/appRoutes.tsx` own route selection. Legacy `pages/*.tsx` files are route adapters; substantive page code belongs under `features/<domain>/`.

```text
apps/web/src/
├── app/          application shell and route registry
├── pages/        thin compatibility route adapters
├── features/     domain pages, hooks, models, and feature-local components
├── components/   reusable UI and cross-feature business components
├── entities/     domain transforms shared across features
├── services/     API clients and browser storage adapters
├── stores/       Zustand application state
├── hooks/        cross-feature React hooks
├── types/        shared frontend contracts
└── utils/        pure cross-feature helpers
```

## Placement Rules

- Put a complete user workflow in `features/<domain>/`. `features/accounts/` demonstrates page, component, hook, model, style, and colocated test organization.
- Keep pure domain calculations in `model/*.ts` beside their feature. Examples include `features/accounts/model/accountQuotaWindowDefinitions.ts` and `features/monitoring/model/codexInspectionOwnership.ts`.
- Put reusable primitives in `components/ui/`; put reusable domain-aware components in a named component area such as `components/providers/`.
- Put transport code in `services/api/`. Keep endpoint-specific normalization and error handling there rather than in rendered components.
- Use the `@/` alias for imports rooted at `apps/web/src`; use relative imports within a tightly coupled local folder.

The enforced dependency direction is:

```text
pages -> features / components / entities / services / stores / hooks / utils
features -> components / entities / services / stores / hooks / utils
components -> entities / services / stores / hooks / utils
```

`tests/frontendArchitectureBoundaries.test.mjs` currently fails any `features/` or `components/` import from `@/pages`.

## Naming And Generated Files

- React components and pages use PascalCase `.tsx`; hooks use `use*.ts` or `use*.tsx`; pure helpers and models use descriptive camelCase filenames.
- Colocate tests as `*.test.ts` or `*.test.tsx`; colocate component styles as `*.module.scss` when they are component-specific.
- Do not hand-edit `apps/web/dist/index.html` or `apps/manager-server/internal/httpapi/web/management.html`. Both are generated outputs.

## Anti-patterns

- Do not put new feature implementations in `pages/`; route adapters such as `pages/AccountsPage.tsx` should only forward to `features/accounts/AccountsPage.tsx`.
- Do not import upward from features or components into pages.
- Do not create a generic shared helper before searching `entities/`, `services/`, `hooks/`, and `utils/` for an existing owner.
