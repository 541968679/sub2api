//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/loadtest"
)

func TestChannelLoadtestService_StartManualSmoke(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("auth")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"model\":\"kimi-k3\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"model\":\"kimi-k3\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":1}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	svc := NewChannelLoadtestService(nil, nil)
	snap, err := svc.Start(context.Background(), ChannelLoadtestStartInput{
		BaseURL:     srv.URL,
		APIKey:      "sk-test",
		Model:       "kimi-k3",
		Profile:     "smoke",
		Concurrency: 1,
		Total:       1,
		MaxTokens:   8,
		Tools:       "off",
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, getErr := svc.Get(snap.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if got.Status == "done" {
			if got.OK < 1 || got.ModelMissingReqs != 0 {
				t.Fatalf("%+v", got)
			}
			return
		}
		if got.Status == "failed" {
			t.Fatalf("failed: %s", got.Error)
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("timed out waiting for run")
}

func TestChannelLoadtestService_ExportExcelAfterDone(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"model\":\"kimi-k3\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"model\":\"kimi-k3\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":1}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	svc := NewChannelLoadtestService(nil, nil)
	snap, err := svc.Start(context.Background(), ChannelLoadtestStartInput{
		BaseURL:     srv.URL,
		APIKey:      "sk-test",
		Model:       "kimi-k3",
		Profile:     "smoke",
		Concurrency: 1,
		Total:       2,
		MaxTokens:   8,
		Tools:       "off",
		Tiers:       []loadtest.Tier{{InputTokens: 80, Count: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, getErr := svc.Get(snap.ID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		if got.Status == "done" {
			raw, name, expErr := svc.ExportExcel(snap.ID)
			if expErr != nil {
				t.Fatal(expErr)
			}
			if !strings.HasSuffix(name, ".xlsx") || len(raw) < 100 {
				t.Fatalf("name=%s size=%d", name, len(raw))
			}
			if _, _, still := svc.ExportExcel(snap.ID); still != nil && strings.Contains(still.Error(), "running") {
				t.Fatal(still)
			}
			return
		}
		if got.Status == "failed" {
			t.Fatalf("failed: %s", got.Error)
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("timed out waiting for run")
}

func TestChannelLoadtestService_RejectsTierShape(t *testing.T) {
	svc := NewChannelLoadtestService(nil, nil)
	_, err := svc.Start(context.Background(), ChannelLoadtestStartInput{
		Model: "glm-5.3",
		Tiers: []loadtest.Tier{{InputTokens: 0, Count: 1}},
	})
	if err == nil || !strings.Contains(err.Error(), "input_tokens") {
		t.Fatal(err)
	}
	_, err = svc.Start(context.Background(), ChannelLoadtestStartInput{
		Model: "glm-5.3",
		Tiers: []loadtest.Tier{{InputTokens: 100, Count: 300}, {InputTokens: 100, Count: 300}},
	})
	if err == nil || !strings.Contains(err.Error(), "sum") {
		t.Fatal(err)
	}
}

func TestChannelLoadtestService_TierEstimateRequiresConfirm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream should not be called")
	}))
	defer srv.Close()
	svc := NewChannelLoadtestService(nil, nil)
	_, err := svc.Start(context.Background(), ChannelLoadtestStartInput{
		BaseURL: srv.URL,
		APIKey:  "sk-test",
		Model:   "glm-5.3",
		Total:   40,
		Tiers:   []loadtest.Tier{{InputTokens: 50000, Count: 50}},
	})
	if err == nil || !strings.Contains(err.Error(), "confirm_cost") {
		t.Fatal(err)
	}
}

func TestChannelLoadtestService_TiersOverrideTotal(t *testing.T) {
	var mu sync.Mutex
	var maxTokens []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			MaxTokens *int `json:"max_tokens"`
		}
		_ = json.Unmarshal(body, &req)
		mu.Lock()
		if req.MaxTokens != nil {
			maxTokens = append(maxTokens, *req.MaxTokens)
		} else {
			maxTokens = append(maxTokens, 0)
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"model\":\"glm-5.3\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"model\":\"glm-5.3\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":1}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	svc := NewChannelLoadtestService(nil, nil)
	snap, err := svc.Start(context.Background(), ChannelLoadtestStartInput{
		BaseURL:     srv.URL,
		APIKey:      "sk-test",
		Model:       "glm-5.3",
		Profile:     "user363-sla",
		Concurrency: 1,
		Total:       40,
		DurationSec: 5,
		MaxTokens:   256,
		InputTokens: 50,
		SizeCap:     10,
		Tools:       "off",
		Tiers: []loadtest.Tier{
			{InputTokens: 1000, Count: 2},
			{InputTokens: 2000, Count: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Total != 3 || len(snap.Tiers) != 2 || snap.DurationSec != 0 {
		t.Fatalf("snapshot total=%d tiers=%d duration=%d", snap.Total, len(snap.Tiers), snap.DurationSec)
	}
	deadline := time.Now().Add(5 * time.Second)
	var got *ChannelLoadtestSnapshot
	for time.Now().Before(deadline) {
		got, err = svc.Get(snap.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status == "done" || got.Status == "failed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got == nil || got.Status != "done" || got.Done != 3 {
		t.Fatalf("status=%s done=%d err=%s", got.Status, got.Done, got.Error)
	}
	counts := map[int]int{}
	for _, row := range got.Results {
		counts[row.TargetTokens]++
	}
	if counts[1000] != 2 || counts[2000] != 1 {
		t.Fatalf("targets %v", counts)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(maxTokens) != 3 {
		t.Fatalf("bodies %v", maxTokens)
	}
	for _, n := range maxTokens {
		if n != 256 {
			t.Fatalf("max_tokens %v", maxTokens)
		}
	}
}

func TestChannelLoadtestService_RequiresTarget(t *testing.T) {
	svc := NewChannelLoadtestService(nil, nil)
	_, err := svc.Start(context.Background(), ChannelLoadtestStartInput{Model: "kimi-k3", Profile: "smoke", Concurrency: 1, Total: 1})
	if err == nil {
		t.Fatal("expected error")
	}
}
