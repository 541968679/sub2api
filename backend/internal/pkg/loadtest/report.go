package loadtest

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Result struct {
	Seq              int      `json:"seq"`
	StartedAt        string   `json:"started_at"`
	Class            string   `json:"class"`
	Bucket           string   `json:"bucket"`
	Stream           bool     `json:"stream"`
	APIMode          string   `json:"api_mode,omitempty"`
	RequestedModel   string   `json:"requested_model,omitempty"`
	TargetTokens     int      `json:"target_input_tokens"`
	InputBand        string   `json:"input_band,omitempty"`
	BodyBytes        int      `json:"body_bytes"`
	StatusCode       int      `json:"status_code"`
	Outcome          string   `json:"outcome"`
	ErrorCategory    string   `json:"error_category,omitempty"`
	ErrorMessage     string   `json:"error_message,omitempty"`
	DurationMs       int      `json:"duration_ms"`
	HeaderMs         int      `json:"header_ms,omitempty"`
	FirstSSEMs       int      `json:"first_sse_ms,omitempty"`
	FirstContentMs   int      `json:"first_content_ms,omitempty"`
	GenerationMs     int      `json:"generation_ms,omitempty"`
	TPOT             float64  `json:"tpot_tok_s,omitempty"`
	Chunks           int      `json:"chunks"`
	ContentChars     int      `json:"content_chars"`
	FinishReason     string   `json:"finish_reason,omitempty"`
	PromptTokens     int      `json:"prompt_tokens,omitempty"`
	CompletionTokens int      `json:"completion_tokens,omitempty"`
	CachedTokens     int      `json:"cached_tokens,omitempty"`
	ResponseModel    string   `json:"response_model,omitempty"`
	ModelMissingN    int      `json:"model_missing_chunks,omitempty"`
	ModelEmptyN      int      `json:"model_empty_chunks,omitempty"`
	ModelPresentN    int      `json:"model_present_chunks,omitempty"`
	ModelMismatchN   int      `json:"model_mismatch_chunks,omitempty"`
	InspectedChunks  int      `json:"inspected_chunks,omitempty"`
	ContractIssues   []string `json:"contract_issues,omitempty"`
	Proto            string   `json:"proto,omitempty"`
	RequestID        string   `json:"request_id,omitempty"`
}

func (r Result) SawSuccess() bool {
	return r.ContentChars > 0 || r.FinishReason == "tool_calls" || r.CompletionTokens > 0
}

func (r *Result) computeTPOT() {
	out := r.CompletionTokens
	if out == 0 && r.ContentChars > 0 {
		out = r.ContentChars
	}
	if r.FirstContentMs <= 0 || r.DurationMs <= r.FirstContentMs+20 || out < 8 {
		return
	}
	gen := r.DurationMs - r.FirstContentMs
	r.GenerationMs = gen
	r.TPOT = float64(out) / (float64(gen) / 1000.0)
}

type SLAVerdict struct {
	Name      string  `json:"name"`
	Want      string  `json:"want"`
	Got       string  `json:"got"`
	Pass      bool    `json:"pass"`
	Skip      bool    `json:"skip"`
	Metric    string  `json:"metric,omitempty"`
	Band      string  `json:"band,omitempty"`
	WantValue float64 `json:"want_value,omitempty"`
	GotValue  float64 `json:"got_value,omitempty"`
	Samples   int     `json:"samples,omitempty"`
}

type TokenPercentiles struct {
	N   int `json:"n"`
	P50 int `json:"p50"`
	P90 int `json:"p90"`
	P99 int `json:"p99"`
	Avg int `json:"avg"`
}

type DataProfile struct {
	TargetInput TokenPercentiles `json:"target_input"`
	UsageInput  TokenPercentiles `json:"usage_input"`
	UsageOutput TokenPercentiles `json:"usage_output"`
}

