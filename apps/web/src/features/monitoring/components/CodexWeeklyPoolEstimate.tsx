import type { TFunction } from 'i18next';
import type {
  CodexWeeklyPoolEstimate as WeeklyPoolEstimate,
  CodexWeeklyPoolEstimateStatus,
} from '@/features/monitoring/codexInspection';
import styles from '../CodexInspectionPage.module.scss';

type CodexWeeklyPoolEstimateProps = {
  estimate?: WeeklyPoolEstimate | null;
  planType?: string | null;
  usedPercent?: number | null;
  t: TFunction;
};

const PRO_20X_WEEKLY_HEURISTIC_USD = 2000;

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

export function CodexWeeklyPoolEstimate({
  estimate,
  planType,
  usedPercent,
  t,
}: CodexWeeklyPoolEstimateProps) {
  if (!estimate) return null;

  const hasValue =
    typeof estimate.weeklyPoolUsd === 'number' && Number.isFinite(estimate.weeklyPoolUsd);
  const showProHeuristic = !hasValue && planType?.trim().toLowerCase() === 'pro';
  const normalizedUsedPercent =
    typeof usedPercent === 'number' && Number.isFinite(usedPercent)
      ? Math.min(100, Math.max(0, usedPercent))
      : null;
  const heuristicUsedUsd =
    normalizedUsedPercent === null
      ? null
      : (PRO_20X_WEEKLY_HEURISTIC_USD * normalizedUsedPercent) / 100;
  const heuristicRemainingUsd =
    heuristicUsedUsd === null ? null : PRO_20X_WEEKLY_HEURISTIC_USD - heuristicUsedUsd;
  const sources = estimate.priceSources?.filter(Boolean).join(', ') ?? '';
  const syncedAt = estimate.priceSyncedAtMs
    ? new Date(estimate.priceSyncedAtMs).toLocaleString()
    : '';
  const updatedAt = estimate.updatedAtMs ? new Date(estimate.updatedAtMs).toLocaleString() : '';
  const estimateSource = t(sourceKey(estimate));

  return (
    <div className={styles.weeklyEstimate} data-status={estimate.status}>
      <div className={styles.weeklyEstimateHeader}>
        <span className={styles.weeklyEstimateLabel}>
          {t('monitoring.codex_inspection_weekly_estimate_title')}
        </span>
        <span className={`${styles.weeklyEstimateStatus} ${statusClass[estimate.status]}`}>
          {t('monitoring.codex_inspection_weekly_estimate_source_status', {
            source: estimateSource,
            status: t(`monitoring.codex_inspection_weekly_estimate_status_${estimate.status}`),
          })}
        </span>
      </div>

      {hasValue ? (
        <>
          <strong className={styles.weeklyEstimateValue}>
            {formatUSD(estimate.weeklyPoolUsd!)}
          </strong>
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
          <span className={styles.weeklyEstimateUpdateHint}>
            {t('monitoring.codex_inspection_weekly_current_source', {
              source: estimateSource,
              time: updatedAt || t('monitoring.codex_inspection_weekly_time_unknown'),
            })}
          </span>
        </>
      ) : (
        <>
          {showProHeuristic ? (
            <div className={styles.weeklyEstimateHeuristic}>
              <span>{t('monitoring.codex_inspection_weekly_heuristic_title')}</span>
              <strong>≈ {formatUSD(PRO_20X_WEEKLY_HEURISTIC_USD)} / week</strong>
              <small>{t('monitoring.codex_inspection_weekly_heuristic_formula')}</small>
              {normalizedUsedPercent !== null &&
              heuristicUsedUsd !== null &&
              heuristicRemainingUsd !== null ? (
                <small>
                  {t('monitoring.codex_inspection_weekly_heuristic_usage', {
                    percent: formatPercent(normalizedUsedPercent),
                    used: formatUSD(heuristicUsedUsd),
                    remaining: formatUSD(heuristicRemainingUsd),
                  })}
                </small>
              ) : null}
              <small>{t('monitoring.codex_inspection_weekly_heuristic_refresh')}</small>
            </div>
          ) : null}
          <span className={styles.weeklyEstimateReason}>
            {t(reasonKey(estimate.reason), {
              percent: formatPercent(estimate.usedPercentDelta ?? 0),
            })}
          </span>
        </>
      )}

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
      {!hasValue ? (
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
