package codexinspection

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/model"
	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/service/pricing"
	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/store"
)

const (
	weeklyEstimateBasisAPIEquivalent   = "api_equivalent_cost"
	weeklyEstimateBasisCredits         = "credits"
	weeklyEstimateSourceCPACurrent     = "cpa_current"
	weeklyEstimateSourceCreditsCurrent = "credits_current"
	weeklyEstimateSourceCPALearned     = "cpa_learned"
	weeklyEstimateSourceCreditsLearned = "credits_learned"
	weeklyEstimateUSDPerCredit         = 0.04
	weeklyResetDriftTolerance          = time.Minute
	creditsQuotaBoundaryTolerance      = 15 * time.Minute
	cpaCaptureCloseDelay               = 2 * time.Minute
	cpaQuotaResolutionPP               = 1.0
	cpaCalculationVersion              = "cpa_matched_interval_v2"
	creditsCalculationVersion          = "credits_closed_interval_v1"
	weeklyEstimateRoleCurrent          = "current_estimate"
	weeklyEstimateRoleFormal           = "formal_baseline"
	weeklyEstimateIntervalPartial      = "partial_cycle"
	weeklyEstimateIntervalComplete     = "cycle_complete"
	weeklyEstimateIntervalApproximate  = "cycle_approximate"
	weeklyEstimateQuotaKind            = "weekly"

	weeklyEstimateStatusUnavailable  = "unavailable"
	weeklyEstimateStatusInsufficient = "insufficient"
	weeklyEstimateStatusPreliminary  = "preliminary"
	weeklyEstimateStatusReliable     = "reliable"
)

type pendingWeeklyEstimate struct {
	resultIndex int
	baseline    model.CodexInspectionResult
	endpoint    model.CodexInspectionResult
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
			Official:           false,
			Basis:              weeklyEstimateBasisAPIEquivalent,
			Role:               weeklyEstimateRoleCurrent,
			IntervalKind:       weeklyEstimateIntervalPartial,
			CalculationVersion: cpaCalculationVersion,
			Status:             weeklyEstimateStatusUnavailable,
			CurrentAtMS:        current.CreatedAtMS,
			WeeklyResetAtMS:    weekly.ResetAtMS,
			QuotaKind:          weeklyEstimateQuotaKind,
			CaptureState:       "pending",
			RouteScope:         "observed",
			QuotaScope:         cpaQuotaScope(current.QuotaWindows),
			QuotaResolutionPP:  cpaQuotaResolutionPP,
		}
		current.WeeklyPoolEstimate = estimate

		if strings.TrimSpace(current.AuthIndex) == "" || strings.TrimSpace(current.AccountID) == "" || strings.TrimSpace(current.AccountSnapshot) == "" {
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
		baseline, endpoint, found, reason := selectClosedCPAInterval(history, *current, *weekly)
		if !found {
			estimate.Reason = reason
			continue
		}
		baselineWindow := standardWeeklyQuotaWindow(baseline.QuotaWindows)
		endpointWindow := standardWeeklyQuotaWindow(endpoint.QuotaWindows)
		if baselineWindow == nil || baselineWindow.UsedPercent == nil || endpointWindow == nil || endpointWindow.UsedPercent == nil {
			estimate.Reason = "baseline_missing"
			continue
		}
		delta := *endpointWindow.UsedPercent - *baselineWindow.UsedPercent
		startMin, startMax := quotaValueBounds(*baselineWindow.UsedPercent, cpaQuotaResolutionPP)
		endMin, endMax := quotaValueBounds(*endpointWindow.UsedPercent, cpaQuotaResolutionPP)
		minDelta := endMin - startMax
		maxDelta := endMax - startMin
		estimate.BaselineAtMS = baseline.CreatedAtMS
		estimate.CurrentAtMS = endpoint.CreatedAtMS
		estimate.IntervalStartMS = baseline.CreatedAtMS
		estimate.IntervalEndMS = endpoint.CreatedAtMS
		estimate.CaptureState = "closed"
		estimate.UsedPercentDelta = delta
		estimate.UsedPercentMinDelta = minDelta
		estimate.UsedPercentMaxDelta = maxDelta
		if minDelta < 1 || delta <= 0 {
			estimate.Status = weeklyEstimateStatusInsufficient
			estimate.Reason = "delta_too_small"
			continue
		}
		pending = append(pending, pendingWeeklyEstimate{resultIndex: index, baseline: baseline, endpoint: endpoint})
	}

	if len(pending) > 0 {
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
					ToMS:          item.endpoint.CreatedAtMS,
					Providers:     []string{model.CodexInspectionTargetCodex},
					Accounts:      []string{current.AccountSnapshot},
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
	s.adoptWeeklyEstimateBaselines(ctx, results)
}

