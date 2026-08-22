import type { ComponentProps } from 'react';
import { act, create, type ReactTestRenderer } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';
import type { AuthFileItem } from '@/types';
import type {
  CodexInspectionResultItem,
  CodexInspectionRunResult,
} from '@/features/monitoring/codexInspection';
import { Button } from '@/components/ui/Button';
import inspectionStyles from '@/features/monitoring/CodexInspectionPage.module.scss';
import tooltipStyles from './FailureDetailsTooltip.module.scss';
import { CodexInspectionResultsPanel } from './CodexInspectionResultsPanel';

const t = ((key: string, options?: Record<string, unknown>) => {
  if (options?.capture && options?.route && options?.quota)
    return `${key}:${options.capture}:${options.route}:${options.quota}`;
  if (options?.cost && options?.min && options?.max)
    return `${key}:${options.cost}:${options.min}:${options.max}`;
  if (options?.cost) return `${key}:${options.cost}:${options.percent}`;
  if (options?.source && options?.status) return `${key}:${options.source}:${options.status}`;
  if (options?.source && options?.time) return `${key}:${options.source}:${options.time}`;
  if (options?.start && options?.end)
    return `${key}:${options.start}:${options.end}:${options.timezone}`;
  if (options?.credits && options?.min && options?.max)
    return `${key}:${options.credits}:${options.min}:${options.max}`;
  if (options?.credits) return `${key}:${options.credits}:${options.percent}:${options.rate}`;
  if (options?.min && options?.max) return `${key}:${options.min}:${options.max}`;
  if (options?.percent) return `${key}:${options.percent}`;
  if (options?.count !== undefined) return `${key}:${options.count}`;
  return key;
}) as never;

const createItem = (
  overrides: Partial<CodexInspectionResultItem> = {}
): CodexInspectionResultItem => ({
  key: 'credential-1',
  fileName: 'codex-account-free.json',
  displayAccount: 'account@example.com',
  authIndex: null,
  accountId: null,
  provider: 'codex',
  disabled: false,
  autoRecoverOwned: false,
  status: 'active',
  state: 'enabled',
  raw: {} as AuthFileItem,
  action: 'keep',
  actionReason: 'healthy quota does not require handling',
  statusCode: 200,
  usedPercent: 3,
  isQuota: false,
  autoRecoverEligible: false,
  error: '',
  errorKind: '',
  quotaWindows: [
    {
      id: 'monthly',
      labelKey: 'monthly',
      usedPercent: 3,
      resetLabel: '',
      limitWindowSeconds: null,
    },
  ],
  ...overrides,
});

const renderPanel = (
  item: CodexInspectionResultItem,
  overrides: Partial<ComponentProps<typeof CodexInspectionResultsPanel>> = {}
) => {
  let renderer: ReactTestRenderer;
  act(() => {
    renderer = create(
      <CodexInspectionResultsPanel
        result={{} as CodexInspectionRunResult}
        filteredResults={[item]}
        pendingActionCount={0}
        handlingFilterCounts={{ all: 1, pending: 0, no_action: 1 }}
        filterCounts={{ all: 1, delete: 0, disable: 0, enable: 0, reauth: 0, keep: 1 }}
        handlingFilter="all"
        actionFilter="all"
        pagination={{
          currentPage: 1,
          totalPages: 1,
          pageItems: [item],
          startItem: 1,
          endItem: 1,
          count: 1,
        }}
        pageSize={10}
        pageSizeOptions={[10, 20]}
        executing={false}
        isInspectionInFlight={false}
        t={t}
        onActionFilterChange={vi.fn()}
        onHandlingFilterChange={vi.fn()}
        onPageChange={vi.fn()}
        onPageSizeChange={vi.fn()}
        onExecutePlanned={vi.fn()}
        onExecuteSingle={vi.fn()}
        filterLabel={(filter) => filter}
        handlingFilterLabel={(filter) => filter}
        {...overrides}
      />
    );
  });
  return renderer!;
};

const collectText = (renderer: ReactTestRenderer) =>
  renderer.root
    .findAll((node) => typeof node.children[0] === 'string')
    .flatMap((node) => node.children.filter((child): child is string => typeof child === 'string'));

