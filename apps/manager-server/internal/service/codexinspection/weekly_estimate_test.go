package codexinspection

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/model"
	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/store"
	"github.com/seakee/cpa-manager-plus/apps/manager-server/internal/usage"
)

func TestBuildCodexInspectionQuotaWindowsAllowsWeeklyWithoutFiveHour(t *testing.T) {
	resetAtSeconds := float64(1_800_604_800)
	windows := buildCodexInspectionQuotaWindows(map[string]any{
		"plan_type": "pro",
		"rate_limit": map[string]any{
			"primary_window": nil,
			"secondary_window": map[string]any{
				"used_percent":         79,
				"limit_window_seconds": codexWeekWindow,
				"reset_at":             resetAtSeconds,
			},
		},
	}, "pro")

	if len(windows) != 1 {
		t.Fatalf("windows = %#v, want weekly only", windows)
	}
	if windows[0].ID != "weekly" {
		t.Fatalf("window id = %q, want weekly", windows[0].ID)
	}
	if windows[0].UsedPercent == nil || *windows[0].UsedPercent != 79 {
		t.Fatalf("weekly used percent = %#v, want 79", windows[0].UsedPercent)
	}
	if windows[0].ResetAtMS != int64(resetAtSeconds)*1000 {
		t.Fatalf("weekly reset = %d, want %d", windows[0].ResetAtMS, int64(resetAtSeconds)*1000)
	}
}

func TestWeeklyEstimateStatusThresholds(t *testing.T) {
	tests := []struct {
		delta float64
		want  string
	}{
		{delta: 0.99, want: weeklyEstimateStatusInsufficient},
		{delta: 1, want: weeklyEstimateStatusPreliminary},
		{delta: 4.99, want: weeklyEstimateStatusPreliminary},
		{delta: 5, want: weeklyEstimateStatusReliable},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%.2f", test.delta), func(t *testing.T) {
			if got := weeklyEstimateStatus(test.delta); got != test.want {
				t.Fatalf("status(%v) = %q, want %q", test.delta, got, test.want)
			}
		})
	}
}

func TestCreditsEstimateCompatibilityRejectsLegacyCalculation(t *testing.T) {
	value := 2_000.0
	legacy := &model.CodexWeeklyPoolEstimate{Basis: weeklyEstimateBasisCredits, WeeklyPoolUSD: &value}
	current := &model.CodexWeeklyPoolEstimate{Basis: weeklyEstimateBasisCredits, CalculationVersion: creditsCalculationVersion, WeeklyPoolUSD: &value}
	if creditsEstimateCompatible(legacy) {
		t.Fatal("legacy Credits baseline unexpectedly qualified")
	}
	if !creditsEstimateCompatible(current) {
		t.Fatal("current Credits baseline did not qualify")
	}
}

func TestSummarizeCodexCreditsRowsExcludesLatestOpenDate(t *testing.T) {
	rows := func(openCredits float64) []any {
		return []any{
			map[string]any{"date": "2027-01-01", "totals": map[string]any{"credits": 100.0}},
			map[string]any{"date": "2027-01-02", "totals": map[string]any{"credits": openCredits}},
		}
	}
	first := summarizeCodexCreditsRows(rows(20), "2027-01-01", "2026-12-25", "UTC+08:00", 1)
	second := summarizeCodexCreditsRows(rows(80), "2027-01-01", "2026-12-25", "UTC+08:00", 2)
	if first.CurrentCycleCredits != 120 || second.CurrentCycleCredits != 180 {
		t.Fatalf("open totals = %.2f and %.2f, want 120 and 180", first.CurrentCycleCredits, second.CurrentCycleCredits)
	}
	if first.ClosedCycleCredits != 100 || second.ClosedCycleCredits != 100 || first.ClosedBoundaryDate != "2027-01-02" || second.ClosedBoundaryDate != "2027-01-02" {
		t.Fatalf("closed totals = %#v and %#v, want stable boundary at 100 Credits", first, second)
	}
}

func TestQuotaEnvelopeUsesAsOfSampleAndRejectsStaleBoundary(t *testing.T) {
	boundaryMS := int64(1_800_000_000_000)
	resetAtMS := boundaryMS + int64(4*24*time.Hour/time.Millisecond)
	history := []model.CodexInspectionResult{
		weeklyQuotaResult(boundaryMS-int64(3*time.Minute/time.Millisecond), resetAtMS, 32),
		weeklyQuotaResult(boundaryMS+int64(3*time.Minute/time.Millisecond), resetAtMS, 33),
	}
	envelope, ok := quotaEnvelopeAtBoundary(history, resetAtMS, boundaryMS, resetAtMS-int64(codexWeekWindow)*1000)
	if !ok || envelope.beforeUsed != 32 || envelope.afterUsed != 33 {
		t.Fatalf("quota envelope = %#v ok=%v, want 32%% before and 33%% after", envelope, ok)
	}
	stale := []model.CodexInspectionResult{
		weeklyQuotaResult(boundaryMS-int64(16*time.Minute/time.Millisecond), resetAtMS, 32),
		weeklyQuotaResult(boundaryMS+int64(3*time.Minute/time.Millisecond), resetAtMS, 33),
	}
	if _, ok := quotaEnvelopeAtBoundary(stale, resetAtMS, boundaryMS, resetAtMS-int64(codexWeekWindow)*1000); ok {
		t.Fatal("stale pre-boundary quota sample unexpectedly qualified")
	}
}

func TestClosedIntervalCreditsEstimateIncludesQuotaEnvelopeRange(t *testing.T) {
	startMS := int64(1_800_000_000_000)
	endMS := startMS + int64(24*time.Hour/time.Millisecond)
	resetAtMS := startMS + int64(6*24*time.Hour/time.Millisecond)
	windowSeconds := float64(codexWeekWindow)
	estimate := closedIntervalCreditsEstimate(
		creditsBoundarySample{credits: 13_121.9694, boundaryMS: startMS, observedAtMS: startMS, timezone: "UTC+08:00"},
		creditsBoundarySample{credits: 20_727.4264, boundaryMS: endMS, observedAtMS: endMS, timezone: "UTC+08:00"},
		quotaBoundaryEnvelope{beforeUsed: 18, afterUsed: 19, beforeAtMS: startMS - 1, afterAtMS: startMS + 1},
		quotaBoundaryEnvelope{beforeUsed: 32, afterUsed: 33, beforeAtMS: endMS - 1, afterAtMS: endMS + 1},
		model.CodexInspectionQuotaWindow{ID: "weekly", ResetAtMS: resetAtMS, LimitWindowSeconds: &windowSeconds},
	)
	if estimate == nil || estimate.WeeklyPoolUSD == nil || estimate.WeeklyPoolMinUSD == nil || estimate.WeeklyPoolMaxUSD == nil {
		t.Fatalf("estimate = %#v, want point and range", estimate)
	}
	if math.Abs(*estimate.WeeklyPoolUSD-2_172.987714285714) > 0.000001 || math.Abs(*estimate.WeeklyPoolMinUSD-2_028.121866666667) > 0.000001 || math.Abs(*estimate.WeeklyPoolMaxUSD-2_340.140615384615) > 0.000001 {
		t.Fatalf("estimate values = %#v", estimate)
	}
	if estimate.UsedPercentDelta != 14 || estimate.UsedPercentMinDelta != 13 || estimate.UsedPercentMaxDelta != 15 {
		t.Fatalf("quota deltas = %#v, want point 14 and range 13-15", estimate)
	}
}

