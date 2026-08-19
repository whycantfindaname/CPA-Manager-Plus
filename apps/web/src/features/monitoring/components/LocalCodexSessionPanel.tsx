import { useMemo } from 'react';
import type { TFunction } from 'i18next';
import { Button } from '@/components/ui/Button';
import type {
  CodexInspectionResult,
  LocalCodexQuotaBucket,
  LocalCodexQuotaWindow,
  LocalCodexSessionResponse,
} from '@/services/api/usageService';
import styles from '../CodexInspectionPage.module.scss';
import { CodexInspectionQuotaWindows } from './CodexInspectionQuotaWindows';
import { Panel } from './CodexInspectionPanels';

type LocalCodexSessionPanelProps = {
  response: LocalCodexSessionResponse | null;
  comparison: CodexInspectionResult | null;
  loading: boolean;
  error: string;
  locale: string;
  t: TFunction;
  onRefresh: () => void;
};

type FlatLocalWindow = {
  id: string;
  name: string;
  durationSeconds: number | null;
  usedPercent: number | null;
  resetAtMs: number | null;
};

const finiteNumber = (value: number | null | undefined) =>
  typeof value === 'number' && Number.isFinite(value) ? value : null;

const flattenWindow = (
  bucket: LocalCodexQuotaBucket,
  kind: 'primary' | 'secondary',
  window: LocalCodexQuotaWindow | null | undefined
): FlatLocalWindow | null => {
  if (!window) return null;
  const durationMinutes = finiteNumber(window.windowDurationMins);
  const resetSeconds = finiteNumber(window.resetsAt);
  return {
    id: `${bucket.id}-${kind}`,
    name: bucket.name || bucket.id,
    durationSeconds: durationMinutes === null ? null : durationMinutes * 60,
    usedPercent: finiteNumber(window.usedPercent),
    resetAtMs: resetSeconds === null ? null : resetSeconds * 1000,
  };
};

const flattenBuckets = (buckets: readonly LocalCodexQuotaBucket[]) =>
  buckets.flatMap((bucket) =>
    [
      flattenWindow(bucket, 'primary', bucket.primary),
      flattenWindow(bucket, 'secondary', bucket.secondary),
    ].filter((window): window is FlatLocalWindow => Boolean(window))
  );

const normalizedName = (value?: string) => value?.trim().toLowerCase() ?? '';

const compareWithCPA = (
  localWindows: readonly FlatLocalWindow[],
  comparison: CodexInspectionResult | null
) => {
  if (!comparison?.quotaWindows?.length) return null;
  const unusedCPAWindows = [...comparison.quotaWindows];
  let matched = 0;
  let maxUsedDelta = 0;
  let maxResetDeltaSeconds = 0;

  for (const local of localWindows) {
    const namedIndex = unusedCPAWindows.findIndex(
      (window) =>
        normalizedName(String(window.labelParams?.name ?? '')) === normalizedName(local.name)
    );
    const durationIndex = unusedCPAWindows.findIndex(
      (window) =>
        local.durationSeconds !== null && window.limitWindowSeconds === local.durationSeconds
    );
    const index = namedIndex >= 0 ? namedIndex : durationIndex;
    if (index < 0) continue;
    const [cpaWindow] = unusedCPAWindows.splice(index, 1);
    matched += 1;
    const cpaUsed = finiteNumber(cpaWindow.usedPercent);
    if (local.usedPercent !== null && cpaUsed !== null) {
      maxUsedDelta = Math.max(maxUsedDelta, Math.abs(local.usedPercent - cpaUsed));
    }
    const cpaResetAtMs = finiteNumber(cpaWindow.resetAtMs);
    if (local.resetAtMs !== null && cpaResetAtMs !== null) {
      maxResetDeltaSeconds = Math.max(
        maxResetDeltaSeconds,
        Math.abs(local.resetAtMs - cpaResetAtMs) / 1000
      );
    }
  }

  return { matched, total: localWindows.length, maxUsedDelta, maxResetDeltaSeconds };
};

