---
name: cpamp-web-frontend
description: Entry point for CPA Manager Plus React frontend conventions
paths:
  - apps/web/**
---

# CPA Manager Plus Web Guidelines

`apps/web` is the React 19 management panel. It runs in two product modes: a CPA-hosted Lightweight Panel and Manager Server Full Mode. Changes to authentication, setup, API routing, monitoring, or feature availability must preserve both modes; `apps/web/src/hooks/usePanelFeatureAvailability.ts` and its tests are the reference boundary.

## Guides

| Guide | Use it for |
| --- | --- |
| [Directory Structure](./directory-structure.md) | Placement, imports, feature boundaries, generated output |
| [Component Guidelines](./component-guidelines.md) | Components, styling, accessibility, visible text |
| [Hook Guidelines](./hook-guidelines.md) | Async effects, refresh orchestration, reusable hooks |
| [State Management](./state-management.md) | Local state, Zustand, persistence, scoped server state |
| [Type Safety](./type-safety.md) | API contracts, unknown data, unions, validation |
| [Quality Guidelines](./quality-guidelines.md) | Tests, checks, single-file build, prohibited patterns |

Read the topic that matches the files being changed. Cross-layer changes should also use [Cross-Layer Thinking Guide](../../guides/cross-layer-thinking-guide.md).

## Baseline Commands

Run from the repository root:

```bash
npm run type-check
npm run lint
npm run test
```

Add `npm run build` when changing bundling, assets, routing, entry points, or code that could affect the embedded single-file panel.
