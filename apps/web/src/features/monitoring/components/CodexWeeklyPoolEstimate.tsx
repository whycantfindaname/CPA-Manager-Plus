import type { TFunction } from 'i18next';
import type {
  CodexWeeklyPoolEstimate as WeeklyPoolEstimate,
  CodexWeeklyPoolEstimateStatus,
} from '@/features/monitoring/codexInspection';
import styles from '../CodexInspectionPage.module.scss';

type CodexWeeklyPoolEstimateProps = {
  estimate?: WeeklyPoolEstimate | null;
  estimates?: WeeklyPoolEstimate[];
  t: TFunction;
};

const statusClass: Record<CodexWeeklyPoolEstimateStatus, string> = {
  unavailable: styles.weeklyEstimateStatusUnavailable,
  insufficient: styles.weeklyEstimateStatusInsufficient,
  preliminary: styles.weeklyEstimateStatusPreliminary,
  reliable: styles.weeklyEstimateStatusReliable,
};

const formatUSD = (value: number) =>
  new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: value >= 100 ? 0 : 2,
    maximumFractionDigits: value >= 100 ? 0 : 2,
  }).format(value);

const formatPercent = (value: number) =>
  `${Number.isInteger(value) ? value.toFixed(0) : value.toFixed(1)}%`;

const formatBoundary = (value: number, timezone?: string) => {
  const fixedOffset = timezone?.match(/^UTC([+-])(\d{2}):(\d{2})$/);
  if (fixedOffset) {
    const direction = fixedOffset[1] === '-' ? -1 : 1;
    const offsetMinutes = direction * (Number(fixedOffset[2]) * 60 + Number(fixedOffset[3]));
    return new Intl.DateTimeFormat(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      timeZone: 'UTC',
    }).format(new Date(value + offsetMinutes * 60_000));
  }
  try {
    return new Intl.DateTimeFormat(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      timeZone: timezone,
    }).format(new Date(value));
  } catch {
    return new Date(value).toLocaleString();
  }
};

const formatBoundaryDate = (value: number, timezone?: string) => {
  const fixedOffset = timezone?.match(/^UTC([+-])(\d{2}):(\d{2})$/);
  if (fixedOffset) {
    const direction = fixedOffset[1] === '-' ? -1 : 1;
    const offsetMinutes = direction * (Number(fixedOffset[2]) * 60 + Number(fixedOffset[3]));
    return new Intl.DateTimeFormat(undefined, {
      month: 'short',
      day: 'numeric',
      timeZone: 'UTC',
    }).format(new Date(value + offsetMinutes * 60_000));
  }
  try {
    return new Intl.DateTimeFormat(undefined, {
      month: 'short',
      day: 'numeric',
      timeZone: timezone,
    }).format(new Date(value));
  } catch {
    return new Date(value).toLocaleDateString();
  }
};

const reasonKey = (reason?: string) => {
  switch (reason) {
    case 'baseline_missing':
      return 'monitoring.codex_inspection_weekly_estimate_reason_baseline_missing';
    case 'delta_too_small':
      return 'monitoring.codex_inspection_weekly_estimate_reason_delta_small';
    case 'identity_missing':
      return 'monitoring.codex_inspection_weekly_estimate_reason_identity_missing';
    case 'weekly_sample_incomplete':
      return 'monitoring.codex_inspection_weekly_estimate_reason_sample_incomplete';
    case 'quota_decreased':
      return 'monitoring.codex_inspection_weekly_estimate_reason_quota_decreased';
    case 'capture_pending':
      return 'monitoring.codex_inspection_weekly_estimate_reason_capture_pending';
    case 'credits_boundary_pending':
      return 'monitoring.codex_inspection_weekly_estimate_reason_credits_boundary_pending';
    case 'quota_boundary_missing':
      return 'monitoring.codex_inspection_weekly_estimate_reason_quota_boundary_missing';
    case 'identity_changed':
      return 'monitoring.codex_inspection_weekly_estimate_reason_identity_changed';
    case 'price_missing':
      return 'monitoring.codex_inspection_weekly_estimate_reason_price_missing';
    case 'cost_missing':
      return 'monitoring.codex_inspection_weekly_estimate_reason_cost_missing';
    default:
      return 'monitoring.codex_inspection_weekly_estimate_reason_data_unavailable';
  }
};