func (s *Service) adoptWeeklyEstimateBaselines(ctx context.Context, results []model.CodexInspectionResult) {
	for index := range results {
		current := &results[index]
		apiCurrent := current.WeeklyPoolEstimate
		if apiCurrent == nil {
			continue
		}
		learned, err := s.store.ListCodexWeeklyEstimateBaselines(ctx, current.AuthIndex, current.AccountID)
		if err != nil {
			learned = nil
		}
		weekly := standardWeeklyQuotaWindow(current.QuotaWindows)
		if weekly != nil {
			previousAPI := weeklyEstimateForBasisRole(learned, weeklyEstimateBasisAPIEquivalent, weeklyEstimateRoleCurrent)
			existingFormal := weeklyEstimateForBasisRole(learned, weeklyEstimateBasisAPIEquivalent, weeklyEstimateRoleFormal)
			alreadyPromoted := existingFormal != nil && previousAPI != nil && sameWeeklyResetWindow(existingFormal.WeeklyResetAtMS, previousAPI.WeeklyResetAtMS)
			if cpaEstimateFormalEligible(previousAPI) && !alreadyPromoted && !sameWeeklyResetWindow(previousAPI.WeeklyResetAtMS, weekly.ResetAtMS) {
				promoted := *previousAPI
				promoted.Source = weeklyEstimateSourceCPALearned
				promoted.Role = weeklyEstimateRoleFormal
				promoted.IntervalKind = weeklyEstimateIntervalApproximate
				promoted.WaitingForSync = false
				_ = s.store.UpsertCodexWeeklyEstimateBaseline(ctx, current.AuthIndex, current.AccountID, promoted)
			}
		}
		if apiCurrent.WeeklyPoolUSD != nil {
			apiCurrent.Source = weeklyEstimateSourceCPACurrent
			apiCurrent.UpdatedAtMS = current.CreatedAtMS
			_ = s.store.UpsertCodexWeeklyEstimateBaseline(ctx, current.AuthIndex, current.AccountID, *apiCurrent)
		}

		learned, err = s.store.ListCodexWeeklyEstimateBaselines(ctx, current.AuthIndex, current.AccountID)
		if err != nil {
			learned = nil
		}
		if weekly != nil {
			previousCurrent := weeklyEstimateForBasisRole(learned, weeklyEstimateBasisCredits, weeklyEstimateRoleCurrent)
			existingFormal := weeklyEstimateForBasisRole(learned, weeklyEstimateBasisCredits, weeklyEstimateRoleFormal)
			if creditsEstimateCompatible(previousCurrent) &&
				!sameWeeklyResetWindow(previousCurrent.WeeklyResetAtMS, weekly.ResetAtMS) &&
				!weeklyEstimateComplete(existingFormal) {
				promoted := *previousCurrent
				promoted.Source = weeklyEstimateSourceCreditsLearned
				promoted.Role = weeklyEstimateRoleFormal
				if promoted.IntervalKind != weeklyEstimateIntervalComplete {
					promoted.IntervalKind = weeklyEstimateIntervalApproximate
				}
				promoted.WaitingForSync = false
				promoted.UpdatedAtMS = current.CreatedAtMS
				_ = s.store.UpsertCodexWeeklyEstimateBaseline(ctx, current.AuthIndex, current.AccountID, promoted)
			}
		}

		creditsFormal := s.previousCycleCompleteCreditsEstimate(ctx, *current)
		if creditsFormal != nil && creditsFormal.WeeklyPoolUSD != nil {
			_ = s.store.UpsertCodexWeeklyEstimateBaseline(ctx, current.AuthIndex, current.AccountID, *creditsFormal)
		}
		creditsCurrent := s.currentCreditsWeeklyEstimate(ctx, *current)
		if creditsCurrent != nil && creditsCurrent.WeeklyPoolUSD != nil {
			_ = s.store.UpsertCodexWeeklyEstimateBaseline(ctx, current.AuthIndex, current.AccountID, *creditsCurrent)
		}

		learned, err = s.store.ListCodexWeeklyEstimateBaselines(ctx, current.AuthIndex, current.AccountID)
		if err != nil {
			learned = nil
		}
		apiCurrentDisplay := apiCurrent
		if apiCurrentDisplay.WeeklyPoolUSD == nil && weekly != nil {
			baseline := weeklyEstimateForBasisRole(learned, weeklyEstimateBasisAPIEquivalent, weeklyEstimateRoleCurrent)
			if cpaEstimateCompatible(baseline) && sameWeeklyResetWindow(baseline.WeeklyResetAtMS, weekly.ResetAtMS) {
				apiCurrentDisplay = learnedWeeklyEstimate(*baseline, true)
			}
		}
		apiFormalDisplay := weeklyEstimateForBasisRole(learned, weeklyEstimateBasisAPIEquivalent, weeklyEstimateRoleFormal)
		if !cpaEstimateCompatible(apiFormalDisplay) {
			apiFormalDisplay = nil
		}
		creditsFormalDisplay := creditsFormal
		if creditsFormalDisplay == nil {
			baseline := weeklyEstimateForBasisRole(learned, weeklyEstimateBasisCredits, weeklyEstimateRoleFormal)
			if creditsEstimateCompatible(baseline) {
				creditsFormalDisplay = learnedWeeklyEstimate(*baseline, false)
			}
		}
		creditsCurrentDisplay := creditsCurrent
		if creditsCurrentDisplay == nil && weekly != nil {
			baseline := weeklyEstimateForBasisRole(learned, weeklyEstimateBasisCredits, weeklyEstimateRoleCurrent)
			if creditsEstimateCompatible(baseline) && sameWeeklyResetWindow(baseline.WeeklyResetAtMS, weekly.ResetAtMS) {
				creditsCurrentDisplay = learnedWeeklyEstimate(*baseline, true)
			}
		}

		current.WeeklyPoolEstimates = make([]model.CodexWeeklyPoolEstimate, 0, 4)
		if apiFormalDisplay != nil {
			current.WeeklyPoolEstimates = append(current.WeeklyPoolEstimates, *apiFormalDisplay)
		}
		if apiCurrentDisplay != nil {
			current.WeeklyPoolEstimates = append(current.WeeklyPoolEstimates, *apiCurrentDisplay)
		}
		if creditsFormalDisplay != nil {
			current.WeeklyPoolEstimates = append(current.WeeklyPoolEstimates, *creditsFormalDisplay)
		}
		if creditsCurrentDisplay != nil {
			current.WeeklyPoolEstimates = append(current.WeeklyPoolEstimates, *creditsCurrentDisplay)
		}

		switch {
		case weeklyEstimateComplete(apiCurrentDisplay):
			current.WeeklyPoolEstimate = apiCurrentDisplay
		case weeklyEstimateComplete(creditsCurrentDisplay):
			current.WeeklyPoolEstimate = creditsCurrentDisplay
		case weeklyEstimateComplete(apiFormalDisplay):
			current.WeeklyPoolEstimate = apiFormalDisplay
		case weeklyEstimateComplete(creditsFormalDisplay):
			current.WeeklyPoolEstimate = creditsFormalDisplay
		case apiCurrentDisplay != nil && apiCurrentDisplay.WeeklyPoolUSD != nil:
			current.WeeklyPoolEstimate = apiCurrentDisplay
		case creditsCurrent != nil && creditsCurrent.WeeklyPoolUSD != nil:
			current.WeeklyPoolEstimate = creditsCurrentDisplay
		case apiFormalDisplay != nil && apiFormalDisplay.WeeklyPoolUSD != nil:
			current.WeeklyPoolEstimate = apiFormalDisplay
		case creditsFormalDisplay != nil && creditsFormalDisplay.WeeklyPoolUSD != nil:
			current.WeeklyPoolEstimate = creditsFormalDisplay
		case creditsCurrentDisplay != nil && creditsCurrentDisplay.WeeklyPoolUSD != nil:
			current.WeeklyPoolEstimate = creditsCurrentDisplay
		default:
			current.WeeklyPoolEstimate = apiCurrentDisplay
		}
	}
}

