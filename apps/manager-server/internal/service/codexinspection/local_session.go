package codexinspection

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const (
	localCodexSessionSource  = "codex_app_server"
	localCodexSessionTimeout = 20 * time.Second
	maxAppServerMessageBytes = 2 * 1024 * 1024
)

var ErrLocalCodexUnavailable = errors.New("codex app-server is unavailable")

type LocalCodexSessionReader interface {
	Read(context.Context) (LocalCodexSessionSnapshot, error)
}

type LocalCodexSessionResponse struct {
	Status   string                     `json:"status"`
	Source   string                     `json:"source"`
	Reason   string                     `json:"reason,omitempty"`
	Snapshot *LocalCodexSessionSnapshot `json:"snapshot,omitempty"`
}

type LocalCodexSessionSnapshot struct {
	CapturedAtMS int64                   `json:"capturedAtMs"`
	Account      LocalCodexAccount       `json:"account"`
	QuotaBuckets []LocalCodexQuotaBucket `json:"quotaBuckets"`
	Usage        LocalCodexUsage         `json:"usage"`
}

type LocalCodexAccount struct {
	Type     string `json:"type,omitempty"`
	PlanType string `json:"planType,omitempty"`
	Email    string `json:"email,omitempty"`
}

type LocalCodexQuotaBucket struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name,omitempty"`
	PlanType  string                 `json:"planType,omitempty"`
	Primary   *LocalCodexQuotaWindow `json:"primary,omitempty"`
	Secondary *LocalCodexQuotaWindow `json:"secondary,omitempty"`
}

type LocalCodexQuotaWindow struct {
	UsedPercent       *float64 `json:"usedPercent,omitempty"`
	WindowDurationMin *float64 `json:"windowDurationMins,omitempty"`
	ResetsAt          *float64 `json:"resetsAt,omitempty"`
}

type LocalCodexUsage struct {
	Summary           LocalCodexUsageSummary `json:"summary"`
	DailyUsageBuckets []LocalCodexDailyUsage `json:"dailyUsageBuckets,omitempty"`
}

type LocalCodexUsageSummary struct {
	LifetimeTokens        *int64 `json:"lifetimeTokens,omitempty"`
	PeakDailyTokens       *int64 `json:"peakDailyTokens,omitempty"`
	LongestRunningTurnSec *int64 `json:"longestRunningTurnSec,omitempty"`
	CurrentStreakDays     *int64 `json:"currentStreakDays,omitempty"`
	LongestStreakDays     *int64 `json:"longestStreakDays,omitempty"`
}

type LocalCodexDailyUsage struct {
	StartDate string `json:"startDate"`
	Tokens    int64  `json:"tokens"`
}

type appServerSessionReader struct {
	executable     string
	timeout        time.Duration
	commandFactory func(context.Context, string, ...string) *exec.Cmd
}

type appServerRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type appServerRPCResponse struct {
	ID     int                `json:"id"`
	Result json.RawMessage    `json:"result"`
	Error  *appServerRPCError `json:"error"`
}

type appServerAccountResult struct {
	Account LocalCodexAccount `json:"account"`
}

type appServerRateLimit struct {
	LimitID   string                 `json:"limitId"`
	LimitName string                 `json:"limitName"`
	PlanType  string                 `json:"planType"`
	Primary   *LocalCodexQuotaWindow `json:"primary"`
	Secondary *LocalCodexQuotaWindow `json:"secondary"`
}

type appServerRateLimitsResult struct {
	RateLimits          *appServerRateLimit           `json:"rateLimits"`
	RateLimitsByLimitID map[string]appServerRateLimit `json:"rateLimitsByLimitId"`
}

type appServerUsageResult struct {
	Summary           LocalCodexUsageSummary `json:"summary"`
	DailyUsageBuckets []LocalCodexDailyUsage `json:"dailyUsageBuckets"`
}

func NewAppServerSessionReader() LocalCodexSessionReader {
	return &appServerSessionReader{
		executable:     strings.TrimSpace(os.Getenv("CPAMP_CODEX_EXECUTABLE")),
		timeout:        localCodexSessionTimeout,
		commandFactory: exec.CommandContext,
	}
}

func (s *Service) ReadLocalCodexSession(ctx context.Context) LocalCodexSessionResponse {
	snapshot, err := s.localSessionReader.Read(ctx)
	if err == nil {
		return LocalCodexSessionResponse{
			Status:   "available",
			Source:   localCodexSessionSource,
			Snapshot: &snapshot,
		}
	}
	reason := "app_server_failed"
	if errors.Is(err, ErrLocalCodexUnavailable) {
		reason = "codex_executable_not_found"
	}
	return LocalCodexSessionResponse{
		Status: "unavailable",
		Source: localCodexSessionSource,
		Reason: reason,
	}
}