func weeklyQuotaResult(createdAtMS, resetAtMS int64, usedPercent float64) model.CodexInspectionResult {
	windowSeconds := float64(codexWeekWindow)
	return model.CodexInspectionResult{
		CreatedAtMS: createdAtMS,
		QuotaWindows: []model.CodexInspectionQuotaWindow{{
			ID:                 "weekly",
			UsedPercent:        &usedPercent,
			ResetAtMS:          resetAtMS,
			LimitWindowSeconds: &windowSeconds,
		}},
	}
}

func TestGetRunEstimatesWeeklyPoolPerAccountAndResetWindow(t *testing.T) {
	ctx := context.Background()
	db := newCodexInspectionTestStore(t)
	svc := newCodexInspectionTestService(t, db)
	windowStartMS := int64(1_800_000_000_000)
	resetAtMS := windowStartMS + int64(codexWeekWindow)*1000
	baselineAtMS := windowStartMS + int64(time.Hour/time.Millisecond)
	currentAtMS := baselineAtMS + int64(2*time.Hour/time.Millisecond)
	priceSyncedAtMS := windowStartMS - 1

	if err := db.SaveModelPrices(ctx, map[string]store.ModelPrice{
		"gpt-5.4": {
			Prompt:      2.5,
			Completion:  15,
			Cache:       0.25,
			Source:      "models.dev",
			SyncedAtMS:  &priceSyncedAtMS,
			UpdatedAtMS: priceSyncedAtMS,
		},
	}); err != nil {
		t.Fatalf("save prices: %v", err)
	}

	insertWeeklyInspectionRun(t, db, baselineAtMS, resetAtMS, []weeklyInspectionSample{
		{authIndex: "auth-a", accountID: "account-a", usedPercent: 20},
		{authIndex: "auth-b", accountID: "account-b", usedPercent: 40},
		{authIndex: "auth-c", accountID: "account-c", usedPercent: 79},
	})
	insertWeeklyInspectionRun(t, db, baselineAtMS-2, resetAtMS+int64(2*time.Minute/time.Millisecond), []weeklyInspectionSample{
		{authIndex: "auth-a", accountID: "account-a", usedPercent: 0},
	})
	insertWeeklyInspectionRun(t, db, baselineAtMS-1, resetAtMS, []weeklyInspectionSample{
		{authIndex: "auth-a", accountID: "replaced-account", usedPercent: 0},
	})

	eventAtMS := baselineAtMS + int64(time.Hour/time.Millisecond)
	if _, err := db.InsertEvents(ctx, []usage.Event{
		weeklyEstimateUsageEvent("event-a", eventAtMS, "auth-a", "account-a", 100_000),
		weeklyEstimateUsageEvent("event-a-sibling", eventAtMS, "auth-a", "account-sibling", 20_000_000),
		weeklyEstimateUsageEvent("event-b", eventAtMS, "auth-b", "account-b", 200_000),
		weeklyEstimateUsageEvent("event-c", eventAtMS, "auth-c", "account-c", 200_000),
		weeklyEstimateUsageEvent("event-other", eventAtMS, "auth-other", "account-other", 20_000_000),
	}); err != nil {
		t.Fatalf("insert usage events: %v", err)
	}

	currentResetAtMS := resetAtMS + 1_000
	insertWeeklyInspectionRun(t, db, currentAtMS, currentResetAtMS, []weeklyInspectionSample{
		{authIndex: "auth-a", accountID: "account-a", usedPercent: 25},
		{authIndex: "auth-b", accountID: "account-b", usedPercent: 45},
		{authIndex: "auth-c", accountID: "account-c", usedPercent: 79},
	})
	observerAtMS := currentAtMS + int64(3*time.Minute/time.Millisecond)
	currentRun := insertWeeklyInspectionRun(t, db, observerAtMS, currentResetAtMS, []weeklyInspectionSample{
		{authIndex: "auth-a", accountID: "account-a", usedPercent: 25},
		{authIndex: "auth-b", accountID: "account-b", usedPercent: 45},
		{authIndex: "auth-c", accountID: "account-c", usedPercent: 79},
	})
	detail, err := svc.GetRun(ctx, currentRun.ID)
	if err != nil {
		t.Fatalf("get current run: %v", err)
	}
	if len(detail.Results) != 3 {
		t.Fatalf("results = %#v, want three accounts", detail.Results)
	}

	estimates := map[string]*model.CodexWeeklyPoolEstimate{}
	for index := range detail.Results {
		estimates[detail.Results[index].AccountID] = detail.Results[index].WeeklyPoolEstimate
	}
	assertWeeklyEstimate(t, estimates["account-a"], 0.25, 5, 5, baselineAtMS, currentResetAtMS)
	assertWeeklyEstimate(t, estimates["account-b"], 0.5, 5, 10, baselineAtMS, currentResetAtMS)
	if estimate := estimates["account-c"]; estimate == nil || estimate.Status != weeklyEstimateStatusUnavailable || estimate.WeeklyPoolUSD != nil || estimate.Reason != "baseline_missing" {
		t.Fatalf("zero-delta estimate = %#v, want unavailable without a lower quota baseline", estimate)
	}
}