func weeklyEstimateForBasis(estimates []model.CodexWeeklyPoolEstimate, basis string) *model.CodexWeeklyPoolEstimate {
	for index := range estimates {
		if estimates[index].Basis == basis && estimates[index].WeeklyPoolUSD != nil {
			return &estimates[index]
		}
	}
	return nil
}

func weeklyEstimateForBasisRole(estimates []model.CodexWeeklyPoolEstimate, basis, role string) *model.CodexWeeklyPoolEstimate {
	for index := range estimates {
		if estimates[index].Basis == basis && estimates[index].Role == role && estimates[index].WeeklyPoolUSD != nil {
			return &estimates[index]
		}
	}
	return nil
}

func creditsEstimateCompatible(estimate *model.CodexWeeklyPoolEstimate) bool {
	return estimate != nil && estimate.CalculationVersion == creditsCalculationVersion
}

func weeklyEstimateComplete(estimate *model.CodexWeeklyPoolEstimate) bool {
	return estimate != nil && estimate.WeeklyPoolUSD != nil && estimate.IntervalKind == weeklyEstimateIntervalComplete
}

func learnedWeeklyEstimate(estimate model.CodexWeeklyPoolEstimate, waitingForSync bool) *model.CodexWeeklyPoolEstimate {
	switch estimate.Basis {
	case weeklyEstimateBasisCredits:
		estimate.Source = weeklyEstimateSourceCreditsLearned
	default:
		estimate.Source = weeklyEstimateSourceCPALearned
	}
	estimate.WaitingForSync = waitingForSync
	return &estimate
}

