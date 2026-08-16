package codexinspection

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/model"
	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/service/pricing"
	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/store"
)

const (
	weeklyEstimateBasisAPIEquivalent = "api_equivalent_cost"
	weeklyResetDriftTolerance        = time.Minute

	weeklyEstimateStatusUnavailable  = "unavailable"
	weeklyEstimateStatusInsufficient = "insufficient"
	weeklyEstimateStatusPreliminary  = "preliminary"
	weeklyEstimateStatusReliable     = "reliable"
)

type pendingWeeklyEstimate struct {
	resultIndex int
	baseline    model.CodexInspectionResult
}

func (s *Service) attachWeeklyPoolEstimates(ctx context.Context, results []model.CodexInspectionResult) {
	pending := make([]pendingWeeklyEstimate, 0)
	for index := range results {
		current := &results[index]
		if !strings.EqualFold(strings.TrimSpace(current.Provider), model.CodexInspectionTargetCodex) {
			continue
		}
		weekly := standardWeeklyQuotaWindow(current.QuotaWindows)
		if weekly == nil {
			continue
		}
		estimate := &model.CodexWeeklyPoolEstimate{
			Official:        false,
			Basis:           weeklyEstimateBasisAPIEquivalent,
			Status:          weeklyEstimateStatusUnavailable,
			CurrentAtMS:     current.CreatedAtMS,
			WeeklyResetAtMS: weekly.ResetAtMS,
		}
		current.WeeklyPoolEstimate = estimate

		if strings.TrimSpace(current.AuthIndex) == "" || strings.TrimSpace(current.AccountID) == "" {
			estimate.Reason = "identity_missing"
			continue
		}
		if weekly.UsedPercent == nil || weekly.ResetAtMS <= 0 || current.CreatedAtMS <= 0 {
			estimate.Reason = "weekly_sample_incomplete"
			continue
		}
		fromMS := weekly.ResetAtMS - int64(codexWeekWindow)*1000
		history, err := s.store.ListCodexInspectionResultsByIdentity(
			ctx,
			current.AuthIndex,
			current.AccountID,
			fromMS,
			current.CreatedAtMS,
		)
		if err != nil {
			estimate.Reason = "history_unavailable"
			continue
		}
		baseline, found, quotaDecreased := selectWeeklyBaseline(history, *weekly, *weekly.UsedPercent)
		if !found {
			if quotaDecreased {
				estimate.Reason = "quota_decreased"
			} else {
				estimate.Reason = "baseline_missing"
			}
			continue
		}
		baselineWindow := standardWeeklyQuotaWindow(baseline.QuotaWindows)
		if baselineWindow == nil || baselineWindow.UsedPercent == nil {
			estimate.Reason = "baseline_missing"
			continue
		}
		delta := *weekly.UsedPercent - *baselineWindow.UsedPercent
		estimate.BaselineAtMS = baseline.CreatedAtMS
		estimate.UsedPercentDelta = delta
		if weeklyEstimateStatus(delta) == weeklyEstimateStatusInsufficient {
			estimate.Status = weeklyEstimateStatusInsufficient
			estimate.Reason = "delta_too_small"
			continue
		}
		pending = append(pending, pendingWeeklyEstimate{resultIndex: index, baseline: baseline})
	}

	if len(pending) == 0 {
		return
	}
	_ = s.store.WithModelPriceSnapshot(func() error {
		prices, err := s.store.LoadModelPrices(ctx)
		if err != nil {
			for _, item := range pending {
				results[item.resultIndex].WeeklyPoolEstimate.Reason = "prices_unavailable"
			}
			return nil
		}
		for _, item := range pending {
			current := &results[item.resultIndex]
			estimate := current.WeeklyPoolEstimate
			stats, statsErr := s.store.ModelStatsWithFilter(ctx, store.AnalyticsFilter{
				FromMS:        item.baseline.CreatedAtMS,
				ToMS:          current.CreatedAtMS,
				Providers:     []string{model.CodexInspectionTargetCodex},
				AuthIndices:   []string{current.AuthIndex},
				IncludeFailed: true,
			}, 0)
			if statsErr != nil {
				estimate.Reason = "cost_unavailable"
				continue
			}
			applyWeeklyEstimateCost(estimate, stats, prices)
		}
		return nil
	})
}

func standardWeeklyQuotaWindow(windows []model.CodexInspectionQuotaWindow) *model.CodexInspectionQuotaWindow {
	for index := range windows {
		window := &windows[index]
		if window.ID != "weekly" {
			continue
		}
		if window.LimitWindowSeconds != nil && int(math.Round(*window.LimitWindowSeconds)) != codexWeekWindow {
			continue
		}
		return window
	}
	return nil
}

