//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestChannelLoadtestService_RequiresTarget(t *testing.T) {
	svc := NewChannelLoadtestService(nil, nil)
	_, err := svc.Start(context.Background(), ChannelLoadtestStartInput{Model: "kimi-k3", Profile: "smoke", Concurrency: 1, Total: 1})
	if err == nil {
		t.Fatal("expected error")
	}
}