func (s *Service) previousCycleCompleteCreditsEstimate(ctx context.Context, result model.CodexInspectionResult) *model.CodexWeeklyPoolEstimate {
	weekly := standardWeeklyQuotaWindow(result.QuotaWindows)
	usage := result.CreditsUsage
	if weekly == nil || weekly.ResetAtMS <= 0 || usage == nil || usage.PreviousCycleCredits <= 0 || usage.PreviousCycleStartDate == "" || usage.CycleStartDate == "" || usage.AnalyticsTimezone == "" {
		return nil
	}
	previousResetAtMS := weekly.ResetAtMS - int64(codexWeekWindow)*1000
	startBoundaryMS, ok := creditsBoundaryMS(usage.PreviousCycleStartDate, usage.AnalyticsTimezone)
	if !ok {
		return nil
	}
	endBoundaryMS, ok := creditsBoundaryMS(usage.CycleStartDate, usage.AnalyticsTimezone)
	if !ok || endBoundaryMS-startBoundaryMS != int64(codexWeekWindow)*1000 {
		return nil
	}
	latestBoundaryMS, ok := creditsBoundaryMS(usage.LatestDate, usage.AnalyticsTimezone)
	if !ok || latestBoundaryMS < endBoundaryMS {
		return nil
	}
	var previous model.CodexInspectionResult
	found := false
	if endBoundaryMS == previousResetAtMS && startBoundaryMS == previousResetAtMS-int64(codexWeekWindow)*1000 {
		previous, found = s.previousWeeklySample(ctx, result, previousResetAtMS)
	}
	if !found {
		previous, found = s.previousWeeklySampleAtResetTransition(ctx, result, startBoundaryMS)
	}
	previousWindow := standardWeeklyQuotaWindow(previous.QuotaWindows)
	if !found || previousWindow == nil || previousWindow.UsedPercent == nil || *previousWindow.UsedPercent < 5 {
		return nil
	}
	credits := usage.PreviousCycleCredits
	usedPercent := math.Min(100, *previousWindow.UsedPercent)
	cost := credits * weeklyEstimateUSDPerCredit
	value := cost / (usedPercent / 100)
	minValue := cost
	maxValue := value
	return &model.CodexWeeklyPoolEstimate{
		Official:            false,
		Basis:               weeklyEstimateBasisCredits,
		Source:              weeklyEstimateSourceCreditsCurrent,
		Role:                weeklyEstimateRoleFormal,
		IntervalKind:        weeklyEstimateIntervalComplete,
		CalculationVersion:  creditsCalculationVersion,
		Status:              creditsEstimateStatus(value, minValue, maxValue, usedPercent),
		WeeklyPoolUSD:       &value,
		WeeklyPoolMinUSD:    &minValue,
		WeeklyPoolMaxUSD:    &maxValue,
		CostDeltaUSD:        cost,
		UsedPercentDelta:    usedPercent,
		UsedPercentMinDelta: usedPercent,
		UsedPercentMaxDelta: 100,
		BaselineAtMS:        previous.CreatedAtMS,
		CurrentAtMS:         previous.CreatedAtMS,
		IntervalStartMS:     startBoundaryMS,
		IntervalEndMS:       endBoundaryMS,
		WeeklyResetAtMS:     previousResetAtMS,
		QuotaKind:           weeklyEstimateQuotaKind,
		AnalyticsTimezone:   usage.AnalyticsTimezone,
		Credits:             credits,
		USDPerCredit:        weeklyEstimateUSDPerCredit,
		UpdatedAtMS:         usage.ObservedAtMS,
	}
}