func TestSelectClosedCPAIntervalWaitsForNextInspection(t *testing.T) {
	resetAtMS := int64(1_800_604_800_000)
	baselineAtMS := int64(1_800_000_000_000)
	endpointAtMS := baselineAtMS + int64(5*time.Minute/time.Millisecond)
	baseline := weeklyQuotaResult(baselineAtMS, resetAtMS, 20)
	endpoint := weeklyQuotaResult(endpointAtMS, resetAtMS, 30)
	current := weeklyQuotaResult(endpointAtMS+int64(time.Minute/time.Millisecond), resetAtMS, 30)
	currentWindow := standardWeeklyQuotaWindow(current.QuotaWindows)
	if _, _, found, reason := selectClosedCPAInterval([]model.CodexInspectionResult{baseline, endpoint}, current, *currentWindow); found || reason != "capture_pending" {
		t.Fatalf("early interval found=%v reason=%q, want capture_pending", found, reason)
	}
	current.CreatedAtMS = endpointAtMS + int64(3*time.Minute/time.Millisecond)
	selectedBaseline, selectedEndpoint, found, reason := selectClosedCPAInterval([]model.CodexInspectionResult{baseline, endpoint}, current, *currentWindow)
	if !found || reason != "" || selectedBaseline.CreatedAtMS != baselineAtMS || selectedEndpoint.CreatedAtMS != endpointAtMS {
		t.Fatalf("closed interval baseline=%#v endpoint=%#v found=%v reason=%q", selectedBaseline, selectedEndpoint, found, reason)
	}
}

func TestSelectClosedCPAIntervalUsesNearestLowerQuotaTransition(t *testing.T) {
	resetAtMS := int64(1_800_604_800_000)
	startMS := int64(1_800_000_000_000)
	oldBaseline := weeklyQuotaResult(startMS, resetAtMS, 38)
	nearBaseline := weeklyQuotaResult(startMS+int64(10*time.Minute/time.Millisecond), resetAtMS, 46)
	latestNearBaseline := weeklyQuotaResult(startMS+int64(15*time.Minute/time.Millisecond), resetAtMS, 46)
	endpointAtMS := startMS + int64(20*time.Minute/time.Millisecond)
	endpoint := weeklyQuotaResult(endpointAtMS, resetAtMS, 47)
	current := weeklyQuotaResult(endpointAtMS+int64(3*time.Minute/time.Millisecond), resetAtMS, 47)
	currentWindow := standardWeeklyQuotaWindow(current.QuotaWindows)

	baseline, selectedEndpoint, found, reason := selectClosedCPAInterval(
		[]model.CodexInspectionResult{oldBaseline, nearBaseline, latestNearBaseline, endpoint},
		current,
		*currentWindow,
	)
	if !found || reason != "" || baseline.CreatedAtMS != latestNearBaseline.CreatedAtMS || selectedEndpoint.CreatedAtMS != endpointAtMS {
		t.Fatalf("closed interval baseline=%#v endpoint=%#v found=%v reason=%q", baseline, selectedEndpoint, found, reason)
	}
	baselineWindow := standardWeeklyQuotaWindow(baseline.QuotaWindows)
	endpointWindow := standardWeeklyQuotaWindow(selectedEndpoint.QuotaWindows)
	startMin, startMax := quotaValueBounds(*baselineWindow.UsedPercent, cpaQuotaResolutionPP)
	endMin, endMax := quotaValueBounds(*endpointWindow.UsedPercent, cpaQuotaResolutionPP)
	if pointDelta, minDelta, maxDelta := *endpointWindow.UsedPercent-*baselineWindow.UsedPercent, endMin-startMax, endMax-startMin; pointDelta != 1 || minDelta != 0 || maxDelta != 2 {
		t.Fatalf("quota delta = %.1f [%.1f, %.1f], want 1.0 [0.0, 2.0] so the estimate remains insufficient", pointDelta, minDelta, maxDelta)
	}
}

func TestGetRunPromotesReliableCPAObservationAcrossReset(t *testing.T) {
	ctx := context.Background()
	db := newCodexInspectionTestStore(t)
	svc := newCodexInspectionTestService(t, db)
	startMS := int64(1_800_000_000_000)
	resetAtMS := startMS + int64(codexWeekWindow)*1000
	priceAtMS := startMS - 1
	if err := db.SaveModelPrices(ctx, map[string]store.ModelPrice{
		"gpt-5.4": {Prompt: 2.5, Completion: 15, Cache: 0.25, Source: "models.dev", SyncedAtMS: &priceAtMS, UpdatedAtMS: priceAtMS},
	}); err != nil {
		t.Fatalf("save prices: %v", err)
	}
	baselineAtMS := startMS + int64(time.Hour/time.Millisecond)
	endpointAtMS := baselineAtMS + int64(time.Hour/time.Millisecond)
	insertWeeklyInspectionRun(t, db, baselineAtMS, resetAtMS, []weeklyInspectionSample{{authIndex: "auth-a", accountID: "account-a", usedPercent: 20}})
	if _, err := db.InsertEvents(ctx, []usage.Event{weeklyEstimateUsageEvent("formal-event", baselineAtMS+1, "auth-a", "account-a", 800_000)}); err != nil {
		t.Fatalf("insert usage event: %v", err)
	}
	insertWeeklyInspectionRun(t, db, endpointAtMS, resetAtMS, []weeklyInspectionSample{{authIndex: "auth-a", accountID: "account-a", usedPercent: 30}})
	observer := insertWeeklyInspectionRun(t, db, endpointAtMS+int64(3*time.Minute/time.Millisecond), resetAtMS, []weeklyInspectionSample{{authIndex: "auth-a", accountID: "account-a", usedPercent: 30}})
	if _, err := svc.GetRun(ctx, observer.ID); err != nil {
		t.Fatalf("get CPA observation: %v", err)
	}
	nextResetAtMS := resetAtMS + int64(codexWeekWindow)*1000
	next := insertWeeklyInspectionRun(t, db, resetAtMS+int64(time.Hour/time.Millisecond), nextResetAtMS, []weeklyInspectionSample{{authIndex: "auth-a", accountID: "account-a", usedPercent: 0}})
	detail, err := svc.GetRun(ctx, next.ID)
	if err != nil {
		t.Fatalf("get next-cycle run: %v", err)
	}
	formal := weeklyEstimateForBasisRole(detail.Results[0].WeeklyPoolEstimates, weeklyEstimateBasisAPIEquivalent, weeklyEstimateRoleFormal)
	if formal == nil || formal.Source != weeklyEstimateSourceCPALearned || formal.IntervalKind != weeklyEstimateIntervalApproximate || !cpaEstimateCompatible(formal) {
		t.Fatalf("formal CPA baseline = %#v", formal)
	}
	if formal.UpdatedAtMS != observer.CreatedAtMS {
		t.Fatalf("formal CPA update time = %d, want original observation time %d", formal.UpdatedAtMS, observer.CreatedAtMS)
	}
	thirdResetAtMS := nextResetAtMS + int64(codexWeekWindow)*1000
	third := insertWeeklyInspectionRun(t, db, nextResetAtMS+int64(time.Hour/time.Millisecond), thirdResetAtMS, []weeklyInspectionSample{{authIndex: "auth-a", accountID: "account-a", usedPercent: 0}})
	thirdDetail, err := svc.GetRun(ctx, third.ID)
	if err != nil {
		t.Fatalf("get third-cycle run: %v", err)
	}
	formal = weeklyEstimateForBasisRole(thirdDetail.Results[0].WeeklyPoolEstimates, weeklyEstimateBasisAPIEquivalent, weeklyEstimateRoleFormal)
	if formal == nil || formal.UpdatedAtMS != observer.CreatedAtMS {
		t.Fatalf("repeated promotion refreshed formal CPA baseline: %#v", formal)
	}
}

