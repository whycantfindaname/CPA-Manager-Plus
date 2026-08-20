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
		weeklyEstimateUsageEvent("event-a", eventAtMS, "auth-a", 100_000),
		weeklyEstimateUsageEvent("event-b", eventAtMS, "auth-b", 200_000),
		weeklyEstimateUsageEvent("event-c", eventAtMS, "auth-c", 200_000),
		weeklyEstimateUsageEvent("event-other", eventAtMS, "auth-other", 20_000_000),
	}); err != nil {
		t.Fatalf("insert usage events: %v", err)
	}

	currentResetAtMS := resetAtMS + 1_000
	currentRun := insertWeeklyInspectionRun(t, db, currentAtMS, currentResetAtMS, []weeklyInspectionSample{
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
	if estimate := estimates["account-c"]; estimate == nil || estimate.Status != weeklyEstimateStatusInsufficient || estimate.WeeklyPoolUSD != nil || estimate.Reason != "delta_too_small" {
		t.Fatalf("zero-delta estimate = %#v, want insufficient without a dollar value", estimate)
	}
}

func TestGetRunUsesCreditsThenCarriesLearnedBaselineAcrossReset(t *testing.T) {
	ctx := context.Background()
	db := newCodexInspectionTestStore(t)
	svc := newCodexInspectionTestService(t, db)
	firstAtMS := int64(1_800_000_000_000)
	firstResetAtMS := firstAtMS + int64(codexWeekWindow)*1000
	firstRun := insertWeeklyInspectionRun(t, db, firstAtMS, firstResetAtMS, []weeklyInspectionSample{
		{
			authIndex:   "auth-a",
			accountID:   "account-a",
			usedPercent: 4,
			creditsUsage: &model.CodexCreditsUsage{
				CurrentCycleCredits: 2_000,
				CycleStartDate:      "2027-01-01",
				LatestDate:          "2027-01-02",
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
	}
	if estimate := learned["account-b"]; estimate == nil || estimate.WeeklyPoolUSD != nil {
		t.Fatalf("account-b estimate = %#v, want isolated empty baseline", estimate)
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
			RunID:          run.ID,
			AccountKey:     fmt.Sprintf("%s-%d", sample.accountID, index),
			FileName:       sample.accountID + ".json",
			DisplayAccount: sample.accountID,
			AuthIndex:      sample.authIndex,
			AccountID:      sample.accountID,
			Provider:       model.CodexInspectionTargetCodex,
			Action:         "keep",
			PlanType:       "pro",
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

func weeklyEstimateUsageEvent(hash string, timestampMS int64, authIndex string, inputTokens int64) usage.Event {
	return usage.Event{
		EventHash:     hash,
		TimestampMS:   timestampMS,
		Timestamp:     time.UnixMilli(timestampMS).UTC().Format(time.RFC3339Nano),
		Provider:      model.CodexInspectionTargetCodex,
		Model:         "gpt-5.4",
		ResolvedModel: "gpt-5.4",
		AuthIndex:     authIndex,
		InputTokens:   inputTokens,
		TotalTokens:   inputTokens,
		CreatedAtMS:   timestampMS,
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
	if estimate.Status != weeklyEstimateStatusReliable {
		t.Fatalf("status = %q, want reliable", estimate.Status)
	}
	if math.Abs(estimate.CostDeltaUSD-cost) > 0.000001 || math.Abs(estimate.UsedPercentDelta-delta) > 0.000001 || math.Abs(*estimate.WeeklyPoolUSD-value) > 0.000001 {
		t.Fatalf("estimate = %#v, want cost %.2f delta %.2f value %.2f", estimate, cost, delta, value)
	}
	if estimate.BaselineAtMS != baselineAtMS || estimate.WeeklyResetAtMS != resetAtMS {
		t.Fatalf("estimate window = %#v, want baseline %d reset %d", estimate, baselineAtMS, resetAtMS)
	}
	if len(estimate.PriceSources) != 1 || estimate.PriceSources[0] != "models.dev" {
		t.Fatalf("price sources = %#v, want models.dev", estimate.PriceSources)
	}
}