func (s *Service) currentCreditsWeeklyEstimate(ctx context.Context, result model.CodexInspectionResult) *model.CodexWeeklyPoolEstimate {
	weekly := standardWeeklyQuotaWindow(result.QuotaWindows)
	usage := result.CreditsUsage
	if weekly == nil || weekly.ResetAtMS <= 0 || usage == nil || usage.CycleStartDate == "" || usage.ClosedBoundaryDate == "" || usage.AnalyticsTimezone == "" {
		return nil
	}
	fromMS := weekly.ResetAtMS - int64(codexWeekWindow)*1000
	history, err := s.store.ListCodexInspectionResultsByIdentity(ctx, result.AuthIndex, result.AccountID, fromMS, result.CreatedAtMS)
	if err != nil {
		return nil
	}
	history = append(history, result)
	boundaries := closedCreditsBoundarySamples(history, usage.CycleStartDate, usage.AnalyticsTimezone)
	if len(boundaries) < 2 {
		return nil
	}
	cycleStartMS := weekly.ResetAtMS - int64(codexWeekWindow)*1000
	var selected *model.CodexWeeklyPoolEstimate
	for startIndex := 0; startIndex < len(boundaries)-1; startIndex++ {
		startEnvelope, ok := quotaEnvelopeAtBoundary(history, weekly.ResetAtMS, boundaries[startIndex].boundaryMS, cycleStartMS)
		if !ok {
			continue
		}
		for endIndex := startIndex + 1; endIndex < len(boundaries); endIndex++ {
			endEnvelope, ok := quotaEnvelopeAtBoundary(history, weekly.ResetAtMS, boundaries[endIndex].boundaryMS, cycleStartMS)
			if !ok {
				continue
			}
			candidate := closedIntervalCreditsEstimate(boundaries[startIndex], boundaries[endIndex], startEnvelope, endEnvelope, *weekly)
			if candidate == nil {
				continue
			}
			if selected == nil || candidate.IntervalEndMS-candidate.IntervalStartMS > selected.IntervalEndMS-selected.IntervalStartMS {
				selected = candidate
			}
		}
	}
	return selected
}

type creditsBoundarySample struct {
	credits      float64
	boundaryMS   int64
	observedAtMS int64
	timezone     string
}

func closedCreditsBoundarySamples(history []model.CodexInspectionResult, cycleStartDate, timezone string) []creditsBoundarySample {
	byBoundary := map[int64]creditsBoundarySample{}
	for _, candidate := range history {
		usage := candidate.CreditsUsage
		if usage == nil || usage.CycleStartDate != cycleStartDate || usage.AnalyticsTimezone != timezone || usage.ClosedBoundaryDate == "" {
			continue
		}
		boundaryMS, ok := creditsBoundaryMS(usage.ClosedBoundaryDate, timezone)
		if !ok {
			continue
		}
		sample := creditsBoundarySample{credits: usage.ClosedCycleCredits, boundaryMS: boundaryMS, observedAtMS: usage.ObservedAtMS, timezone: timezone}
		existing, found := byBoundary[boundaryMS]
		if !found || sample.observedAtMS < existing.observedAtMS {
			byBoundary[boundaryMS] = sample
		}
	}
	result := make([]creditsBoundarySample, 0, len(byBoundary))
	for _, sample := range byBoundary {
		result = append(result, sample)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].boundaryMS < result[j].boundaryMS })
	return result
}

func creditsBoundaryMS(dateValue, timezone string) (int64, bool) {
	location, ok := analyticsLocation(timezone)
	if !ok {
		return 0, false
	}
	parsed, err := time.ParseInLocation(time.DateOnly, dateValue, location)
	if err != nil {
		return 0, false
	}
	return parsed.UnixMilli(), true
}

func analyticsLocation(value string) (*time.Location, bool) {
	if strings.HasPrefix(value, "UTC") && len(value) == len("UTC+00:00") {
		sign := 1
		if value[3] == '-' {
			sign = -1
		} else if value[3] != '+' {
			return nil, false
		}
		hours, hourErr := strconv.Atoi(value[4:6])
		minutes, minuteErr := strconv.Atoi(value[7:9])
		if hourErr != nil || minuteErr != nil || value[6] != ':' || hours > 23 || minutes > 59 {
			return nil, false
		}
		return time.FixedZone(value, sign*(hours*60+minutes)*60), true
	}
	location, err := time.LoadLocation(value)
	return location, err == nil
}

type quotaBoundaryEnvelope struct {
	beforeUsed float64
	afterUsed  float64
	beforeAtMS int64
	afterAtMS  int64
}