func TestGetRunRecordsCPAAndCreditsEstimatesIndependently(t *testing.T) {
	ctx := context.Background()
	db := newCodexInspectionTestStore(t)
	svc := newCodexInspectionTestService(t, db)
	cycleStart := time.Date(2027, time.February, 1, 16, 0, 0, 0, time.Local)
	resetAtMS := cycleStart.Add(7 * 24 * time.Hour).UnixMilli()
	firstBoundaryAtMS := time.Date(2027, time.February, 2, 0, 0, 0, 0, time.Local).UnixMilli()
	secondBoundaryAtMS := time.Date(2027, time.February, 3, 0, 0, 0, 0, time.Local).UnixMilli()
	priceAtMS := firstBoundaryAtMS - 1

	if err := db.SaveModelPrices(ctx, map[string]store.ModelPrice{
		"gpt-5.4": {
			Prompt:      2.5,
			Completion:  15,
			Cache:       0.25,
			Source:      "models.dev",
			SyncedAtMS:  &priceAtMS,
			UpdatedAtMS: priceAtMS,
		},
	}); err != nil {
		t.Fatalf("save prices: %v", err)
	}

	insertWeeklyInspectionRun(t, db, firstBoundaryAtMS, resetAtMS, []weeklyInspectionSample{{
		authIndex:   "auth-a",
		accountID:   "account-a",
		usedPercent: 10,
		creditsUsage: &model.CodexCreditsUsage{
			CurrentCycleCredits: 1_000,
			ClosedCycleCredits:  1_000,
			ClosedBoundaryDate:  "2027-02-02",
			CycleStartDate:      "2027-02-01",
			LatestDate:          "2027-02-02",
			AnalyticsTimezone:   "UTC+08:00",
			ObservedAtMS:        firstBoundaryAtMS,
		},
	}})
	insertWeeklyInspectionRun(t, db, secondBoundaryAtMS, resetAtMS, []weeklyInspectionSample{{
		authIndex: "auth-a", accountID: "account-a", usedPercent: 15,
	}})
	if _, err := db.InsertEvents(ctx, []usage.Event{
		weeklyEstimateUsageEvent("dual-event", secondBoundaryAtMS-int64(time.Hour/time.Millisecond), "auth-a", "account-a", 100_000),
	}); err != nil {
		t.Fatalf("insert usage event: %v", err)
	}

	currentAtMS := secondBoundaryAtMS + int64(time.Hour/time.Millisecond)
	run := insertWeeklyInspectionRun(t, db, currentAtMS, resetAtMS, []weeklyInspectionSample{{
		authIndex:   "auth-a",
		accountID:   "account-a",
		usedPercent: 16,
		creditsUsage: &model.CodexCreditsUsage{
			CurrentCycleCredits: 3_500,
			ClosedCycleCredits:  3_500,
			ClosedBoundaryDate:  "2027-02-03",
			CycleStartDate:      "2027-02-01",
			LatestDate:          "2027-02-03",
			AnalyticsTimezone:   "UTC+08:00",
			ObservedAtMS:        currentAtMS,
		},
	}})
	detail, err := svc.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("get dual estimate run: %v", err)
	}
	result := detail.Results[0]
	if result.WeeklyPoolEstimate == nil || result.WeeklyPoolEstimate.Basis != weeklyEstimateBasisAPIEquivalent {
		t.Fatalf("primary estimate = %#v, want CPA", result.WeeklyPoolEstimate)
	}
	if len(result.WeeklyPoolEstimates) != 2 {
		t.Fatalf("method estimates = %#v, want two", result.WeeklyPoolEstimates)
	}
	if api := weeklyEstimateForBasis(result.WeeklyPoolEstimates, weeklyEstimateBasisAPIEquivalent); api == nil || api.WeeklyPoolUSD == nil {
		t.Fatalf("API estimate = %#v", api)
	}
	if credits := weeklyEstimateForBasis(result.WeeklyPoolEstimates, weeklyEstimateBasisCredits); credits == nil || credits.WeeklyPoolUSD == nil || math.Abs(*credits.WeeklyPoolUSD-2_000) > 0.000001 {
		t.Fatalf("Credits estimate = %#v, want $2,000", credits)
	}
	stored, err := db.ListCodexWeeklyEstimateBaselines(ctx, "auth-a", "account-a")
	if err != nil || len(stored) != 2 {
		t.Fatalf("stored method baselines = %#v err=%v", stored, err)
	}
}