const sourceKey = (estimate: WeeklyPoolEstimate) => {
  switch (estimate.source) {
    case 'cpa_current':
      return 'monitoring.codex_inspection_weekly_source_cpa_current';
    case 'credits_current':
      return 'monitoring.codex_inspection_weekly_source_credits_current';
    case 'cpa_learned':
      return 'monitoring.codex_inspection_weekly_source_cpa_learned';
    case 'credits_learned':
      return 'monitoring.codex_inspection_weekly_source_credits_learned';
    default:
      return 'monitoring.codex_inspection_weekly_source_collecting';
  }
};

const intervalKey = (estimate: WeeklyPoolEstimate) => {
  switch (estimate.intervalKind) {
    case 'cycle_complete':
      return 'monitoring.codex_inspection_weekly_interval_complete';
    case 'cycle_approximate':
      return 'monitoring.codex_inspection_weekly_interval_approximate';
    default:
      return 'monitoring.codex_inspection_weekly_interval_partial';
  }
};

type WeeklyEstimateMethodResultProps = {
  estimate: WeeklyPoolEstimate;
  t: TFunction;
};

function WeeklyEstimateMethodResult({ estimate, t }: WeeklyEstimateMethodResultProps) {
  const hasValue =
    typeof estimate.weeklyPoolUsd === 'number' && Number.isFinite(estimate.weeklyPoolUsd);
  const sources = estimate.priceSources?.filter(Boolean).join(', ') ?? '';
  const syncedAt = estimate.priceSyncedAtMs
    ? new Date(estimate.priceSyncedAtMs).toLocaleString()
    : '';
  const updatedAt = estimate.updatedAtMs ? new Date(estimate.updatedAtMs).toLocaleString() : '';
  const estimateSource = t(sourceKey(estimate));
  const hasRange =
    typeof estimate.weeklyPoolMinUsd === 'number' &&
    Number.isFinite(estimate.weeklyPoolMinUsd) &&
    typeof estimate.weeklyPoolMaxUsd === 'number' &&
    Number.isFinite(estimate.weeklyPoolMaxUsd);
  const hasInterval =
    typeof estimate.intervalStartMs === 'number' &&
    estimate.intervalStartMs > 0 &&
    typeof estimate.intervalEndMs === 'number' &&
    estimate.intervalEndMs > estimate.intervalStartMs;
  const hasQuotaRange =
    typeof estimate.usedPercentMinDelta === 'number' &&
    Number.isFinite(estimate.usedPercentMinDelta) &&
    typeof estimate.usedPercentMaxDelta === 'number' &&
    Number.isFinite(estimate.usedPercentMaxDelta);
  const hasEquationInputs =
    estimate.basis === 'credits'
      ? typeof estimate.credits === 'number' &&
        Number.isFinite(estimate.credits) &&
        estimate.credits > 0 &&
        typeof estimate.usedPercentDelta === 'number' &&
        Number.isFinite(estimate.usedPercentDelta) &&
        estimate.usedPercentDelta > 0
      : typeof estimate.costDeltaUsd === 'number' &&
        Number.isFinite(estimate.costDeltaUsd) &&
        estimate.costDeltaUsd > 0 &&
        typeof estimate.usedPercentDelta === 'number' &&
        Number.isFinite(estimate.usedPercentDelta) &&
        estimate.usedPercentDelta > 0;
  const hasBoundaryProgress =
    estimate.basis === 'credits' &&
    typeof estimate.requiredBoundaryCount === 'number' &&
    estimate.requiredBoundaryCount > 0;

  return (
    <div
      className={styles.weeklyEstimateMethodResult}
      data-basis={estimate.basis}
    >
      <div className={styles.weeklyEstimateMethodHeader}>
        <div className={styles.weeklyEstimateBadges}>
          <span className={`${styles.weeklyEstimateStatus} ${statusClass[estimate.status]}`}>
            {t('monitoring.codex_inspection_weekly_estimate_source_status', {
              source: estimateSource,
              status: t(`monitoring.codex_inspection_weekly_estimate_status_${estimate.status}`),
            })}
          </span>
          {estimate.intervalKind !== 'partial_cycle' ? (
            <span className={styles.weeklyEstimateInterval}>{t(intervalKey(estimate))}</span>
          ) : null}
        </div>
      </div>

      {hasValue ? (
        <>
          <div className={styles.weeklyEstimateValueRow}>
            <strong className={styles.weeklyEstimateValue}>
              {formatUSD(estimate.weeklyPoolUsd!)}
            </strong>
            <span>{t('monitoring.codex_inspection_weekly_per_week')}</span>
          </div>
          {hasRange ? (
            <span className={styles.weeklyEstimateRange}>
              {t('monitoring.codex_inspection_weekly_range', {
                min: formatUSD(estimate.weeklyPoolMinUsd!),
                max: formatUSD(estimate.weeklyPoolMaxUsd!),
              })}
            </span>
          ) : null}
          {hasEquationInputs ? (
            <span className={styles.weeklyEstimateEquation}>
              {estimate.basis === 'credits'
                ? t(
                    estimate.source === 'credits_learned'
                      ? 'monitoring.codex_inspection_weekly_credits_previous_equation'
                      : 'monitoring.codex_inspection_weekly_credits_equation',
                    {
                      credits: estimate.credits!.toFixed(2),
                      percent: formatPercent(estimate.usedPercentDelta!),
                      rate: formatUSD(estimate.usdPerCredit ?? 0.04),
                    }
                  )
                : t('monitoring.codex_inspection_weekly_estimate_equation', {
                    cost: formatUSD(estimate.costDeltaUsd!),
                    percent: formatPercent(estimate.usedPercentDelta!),
                  })}
            </span>
          ) : (
            <span className={styles.weeklyEstimateUpdateHint}>
              {t('monitoring.codex_inspection_weekly_formula_details_missing')}
            </span>
          )}
          {hasQuotaRange ? (
            <span className={styles.weeklyEstimateEvidence}>
              {estimate.basis === 'credits'
                ? t('monitoring.codex_inspection_weekly_credits_evidence', {
                    credits: (estimate.credits ?? 0).toFixed(2),
                    min: formatPercent(estimate.usedPercentMinDelta!),
                    max: formatPercent(estimate.usedPercentMaxDelta!),
                  })
                : t('monitoring.codex_inspection_weekly_cpa_evidence', {
                    cost: formatUSD(estimate.costDeltaUsd ?? 0),
                    min: formatPercent(estimate.usedPercentMinDelta!),
                    max: formatPercent(estimate.usedPercentMaxDelta!),
                  })}
            </span>
          ) : null}
          {hasInterval ? (
            <span className={styles.weeklyEstimateIntervalTime}>
              {estimate.basis === 'credits'
                ? t('monitoring.codex_inspection_weekly_interval_time', {
                    start: formatBoundary(estimate.intervalStartMs!, estimate.analyticsTimezone),
                    end: formatBoundary(estimate.intervalEndMs!, estimate.analyticsTimezone),
                    timezone: estimate.analyticsTimezone ?? '',
                  })
                : t('monitoring.codex_inspection_weekly_cpa_interval_time', {
                    start: formatBoundary(estimate.intervalStartMs!),
                    end: formatBoundary(estimate.intervalEndMs!),
                  })}
            </span>
          ) : null}
          {estimate.waitingForSync ? (
            <span className={styles.weeklyEstimateSyncState}>
              {t('monitoring.codex_inspection_weekly_waiting_sync')}
            </span>
          ) : null}
          <span className={styles.weeklyEstimateUpdateHint}>
            {t('monitoring.codex_inspection_weekly_current_source', {
              source: estimateSource,
              time: updatedAt || t('monitoring.codex_inspection_weekly_time_unknown'),
            })}
          </span>
        </>
      ) : (
        <div className={styles.weeklyEstimateCollecting}>
          {hasBoundaryProgress ? (
            <>
              <span className={styles.weeklyEstimateProgress}>
                {t('monitoring.codex_inspection_weekly_boundary_progress', {
                  current: estimate.closedBoundaryCount ?? 0,
                  required: estimate.requiredBoundaryCount,
                })}
              </span>
              {estimate.nextBoundaryAtMs ? (
                <span className={styles.weeklyEstimateReason}>
                  {t('monitoring.codex_inspection_weekly_boundary_waiting', {
                    date: formatBoundaryDate(
                      estimate.nextBoundaryAtMs,
                      estimate.analyticsTimezone
                    ),
                  })}
                </span>
              ) : (
                <span className={styles.weeklyEstimateReason}>
                  {t(reasonKey(estimate.reason), {
                    percent: formatPercent(estimate.usedPercentDelta ?? 0),
                  })}
                </span>
              )}
            </>
          ) : (
            <span className={styles.weeklyEstimateReason}>
              {t(reasonKey(estimate.reason), {
                percent: formatPercent(estimate.usedPercentDelta ?? 0),
              })}
            </span>
          )}
        </div>
      )}

      {estimate.basis !== 'credits' ? (
        <span className={styles.weeklyEstimateScopeState}>
          {t('monitoring.codex_inspection_weekly_cpa_scope', {
            capture: t(
              `monitoring.codex_inspection_weekly_capture_${estimate.captureState ?? 'pending'}`
            ),
            route: t(
              `monitoring.codex_inspection_weekly_route_${estimate.routeScope ?? 'observed'}`
            ),
            quota: t(
              `monitoring.codex_inspection_weekly_quota_${estimate.quotaScope ?? 'matched'}`
            ),
          })}
        </span>
      ) : null}

      {sources ? (
        <span className={styles.weeklyEstimatePriceSource}>
          {t(
            syncedAt
              ? 'monitoring.codex_inspection_weekly_estimate_price_source_synced'
              : 'monitoring.codex_inspection_weekly_estimate_price_source',
            { sources, time: syncedAt }
          )}
        </span>
      ) : null}
    </div>
  );
}

