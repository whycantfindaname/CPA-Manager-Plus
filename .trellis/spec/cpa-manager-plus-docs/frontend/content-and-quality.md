---
name: cpamp-docs-content-quality
description: Product terminology, bilingual parity, and documentation verification
paths:
  - apps/docs/**/*.md
  - apps/docs/.vitepress/config.ts
  - README.md
  - README_CN.md
  - tests/docsContentIntegrity.test.mjs
---

# Content And Quality

## Product Truths To Preserve

Documentation must keep these runtime boundaries explicit:

- Clients such as Codex and Claude Code connect to CPA model endpoints, not CPAMP.
- The CPAMP Lightweight Panel is hosted by CPA, usually at `:8317/management.html`, and uses the CPA Management Key.
- CPAMP Full Mode is hosted by Manager Server, usually at `:18317/management.html`, and uses the CPAMP Admin Key.
- Full Mode connects to CPA with a separate CPA Management Key.
- The Live Demo uses fictional data and is not a deployment or runtime mode.

`guide/runtime-model.md`, `en/guide/runtime-model.md`, and the two `reference/capability-matrix.md` pages are source-backed examples. Do not mix the three key types or imply that CPAMP forwards model traffic.

## Writing Pattern

Write for the action the reader is trying to complete. Lead with the normal path, keep required commands copyable, state the expected result, and move advanced diagnostics after the basic checks. `troubleshooting/request-monitoring.md` and its English counterpart deliberately place ordered checks before advanced diagnostics.

Keep Chinese and English pages semantically aligned rather than mechanically word-for-word. Product names, file names, environment variables, ports, keys, commands, and route paths must remain exact in both versions.

```markdown
## Verify

1. Open the expected panel URL.
2. Sign in with the key for that panel mode.
3. Confirm the named status or page is visible.
```

## Verification

- `npm --workspace apps/web run test -- ../../tests/docsContentIntegrity.test.mjs` checks bilingual page parity, local VitePress links, required frontmatter, mode terminology, and progressive organization.
- `npm run docs:build` catches VitePress configuration, Markdown, component, and asset resolution failures.
- Run `npm run test` when README discovery language or cross-repository integrity expectations change.

## Anti-patterns

- Do not use source-code layer names as the opening explanation for ordinary users.
- Do not present a build, screenshot, or local preview as deployment verification.
- Do not mention capabilities in one locale only.
- Do not claim Lightweight Panel gains Full Mode persistence, analytics, inspection, or automation.