func TestGetRunUsesCreditsThenCarriesLearnedBaselineAcrossReset(t *testing.T) {
	ctx := context.Background()
	db := newCodexInspectionTestStore(t)
	svc := newCodexInspectionTestService(t, db)
	cycleStart := time.Date(2027, time.January, 1, 16, 0, 0, 0, time.Local)
	firstResetAtMS := cycleStart.Add(7 * 24 * time.Hour).UnixMilli()
	firstBoundaryAtMS := time.Date(2027, time.January, 2, 0, 0, 0, 0, time.Local).UnixMilli()
	secondBoundaryAtMS := time.Date(2027, time.January, 3, 0, 0, 0, 0, time.Local).UnixMilli()
	insertWeeklyInspectionRun(t, db, firstBoundaryAtMS, firstResetAtMS, []weeklyInspectionSample{
		{
			authIndex:   "auth-a",
			accountID:   "account-a",
			usedPercent: 4,
			creditsUsage: &model.CodexCreditsUsage{
				CurrentCycleCredits: 1_000,
				ClosedCycleCredits:  1_000,
				ClosedBoundaryDate:  "2027-01-02",
				CycleStartDate:      "2027-01-01",
				LatestDate:          "2027-01-02",
				AnalyticsTimezone:   "UTC+08:00",
				ObservedAtMS:        firstBoundaryAtMS,
			},
		},
	})
	insertWeeklyInspectionRun(t, db, secondBoundaryAtMS, firstResetAtMS, []weeklyInspectionSample{
		{authIndex: "auth-a", accountID: "account-a", usedPercent: 14},
	})
	firstAtMS := secondBoundaryAtMS + int64(5*time.Hour/time.Millisecond)
	firstRun := insertWeeklyInspectionRun(t, db, firstAtMS, firstResetAtMS, []weeklyInspectionSample{
		{
			authIndex:   "auth-a",
			accountID:   "account-a",
			usedPercent: 15,
			creditsUsage: &model.CodexCreditsUsage{
				CurrentCycleCredits: 6_000,
				ClosedCycleCredits:  6_000,
				ClosedBoundaryDate:  "2027-01-03",
				CycleStartDate:      "2027-01-01",
				LatestDate:          "2027-01-03",
				AnalyticsTimezone:   "UTC+08:00",
				ObservedAtMS:        firstAtMS,
			},
		},
	})
	firstDetail, err := svc.GetRun(ctx, firstRun.ID)
	if err != nil {
		t.Fatalf("get credits run: %v", err)
	}
	first := firstDetail.Results[0].WeeklyPoolEstimate
	if first == nil || first.WeeklyPoolUSD == nil || math.Abs(*first.WeeklyPoolUSD-2_000) > 0.000001 {
		t.Fatalf("credits estimate = %#v, want $2,000", first)
	}
	if first.Basis != weeklyEstimateBasisCredits || first.Source != weeklyEstimateSourceCreditsCurrent {
		t.Fatalf("credits provenance = %#v", first)
	}
	if first.Credits != 5_000 || first.UsedPercentDelta != 10 || first.Status != weeklyEstimateStatusPreliminary {
		t.Fatalf("matched credits interval = %#v, want 5,000 Credits across 10%%", first)
	}

	staleAtMS := firstAtMS + int64(8*time.Hour/time.Millisecond)
	staleRun := insertWeeklyInspectionRun(t, db, staleAtMS, firstResetAtMS, []weeklyInspectionSample{
		{
			authIndex:   "auth-a",
			accountID:   "account-a",
			usedPercent: 37,
			creditsUsage: &model.CodexCreditsUsage{
				CurrentCycleCredits: 9_000,
				ClosedCycleCredits:  6_000,
				ClosedBoundaryDate:  "2027-01-03",
				CycleStartDate:      "2027-01-01",
				LatestDate:          "2027-01-03",
				AnalyticsTimezone:   "UTC+08:00",
				ObservedAtMS:        staleAtMS,
			},
		},
	})
	staleDetail, err := svc.GetRun(ctx, staleRun.ID)
	if err != nil {
		t.Fatalf("get stale credits run: %v", err)
	}
	stale := staleDetail.Results[0].WeeklyPoolEstimate
	if stale == nil || stale.WeeklyPoolUSD == nil || math.Abs(*stale.WeeklyPoolUSD-2_000) > 0.000001 {
		t.Fatalf("stale credits estimate = %#v, want matched estimate to remain $2,000", stale)
	}
	if stale.UsedPercentDelta != 10 || stale.Credits != 5_000 {
		t.Fatalf("stale credits interval = %#v, want frozen matched interval", stale)
	}

	secondAtMS := firstResetAtMS + int64(time.Hour/time.Millisecond)
	secondResetAtMS := firstResetAtMS + int64(codexWeekWindow)*1000
	secondRun := insertWeeklyInspectionRun(t, db, secondAtMS, secondResetAtMS, []weeklyInspectionSample{
		{authIndex: "auth-a", accountID: "account-a", usedPercent: 0},
		{authIndex: "auth-b", accountID: "account-b", usedPercent: 0},
	})
	secondDetail, err := svc.GetRun(ctx, secondRun.ID)
	if err != nil {
		t.Fatalf("get learned run: %v", err)
	}
	learned := map[string]*model.CodexWeeklyPoolEstimate{}
	for index := range secondDetail.Results {
		learned[secondDetail.Results[index].AccountID] = secondDetail.Results[index].WeeklyPoolEstimate
	}
	if estimate := learned["account-a"]; estimate == nil || estimate.WeeklyPoolUSD == nil ||
		math.Abs(*estimate.WeeklyPoolUSD-2_000) > 0.000001 || estimate.Source != weeklyEstimateSourceCreditsLearned {
		t.Fatalf("learned estimate = %#v, want account-a credits baseline", estimate)
	} else if estimate.Role != weeklyEstimateRoleFormal || estimate.IntervalKind != weeklyEstimateIntervalApproximate {
		t.Fatalf("learned estimate role = %#v, want approximate formal baseline", estimate)
	}
	if estimate := learned["account-b"]; estimate == nil || estimate.WeeklyPoolUSD != nil {
		t.Fatalf("account-b estimate = %#v, want isolated empty baseline", estimate)
	}
}

func TestGetRunLearnsCreditsFromPreviousCycleWhenCurrentAnalyticsLags(t *testing.T) {
	ctx := context.Background()
	db := newCodexInspectionTestStore(t)
	svc := newCodexInspectionTestService(t, db)
	analyticsZone := time.FixedZone("UTC+08:00", 8*60*60)
	previousCycleStart := time.Date(2027, time.January, 1, 0, 0, 0, 0, analyticsZone)
	previousResetAtMS := previousCycleStart.Add(7 * 24 * time.Hour).UnixMilli()
	previousAtMS := previousResetAtMS - int64(5*time.Minute/time.Millisecond)
	insertWeeklyInspectionRun(t, db, previousAtMS, previousResetAtMS, []weeklyInspectionSample{
		{authIndex: "auth-a", accountID: "account-a", usedPercent: 80},
	})
	currentAtMS := previousResetAtMS + int64(time.Hour/time.Millisecond)
	currentResetAtMS := previousResetAtMS + int64(codexWeekWindow)*1000
	currentRun := insertWeeklyInspectionRun(t, db, currentAtMS, currentResetAtMS, []weeklyInspectionSample{
		{
			authIndex:   "auth-a",
			accountID:   "account-a",
			usedPercent: 2,
			creditsUsage: &model.CodexCreditsUsage{
				CycleStartDate:         "2027-01-08",
				PreviousCycleCredits:   40_000,
				PreviousCycleStartDate: "2027-01-01",
				LatestDate:             "2027-01-08",
				AnalyticsTimezone:      "UTC+08:00",
				ObservedAtMS:           currentAtMS,
			},
		},
	})
	detail, err := svc.GetRun(ctx, currentRun.ID)
	if err != nil {
		t.Fatalf("get previous-cycle credits run: %v", err)
	}
	estimate := detail.Results[0].WeeklyPoolEstimate
	if estimate == nil || estimate.WeeklyPoolUSD == nil || math.Abs(*estimate.WeeklyPoolUSD-2_000) > 0.000001 {
		t.Fatalf("previous-cycle estimate = %#v, want $2,000", estimate)
	}
	if estimate.Source != weeklyEstimateSourceCreditsCurrent || estimate.Role != weeklyEstimateRoleFormal || estimate.IntervalKind != weeklyEstimateIntervalComplete || estimate.BaselineAtMS != previousAtMS {
		t.Fatalf("previous-cycle provenance = %#v", estimate)
	}
	if estimate.WeeklyPoolMinUSD == nil || math.Abs(*estimate.WeeklyPoolMinUSD-1_600) > 0.000001 || estimate.WeeklyPoolMaxUSD == nil || math.Abs(*estimate.WeeklyPoolMaxUSD-2_000) > 0.000001 {
		t.Fatalf("previous-cycle range = %#v, want $1,600-$2,000", estimate)
	}
}