describe('CodexInspectionResultsPanel', () => {
  it('renders one four-section result card without duplicated healthy copy or empty actions', () => {
    const renderer = renderPanel(createItem());
    const text = collectText(renderer);

    expect(renderer.root.findAllByType('table')).toHaveLength(0);
    expect(renderer.root.findAllByType('article')).toHaveLength(1);
    expect(renderer.root.findAllByType('section')).toHaveLength(4);
    expect(text).toContain('monitoring.codex_inspection_quota_remaining:97%');
    expect(text).not.toContain('healthy quota does not require handling');
    expect(text).not.toContain('—');
  });

  it('places a custom server operation inside the same result card', () => {
    const renderer = renderPanel(
      createItem({ action: 'delete', actionReason: 'invalid account' }),
      {
        pendingActionCount: 1,
        renderOperation: () => <button data-testid="custom-operation">execute</button>,
      }
    );

    expect(renderer.root.findByProps({ 'data-testid': 'custom-operation' })).toBeDefined();
    expect(renderer.root.findAllByType('article')).toHaveLength(1);
  });

  it('shows the per-account weekly estimate, reliability, formula, and disclaimer', () => {
    const renderer = renderPanel(
      createItem({
        weeklyPoolEstimate: {
          official: false,
          basis: 'api_equivalent_cost',
          source: 'cpa_current',
          role: 'current_estimate',
          intervalKind: 'partial_cycle',
          calculationVersion: 'cpa_matched_interval_v2',
          status: 'reliable',
          weeklyPoolUsd: 2040,
          weeklyPoolMinUsd: 1854.55,
          weeklyPoolMaxUsd: 2266.67,
          costDeltaUsd: 204,
          usedPercentDelta: 10,
          usedPercentMinDelta: 9,
          usedPercentMaxDelta: 11,
          intervalStartMs: 1_800_000_000_000,
          intervalEndMs: 1_800_003_600_000,
          captureState: 'closed',
          routeScope: 'observed',
          quotaScope: 'matched',
          priceSources: ['models.dev'],
        },
      })
    );
    const text = collectText(renderer);
    const estimate = renderer.root.findByProps({ 'data-status': 'reliable' });

    expect(estimate).toBeDefined();
    expect(text).toContain('monitoring.codex_inspection_weekly_estimate_title');
    expect(text).toContain(
      'monitoring.codex_inspection_weekly_estimate_source_status:monitoring.codex_inspection_weekly_source_cpa_current:monitoring.codex_inspection_weekly_estimate_status_reliable'
    );
    expect(text).toContain('monitoring.codex_inspection_weekly_estimate_equation:$204:10%');
    expect(text).toContain('monitoring.codex_inspection_weekly_cpa_evidence:$204:9%:11%');
    expect(
      text.some((value) => value.includes('monitoring.codex_inspection_weekly_capture_closed'))
    ).toBe(true);
    expect(text).toContain('monitoring.codex_inspection_weekly_estimate_disclaimer');
    expect(text).toContain('monitoring.codex_inspection_weekly_estimate_price_source');
  });

  it('labels a Credits-learned value and shows its own formula', () => {
    const renderer = renderPanel(
      createItem({
        weeklyPoolEstimate: {
          official: false,
          basis: 'credits',
          source: 'credits_learned',
          role: 'formal_baseline',
          intervalKind: 'cycle_approximate',
          status: 'preliminary',
          weeklyPoolUsd: 2000,
          weeklyPoolMinUsd: 1900,
          weeklyPoolMaxUsd: 2150,
          credits: 2000,
          usdPerCredit: 0.04,
          usedPercentDelta: 4,
          updatedAtMs: 1_800_000_000_000,
        },
      })
    );
    const text = collectText(renderer);

    expect(text).toContain(
      'monitoring.codex_inspection_weekly_estimate_source_status:monitoring.codex_inspection_weekly_source_credits_learned:monitoring.codex_inspection_weekly_estimate_status_preliminary'
    );
    expect(text).toContain(
      'monitoring.codex_inspection_weekly_credits_previous_equation:2000.00:4%:$0.04'
    );
    expect(text).toContain('monitoring.codex_inspection_weekly_role_formal');
    expect(text).toContain('monitoring.codex_inspection_weekly_range:$1,900:$2,150');
    expect(
      text.some((value) => value.startsWith('monitoring.codex_inspection_weekly_current_source:'))
    ).toBe(true);
  });

  it('shows and labels the CPA and Credits estimates independently', () => {
    const cpaEstimate = {
      official: false as const,
      basis: 'api_equivalent_cost' as const,
      source: 'cpa_current' as const,
      role: 'current_estimate' as const,
      intervalKind: 'partial_cycle' as const,
      calculationVersion: 'cpa_matched_interval_v2',
      status: 'preliminary' as const,
      weeklyPoolUsd: 2040,
      weeklyPoolMinUsd: 1700,
      weeklyPoolMaxUsd: 2550,
      costDeltaUsd: 102,
      usedPercentDelta: 5,
      usedPercentMinDelta: 4,
      usedPercentMaxDelta: 6,
      captureState: 'closed' as const,
      routeScope: 'observed' as const,
      quotaScope: 'matched' as const,
    };
    const formalCpaEstimate = {
      ...cpaEstimate,
      source: 'cpa_learned' as const,
      role: 'formal_baseline' as const,
      intervalKind: 'cycle_approximate' as const,
      weeklyPoolUsd: 2010,
      weeklyPoolMinUsd: 1900,
      weeklyPoolMaxUsd: 2130,
      costDeltaUsd: 402,
      usedPercentDelta: 20,
      usedPercentMinDelta: 19,
      usedPercentMaxDelta: 21,
      status: 'reliable' as const,
    };
    const creditsEstimate = {
      official: false as const,
      basis: 'credits' as const,
      source: 'credits_current' as const,
      role: 'current_estimate' as const,
      intervalKind: 'partial_cycle' as const,
      status: 'preliminary' as const,
      weeklyPoolUsd: 1960,
      weeklyPoolMinUsd: 1820,
      weeklyPoolMaxUsd: 2110,
      credits: 980,
      usdPerCredit: 0.04,
      usedPercentDelta: 2,
      usedPercentMinDelta: 1.5,
      usedPercentMaxDelta: 2.5,
    };
    const formalCreditsEstimate = {
      ...creditsEstimate,
      source: 'credits_learned' as const,
      role: 'formal_baseline' as const,
      intervalKind: 'cycle_approximate' as const,
      weeklyPoolUsd: 2030,
      weeklyPoolMinUsd: 1950,
      credits: 32_000,
      usedPercentDelta: 82,
      usedPercentMinDelta: 81,
      usedPercentMaxDelta: 83,
    };
    const renderer = renderPanel(
      createItem({
        weeklyPoolEstimate: cpaEstimate,
        weeklyPoolEstimates: [formalCpaEstimate, cpaEstimate, formalCreditsEstimate, creditsEstimate],
      })
    );
    const text = collectText(renderer);

    expect(renderer.root.findAllByProps({ 'data-basis': 'api_equivalent_cost' })).toHaveLength(2);
    expect(renderer.root.findAllByProps({ 'data-basis': 'credits' })).toHaveLength(2);
    expect(text).toContain('monitoring.codex_inspection_weekly_estimate_equation:$102:5%');
    expect(text).toContain('monitoring.codex_inspection_weekly_cpa_evidence:$102:4%:6%');
    expect(text).toContain('monitoring.codex_inspection_weekly_credits_equation:980.00:2%:$0.04');
    expect(text).toContain('monitoring.codex_inspection_weekly_range:$1,820:$2,110');
    expect(text).toContain('monitoring.codex_inspection_weekly_range:$1,950:$2,110');
    expect(text).toContain('monitoring.codex_inspection_weekly_credits_evidence:980.00:1.5%:2.5%');
    expect(text).toContain('monitoring.codex_inspection_weekly_role_formal');
    expect(text).toContain('monitoring.codex_inspection_weekly_role_current');
  });

  it('shows collection status without a fixed-value fallback', () => {
    const renderer = renderPanel(
      createItem({
        planType: 'pro',
        usedPercent: 3,
        weeklyPoolEstimate: {
          official: false,
          basis: 'api_equivalent_cost',
          status: 'unavailable',
          role: 'current_estimate',
          intervalKind: 'partial_cycle',
          calculationVersion: 'cpa_matched_interval_v2',
          captureState: 'pending',
          routeScope: 'observed',
          quotaScope: 'matched',
          reason: 'capture_pending',
        },
      })
    );
    const text = collectText(renderer);

    expect(text).not.toContain('$2,000');
    expect(text).toContain('monitoring.codex_inspection_weekly_measured_refresh');
    expect(
      text.some((value) =>
        value.startsWith('monitoring.codex_inspection_weekly_estimate_reason_capture_pending')
      )
    ).toBe(true);
    expect(
      text.some((value) => value.includes('monitoring.codex_inspection_weekly_capture_pending'))
    ).toBe(true);
  });

  it('opens the credential represented by a result card', () => {
    const item = createItem({ authIndex: 'auth-1' });
    const onOpenCredential = vi.fn();
    const renderer = renderPanel(item, { onOpenCredential });
    const openButton = renderer.root
      .findAllByType(Button)
      .find((node) => node.props.children === 'monitoring.codex_inspection_view_credential');

    expect(openButton).toBeDefined();

    act(() => {
      openButton?.props.onClick();
    });

    expect(onOpenCredential).toHaveBeenCalledWith(item);
  });

  it('keeps credential navigation next to a custom server operation', () => {
    const renderer = renderPanel(
      createItem({ action: 'delete', actionReason: 'invalid account' }),
      {
        pendingActionCount: 1,
        onOpenCredential: vi.fn(),
        renderOperation: () => <button data-testid="custom-operation">execute</button>,
      }
    );

    expect(renderer.root.findByProps({ 'data-testid': 'custom-operation' })).toBeDefined();
    expect(
      renderer.root
        .findAllByType(Button)
        .some((node) => node.props.children === 'monitoring.codex_inspection_view_credential')
    ).toBe(true);
  });

  it('renders the xAI probe HTTP status when billing health returns one', () => {
    const renderer = renderPanel(
      createItem({
        provider: 'xai',
        statusCode: 200,
        errorKind: 'billing_healthy',
      })
    );

    expect(renderer.root.findAll((node) => node.children.join('') === 'HTTP 200')).toHaveLength(1);
  });

  it('moves failed probe diagnostics into the failure status tooltip', () => {
    const rawResponse = '{"code":"personal-team-blocked:spending-limit"}';
    const renderer = renderPanel(
      createItem({
        provider: 'xai',
        statusCode: 402,
        errorKind: 'http_status',
        error: rawResponse,
      })
    );
    const text = collectText(renderer);
    const tooltip = renderer.root.findByProps({ role: 'tooltip' });

    expect(text).toContain('monitoring.codex_inspection_probe_state_failed');
    expect(text).toContain('monitoring.codex_inspection_error_summary_http_status');
    expect(text).toContain(rawResponse);
    expect(tooltip.props.className).toContain(tooltipStyles.tooltip);
    expect(renderer.root.findAllByType('details')).toHaveLength(0);
  });

  it('keeps all action filters visible and renders the plan without a label prefix', () => {
    const renderer = renderPanel(createItem({ planType: 'free' }));
    const text = collectText(renderer);

    expect(text).toEqual(
      expect.arrayContaining(['all', 'reauth', 'delete', 'disable', 'enable', 'keep'])
    );
    expect(
      renderer.root.findAllByProps({
        'aria-label': 'monitoring.codex_inspection_action_filter_label',
      })
    ).toHaveLength(1);
    expect(text).not.toContain('monitoring.codex_inspection_action_filter_label');
    expect(text).not.toEqual(expect.arrayContaining(['pending', 'no_action']));
    expect(text).toContain('codex_quota.plan_free');
    expect(text).not.toContain('codex_quota.plan_label');
  });

  it('gives disabled credential state chips a visible neutral background', () => {
    const renderer = renderPanel(createItem({ disabled: true }));
    const stateChip = renderer.root.find(
      (node) => node.children.join('') === 'monitoring.codex_inspection_state_disabled'
    );

    expect(stateChip.props.className).toContain(inspectionStyles.stateDisabled);
  });

  it('localizes keyed conclusion reasons without repeating a conclusion heading', () => {
    const translatedT = ((key: string) => `translated:${key}`) as never;
    const renderer = renderPanel(
      createItem({
        action: 'disable',
        actionReason: 'monitoring.codex_inspection_reason_quota_threshold',
      }),
      { t: translatedT }
    );
    const text = collectText(renderer);

    expect(text).toContain('translated:monitoring.codex_inspection_reason_quota_threshold');
    expect(text).not.toContain('translated:monitoring.codex_inspection_conclusion');
    expect(
      renderer.root.findAll((node) =>
        String(node.props.className ?? '').includes(inspectionStyles.actionBadge)
      )
    ).toHaveLength(1);
  });
});
