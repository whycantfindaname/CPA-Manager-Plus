---
name: cpamp-web-components
description: React component, styling, accessibility, and localization patterns
paths:
  - apps/web/src/components/**
  - apps/web/src/features/**/*.tsx
  - apps/web/src/pages/**/*.tsx
---

# Component Guidelines

## Component Shape

Use typed function components and name props locally. Keep transformation and policy logic outside JSX when it can be tested as a pure function. `features/accounts/components/QuotaWindowCard.tsx` delegates window interpretation to account model helpers; `components/providers/ProviderTable/rowData.ts` and `sort.ts` keep table policy out of rendering.

```tsx
interface QuotaWindowCardProps {
  window: AccountQuotaWindow;
  nowMs: number;
}

export function QuotaWindowCard({ window, nowMs }: QuotaWindowCardProps) {
  // Render a view model produced by feature/model helpers.
}
```

Use shared primitives from `components/ui/` before adding one-off buttons, drawers, modals, selects, or toggles. `ConfirmationModal.tsx`, `Drawer.tsx`, and `Button.tsx` are representative reusable boundaries.

## Styling

- Use component-scoped `*.module.scss` for feature and reusable component styles.
- Use `styles/variables.scss`, `mixins.scss`, `themes.scss`, and the existing CSS custom properties instead of literal theme colors.
- Global resets and application-wide layout belong in `styles/` or the existing top-level stylesheet, not in a feature module.
- Preserve the panel's responsive behavior; large tables and drawers must remain usable on narrow viewports.

## Interaction And Accessibility

- User-visible text must go through `react-i18next`; follow the `useTranslation()` usage in feature pages and shared dialogs.
- Use real buttons for actions and labels for form controls. Preserve keyboard and focus behavior in shared modal, drawer, dropdown, and segmented-tab primitives.
- Give icon-only actions an accessible name with visible text, `aria-label`, or an existing tooltip pattern.
- Represent loading, empty, error, disabled, and confirmation states explicitly. Destructive account operations require the existing preview/confirmation flow rather than immediate mutation.

## Composition Boundaries

Pages orchestrate data and workflows; feature components render a coherent section; `components/ui/` stays product-agnostic. A component that starts parsing raw API payloads or duplicating account policy belongs behind a typed service/model helper.

## Anti-patterns

- Do not embed visible English or Chinese literals when an i18n key exists or should exist.
- Do not copy shared modal/drawer behavior into feature-local markup.
- Do not hide unsupported features only with CSS. Resolve capability through `usePanelFeatureAvailability.ts` so Lightweight Panel and Full Mode behavior remain correct.
- Do not add dynamic imports, workers, or runtime asset URLs; the production panel must remain a single HTML file.