func TestGetRunLearnsCreditsFromStaleResetAtTransition(t *testing.T) {
	ctx := context.Background()
	db := newCodexInspectionTestStore(t)
	svc := newCodexInspectionTestService(t, db)
	analyticsZone := time.FixedZone("UTC+08:00", 8*60*60)
	previousCycleStart := time.Date(2026, time.August, 17, 0, 0, 0, 0, analyticsZone)
	currentCycleStart := previousCycleStart.Add(7 * 24 * time.Hour)
	preTransitionAtMS := time.Date(2026, time.August, 24, 8, 38, 0, 0, analyticsZone).UnixMilli()
	firstCurrentAtMS := time.Date(2026, time.August, 24, 8, 43, 0, 0, analyticsZone).UnixMilli()
	laterCurrentAtMS := firstCurrentAtMS + int64(10*time.Minute/time.Millisecond)
	staleResetAtMS := int64(1_787_818_055_000)
	currentResetAtMS := int64(1_788_136_946_000)

	insertWeeklyInspectionRun(t, db, preTransitionAtMS, staleResetAtMS, []weeklyInspectionSample{
		{authIndex: "auth-a", accountID: "account-a", usedPercent: 92},
	})
	firstCurrent := insertWeeklyInspectionRun(t, db, firstCurrentAtMS, currentResetAtMS, []weeklyInspectionSample{
		{
			authIndex: "auth-a", accountID: "account-a", usedPercent: 0,
			creditsUsage: &model.CodexCreditsUsage{
				CycleStartDate:         "2026-08-24",
				PreviousCycleCredits:   53_527.8553,
				PreviousCycleStartDate: "2026-08-17",
				LatestDate:             "2026-08-24",
				AnalyticsTimezone:      "UTC+08:00",
				ObservedAtMS:           firstCurrentAtMS,
			},
		},
	})
	if _, err := svc.GetRun(ctx, firstCurrent.ID); err != nil {
		t.Fatalf("get first post-transition run: %v", err)
	}
	laterCurrent := insertWeeklyInspectionRun(t, db, laterCurrentAtMS, currentResetAtMS, []weeklyInspectionSample{
		{
			authIndex: "auth-a", accountID: "account-a", usedPercent: 0,
			creditsUsage: &model.CodexCreditsUsage{
				CycleStartDate:         "2026-08-24",
				PreviousCycleCredits:   55_784.2201,
				PreviousCycleStartDate: "2026-08-17",
				LatestDate:             "2026-08-24",
				AnalyticsTimezone:      "UTC+08:00",
				ObservedAtMS:           laterCurrentAtMS,
			},
		},
	})
	detail, err := svc.GetRun(ctx, laterCurrent.ID)
	if err != nil {
		t.Fatalf("get updated post-transition run: %v", err)
	}
	estimate := weeklyEstimateForBasisRole(detail.Results[0].WeeklyPoolEstimates, weeklyEstimateBasisCredits, weeklyEstimateRoleFormal)
	if estimate == nil || estimate.WeeklyPoolUSD == nil {
		t.Fatalf("stale-reset transition estimate = %#v, want formal Credits estimate", estimate)
	}
	wantValue := 55_784.2201 * weeklyEstimateUSDPerCredit / 0.92
	if math.Abs(*estimate.WeeklyPoolUSD-wantValue) > 0.000001 || estimate.Credits != 55_784.2201 || estimate.UsedPercentDelta != 92 {
		t.Fatalf("stale-reset transition estimate = %#v, want latest previous-cycle Credits over 92%%", estimate)
	}
	if estimate.IntervalKind != weeklyEstimateIntervalComplete || estimate.CalculationVersion != creditsCalculationVersion || estimate.BaselineAtMS != preTransitionAtMS || estimate.UpdatedAtMS != laterCurrentAtMS {
		t.Fatalf("stale-reset transition provenance = %#v", estimate)
	}
	if estimate.IntervalStartMS != previousCycleStart.UnixMilli() || estimate.IntervalEndMS != currentCycleStart.UnixMilli() {
		t.Fatalf("stale-reset interval = [%d, %d], want [%d, %d]", estimate.IntervalStartMS, estimate.IntervalEndMS, previousCycleStart.UnixMilli(), currentCycleStart.UnixMilli())
	}
}

