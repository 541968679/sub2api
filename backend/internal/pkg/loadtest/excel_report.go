package loadtest

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// ExcelExportInput is the generic load-test report payload (no API keys).
type ExcelExportInput struct {
	RunID       string
	Status      string
	StartedAt   time.Time
	EndedAt     *time.Time
	Source      string
	AccountID   *int64
	AccountName string
	BaseURL     string
	Path        string
	ProxyID     *int64
	ProxyName   string
	Models      []string
	APIMode     string
	StreamMode  string
	Profile     string
	Tiers       []Tier
	Concurrency int
	Total       int
	MaxTokens   int
	SizeCap     int
	InputTokens int
	Tools       string
	Peak        int64
	Done        int64
	OK          int64
	SuccessRate float64
	RPMPeak     int
	RPMAvg      float64
	Results     []Result
}

// BuildExcelReport builds a generic conditions + metric×input-size workbook.
func BuildExcelReport(in ExcelExportInput) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	if err := writeConditionsSheet(f, in); err != nil {
		return nil, err
	}
	if err := writeOverviewSheet(f, in); err != nil {
		return nil, err
	}

	rowKeys := reportRowKeys(in.Tiers, in.Results)
	if err := writePercentileMetricSheet(f, "首字延迟(ms)", rowKeys, in.Results, func(r Result) (int, bool) {
		if r.Outcome != "success" {
			return 0, false
		}
		ms := measuredTTFT(r)
		return ms, ms > 0
	}); err != nil {
		return nil, err
	}
	if err := writePercentileMetricSheet(f, "总耗时(ms)", rowKeys, in.Results, func(r Result) (int, bool) {
		if r.Outcome != "success" {
			return 0, false
		}
		return r.DurationMs, r.DurationMs > 0
	}); err != nil {
		return nil, err
	}
	if err := writeSuccessRateSheet(f, rowKeys, in.Results); err != nil {
		return nil, err
	}

	_ = f.DeleteSheet("Sheet1")
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeConditionsSheet(f *excelize.File, in ExcelExportInput) error {
	const name = "测试条件"
	idx, err := f.NewSheet(name)
	if err != nil {
		return err
	}
	f.SetActiveSheet(idx)
	ended := ""
	durationMs := ""
	if in.EndedAt != nil {
		ended = in.EndedAt.Format(time.RFC3339)
		durationMs = fmt.Sprintf("%d", in.EndedAt.Sub(in.StartedAt).Milliseconds())
	}
	account := in.AccountName
	if in.AccountID != nil {
		if account == "" {
			account = fmt.Sprintf("%d", *in.AccountID)
		} else {
			account = fmt.Sprintf("%s (%d)", account, *in.AccountID)
		}
	}
	proxy := in.ProxyName
	if in.ProxyID != nil && *in.ProxyID > 0 {
		if proxy == "" {
			proxy = fmt.Sprintf("%d", *in.ProxyID)
		} else {
			proxy = fmt.Sprintf("%s (%d)", proxy, *in.ProxyID)
		}
	} else {
		proxy = "direct"
	}
	type kv struct {
		item  string
		value string
		unit  string
	}
	rows := []kv{
		{"run_id", in.RunID, "-"},
		{"status", in.Status, "-"},
		{"started_at", in.StartedAt.Format(time.RFC3339), "RFC3339"},
		{"ended_at", ended, "RFC3339"},
		{"duration", durationMs, "ms"},
		{"source", in.Source, "-"},
		{"account", account, "-"},
		{"base_url", in.BaseURL, "-"},
		{"path", in.Path, "-"},
		{"proxy", proxy, "-"},
		{"models", strings.Join(in.Models, ", "), "-"},
		{"api_mode", in.APIMode, "-"},
		{"stream_mode", in.StreamMode, "-"},
		{"profile", in.Profile, "-"},
		{"tiers", formatTiersLine(in.Tiers), "条×输入tokens"},
		{"concurrency", fmt.Sprintf("%d", in.Concurrency), "并发数"},
		{"total", fmt.Sprintf("%d", in.Total), "请求数"},
		{"max_tokens", fmt.Sprintf("%d", in.MaxTokens), "输出tokens上限"},
		{"size_cap", fmt.Sprintf("%d", in.SizeCap), "输入tokens上限"},
		{"input_tokens", fmt.Sprintf("%d", in.InputTokens), "输入tokens"},
		{"tools", in.Tools, "-"},
		{"notes", "TTFT=first_token_ms（含 reasoning）；RPM 只计成功；延迟百分位只用成功样本；构造输入列的 K=千 tokens", "-"},
	}
	_ = f.SetSheetRow(name, "A1", &[]any{"项", "值", "单位"})
	for i, row := range rows {
		_ = f.SetSheetRow(name, fmt.Sprintf("A%d", i+2), &[]any{row.item, row.value, row.unit})
	}
	return nil
}