func quotaEnvelopeAtBoundary(history []model.CodexInspectionResult, resetAtMS, boundaryMS, cycleStartMS int64) (quotaBoundaryEnvelope, bool) {
	if boundaryMS == cycleStartMS {
		return quotaBoundaryEnvelope{beforeAtMS: boundaryMS, afterAtMS: boundaryMS}, true
	}
	var envelope quotaBoundaryEnvelope
	foundBefore := false
	foundAfter := false
	for _, candidate := range history {
		window := standardWeeklyQuotaWindow(candidate.QuotaWindows)
		if window == nil || window.UsedPercent == nil || !sameWeeklyResetWindow(window.ResetAtMS, resetAtMS) {
			continue
		}
		if candidate.CreatedAtMS <= boundaryMS && (!foundBefore || candidate.CreatedAtMS > envelope.beforeAtMS) {
			envelope.beforeUsed = *window.UsedPercent
			envelope.beforeAtMS = candidate.CreatedAtMS
			foundBefore = true
		}
		if candidate.CreatedAtMS >= boundaryMS && (!foundAfter || candidate.CreatedAtMS < envelope.afterAtMS) {
			envelope.afterUsed = *window.UsedPercent
			envelope.afterAtMS = candidate.CreatedAtMS
			foundAfter = true
		}
	}
	toleranceMS := int64(creditsQuotaBoundaryTolerance / time.Millisecond)
	if !foundBefore || !foundAfter || boundaryMS-envelope.beforeAtMS > toleranceMS || envelope.afterAtMS-boundaryMS > toleranceMS {
		return quotaBoundaryEnvelope{}, false
	}
	return envelope, true
}

func closedIntervalCreditsEstimate(start, end creditsBoundarySample, startQuota, endQuota quotaBoundaryEnvelope, weekly model.CodexInspectionQuotaWindow) *model.CodexWeeklyPoolEstimate {
	creditsDelta := end.credits - start.credits
	pointDelta := endQuota.beforeUsed - startQuota.beforeUsed
	minDelta := endQuota.beforeUsed - startQuota.afterUsed
	maxDelta := endQuota.afterUsed - startQuota.beforeUsed
	if creditsDelta <= 0 || pointDelta < 5 || minDelta <= 0 || maxDelta < pointDelta || minDelta > pointDelta {
		return nil
	}
	cost := creditsDelta * weeklyEstimateUSDPerCredit
	value := cost / (pointDelta / 100)
	minValue := cost / (maxDelta / 100)
	maxValue := cost / (minDelta / 100)
	cycleStartMS := weekly.ResetAtMS - int64(codexWeekWindow)*1000
	intervalKind := weeklyEstimateIntervalPartial
	role := weeklyEstimateRoleCurrent
	if start.boundaryMS == cycleStartMS && end.boundaryMS == weekly.ResetAtMS {
		intervalKind = weeklyEstimateIntervalComplete
		role = weeklyEstimateRoleFormal
	}
	return &model.CodexWeeklyPoolEstimate{
		Official:            false,
		Basis:               weeklyEstimateBasisCredits,
		Source:              weeklyEstimateSourceCreditsCurrent,
		Role:                role,
		IntervalKind:        intervalKind,
		CalculationVersion:  creditsCalculationVersion,
		Status:              creditsEstimateStatus(value, minValue, maxValue, pointDelta),
		WeeklyPoolUSD:       &value,
		WeeklyPoolMinUSD:    &minValue,
		WeeklyPoolMaxUSD:    &maxValue,
		CostDeltaUSD:        cost,
		UsedPercentDelta:    pointDelta,
		UsedPercentMinDelta: minDelta,
		UsedPercentMaxDelta: maxDelta,
		BaselineAtMS:        startQuota.beforeAtMS,
		CurrentAtMS:         endQuota.beforeAtMS,
		IntervalStartMS:     start.boundaryMS,
		IntervalEndMS:       end.boundaryMS,
		WeeklyResetAtMS:     weekly.ResetAtMS,
		QuotaKind:           weeklyEstimateQuotaKind,
		AnalyticsTimezone:   end.timezone,
		Credits:             creditsDelta,
		USDPerCredit:        weeklyEstimateUSDPerCredit,
		UpdatedAtMS:         end.observedAtMS,
	}
}

func creditsEstimateStatus(value, minValue, maxValue, pointDelta float64) string {
	if pointDelta < 5 || value <= 0 || minValue <= 0 || maxValue < minValue {
		return weeklyEstimateStatusInsufficient
	}
	relativeHalfWidth := (maxValue - minValue) / (2 * value)
	if pointDelta >= 20 && relativeHalfWidth <= 0.10 {
		return weeklyEstimateStatusReliable
	}
	return weeklyEstimateStatusPreliminary
}

