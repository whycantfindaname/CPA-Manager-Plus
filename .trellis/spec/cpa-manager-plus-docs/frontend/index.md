---
name: cpamp-docs
description: Entry point for bilingual VitePress documentation conventions
paths:
  - apps/docs/**
---

# CPA Manager Plus Documentation Guidelines

`apps/docs` is a bilingual VitePress site, not a React application. The generated React frontend templates were removed because component, hook, state-management, and TypeScript application guidance does not apply to documentation pages.

## Guides

| Guide | Use it for |
| --- | --- |
| [Structure And Navigation](./structure-and-navigation.md) | Bilingual page layout, VitePress routes, navigation, assets |
| [Content And Quality](./content-and-quality.md) | User-facing terminology, mode boundaries, tests, build verification |

## Baseline Commands

```bash
npm run test
npm run docs:build
```

Run the documentation integrity test directly while iterating:

```bash
npm --workspace apps/web run test -- ../../tests/docsContentIntegrity.test.mjs
```