export function CodexWeeklyPoolEstimate({ estimate, estimates, t }: CodexWeeklyPoolEstimateProps) {
  const methods = (estimates?.length ? estimates : estimate ? [estimate] : []).filter(
    (item, index, items) =>
      items.findIndex(
        (candidate) => candidate.basis === item.basis && candidate.role === item.role
      ) === index
  );
  if (methods.length === 0) return null;
  const currentMethods = methods.filter((item) => item.role !== 'formal_baseline');
  const previousMethods = methods.filter((item) => item.role === 'formal_baseline');
  const hasAnyValue = methods.some(
    (item) => typeof item.weeklyPoolUsd === 'number' && Number.isFinite(item.weeklyPoolUsd)
  );

  return (
    <div className={styles.weeklyEstimate} data-status={estimate?.status}>
      <div className={styles.weeklyEstimateHeader}>
        <span className={styles.weeklyEstimateLabel}>
          {t('monitoring.codex_inspection_weekly_estimate_title')}
        </span>
      </div>
      {currentMethods.length ? (
        <section className={styles.weeklyEstimatePeriod} data-estimate-period="current">
          <h4 className={styles.weeklyEstimatePeriodTitle}>
            {t('monitoring.codex_inspection_weekly_period_current')}
          </h4>
          <div className={styles.weeklyEstimateMethods}>
            {currentMethods.map((method) => (
              <WeeklyEstimateMethodResult
                key={`${method.basis}:${method.role ?? 'default'}`}
                estimate={method}
                t={t}
              />
            ))}
          </div>
        </section>
      ) : null}
      {previousMethods.length ? (
        <section className={styles.weeklyEstimatePeriod} data-estimate-period="previous">
          <h4 className={styles.weeklyEstimatePeriodTitle}>
            {t('monitoring.codex_inspection_weekly_period_previous')}
          </h4>
          <div className={styles.weeklyEstimateMethods}>
            {previousMethods.map((method) => (
              <WeeklyEstimateMethodResult
                key={`${method.basis}:${method.role ?? 'default'}`}
                estimate={method}
                t={t}
              />
            ))}
          </div>
        </section>
      ) : null}
      {!hasAnyValue ? (
        <span className={styles.weeklyEstimateUpdateHint}>
          {t('monitoring.codex_inspection_weekly_measured_refresh')}
        </span>
      ) : null}
      <details className={styles.weeklyEstimateMethod}>
        <summary>{t('monitoring.codex_inspection_estimate_method_title')}</summary>
        <span>{t('monitoring.codex_inspection_estimate_method_api')}</span>
        <span>{t('monitoring.codex_inspection_estimate_method_credits')}</span>
      </details>
      <span className={styles.weeklyEstimateDisclaimer}>
        {t('monitoring.codex_inspection_weekly_estimate_disclaimer')}
      </span>
    </div>
  );
}
