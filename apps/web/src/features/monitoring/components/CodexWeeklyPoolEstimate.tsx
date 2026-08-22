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

const roleKey = (estimate: WeeklyPoolEstimate) =>
  estimate.role === 'formal_baseline'
    ? 'monitoring.codex_inspection_weekly_role_formal'
    : 'monitoring.codex_inspection_weekly_role_current';

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

  return (
    <div
      className={styles.weeklyEstimateMethodResult}
      data-basis={estimate.basis}
    >
      <div className={styles.weeklyEstimateMethodHeader}>
        <div className={styles.weeklyEstimateBadges}>
          <span className={styles.weeklyEstimateRole}>{t(roleKey(estimate))}</span>
          <span className={`${styles.weeklyEstimateStatus} ${statusClass[estimate.status]}`}>
            {t('monitoring.codex_inspection_weekly_estimate_source_status', {
              source: estimateSource,
              status: t(`monitoring.codex_inspection_weekly_estimate_status_${estimate.status}`),
            })}
          </span>
          <span className={styles.weeklyEstimateInterval}>{t(intervalKey(estimate))}</span>
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
          <span className={styles.weeklyEstimateEquation}>
            {estimate.basis === 'credits'
              ? t(
                  estimate.source === 'credits_learned'
                    ? 'monitoring.codex_inspection_weekly_credits_previous_equation'
                    : 'monitoring.codex_inspection_weekly_credits_equation',
                  {
                    credits: (estimate.credits ?? 0).toFixed(2),
                    percent: formatPercent(estimate.usedPercentDelta ?? 0),
                    rate: formatUSD(estimate.usdPerCredit ?? 0.04),
                  }
                )
              : t('monitoring.codex_inspection_weekly_estimate_equation', {
                  cost: formatUSD(estimate.costDeltaUsd ?? 0),
                  percent: formatPercent(estimate.usedPercentDelta ?? 0),
                })}
          </span>
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
        <span className={styles.weeklyEstimateReason}>
          {t(reasonKey(estimate.reason), {
            percent: formatPercent(estimate.usedPercentDelta ?? 0),
          })}
        </span>
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
      <div className={styles.weeklyEstimateMethods}>
        {methods.map((method) => (
          <WeeklyEstimateMethodResult
            key={`${method.basis}:${method.role ?? 'default'}`}
            estimate={method}
            t={t}
          />
        ))}
      </div>
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