func selectWeeklyBaseline(history []model.CodexInspectionResult, currentWindow model.CodexInspectionQuotaWindow, currentUsedPercent float64) (model.CodexInspectionResult, bool, bool) {
	var selected model.CodexInspectionResult
	selectedUsedPercent := math.Inf(1)
	found := false
	quotaDecreased := false
	for _, candidate := range history {
		window := standardWeeklyQuotaWindow(candidate.QuotaWindows)
		if window == nil || window.UsedPercent == nil || !sameWeeklyResetWindow(window.ResetAtMS, currentWindow.ResetAtMS) {
			continue
		}
		if *window.UsedPercent > currentUsedPercent {
			quotaDecreased = true
			continue
		}
		if !found || *window.UsedPercent < selectedUsedPercent ||
			(*window.UsedPercent == selectedUsedPercent && candidate.CreatedAtMS < selected.CreatedAtMS) {
			selected = candidate
			selectedUsedPercent = *window.UsedPercent
			found = true
		}
	}
	return selected, found, quotaDecreased
}

func sameWeeklyResetWindow(leftMS, rightMS int64) bool {
	if leftMS <= 0 || rightMS <= 0 {
		return false
	}
	delta := leftMS - rightMS
	if delta < 0 {
		delta = -delta
	}
	return delta <= int64(weeklyResetDriftTolerance/time.Millisecond)
}

func weeklyEstimateStatus(delta float64) string {
	switch {
	case math.IsNaN(delta), math.IsInf(delta, 0), delta < 1:
		return weeklyEstimateStatusInsufficient
	case delta < 5:
		return weeklyEstimateStatusPreliminary
	default:
		return weeklyEstimateStatusReliable
	}
}

func applyWeeklyEstimateCost(estimate *model.CodexWeeklyPoolEstimate, stats []store.ModelStat, prices map[string]store.ModelPrice) {
	if estimate == nil {
		return
	}
	totalCost := 0.0
	hasUsage := false
	hasUnpricedUsage := false
	sources := map[string]struct{}{}
	for _, stat := range stats {
		tokens := pricing.ModelTokens{
			PricingModel:            stat.PricingModel,
			ContextThresholdTokens:  stat.ContextThresholdTokens,
			InputTokens:             stat.InputTokens,
			OutputTokens:            stat.OutputTokens,
			CachedTokens:            stat.CachedTokens,
			CacheReadTokens:         stat.CacheReadTokens,
			CacheCreationTokens:     stat.CacheCreationTokens,
			LongInputTokens:         stat.LongInputTokens,
			LongOutputTokens:        stat.LongOutputTokens,
			LongCachedTokens:        stat.LongCachedTokens,
			LongCacheReadTokens:     stat.LongCacheReadTokens,
			LongCacheCreationTokens: stat.LongCacheCreationTokens,
		}
		if !hasBillableTokens(tokens) {
			continue
		}
		hasUsage = true
		cost := pricing.CostForModelCandidatesWithServiceTier(
			[]string{stat.BillingModel, stat.Model},
			stat.ServiceTier,
			tokens,
			prices,
		)
		if cost <= 0 {
			hasUnpricedUsage = true
			continue
		}
		totalCost += cost
		source, syncedAtMS, updatedAtMS := priceMetadataForStat(stat, prices)
		if source != "" {
			sources[source] = struct{}{}
		}
		if syncedAtMS > estimate.PriceSyncedAtMS {
			estimate.PriceSyncedAtMS = syncedAtMS
		}
		if updatedAtMS > estimate.PriceUpdatedAtMS {
			estimate.PriceUpdatedAtMS = updatedAtMS
		}
	}
	if !hasUsage {
		estimate.Reason = "cost_missing"
		return
	}
	if hasUnpricedUsage {
		estimate.Reason = "price_missing"
		return
	}
	if totalCost <= 0 || estimate.UsedPercentDelta <= 0 {
		estimate.Reason = "cost_missing"
		return
	}
	value := totalCost / (estimate.UsedPercentDelta / 100)
	estimate.WeeklyPoolUSD = &value
	estimate.CostDeltaUSD = totalCost
	estimate.Status = weeklyEstimateStatus(estimate.UsedPercentDelta)
	estimate.Reason = ""
	estimate.PriceSources = make([]string, 0, len(sources))
	for source := range sources {
		estimate.PriceSources = append(estimate.PriceSources, source)
	}
	sort.Strings(estimate.PriceSources)
}

func hasBillableTokens(tokens pricing.ModelTokens) bool {
	return tokens.InputTokens > 0 || tokens.OutputTokens > 0 || tokens.CachedTokens > 0 ||
		tokens.CacheReadTokens > 0 || tokens.CacheCreationTokens > 0
}

func priceMetadataForStat(stat store.ModelStat, prices map[string]store.ModelPrice) (string, int64, int64) {
	for _, modelName := range []string{stat.PricingModel, stat.BillingModel, stat.Model} {
		if strings.TrimSpace(modelName) == "" {
			continue
		}
		price, ok := prices[modelName]
		if !ok {
			continue
		}
		source := strings.TrimSpace(price.Source)
		if source == "" {
			source = "manual"
		}
		syncedAtMS := int64(0)
		if price.SyncedAtMS != nil {
			syncedAtMS = *price.SyncedAtMS
		}
		return source, syncedAtMS, price.UpdatedAtMS
	}
	return "openai_builtin", 0, 0
}
