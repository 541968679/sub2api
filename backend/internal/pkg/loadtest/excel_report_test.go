package loadtest

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestBuildExcelReportMetricByTier(t *testing.T) {
	started := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	ended := started.Add(2 * time.Minute)
	accID := int64(12)
	in := ExcelExportInput{
		RunID:       "run-1",
		Status:      "done",
		StartedAt:   started,
		EndedAt:     &ended,
		Source:      "account",
		AccountID:   &accID,
		AccountName: "kimi",
		BaseURL:     "https://example.com",
		Path:        "/v1/chat/completions",
		Models:      []string{"kimi-k3"},
		APIMode:     APIModeChatCompletions,
		StreamMode:  StreamModeStream,
		Profile:     "user363-sla",
		Tiers: []Tier{
			{InputTokens: 50000, Count: 2},
			{InputTokens: 80000, Count: 1},
		},
		Concurrency: 10,
		Total:       3,
		MaxTokens:   256,
		Peak:        5,
		Done:        3,
		OK:          2,
		SuccessRate: 66.7,
		RPMPeak:     2,
		RPMAvg:      1.5,
		Results: []Result{
			{TargetTokens: 50000, Outcome: "success", Stream: true, FirstTokenMs: 100, DurationMs: 1000, PromptTokens: 10, CompletionTokens: 5},
			{TargetTokens: 50000, Outcome: "success", Stream: true, FirstTokenMs: 300, DurationMs: 2000, PromptTokens: 10, CompletionTokens: 5},
			{TargetTokens: 80000, Outcome: "error", Stream: true, FirstTokenMs: 0, DurationMs: 500, ErrorCategory: "upstream_5xx"},
		},
	}
	raw, err := BuildExcelReport(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < 100 {
		t.Fatalf("workbook too small: %d", len(raw))
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	for _, sheet := range []string{"测试条件", "总览", "首字延迟", "总耗时", "成功率"} {
		if idx, _ := f.GetSheetIndex(sheet); idx < 0 {
			t.Fatalf("missing sheet %s", sheet)
		}
	}

	ttftA2, err := f.GetRows("首字延迟")
	if err != nil {
		t.Fatal(err)
	}
	if len(ttftA2) < 3 {
		t.Fatalf("ttft rows=%d", len(ttftA2))
	}
	if ttftA2[1][0] != "50K" || ttftA2[1][1] != "2" || ttftA2[1][2] != "100" {
		// p50 of [100,300]: ceil(0.5*2)-1 → 100
		t.Fatalf("50K row = %#v", ttftA2[1])
	}
	if ttftA2[2][0] != "80K" || ttftA2[2][1] != "0" {
		t.Fatalf("80K success-latency row = %#v", ttftA2[2])
	}

	okRows, err := f.GetRows("成功率")
	if err != nil {
		t.Fatal(err)
	}
	if okRows[1][0] != "50K" || okRows[1][1] != "2" || okRows[1][2] != "2" {
		t.Fatalf("success 50K = %#v", okRows[1])
	}
	if okRows[2][0] != "80K" || okRows[2][1] != "1" || okRows[2][2] != "0" {
		t.Fatalf("success 80K = %#v", okRows[2])
	}

	cond, err := f.GetRows("测试条件")
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, row := range cond {
		for _, c := range row {
			joined += c + " "
		}
	}
	if bytes.Contains([]byte(joined), []byte("sk-")) || bytes.Contains([]byte(joined), []byte("api_key")) {
		t.Fatalf("conditions leaked key material: %s", joined)
	}
	if !bytes.Contains([]byte(joined), []byte("max_tokens")) {
		t.Fatalf("conditions missing max_tokens: %s", joined)
	}
}