func TestGetRunDoesNotCompleteCreditsCycleWithoutResetTransition(t *testing.T) {
	ctx := context.Background()
	db := newCodexInspectionTestStore(t)
	svc := newCodexInspectionTestService(t, db)
	analyticsZone := time.FixedZone("UTC+08:00", 8*60*60)
	preTransitionAtMS := time.Date(2026, time.August, 24, 8, 38, 0, 0, analyticsZone).UnixMilli()
	currentAtMS := time.Date(2026, time.August, 24, 8, 43, 0, 0, analyticsZone).UnixMilli()
	previousResetAtMS := int64(1_787_818_055_000)
	driftedResetAtMS := previousResetAtMS + int64(5*time.Minute/time.Millisecond)

	insertWeeklyInspectionRun(t, db, preTransitionAtMS, previousResetAtMS, []weeklyInspectionSample{
		{authIndex: "auth-a", accountID: "account-a", usedPercent: 92},
	})
	current := insertWeeklyInspectionRun(t, db, currentAtMS, driftedResetAtMS, []weeklyInspectionSample{
		{
			authIndex: "auth-a", accountID: "account-a", usedPercent: 0,
			creditsUsage: &model.CodexCreditsUsage{
				CycleStartDate:         "2026-08-24",
				PreviousCycleCredits:   55_784.2201,
				PreviousCycleStartDate: "2026-08-17",
				LatestDate:             "2026-08-24",
				AnalyticsTimezone:      "UTC+08:00",
				ObservedAtMS:           currentAtMS,
			},
		},
	})
	detail, err := svc.GetRun(ctx, current.ID)
	if err != nil {
		t.Fatalf("get no-transition run: %v", err)
	}
	if estimate := weeklyEstimateForBasisRole(detail.Results[0].WeeklyPoolEstimates, weeklyEstimateBasisCredits, weeklyEstimateRoleFormal); estimate != nil {
		t.Fatalf("Credits cycle completed from ordinary resetAt drift: %#v", estimate)
	}
}

func TestGetRunDoesNotCompleteCreditsCycleBeforeAnalyticsReachesCycleStart(t *testing.T) {
	ctx := context.Background()
	db := newCodexInspectionTestStore(t)
	svc := newCodexInspectionTestService(t, db)
	analyticsZone := time.FixedZone("UTC+08:00", 8*60*60)
	previousCycleStart := time.Date(2027, time.January, 1, 0, 0, 0, 0, analyticsZone)
	previousResetAtMS := previousCycleStart.Add(7 * 24 * time.Hour).UnixMilli()
	previousAtMS := previousResetAtMS - int64(5*time.Minute/time.Millisecond)
	insertWeeklyInspectionRun(t, db, previousAtMS, previousResetAtMS, []weeklyInspectionSample{
		{authIndex: "auth-a", accountID: "account-a", usedPercent: 80},
	})
	currentAtMS := previousResetAtMS + int64(time.Hour/time.Millisecond)
	currentResetAtMS := previousResetAtMS + int64(codexWeekWindow)*1000
	current := insertWeeklyInspectionRun(t, db, currentAtMS, currentResetAtMS, []weeklyInspectionSample{
		{
			authIndex: "auth-a", accountID: "account-a", usedPercent: 2,
			creditsUsage: &model.CodexCreditsUsage{
				CycleStartDate:         "2027-01-08",
				PreviousCycleCredits:   40_000,
				PreviousCycleStartDate: "2027-01-01",
				LatestDate:             "2027-01-07",
				AnalyticsTimezone:      "UTC+08:00",
				ObservedAtMS:           currentAtMS,
			},
		},
	})
	detail, err := svc.GetRun(ctx, current.ID)
	if err != nil {
		t.Fatalf("get analytics-lag run: %v", err)
	}
	if estimate := weeklyEstimateForBasisRole(detail.Results[0].WeeklyPoolEstimates, weeklyEstimateBasisCredits, weeklyEstimateRoleFormal); estimate != nil {
		t.Fatalf("Credits cycle completed before Analytics reached the new cycle date: %#v", estimate)
	}
}

