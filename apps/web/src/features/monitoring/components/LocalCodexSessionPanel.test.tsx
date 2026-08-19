import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it } from 'vitest';
import type { CodexInspectionResult, LocalCodexSessionResponse } from '@/services/api/usageService';
import { LocalCodexSessionPanel } from './LocalCodexSessionPanel';

const t = ((key: string, options?: Record<string, unknown>) =>
  options ? `${key}:${JSON.stringify(options)}` : key) as never;

const collectText = (renderer: ReactTestRenderer) =>
  renderer.root
    .findAll((node) => typeof node.children[0] === 'string')
    .flatMap((node) => node.children.filter((child): child is string => typeof child === 'string'));

const availableResponse: LocalCodexSessionResponse = {
  status: 'available',
  source: 'codex_app_server',
  snapshot: {
    capturedAtMs: 1780000000000,
    account: { type: 'chatgpt', planType: 'pro', email: 'owner@example.com' },
    quotaBuckets: [
      {
        id: 'codex',
        planType: 'pro',
        primary: { usedPercent: 79, windowDurationMins: 10080, resetsAt: 1780001000 },
      },
    ],
    usage: {
      summary: { lifetimeTokens: 1000000 },
      dailyUsageBuckets: [{ startDate: '2026-08-19', tokens: 12345 }],
    },
  },
};

const comparison: CodexInspectionResult = {
  id: 1,
  runId: 1,
  accountKey: 'owner',
  fileName: 'owner.json',
  displayAccount: 'owner@example.com',
  provider: 'codex',
  disabled: false,
  action: 'keep',
  actionReason: 'ok',
  isQuota: true,
  createdAtMs: 1780000000000,
  quotaWindows: [
    {
      id: 'weekly',
      labelKey: 'weekly',
      usedPercent: 79,
      resetAtMs: 1780001036000,
      limitWindowSeconds: 604800,
    },
  ],
};

describe('LocalCodexSessionPanel', () => {
  it('shows quota, aggregate token activity, and the CPA source comparison', () => {
    let renderer: ReactTestRenderer;
    act(() => {
      renderer = create(
        <LocalCodexSessionPanel
          response={availableResponse}
          comparison={comparison}
          loading={false}
          error=""
          locale="en-US"
          t={t}
          onRefresh={() => {}}
        />
      );
    });

    const text = collectText(renderer!).join(' ');
    expect(text).toContain('owner@example.com');
    expect(text).toContain('monitoring.local_codex_session_daily_tokens');
    expect(text).toContain('"matched":1');
    expect(text).toContain('"percent":"0.0"');
    expect(text).toContain('"seconds":36');
    expect(text).toContain('monitoring.local_codex_session_daily_tokens_hint');
  });

  it('explains that CPA inspection remains usable when app-server is missing', () => {
    let renderer: ReactTestRenderer;
    act(() => {
      renderer = create(
        <LocalCodexSessionPanel
          response={{
            status: 'unavailable',
            source: 'codex_app_server',
            reason: 'codex_executable_not_found',
          }}
          comparison={null}
          loading={false}
          error=""
          locale="en-US"
          t={t}
          onRefresh={() => {}}
        />
      );
    });

    expect(collectText(renderer!)).toContain('monitoring.local_codex_session_missing_runtime');
  });
});