func (s *Service) previousWeeklySample(ctx context.Context, result model.CodexInspectionResult, previousResetAtMS int64) (model.CodexInspectionResult, bool) {
	fromMS := previousResetAtMS - int64(codexWeekWindow)*1000
	history, err := s.store.ListCodexInspectionResultsByIdentity(ctx, result.AuthIndex, result.AccountID, fromMS, result.CreatedAtMS)
	if err != nil {
		return model.CodexInspectionResult{}, false
	}
	var selected model.CodexInspectionResult
	found := false
	for _, candidate := range history {
		window := standardWeeklyQuotaWindow(candidate.QuotaWindows)
		if window == nil || window.UsedPercent == nil {
			continue
		}
		drift := time.Duration(window.ResetAtMS-previousResetAtMS) * time.Millisecond
		if drift < 0 {
			drift = -drift
		}
		age := previousResetAtMS - candidate.CreatedAtMS
		if drift > weeklyResetDriftTolerance || age < 0 || age > int64(creditsQuotaBoundaryTolerance/time.Millisecond) || (found && candidate.CreatedAtMS <= selected.CreatedAtMS) {
			continue
		}
		selected = candidate
		found = true
	}
	return selected, found
}

func (s *Service) previousWeeklySampleAtResetTransition(ctx context.Context, result model.CodexInspectionResult, previousCycleStartMS int64) (model.CodexInspectionResult, bool) {
	currentWindow := standardWeeklyQuotaWindow(result.QuotaWindows)
	if currentWindow == nil || currentWindow.UsedPercent == nil {
		return model.CodexInspectionResult{}, false
	}
	history, err := s.store.ListCodexInspectionResultsByIdentity(ctx, result.AuthIndex, result.AccountID, previousCycleStartMS, result.CreatedAtMS)
	if err != nil {
		return model.CodexInspectionResult{}, false
	}
	history = append(history, result)
	transitionAtMS := currentWindow.ResetAtMS - int64(codexWeekWindow)*1000
	proximityToleranceMS := int64(creditsQuotaBoundaryTolerance / time.Millisecond)
	var previous model.CodexInspectionResult
	var previousWindow *model.CodexInspectionQuotaWindow
	for _, candidate := range history {
		window := standardWeeklyQuotaWindow(candidate.QuotaWindows)
		if window == nil || window.UsedPercent == nil {
			continue
		}
		transitionProximityMS := candidate.CreatedAtMS - transitionAtMS
		if transitionProximityMS < 0 {
			transitionProximityMS = -transitionProximityMS
		}
		adjacentGapMS := candidate.CreatedAtMS - previous.CreatedAtMS
		if previousWindow != nil &&
			sameWeeklyResetWindow(window.ResetAtMS, currentWindow.ResetAtMS) &&
			!sameWeeklyResetWindow(previousWindow.ResetAtMS, currentWindow.ResetAtMS) &&
			window.ResetAtMS > previousWindow.ResetAtMS &&
			transitionProximityMS <= proximityToleranceMS &&
			adjacentGapMS > 0 && adjacentGapMS <= proximityToleranceMS &&
			*window.UsedPercent <= 5 &&
			*previousWindow.UsedPercent >= 5 &&
			*window.UsedPercent < *previousWindow.UsedPercent {
			return previous, true
		}
		previous = candidate
		previousWindow = window
	}
	return model.CodexInspectionResult{}, false
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
	selectedUsedPercent := math.Inf(-1)
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
		if *window.UsedPercent >= currentUsedPercent {
			continue
		}
		if !found || *window.UsedPercent > selectedUsedPercent ||
			(*window.UsedPercent == selectedUsedPercent && candidate.CreatedAtMS > selected.CreatedAtMS) {
			selected = candidate
			selectedUsedPercent = *window.UsedPercent
			found = true
		}
	}
	return selected, found, quotaDecreased
}