func BuildDataProfile(rows []Result) DataProfile {
	var target, usageIn, usageOut []int
	for _, r := range rows {
		if r.TargetTokens > 0 {
			target = append(target, r.TargetTokens)
		}
		if r.PromptTokens > 0 {
			usageIn = append(usageIn, r.PromptTokens)
		}
		if r.CompletionTokens > 0 {
			usageOut = append(usageOut, r.CompletionTokens)
		}
	}
	return DataProfile{
		TargetInput: tokenPct(target),
		UsageInput:  tokenPct(usageIn),
		UsageOutput: tokenPct(usageOut),
	}
}

func tokenPct(values []int) TokenPercentiles {
	if len(values) == 0 {
		return TokenPercentiles{}
	}
	return TokenPercentiles{
		N:   len(values),
		P50: percentile(values, 0.5),
		P90: percentile(values, 0.9),
		P99: percentile(values, 0.99),
		Avg: mean(values),
	}
}

func classifyHTTP(status int, body string) (category, message string) {
	msg := strings.TrimSpace(body)
	if len(msg) > 400 {
		msg = msg[:400]
	}
	lower := strings.ToLower(msg)
	switch {
	case status == 401 || status == 403:
		return "auth_401_403", msg
	case status == 413 || strings.Contains(lower, "entity too large") || strings.Contains(lower, "request too large"):
		return "payload_413", msg
	case status == 429:
		return "rate_limit_429", msg
	case status == 524 || strings.Contains(lower, "error code 524"):
		return "cf_524", msg
	case status == 529:
		return "overloaded_529", msg
	case status == 400 && strings.Contains(lower, "temperature"):
		return "invalid_temperature", msg
	case status == 400 && (strings.Contains(lower, "too long") || strings.Contains(lower, "maximum")):
		return "prompt_too_long", msg
	case status == 503:
		return "unavailable_503", msg
	case status >= 500:
		return "upstream_5xx", msg
	case status >= 400:
		return "client_4xx", msg
	default:
		return "http_error", msg
	}
}