func TestGetRunPreservesCompleteCreditsFormalOverLaterPartialCurrent(t *testing.T) {
	ctx := context.Background()
	db := newCodexInspectionTestStore(t)
	svc := newCodexInspectionTestService(t, db)
	cycleStartAtMS := time.Date(2027, time.March, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	previousResetAtMS := cycleStartAtMS + int64(codexWeekWindow)*1000
	currentResetAtMS := previousResetAtMS + int64(codexWeekWindow)*1000
	formalValue := 2_400.0
	partialValue := 2_100.0
	formalUpdatedAtMS := cycleStartAtMS + 1
	partialUpdatedAtMS := cycleStartAtMS + 2

	if err := db.UpsertCodexWeeklyEstimateBaseline(ctx, "auth-a", "account-a", model.CodexWeeklyPoolEstimate{
		Basis: weeklyEstimateBasisCredits, Source: weeklyEstimateSourceCreditsCurrent, Role: weeklyEstimateRoleFormal,
		IntervalKind: weeklyEstimateIntervalComplete, CalculationVersion: creditsCalculationVersion,
		WeeklyPoolUSD: &formalValue, WeeklyResetAtMS: previousResetAtMS, UpdatedAtMS: formalUpdatedAtMS,
	}); err != nil {
		t.Fatalf("upsert complete formal Credits estimate: %v", err)
	}
	if err := db.UpsertCodexWeeklyEstimateBaseline(ctx, "auth-a", "account-a", model.CodexWeeklyPoolEstimate{
		Basis: weeklyEstimateBasisCredits, Source: weeklyEstimateSourceCreditsCurrent, Role: weeklyEstimateRoleCurrent,
		IntervalKind: weeklyEstimateIntervalPartial, CalculationVersion: creditsCalculationVersion,
		WeeklyPoolUSD: &partialValue, WeeklyResetAtMS: previousResetAtMS, UpdatedAtMS: partialUpdatedAtMS,
	}); err != nil {
		t.Fatalf("upsert previous partial Credits estimate: %v", err)
	}

	firstAtMS := previousResetAtMS + int64(time.Hour/time.Millisecond)
	first := insertWeeklyInspectionRun(t, db, firstAtMS, currentResetAtMS, []weeklyInspectionSample{
		{authIndex: "auth-a", accountID: "account-a", usedPercent: 0},
	})
	if _, err := svc.GetRun(ctx, first.ID); err != nil {
		t.Fatalf("get reset run: %v", err)
	}
	stored, err := db.ListCodexWeeklyEstimateBaselines(ctx, "auth-a", "account-a")
	if err != nil {
		t.Fatalf("list baselines after reset: %v", err)
	}
	formal := weeklyEstimateForBasisRole(stored, weeklyEstimateBasisCredits, weeklyEstimateRoleFormal)
	if !weeklyEstimateComplete(formal) || formal.WeeklyPoolUSD == nil || *formal.WeeklyPoolUSD != formalValue || formal.UpdatedAtMS != formalUpdatedAtMS {
		t.Fatalf("complete formal overwritten by approximate promotion: %#v", formal)
	}

	laterPartialValue := 2_200.0
	if err := db.UpsertCodexWeeklyEstimateBaseline(ctx, "auth-a", "account-a", model.CodexWeeklyPoolEstimate{
		Basis: weeklyEstimateBasisCredits, Source: weeklyEstimateSourceCreditsCurrent, Role: weeklyEstimateRoleCurrent,
		IntervalKind: weeklyEstimateIntervalPartial, CalculationVersion: creditsCalculationVersion,
		WeeklyPoolUSD: &laterPartialValue, WeeklyResetAtMS: currentResetAtMS, UpdatedAtMS: firstAtMS + 1,
	}); err != nil {
		t.Fatalf("upsert later partial Credits estimate: %v", err)
	}
	later := insertWeeklyInspectionRun(t, db, firstAtMS+2, currentResetAtMS, []weeklyInspectionSample{
		{authIndex: "auth-a", accountID: "account-a", usedPercent: 1},
	})
	detail, err := svc.GetRun(ctx, later.ID)
	if err != nil {
		t.Fatalf("get later partial run: %v", err)
	}
	primary := detail.Results[0].WeeklyPoolEstimate
	if !weeklyEstimateComplete(primary) || primary.WeeklyPoolUSD == nil || *primary.WeeklyPoolUSD != formalValue {
		t.Fatalf("primary estimate = %#v, want preserved complete formal Credits estimate", primary)
	}
	current := weeklyEstimateForBasisRole(detail.Results[0].WeeklyPoolEstimates, weeklyEstimateBasisCredits, weeklyEstimateRoleCurrent)
	if current == nil || current.WeeklyPoolUSD == nil || *current.WeeklyPoolUSD != laterPartialValue {
		t.Fatalf("later partial current missing from estimate list: %#v", detail.Results[0].WeeklyPoolEstimates)
	}
}

type weeklyInspectionSample struct {
	authIndex    string
	accountID    string
	usedPercent  float64
	creditsUsage *model.CodexCreditsUsage
}

func insertWeeklyInspectionRun(t *testing.T, db *store.Store, createdAtMS, resetAtMS int64, samples []weeklyInspectionSample) model.CodexInspectionRun {
	t.Helper()
	ctx := context.Background()
	run, err := db.CreateCodexInspectionRun(ctx, model.CodexInspectionRun{
		TriggerType:  model.CodexInspectionTriggerManual,
		Status:       model.CodexInspectionStatusCompleted,
		StartedAtMS:  createdAtMS,
		FinishedAtMS: createdAtMS,
		Settings:     model.DefaultCodexInspectionConfig(),
		CreatedAtMS:  createdAtMS,
	})
	if err != nil {
		t.Fatalf("create inspection run: %v", err)
	}
	for index, sample := range samples {
		usedPercent := sample.usedPercent
		windowSeconds := float64(codexWeekWindow)
		_, err := db.InsertCodexInspectionResult(ctx, model.CodexInspectionResult{
			RunID:           run.ID,
			AccountKey:      fmt.Sprintf("%s-%d", sample.accountID, index),
			FileName:        sample.accountID + ".json",
			DisplayAccount:  sample.accountID,
			AuthIndex:       sample.authIndex,
			AccountID:       sample.accountID,
			AccountSnapshot: sample.accountID,
			Provider:        model.CodexInspectionTargetCodex,
			Action:          "keep",
			PlanType:        "pro",
			QuotaWindows: []model.CodexInspectionQuotaWindow{
				{
					ID:                 "weekly",
					LabelKey:           "codex_quota.secondary_window",
					UsedPercent:        &usedPercent,
					ResetLabel:         formatUnixMilliseconds(resetAtMS),
					ResetAtMS:          resetAtMS,
					LimitWindowSeconds: &windowSeconds,
				},
			},
			CreditsUsage: sample.creditsUsage,
			CreatedAtMS:  createdAtMS,
		})
		if err != nil {
			t.Fatalf("insert inspection result: %v", err)
		}
	}
	return run
}

func weeklyEstimateUsageEvent(hash string, timestampMS int64, authIndex, accountSnapshot string, inputTokens int64) usage.Event {
	return usage.Event{
		EventHash:       hash,
		TimestampMS:     timestampMS,
		Timestamp:       time.UnixMilli(timestampMS).UTC().Format(time.RFC3339Nano),
		Provider:        model.CodexInspectionTargetCodex,
		Model:           "gpt-5.4",
		ResolvedModel:   "gpt-5.4",
		AuthIndex:       authIndex,
		AccountSnapshot: accountSnapshot,
		InputTokens:     inputTokens,
		TotalTokens:     inputTokens,
		CreatedAtMS:     timestampMS,
	}
}

func assertWeeklyEstimate(t *testing.T, estimate *model.CodexWeeklyPoolEstimate, cost, delta, value float64, baselineAtMS, resetAtMS int64) {
	t.Helper()
	if estimate == nil || estimate.WeeklyPoolUSD == nil {
		t.Fatalf("estimate = %#v, want value", estimate)
	}
	if estimate.Official {
		t.Fatal("estimate must not be marked official")
	}
	if estimate.Status != weeklyEstimateStatusPreliminary {
		t.Fatalf("status = %q, want preliminary for a 1 pp-quantized 5 pp interval", estimate.Status)
	}
	if math.Abs(estimate.CostDeltaUSD-cost) > 0.000001 || math.Abs(estimate.UsedPercentDelta-delta) > 0.000001 || math.Abs(*estimate.WeeklyPoolUSD-value) > 0.000001 {
		t.Fatalf("estimate = %#v, want cost %.2f delta %.2f value %.2f", estimate, cost, delta, value)
	}
	if estimate.BaselineAtMS != baselineAtMS || estimate.WeeklyResetAtMS != resetAtMS {
		t.Fatalf("estimate window = %#v, want baseline %d reset %d", estimate, baselineAtMS, resetAtMS)
	}
	if estimate.CalculationVersion != cpaCalculationVersion || estimate.CaptureState != "closed" || estimate.RouteScope != "observed" || estimate.QuotaScope != "matched" {
		t.Fatalf("CPA provenance = %#v", estimate)
	}
	if estimate.WeeklyPoolMinUSD == nil || estimate.WeeklyPoolMaxUSD == nil ||
		math.Abs(*estimate.WeeklyPoolMinUSD-cost/(6.0/100)) > 0.000001 ||
		math.Abs(*estimate.WeeklyPoolMaxUSD-cost/(4.0/100)) > 0.000001 {
		t.Fatalf("CPA range = %#v, want 4-6 pp propagated bounds", estimate)
	}
	if len(estimate.PriceSources) != 1 || estimate.PriceSources[0] != "models.dev" {
		t.Fatalf("price sources = %#v, want models.dev", estimate.PriceSources)
	}
}