func selectClosedCPAInterval(history []model.CodexInspectionResult, current model.CodexInspectionResult, currentWindow model.CodexInspectionQuotaWindow) (model.CodexInspectionResult, model.CodexInspectionResult, bool, string) {
	cutoffMS := current.CreatedAtMS - int64(cpaCaptureCloseDelay/time.Millisecond)
	var endpoint model.CodexInspectionResult
	endpointFound := false
	identityChanged := false
	newerEndpointPending := false
	for _, candidate := range history {
		window := standardWeeklyQuotaWindow(candidate.QuotaWindows)
		if window == nil || window.UsedPercent == nil || !sameWeeklyResetWindow(window.ResetAtMS, currentWindow.ResetAtMS) {
			continue
		}
		if !sameCPAIdentitySnapshot(candidate, current) {
			identityChanged = true
			continue
		}
		if candidate.CreatedAtMS > cutoffMS {
			newerEndpointPending = true
			continue
		}
		if !endpointFound || candidate.CreatedAtMS > endpoint.CreatedAtMS {
			endpoint = candidate
			endpointFound = true
		}
	}
	if !endpointFound {
		if identityChanged {
			return model.CodexInspectionResult{}, model.CodexInspectionResult{}, false, "identity_changed"
		}
		return model.CodexInspectionResult{}, model.CodexInspectionResult{}, false, "capture_pending"
	}
	endpointWindow := standardWeeklyQuotaWindow(endpoint.QuotaWindows)
	if endpointWindow == nil || endpointWindow.UsedPercent == nil {
		return model.CodexInspectionResult{}, model.CodexInspectionResult{}, false, "weekly_sample_incomplete"
	}
	if currentWindow.UsedPercent != nil && *currentWindow.UsedPercent < *endpointWindow.UsedPercent {
		return model.CodexInspectionResult{}, model.CodexInspectionResult{}, false, "quota_decreased"
	}
	prior := make([]model.CodexInspectionResult, 0, len(history))
	for _, candidate := range history {
		if candidate.CreatedAtMS >= endpoint.CreatedAtMS || !sameCPAIdentitySnapshot(candidate, endpoint) {
			continue
		}
		prior = append(prior, candidate)
	}
	baseline, found, quotaDecreased := selectWeeklyBaseline(prior, *endpointWindow, *endpointWindow.UsedPercent)
	if !found {
		if quotaDecreased {
			return model.CodexInspectionResult{}, model.CodexInspectionResult{}, false, "quota_decreased"
		}
		if newerEndpointPending {
			return model.CodexInspectionResult{}, model.CodexInspectionResult{}, false, "capture_pending"
		}
		return model.CodexInspectionResult{}, model.CodexInspectionResult{}, false, "baseline_missing"
	}
	return baseline, endpoint, true, ""
}

func sameCPAIdentitySnapshot(left, right model.CodexInspectionResult) bool {
	leftSnapshot := strings.TrimSpace(left.AccountSnapshot)
	rightSnapshot := strings.TrimSpace(right.AccountSnapshot)
	return leftSnapshot == "" || rightSnapshot == "" || strings.EqualFold(leftSnapshot, rightSnapshot)
}

func quotaValueBounds(value, resolution float64) (float64, float64) {
	half := resolution / 2
	return math.Max(0, value-half), math.Min(100, value+half)
}

func cpaQuotaScope(windows []model.CodexInspectionQuotaWindow) string {
	weeklyWindows := 0
	for _, window := range windows {
		if window.LimitWindowSeconds != nil && int(math.Round(*window.LimitWindowSeconds)) == codexWeekWindow {
			weeklyWindows++
		}
	}
	if weeklyWindows > 1 {
		return "mixed"
	}
	return "matched"
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

func cpaEstimateCompatible(estimate *model.CodexWeeklyPoolEstimate) bool {
	return estimate != nil && estimate.CalculationVersion == cpaCalculationVersion &&
		estimate.CaptureState == "closed" &&
		estimate.WeeklyPoolUSD != nil && estimate.WeeklyPoolMinUSD != nil && estimate.WeeklyPoolMaxUSD != nil
}

func cpaEstimateFormalEligible(estimate *model.CodexWeeklyPoolEstimate) bool {
	return cpaEstimateCompatible(estimate) && estimate.Status == weeklyEstimateStatusReliable
}

func cpaEstimateStatus(minValue, maxValue float64) string {
	if minValue <= 0 || maxValue < minValue {
		return weeklyEstimateStatusInsufficient
	}
	relativeHalfWidth := (maxValue - minValue) / (maxValue + minValue)
	if relativeHalfWidth <= 0.10+1e-12 {
		return weeklyEstimateStatusReliable
	}
	return weeklyEstimateStatusPreliminary
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
	if estimate.UsedPercentMinDelta <= 0 || estimate.UsedPercentMaxDelta <= 0 {
		estimate.Status = weeklyEstimateStatusInsufficient
		estimate.Reason = "delta_too_small"
		return
	}
	minValue := totalCost / (estimate.UsedPercentMaxDelta / 100)
	maxValue := totalCost / (estimate.UsedPercentMinDelta / 100)
	estimate.WeeklyPoolUSD = &value
	estimate.WeeklyPoolMinUSD = &minValue
	estimate.WeeklyPoolMaxUSD = &maxValue
	estimate.CostDeltaUSD = totalCost
	estimate.Status = cpaEstimateStatus(minValue, maxValue)
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
