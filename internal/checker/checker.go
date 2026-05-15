// Package checker performs HTTP uptime checks at a fixed interval.
package checker

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/maxlesscode/ping-itor/internal/notifier"
	"github.com/maxlesscode/ping-itor/internal/store"
)

// CheckResult carries the outcome of a single HTTP check.
type CheckResult struct {
	MonitorID int64
	IsUp      bool
	LatencyMs int64
	HTTPCode  int
	Err       error
}

// RunInput groups the dependencies and settings for Run.
type RunInput struct {
	Store     *store.Store
	Notifier  *notifier.Notifier
	Broadcast func(CheckResult)
	Interval  time.Duration
	Timeout   time.Duration
}

// Run starts the check loop. It ticks at input.Interval, checks every monitor
// in parallel, writes results to the store, and dispatches alerts and broadcasts.
// It returns when ctx is cancelled.
func Run(ctx context.Context, input RunInput) {
	ticker := time.NewTicker(input.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runOnce(ctx, input)
		}
	}
}

func runOnce(ctx context.Context, input RunInput) {
	monitors, err := input.Store.List(ctx)
	if err != nil {
		slog.Error("checker: list monitors", "err", err)
		return
	}

	var wg sync.WaitGroup
	for _, m := range monitors {
		wg.Add(1)
		go func(m store.Monitor) {
			defer wg.Done()
			result := checkOne(ctx, m, input.Timeout)
			handleResult(ctx, result, m, input)
		}(m)
	}
	wg.Wait()
}

func checkOne(ctx context.Context, m store.Monitor, timeout time.Duration) CheckResult {
	result := CheckResult{MonitorID: m.ID}

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, m.URL, nil)
	if err != nil {
		result.Err = fmt.Errorf("build request for %s: %w", m.URL, err)
		return result
	}

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return nil // follow redirects
		},
	}

	start := time.Now()
	resp, err := client.Do(req)
	result.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Err = fmt.Errorf("http get %s: %w", m.URL, err)
		return result
	}
	defer resp.Body.Close()

	result.HTTPCode = resp.StatusCode
	result.IsUp = resp.StatusCode >= 200 && resp.StatusCode < 400
	return result
}

func handleResult(ctx context.Context, result CheckResult, m store.Monitor, input RunInput) {
	slog.Info("checker: result",
		"monitor", m.Name,
		"url", m.URL,
		"up", result.IsUp,
		"latency_ms", result.LatencyMs,
		"http_code", result.HTTPCode,
		"err", result.Err,
	)

	update := store.UpdateStatusInput{
		IsUp:      result.IsUp,
		LatencyMs: result.LatencyMs,
		HTTPCode:  result.HTTPCode,
	}

	if result.IsUp {
		update.ConsecutiveFailures = 0
		if m.AlertSent {
			// Monitor recovered — send recovery alert and reset state.
			update.AlertSent = false
			if err := input.Notifier.SendRecovery(ctx, m); err != nil {
				slog.Error("checker: send recovery alert", "monitor", m.Name, "err", err)
			}
		} else {
			update.AlertSent = false
		}
	} else {
		update.ConsecutiveFailures = m.ConsecutiveFailures + 1
		if update.ConsecutiveFailures >= 2 && !m.AlertSent {
			// Two consecutive failures — send down alert.
			update.AlertSent = true
			if err := input.Notifier.SendDown(ctx, m); err != nil {
				slog.Error("checker: send down alert", "monitor", m.Name, "err", err)
			}
		} else {
			update.AlertSent = m.AlertSent
		}
	}

	if err := input.Store.UpdateStatus(ctx, m.ID, update); err != nil {
		slog.Error("checker: update status", "monitor", m.Name, "err", err)
		return
	}

	input.Broadcast(result)
}