func percentile(values []int, p float64) int {
	if len(values) == 0 {
		return 0
	}
	cp := append([]int(nil), values...)
	sort.Ints(cp)
	if p <= 0 {
		return cp[0]
	}
	if p >= 1 {
		return cp[len(cp)-1]
	}
	idx := int(math.Ceil(p*float64(len(cp)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

func mean(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	return sum / len(values)
}

func meanF(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func percentileF(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	sort.Float64s(cp)
	if p <= 0 {
		return cp[0]
	}
	if p >= 1 {
		return cp[len(cp)-1]
	}
	idx := int(math.Ceil(p*float64(len(cp)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

func fmtTok(n int) string {
	if n >= 1000 {
		v := float64(n) / 1000.0
		if v >= 10 {
			return fmt.Sprintf("%.0fK", v)
		}
		return fmt.Sprintf("%.1fK", v)
	}
	return fmt.Sprintf("%d", n)
}

func evalSLA(rows []Result) []SLAVerdict {
	return append(evalInputBandSLA(rows), evalMixedSLA(rows)...)
}

func evalMixedSLA(rows []Result) []SLAVerdict {
	var ttft []int
	var tpot []float64
	for _, r := range rows {
		if r.Stream && r.FirstContentMs > 0 {
			ttft = append(ttft, r.FirstContentMs)
		}
		if r.TPOT > 0 {
			tpot = append(tpot, r.TPOT)
		}
	}
	return []SLAVerdict{
		withBand(gateMs("TTFT p50 (mixed)", slaTTFTp50Ms, ttft, 0.50), "mixed"),
		withBand(gateMs("TTFT p75 (mixed)", slaTTFTp75Ms, ttft, 0.75), "mixed"),
		withBand(gateMs("TTFT p90 (mixed)", slaTTFTp90Ms, ttft, 0.90), "mixed"),
		withBand(gateMs("TTFT p99 (mixed)", slaTTFTp99Ms, ttft, 0.99), "mixed"),
		withBand(gateTPOT("TPOT p50 (mixed)", slaTPOTp50, tpot, 0.50), "mixed"),
		withBand(gateTPOT("TPOT slow-tail p1 (mixed)", slaTPOTp99, tpot, 0.01), "mixed"),
	}
}

func evalInputBandSLA(rows []Result) []SLAVerdict {
	band50 := filterBand(rows, "50k")
	band160 := filterBand(rows, "160k")
	band380 := filterBand(rows, "380k")
	return []SLAVerdict{
		withBand(gateMs("TTFT p50 @ 50K in", slaTTFTp50Ms, ttftOf(band50), 0.50), "50k"),
		withBand(gateMs("TTFT p75 @ 50K in", slaTTFTp75Ms, ttftOf(band50), 0.75), "50k"),
		withBand(gateTPOT("TPOT p50 @ 50K in", slaTPOTp50, tpotOf(band50), 0.50), "50k"),
		withBand(gateMs("TTFT p50 @ 160K in", slaTTFTp90Ms, ttftOf(band160), 0.50), "160k"),
		withBand(gateMs("TTFT p90 @ 160K in", slaTTFTp90Ms, ttftOf(band160), 0.90), "160k"),
		withBand(gateMs("TTFT p50 @ 380K in", slaTTFTp99Ms, ttftOf(band380), 0.50), "380k"),
		withBand(gateTPOT("TPOT p50 @ 380K in", slaTPOTp99, tpotOf(band380), 0.50), "380k"),
	}
}

func filterBand(rows []Result, band string) []Result {
	var out []Result
	for _, r := range rows {
		b := r.InputBand
		if b == "" {
			b = inputBand(r.TargetTokens)
		}
		if b == band {
			out = append(out, r)
		}
	}
	return out
}

func ttftOf(rows []Result) []int {
	var out []int
	for _, r := range rows {
		if r.Stream && r.FirstContentMs > 0 {
			out = append(out, r.FirstContentMs)
		}
	}
	return out
}

func tpotOf(rows []Result) []float64 {
	var out []float64
	for _, r := range rows {
		if r.TPOT > 0 {
			out = append(out, r.TPOT)
		}
	}
	return out
}

func withBand(v SLAVerdict, band string) SLAVerdict {
	v.Band = band
	return v
}

func gateMs(name string, want int, samples []int, p float64) SLAVerdict {
	if len(samples) == 0 {
		return SLAVerdict{Name: name, Want: fmt.Sprintf("<%dms", want), Got: "n/a", Skip: true, Metric: "ttft", WantValue: float64(want)}
	}
	got := percentile(samples, p)
	return SLAVerdict{
		Name:      name,
		Want:      fmt.Sprintf("<%dms", want),
		Got:       fmt.Sprintf("%dms (n=%d)", got, len(samples)),
		Pass:      got < want,
		Metric:    "ttft",
		WantValue: float64(want),
		GotValue:  float64(got),
		Samples:   len(samples),
	}
}

func gateTPOT(name string, want float64, samples []float64, p float64) SLAVerdict {
	if len(samples) == 0 {
		return SLAVerdict{Name: name, Want: fmt.Sprintf(">%.0f tok/s", want), Got: "n/a", Skip: true, Metric: "tpot", WantValue: want}
	}
	got := percentileF(samples, p)
	return SLAVerdict{
		Name:      name,
		Want:      fmt.Sprintf(">%.0f tok/s", want),
		Got:       fmt.Sprintf("%.1f tok/s (n=%d)", got, len(samples)),
		Pass:      got > want,
		Metric:    "tpot",
		WantValue: want,
		GotValue:  got,
		Samples:   len(samples),
	}
}

func slaPassed(verdicts []SLAVerdict) bool {
	ok := false
	for _, v := range verdicts {
		if v.Band != "50k" {
			continue
		}
		if v.Skip {
			return false
		}
		if !v.Pass {
			return false
		}
		ok = true
	}
	return ok
}

func writeSummary(path string, cfg map[string]any, rows []Result, started, ended time.Time, peakInflight int) error {
	ok := 0
	fail := 0
	byCat := map[string]int{}
	byOutcome := map[string]int{}
	byClass := map[string]int{}
	byClassOK := map[string]int{}
	var dur, ttft, ttfb, firstSSE []int
	var okDur []int
	promptTok := 0
	compTok := 0
	cachedTok := 0
	http2 := 0
	for _, r := range rows {
		byOutcome[r.Outcome]++
		byClass[r.Class]++
		if r.Outcome == "success" {
			ok++
			byClassOK[r.Class]++
			okDur = append(okDur, r.DurationMs)
		} else {
			fail++
			key := r.ErrorCategory
			if key == "" {
				key = r.Outcome
			}
			byCat[key]++
		}
		dur = append(dur, r.DurationMs)
		if r.FirstContentMs > 0 {
			ttft = append(ttft, r.FirstContentMs)
		}
		if r.HeaderMs > 0 {
			ttfb = append(ttfb, r.HeaderMs)
		}
		if r.FirstSSEMs > 0 {
			firstSSE = append(firstSSE, r.FirstSSEMs)
		}
		promptTok += r.PromptTokens
		compTok += r.CompletionTokens
		cachedTok += r.CachedTokens
		if strings.Contains(r.Proto, "HTTP/2") {
			http2++
		}
	}
	total := len(rows)
	succRate := 0.0
	if total > 0 {
		succRate = float64(ok) * 100 / float64(total)
	}

	var constructedIn, usageIn, usageOut []int
	var tpot []float64
	byModel := map[string]int{}
	byModelOK := map[string]int{}
	issueN := map[string]int{}
	modelMissingReqs := 0
	for _, r := range rows {
		if r.TargetTokens > 0 {
			constructedIn = append(constructedIn, r.TargetTokens)
		}
		if r.PromptTokens > 0 {
			usageIn = append(usageIn, r.PromptTokens)
		}
		if r.CompletionTokens > 0 {
			usageOut = append(usageOut, r.CompletionTokens)
		}
		if r.TPOT > 0 {
			tpot = append(tpot, r.TPOT)
		}
		if r.RequestedModel != "" {
			byModel[r.RequestedModel]++
			if r.Outcome == "success" {
				byModelOK[r.RequestedModel]++
			}
		}
		for _, issue := range r.ContractIssues {
			issueN[issue]++
		}
		if r.ModelMissingN > 0 || r.ModelEmptyN > 0 {
			modelMissingReqs++
		}
	}

	var b strings.Builder
	b.WriteString("# loadtest summary\n\n")
	b.WriteString(fmt.Sprintf("- window: %s → %s (%s)\n", started.Format(time.RFC3339), ended.Format(time.RFC3339), ended.Sub(started).Truncate(time.Millisecond)))
	b.WriteString(fmt.Sprintf("- peak in-flight: **%d**\n", peakInflight))
	b.WriteString(fmt.Sprintf("- HTTP/2 responses: %d / %d\n", http2, total))
	b.WriteString(fmt.Sprintf("- profile: %v  model: %v  concurrency: %v  total: %d\n\n", cfg["profile"], cfg["model"], cfg["concurrency"], total))
	b.WriteString("## Overall\n\n")
	b.WriteString(fmt.Sprintf("- success: **%d** (%.2f%%)\n", ok, succRate))
	b.WriteString(fmt.Sprintf("- fail: **%d**\n\n", fail))

	b.WriteString("## 一、请求数据分布（Data Profile）\n\n")
	b.WriteString("性能指标在以下压测请求分布下统计。客户目标：输入 p50/p90/p99/avg = 50K / 160K / 380K / 80K；输出 0.2K / 1.3K / 7K / 0.6K。\n\n")
	b.WriteString("| | p50 | p90 | p99 | avg |\n|---|---:|---:|---:|---:|\n")
	b.WriteString("| 客户目标 输入 tokens | 50K | 160K | 380K | 80K |\n")
	writeTokRow(&b, "构造 输入 tokens", constructedIn)
	writeTokRow(&b, "上游 usage 输入 tokens", usageIn)
	b.WriteString("| 客户目标 输出 tokens | 0.2K | 1.3K | 7K | 0.6K |\n")
	writeTokRow(&b, "上游 usage 输出 tokens", usageOut)
	b.WriteString("\n")

	b.WriteString("## 二、性能指标（Performance）\n\n")
	b.WriteString("TTFT = 流式首个 **content** token（role-only 不计）。TPOT = 生成阶段 output_tokens / (duration − TTFT)，单位 tok/s。\n\n")
	b.WriteString("| 指标 | 要求 | 本次 | 结果 |\n|---|---|---|---|\n")
	verdicts := evalSLA(rows)
	for _, v := range verdicts {
		mark := "FAIL"
		if v.Skip {
			mark = "n/a"
		} else if v.Pass {
			mark = "PASS"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | **%s** |\n", v.Name, v.Want, v.Got, mark))
	}
	b.WriteString("\n")
	writePct(&b, "duration_ms", dur)
	writePct(&b, "duration_success_ms", okDur)
	writePct(&b, "header_ms (TTFB)", ttfb)
	writePct(&b, "first_sse_ms", firstSSE)
	writePct(&b, "first_content_ms (TTFT)", ttft)
	if len(tpot) > 0 {
		b.WriteString(fmt.Sprintf("- TPOT tok/s: n=%d min=%.1f mean=%.1f p1=%.1f p50=%.1f p90=%.1f p99=%.1f max=%.1f\n",
			len(tpot), percentileF(tpot, 0), meanF(tpot), percentileF(tpot, 0.01),
			percentileF(tpot, 0.5), percentileF(tpot, 0.9), percentileF(tpot, 0.99), percentileF(tpot, 1)))
	} else {
		b.WriteString("- TPOT tok/s: (no samples — need stream + usage/output and first content)\n")
	}
	b.WriteString("\n")

	b.WriteString("## Contract / 协议字段\n\n")
	b.WriteString("用于判断当前在修的空 `model` 等问题。Go 客户端把 missing 和 `\"\"` 都解成空字符串（`expected \"kimi-k3\", got \"\"`）。\n\n")
	if len(issueN) == 0 {
		b.WriteString("- 无 contract issue。\n")
	} else {
		b.WriteString(fmt.Sprintf("- 含缺失/空 model 的请求: **%d** / %d\n\n", modelMissingReqs, total))
		b.WriteString("| issue | 请求数 | 说明 |\n|---|---:|---|\n")
		type kv struct {
			k string
			n int
		}
		var items []kv
		for k, n := range issueN {
			items = append(items, kv{k, n})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].n > items[j].n })
		for _, it := range items {
			b.WriteString(fmt.Sprintf("| `%s` | %d | %s |\n", it.k, it.n, contractHelp(it.k)))
		}
	}
	b.WriteString("\n")

	if len(byModel) > 0 {
		b.WriteString("## Model mix\n\n")
		b.WriteString("| model | n | ok | success |\n|---|---:|---:|---:|\n")
		var names []string
		for m := range byModel {
			names = append(names, m)
		}
		sort.Strings(names)
		for _, m := range names {
			n := byModel[m]
			b.WriteString(fmt.Sprintf("| `%s` | %d | %d | %.1f%% |\n", m, n, byModelOK[m], float64(byModelOK[m])*100/float64(n)))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Usage reported by upstream\n\n")
	b.WriteString(fmt.Sprintf("- prompt_tokens: %d\n", promptTok))
	b.WriteString(fmt.Sprintf("- cached_tokens: %d\n", cachedTok))
	b.WriteString(fmt.Sprintf("- completion_tokens: %d\n\n", compTok))
	b.WriteString("## Class mix\n\n")
	b.WriteString("| class | n | ok | success |\n|---|---:|---:|---:|\n")
	for _, name := range []string{"sync-short", "stream-ctx", "stream-sla", "smoke"} {
		n := byClass[name]
		if n == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("| %s | %d | %d | %.1f%% |\n", name, n, byClassOK[name], float64(byClassOK[name])*100/float64(n)))
	}
	b.WriteString("\n## Failures\n\n")
	if fail == 0 {
		b.WriteString("(none)\n\n")
	} else {
		b.WriteString("| category | n | of fail | of all |\n|---|---:|---:|---:|\n")
		type kv struct {
			k string
			n int
		}
		var items []kv
		for k, n := range byCat {
			items = append(items, kv{k, n})
		}
		sort.Slice(items, func(i, j int) bool { return items[i].n > items[j].n })
		for _, it := range items {
			b.WriteString(fmt.Sprintf("| `%s` | %d | %.1f%% | %.1f%% |\n", it.k, it.n, float64(it.n)*100/float64(fail), float64(it.n)*100/float64(total)))
		}
		b.WriteString("\n### First failures\n\n")
		b.WriteString("| seq | status | category | ms | class | request_id | message |\n|---:|---:|---|---:|---|---|---|\n")
		shown := 0
		for _, r := range rows {
			if r.Outcome == "success" {
				continue
			}
			msg := strings.ReplaceAll(strings.ReplaceAll(r.ErrorMessage, "|", "/"), "\n", " ")
			if len(msg) > 140 {
				msg = msg[:140]
			}
			b.WriteString(fmt.Sprintf("| %d | %d | `%s` | %d | %s | `%s` | %s |\n", r.Seq, r.StatusCode, r.ErrorCategory, r.DurationMs, r.Class, dash(r.RequestID), msg))
			shown++
			if shown >= 25 {
				break
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("## Stability gates\n\n")
	b.WriteString("- success ≥ 98%（无 empty/truncated stream）\n")
	b.WriteString("- 客户 SLA：TTFT p50<4s / p75<8s / p90<12s / p99<30s；TPOT p50>70 tok/s，慢尾 >40 tok/s\n")
	b.WriteString("- 经 Sub2API 时：`model_missing` / `model_empty` 应为 0（空 model 填充）\n")
	b.WriteString("- 默认 size-cap 下 413=0；50 in-flight 不被 429 打崩；HTTPS 协商 HTTP/2\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func writeTokRow(b *strings.Builder, name string, values []int) {
	if len(values) == 0 {
		b.WriteString(fmt.Sprintf("| %s | - | - | - | - |\n", name))
		return
	}
	b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
		name, fmtTok(percentile(values, 0.5)), fmtTok(percentile(values, 0.9)),
		fmtTok(percentile(values, 0.99)), fmtTok(mean(values))))
}

func contractHelp(code string) string {
	switch code {
	case issueModelMissing:
		return "JSON 块缺 model 键（Go 解成 \"\"）"
	case issueModelEmpty:
		return `model 为 "" 或 null`
	case issueModelMismatch:
		return "非空 model 与请求模型不一致"
	case issueResponseModelMissing:
		return "Responses 形态缺 response.model"
	case issueUsageMissing:
		return "stream_options.include_usage 但末块无 usage"
	case issueSilentRefusal:
		return "finish_reason=stop 且无 content"
	case issueRoleOnlyFirst:
		return "首帧只有 role，无 content（不一定是故障）"
	default:
		return ""
	}
}

func writePct(b *strings.Builder, name string, values []int) {
	if len(values) == 0 {
		b.WriteString(fmt.Sprintf("- %s: (no samples)\n", name))
		return
	}
	b.WriteString(fmt.Sprintf("- %s: n=%d min=%d mean=%d p50=%d p90=%d p95=%d p99=%d max=%d\n",
		name, len(values), percentile(values, 0), mean(values),
		percentile(values, 0.5), percentile(values, 0.9), percentile(values, 0.95),
		percentile(values, 0.99), percentile(values, 1)))
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func defaultOutDir() string {
	ts := time.Now().Format("20060102-150405")
	if root := findRepoRoot(); root != "" {
		return filepath.Join(root, "tmp", "kimi-loadtest-runs", ts)
	}
	return filepath.Join("kimi-loadtest-out", ts)
}

func findRepoRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir := cwd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