export function LocalCodexSessionPanel({
  response,
  comparison,
  loading,
  error,
  locale,
  t,
  onRefresh,
}: LocalCodexSessionPanelProps) {
  const snapshot = response?.snapshot;
  const localWindows = useMemo(
    () => flattenBuckets(snapshot?.quotaBuckets ?? []),
    [snapshot?.quotaBuckets]
  );
  const quotaWindows = useMemo(
    () =>
      localWindows.map((window) => ({
        id: window.id,
        labelKey: 'monitoring.local_codex_session_quota_bucket',
        labelParams: { name: window.name },
        usedPercent: window.usedPercent,
        resetAtMs: window.resetAtMs,
        resetLabel: window.resetAtMs ? new Date(window.resetAtMs).toISOString() : undefined,
      })),
    [localWindows]
  );
  const sourceComparison = useMemo(
    () => compareWithCPA(localWindows, comparison),
    [comparison, localWindows]
  );
  const dailyBuckets = snapshot?.usage.dailyUsageBuckets ?? [];
  const latestDailyBucket = dailyBuckets[dailyBuckets.length - 1];
  const available = response?.status === 'available' && Boolean(snapshot);
  const statusLabel = available
    ? t('monitoring.local_codex_session_available')
    : t('monitoring.local_codex_session_unavailable');

  return (
    <Panel className={styles.localSessionPanel}>
      <div className={styles.panelHeader}>
        <div className={styles.panelHeading}>
          <div className={styles.localSessionTitleRow}>
            <h2 className={styles.panelTitle}>{t('monitoring.local_codex_session_title')}</h2>
            <span
              className={`${styles.statusBadge} ${
                available ? styles['tone-good'] : styles['tone-idle']
              }`}
            >
              <span className={styles.statusDot} aria-hidden="true" />
              {statusLabel}
            </span>
          </div>
          <p className={styles.panelSubtitle}>{t('monitoring.local_codex_session_desc')}</p>
        </div>
        <div className={styles.panelExtra}>
          <span className={styles.localSessionSource}>codex app-server</span>
          <Button variant="secondary" size="sm" onClick={onRefresh} loading={loading}>
            {t('common.refresh')}
          </Button>
        </div>
      </div>

      {error ? (
        <div className={styles.localSessionMessage} role="alert">
          {error}
        </div>
      ) : null}

      {!available && !error ? (
        <div className={styles.localSessionMessage}>
          {response?.reason === 'codex_executable_not_found'
            ? t('monitoring.local_codex_session_missing_runtime')
            : t('monitoring.local_codex_session_probe_failed')}
        </div>
      ) : null}

      {available && snapshot ? (
        <div className={styles.localSessionContent}>
          <div className={styles.localSessionIdentity}>
            <div>
              <span className={styles.localSessionEyebrow}>
                {t('monitoring.local_codex_session_account')}
              </span>
              <strong>{snapshot.account.email || t('common.unknown')}</strong>
            </div>
            <div>
              <span className={styles.localSessionEyebrow}>
                {t('monitoring.local_codex_session_plan')}
              </span>
              <strong>{snapshot.account.planType || '--'}</strong>
            </div>
            <div>
              <span className={styles.localSessionEyebrow}>
                {t('monitoring.local_codex_session_captured_at')}
              </span>
              <strong>{new Date(snapshot.capturedAtMs).toLocaleTimeString(locale)}</strong>
            </div>
          </div>

          <CodexInspectionQuotaWindows windows={quotaWindows} t={t} />

          <div className={styles.localSessionEvidenceRow}>
            <div>
              <span className={styles.localSessionEyebrow}>
                {t('monitoring.local_codex_session_daily_activity')}
              </span>
              <strong>
                {latestDailyBucket
                  ? t('monitoring.local_codex_session_daily_tokens', {
                      date: latestDailyBucket.startDate,
                      tokens: latestDailyBucket.tokens.toLocaleString(locale),
                    })
                  : '--'}
              </strong>
              <span>{t('monitoring.local_codex_session_daily_tokens_hint')}</span>
            </div>
            <div>
              <span className={styles.localSessionEyebrow}>
                {t('monitoring.local_codex_session_cpa_comparison')}
              </span>
              <strong>
                {sourceComparison
                  ? t('monitoring.local_codex_session_comparison_result', {
                      matched: sourceComparison.matched,
                      total: sourceComparison.total,
                      percent: sourceComparison.maxUsedDelta.toFixed(1),
                      seconds: Math.round(sourceComparison.maxResetDeltaSeconds),
                    })
                  : t('monitoring.local_codex_session_comparison_unavailable')}
              </strong>
              <span>{t('monitoring.local_codex_session_comparison_hint')}</span>
            </div>
          </div>
        </div>
      ) : null}
    </Panel>
  );
}
