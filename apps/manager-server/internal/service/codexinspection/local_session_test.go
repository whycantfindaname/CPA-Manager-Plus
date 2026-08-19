package codexinspection

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestAppServerSessionReaderReadsAccountQuotaAndUsage(t *testing.T) {
	reader := &appServerSessionReader{
		executable: os.Args[0],
		timeout:    5 * time.Second,
		commandFactory: func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestLocalCodexAppServerHelperProcess", "--")
			cmd.Env = append(os.Environ(), "CPAMP_APP_SERVER_HELPER=1")
			return cmd
		},
	}

	snapshot, err := reader.Read(context.Background())
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if snapshot.Account.PlanType != "pro" || snapshot.Account.Email != "owner@example.com" {
		t.Fatalf("account = %#v", snapshot.Account)
	}
	if len(snapshot.QuotaBuckets) != 2 || snapshot.QuotaBuckets[0].ID != "codex" || snapshot.QuotaBuckets[1].ID != "codex_spark" {
		t.Fatalf("quota buckets = %#v", snapshot.QuotaBuckets)
	}
	if got := snapshot.QuotaBuckets[0].Primary.UsedPercent; got == nil || *got != 79 {
		t.Fatalf("primary used percent = %v", got)
	}
	if len(snapshot.Usage.DailyUsageBuckets) != 1 || snapshot.Usage.DailyUsageBuckets[0].Tokens != 12345 {
		t.Fatalf("usage = %#v", snapshot.Usage)
	}
}

func TestAppServerSessionReaderMissingExecutableIsUnavailable(t *testing.T) {
	reader := &appServerSessionReader{executable: t.TempDir() + "/missing-codex"}
	_, err := reader.Read(context.Background())
	if !errors.Is(err, ErrLocalCodexUnavailable) {
		t.Fatalf("Read() error = %v, want ErrLocalCodexUnavailable", err)
	}
}

type stubLocalSessionReader struct {
	snapshot LocalCodexSessionSnapshot
	err      error
}

func (r stubLocalSessionReader) Read(context.Context) (LocalCodexSessionSnapshot, error) {
	return r.snapshot, r.err
}

func TestReadLocalCodexSessionReturnsUnavailableWithoutLeakingError(t *testing.T) {
	service := &Service{localSessionReader: stubLocalSessionReader{err: errors.New("token secret should not escape")}}
	response := service.ReadLocalCodexSession(context.Background())
	if response.Status != "unavailable" || response.Reason != "app_server_failed" || response.Snapshot != nil {
		t.Fatalf("response = %#v", response)
	}
}

func TestLocalCodexAppServerHelperProcess(t *testing.T) {
	if os.Getenv("CPAMP_APP_SERVER_HELPER") != "1" {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for scanner.Scan() {
		var request struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			continue
		}
		switch request.Method {
		case "initialize":
			_ = encoder.Encode(map[string]any{"id": request.ID, "result": map[string]any{}})
		case "account/read":
			_ = encoder.Encode(map[string]any{
				"id": request.ID,
				"result": map[string]any{"account": map[string]any{
					"type": "chatgpt", "planType": "pro", "email": "owner@example.com",
				}},
			})
		case "account/rateLimits/read":
			_ = encoder.Encode(map[string]any{
				"id": request.ID,
				"result": map[string]any{"rateLimitsByLimitId": map[string]any{
					"codex_spark": map[string]any{
						"limitId": "codex_spark", "limitName": "Codex Spark", "planType": "pro",
						"primary": map[string]any{"usedPercent": 12, "windowDurationMins": 10080, "resetsAt": 1780000100},
					},
					"codex": map[string]any{
						"limitId": "codex", "planType": "pro",
						"primary": map[string]any{"usedPercent": 79, "windowDurationMins": 10080, "resetsAt": 1780000000},
					},
				}},
			})
		case "account/usage/read":
			_ = encoder.Encode(map[string]any{
				"id": request.ID,
				"result": map[string]any{
					"summary":           map[string]any{"lifetimeTokens": 987654},
					"dailyUsageBuckets": []map[string]any{{"startDate": "2026-08-19", "tokens": 12345}},
				},
			})
		}
	}
	os.Exit(0)
}