func (r *appServerSessionReader) Read(ctx context.Context) (LocalCodexSessionSnapshot, error) {
	executable, err := r.resolveExecutable()
	if err != nil {
		return LocalCodexSessionSnapshot{}, err
	}
	timeout := r.timeout
	if timeout <= 0 {
		timeout = localCodexSessionTimeout
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	commandFactory := r.commandFactory
	if commandFactory == nil {
		commandFactory = exec.CommandContext
	}
	cmd := commandFactory(probeCtx, executable, "app-server", "--listen", "stdio://")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return LocalCodexSessionSnapshot{}, fmt.Errorf("open app-server stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return LocalCodexSessionSnapshot{}, fmt.Errorf("open app-server stdout: %w", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, os.ErrNotExist) {
			return LocalCodexSessionSnapshot{}, ErrLocalCodexUnavailable
		}
		return LocalCodexSessionSnapshot{}, fmt.Errorf("start codex app-server: %w", err)
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	defer stopAppServer(cmd, stdin, waitCh)

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), maxAppServerMessageBytes)
	if err := writeAppServerMessage(stdin, map[string]any{
		"method": "initialize",
		"id":     1,
		"params": map[string]any{
			"clientInfo": map[string]any{
				"name":    "cpa_manager_plus",
				"title":   "CPA Manager Plus",
				"version": "1",
			},
			"capabilities": map[string]any{},
		},
	}); err != nil {
		return LocalCodexSessionSnapshot{}, err
	}
	if _, err := readAppServerResult(scanner, 1); err != nil {
		return LocalCodexSessionSnapshot{}, err
	}
	if err := writeAppServerMessage(stdin, map[string]any{"method": "initialized", "params": map[string]any{}}); err != nil {
		return LocalCodexSessionSnapshot{}, err
	}

	var account appServerAccountResult
	if err := requestAppServer(scanner, stdin, 2, "account/read", map[string]any{"refreshToken": false}, &account); err != nil {
		return LocalCodexSessionSnapshot{}, err
	}
	var limits appServerRateLimitsResult
	if err := requestAppServer(scanner, stdin, 3, "account/rateLimits/read", nil, &limits); err != nil {
		return LocalCodexSessionSnapshot{}, err
	}
	var usage appServerUsageResult
	if err := requestAppServer(scanner, stdin, 4, "account/usage/read", nil, &usage); err != nil {
		return LocalCodexSessionSnapshot{}, err
	}

	buckets := normalizeAppServerQuotaBuckets(limits)
	return LocalCodexSessionSnapshot{
		CapturedAtMS: time.Now().UnixMilli(),
		Account:      account.Account,
		QuotaBuckets: buckets,
		Usage: LocalCodexUsage{
			Summary:           usage.Summary,
			DailyUsageBuckets: usage.DailyUsageBuckets,
		},
	}, nil
}

func (r *appServerSessionReader) resolveExecutable() (string, error) {
	if r.executable != "" {
		if _, err := os.Stat(r.executable); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", ErrLocalCodexUnavailable
			}
			return "", fmt.Errorf("inspect configured codex executable: %w", err)
		}
		return r.executable, nil
	}
	executable, err := exec.LookPath("codex")
	if err != nil {
		return "", ErrLocalCodexUnavailable
	}
	return executable, nil
}

func requestAppServer(scanner *bufio.Scanner, stdin io.Writer, id int, method string, params any, target any) error {
	message := map[string]any{"method": method, "id": id}
	if params != nil {
		message["params"] = params
	}
	if err := writeAppServerMessage(stdin, message); err != nil {
		return err
	}
	result, err := readAppServerResult(scanner, id)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(result, target); err != nil {
		return fmt.Errorf("decode %s result: %w", method, err)
	}
	return nil
}

func writeAppServerMessage(writer io.Writer, message any) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode app-server request: %w", err)
	}
	payload = append(payload, '\n')
	if _, err := writer.Write(payload); err != nil {
		return fmt.Errorf("write app-server request: %w", err)
	}
	return nil
}

func readAppServerResult(scanner *bufio.Scanner, expectedID int) (json.RawMessage, error) {
	for scanner.Scan() {
		var response appServerRPCResponse
		if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
			continue
		}
		if response.ID != expectedID {
			continue
		}
		if response.Error != nil {
			return nil, fmt.Errorf("app-server request %d failed: %s", expectedID, response.Error.Message)
		}
		return response.Result, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read app-server response: %w", err)
	}
	return nil, errors.New("codex app-server closed before responding")
}

func normalizeAppServerQuotaBuckets(result appServerRateLimitsResult) []LocalCodexQuotaBucket {
	byID := result.RateLimitsByLimitID
	if len(byID) == 0 && result.RateLimits != nil {
		id := strings.TrimSpace(result.RateLimits.LimitID)
		if id == "" {
			id = "preferred"
		}
		byID = map[string]appServerRateLimit{id: *result.RateLimits}
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	buckets := make([]LocalCodexQuotaBucket, 0, len(ids))
	for _, id := range ids {
		bucket := byID[id]
		if bucket.LimitID != "" {
			id = bucket.LimitID
		}
		buckets = append(buckets, LocalCodexQuotaBucket{
			ID:        id,
			Name:      bucket.LimitName,
			PlanType:  bucket.PlanType,
			Primary:   bucket.Primary,
			Secondary: bucket.Secondary,
		})
	}
	return buckets
}

func stopAppServer(cmd *exec.Cmd, stdin io.Closer, waitCh <-chan error) {
	_ = stdin.Close()
	select {
	case <-waitCh:
		return
	case <-time.After(2 * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-waitCh
	}
}