func writeOverviewSheet(f *excelize.File, in ExcelExportInput) error {
	const name = "总览"
	if _, err := f.NewSheet(name); err != nil {
		return err
	}
	fail := in.Done - in.OK
	if fail < 0 {
		fail = 0
	}
	var prompt, cached, completion int
	for _, r := range in.Results {
		prompt += r.PromptTokens
		cached += r.CachedTokens
		completion += r.CompletionTokens
	}
	type kv struct {
		item  string
		value any
		unit  string
	}
	rows := []kv{
		{"done", in.Done, "请求数"},
		{"ok", in.OK, "请求数"},
		{"fail", fail, "请求数"},
		{"success_rate", round1(in.SuccessRate), "%"},
		{"peak_inflight", in.Peak, "并发数"},
		{"rpm_peak", in.RPMPeak, "次/分钟"},
		{"rpm_avg", round1(in.RPMAvg), "次/分钟"},
		{"usage_prompt_tokens", prompt, "tokens"},
		{"usage_cached_tokens", cached, "tokens"},
		{"usage_completion_tokens", completion, "tokens"},
	}
	_ = f.SetSheetRow(name, "A1", &[]any{"指标", "值", "单位"})
	for i, row := range rows {
		_ = f.SetSheetRow(name, fmt.Sprintf("A%d", i+2), &[]any{row.item, row.value, row.unit})
	}
	return nil
}

func writePercentileMetricSheet(f *excelize.File, name string, keys []int, rows []Result, pick func(Result) (int, bool)) error {
	if _, err := f.NewSheet(name); err != nil {
		return err
	}
	_ = f.SetSheetRow(name, "A1", &[]any{"构造输入大小(tokens)", "样本数(n)", "p50(ms)", "p75(ms)", "p90(ms)"})
	by := map[int][]int{}
	for _, r := range rows {
		v, ok := pick(r)
		if !ok {
			continue
		}
		by[r.TargetTokens] = append(by[r.TargetTokens], v)
	}
	for i, key := range keys {
		vals := by[key]
		n := len(vals)
		line := []any{FormatInputK(key), n}
		if n == 0 {
			line = append(line, "-", "-", "-")
		} else {
			line = append(line, percentile(vals, 0.50), percentile(vals, 0.75), percentile(vals, 0.90))
		}
		_ = f.SetSheetRow(name, fmt.Sprintf("A%d", i+2), &line)
	}
	return nil
}

func writeSuccessRateSheet(f *excelize.File, keys []int, rows []Result) error {
	const name = "成功率(%)"
	if _, err := f.NewSheet(name); err != nil {
		return err
	}
	_ = f.SetSheetRow(name, "A1", &[]any{"构造输入大小(tokens)", "样本数(n)", "成功数(次)", "成功率(%)"})
	type agg struct{ n, ok int }
	by := map[int]*agg{}
	for _, r := range rows {
		cur := by[r.TargetTokens]
		if cur == nil {
			cur = &agg{}
			by[r.TargetTokens] = cur
		}
		cur.n++
		if r.Outcome == "success" {
			cur.ok++
		}
	}
	for i, key := range keys {
		cur := by[key]
		n, ok := 0, 0
		if cur != nil {
			n, ok = cur.n, cur.ok
		}
		rate := 0.0
		if n > 0 {
			rate = float64(ok) * 100 / float64(n)
		}
		_ = f.SetSheetRow(name, fmt.Sprintf("A%d", i+2), &[]any{FormatInputK(key), n, ok, round1(rate)})
	}
	return nil
}

func reportRowKeys(tiers []Tier, rows []Result) []int {
	if len(tiers) > 0 {
		out := make([]int, 0, len(tiers))
		seen := map[int]struct{}{}
		for _, t := range tiers {
			if t.InputTokens <= 0 {
				continue
			}
			if _, ok := seen[t.InputTokens]; ok {
				continue
			}
			seen[t.InputTokens] = struct{}{}
			out = append(out, t.InputTokens)
		}
		if len(out) > 0 {
			return out
		}
	}
	seen := map[int]struct{}{}
	for _, r := range rows {
		if r.TargetTokens > 0 {
			seen[r.TargetTokens] = struct{}{}
		}
	}
	out := make([]int, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// FormatInputK renders constructed input size for report headers (e.g. 50K = 50000 tokens).
func FormatInputK(tokens int) string {
	if tokens <= 0 {
		return "-"
	}
	if tokens >= 1000 {
		v := float64(tokens) / 1000.0
		if v >= 10 || float64(int(v)) == v {
			return fmt.Sprintf("%.0fK", v)
		}
		return fmt.Sprintf("%.1fK", v)
	}
	return fmt.Sprintf("%d", tokens)
}

func formatTiersLine(tiers []Tier) string {
	if len(tiers) == 0 {
		return "(empty — profile sampling)"
	}
	parts := make([]string, 0, len(tiers))
	for _, t := range tiers {
		parts = append(parts, fmt.Sprintf("%d×%s tokens", t.Count, FormatInputK(t.InputTokens)))
	}
	return strings.Join(parts, ", ")
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
