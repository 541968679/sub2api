package loadtest

import (
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractErrorTextJSON(t *testing.T) {
	got := extractErrorText(`{"error":{"message":"prompt is too long: 100001 tokens > 100000 maximum","type":"invalid_request_error"}}`)
	if !strings.Contains(got, "prompt is too long") {
		t.Fatalf("%q", got)
	}
	html := `<html><head><title>example.com | 502: Bad gateway</title></head><body>cf</body></html>`
	if extractErrorText(html) != "example.com | 502: Bad gateway" {
		t.Fatalf("%q", extractErrorText(html))
	}
}

func TestConsumeSSECapturesUpstreamError(t *testing.T) {
	body := "data: {\"error\":{\"message\":\"overloaded\"}}\n\n"
	st := consumeSSE(strings.NewReader(body), time.Now().Add(-8*time.Millisecond), false, nil, "kimi-k3")
	if st.UpstreamError != "overloaded" {
		t.Fatalf("%q", st.UpstreamError)
	}
}

func TestEstimateTokensCJK(t *testing.T) {
	n := estimateTokens("你好世界")
	if n < 4 || n > 6 {
		t.Fatalf("cjk tokens=%d", n)
	}
}

func TestJoinURLStripsDuplicateV1(t *testing.T) {
	got := JoinURL("https://api.example.com/v1", "/v1/chat/completions")
	want := "https://api.example.com/v1/chat/completions"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestPickClassMixHasBoth(t *testing.T) {
	rng := rand.New(rand.NewSource(363))
	syncN, streamN := 0, 0
	for i := 0; i < 400; i++ {
		cls := pickClass(rng, "user363", defaultSyncRatio, 80000, defaultCacheShare, i+1)
		if cls.Stream {
			streamN++
			if cls.TargetTokens > 80000 {
				t.Fatalf("size-cap ignored: %d", cls.TargetTokens)
			}
		} else {
			syncN++
		}
	}
	if syncN < 180 || streamN < 120 {
		t.Fatalf("mix not close to 57/43: sync=%d stream=%d", syncN, streamN)
	}
}

func TestBuildPayloadStreamIncludesUsageAndTools(t *testing.T) {
	cls := payloadClass{Name: "stream-ctx", Stream: true, Bucket: "20-50k", TargetTokens: 400, CacheTokens: 250, UniqueTokens: 150}
	p, err := buildPayload(cls, "kimi-k3", 1, 256, nil, "auto", "PREFIX", APIModeChatCompletions)
	if err != nil {
		t.Fatal(err)
	}
	body := string(p.Body)
	for _, needle := range []string{
		`"model":"kimi-k3"`,
		`"stream":true`,
		`"include_usage":true`,
		`"tool_choice":"none"`,
		`"read_file"`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("missing %s in %s", needle, body[:min(len(body), 400)])
		}
	}
	if strings.Contains(body, `"temperature"`) {
		t.Fatal("temperature should be omitted by default")
	}
}

func TestKimiK3PayloadStaysNativeChatCompletions(t *testing.T) {
	cls := payloadClass{Name: "stream-ctx", Stream: true, Bucket: "20-50k", TargetTokens: 400, CacheTokens: 250, UniqueTokens: 150}
	p, err := buildPayload(cls, "kimi-k3", 1, 256, nil, "auto", "PREFIX", APIModeResponses)
	if err != nil {
		t.Fatal(err)
	}
	body := string(p.Body)
	for _, needle := range []string{
		`"model":"kimi-k3"`,
		`"messages"`,
		`"max_completion_tokens":256`,
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("missing %s in %s", needle, body[:min(len(body), 500)])
		}
	}
	for _, banned := range []string{`"input"`, `"instructions"`, `"max_tokens"`, `"role":"assistant"`} {
		if strings.Contains(body, banned) {
			t.Fatalf("kimi-k3 payload must stay native chat completions, found %s in %s", banned, body[:min(len(body), 500)])
		}
	}
}

func TestResolveLoadtestRequestKeepsKimiOnChatCompletions(t *testing.T) {
	mode, path := resolveLoadtestRequest(APIModeResponses, "/v1/responses", "kimi-k3")
	if mode != APIModeChatCompletions || path != "/v1/chat/completions" {
		t.Fatalf("kimi-k3 mode=%s path=%s", mode, path)
	}
	mode, path = resolveLoadtestRequest(APIModeResponses, "/v1/responses", "glm-5.3")
	if mode != APIModeResponses || path != "/v1/responses" {
		t.Fatalf("glm mode=%s path=%s", mode, path)
	}
}

func TestRunKimiK3IgnoresResponsesMode(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"kimi-k3","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	results, err := Run(t.Context(), Config{
		BaseURL:     srv.URL,
		Path:        "/v1/responses",
		APIMode:     APIModeResponses,
		Models:      []string{"kimi-k3"},
		Profile:     "smoke",
		Concurrency: 1,
		Total:       1,
		APIKey:      "test-key",
		Timeout:     5 * time.Second,
		Tools:       "off",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("path %s body %s", gotPath, gotBody)
	}
	if !strings.Contains(gotBody, `"messages"`) || strings.Contains(gotBody, `"input"`) {
		t.Fatalf("body %s", gotBody)
	}
	if len(results) != 1 || results[0].APIMode != APIModeChatCompletions {
		t.Fatalf("%+v", results)
	}
}

func TestBuildPayloadSyncOmitsStreamOptions(t *testing.T) {
	cls := payloadClass{Name: "sync-short", Stream: false, Bucket: "0-500", TargetTokens: 80, UniqueTokens: 80}
	p, err := buildPayload(cls, "kimi-k3", 2, 128, nil, "auto", "", APIModeChatCompletions)
	if err != nil {
		t.Fatal(err)
	}
	body := string(p.Body)
	if !strings.Contains(body, `"stream":false`) {
		t.Fatal(body)
	}
	if strings.Contains(body, "stream_options") || strings.Contains(body, "read_file") {
		t.Fatalf("sync should not send tools/stream_options: %s", body)
	}
}

func TestConsumeSSEReasoningCountsAsFirstToken(t *testing.T) {
	var st streamStats
	t0 := time.Now().Add(-40 * time.Millisecond)
	parseSSELine([]byte(`data: {"choices":[{"delta":{"reasoning_content":""}}]}`), t0, &st, "kimi-k3")
	if st.FirstToken != 0 || st.FirstContent != 0 {
		t.Fatalf("empty reasoning must not start TTFT: %+v", st)
	}
	parseSSELine([]byte(`data: {"choices":[{"delta":{"reasoning_content":"先想"}}]}`), t0, &st, "kimi-k3")
	token := st.FirstToken
	if token <= 0 || st.FirstContent != 0 || st.SawContent {
		t.Fatalf("reasoning should be first token only: %+v", st)
	}
	parseSSELine([]byte(`data: {"choices":[{"delta":{"content":"答"}}]}`), time.Now().Add(-5*time.Millisecond), &st, "kimi-k3")
	if st.FirstToken != token {
		t.Fatalf("answer content moved first token: before %s after %s", token, st.FirstToken)
	}
	if st.FirstContent <= 0 || st.FirstContent >= token || !st.SawContent || st.ContentChars != 1 {
		t.Fatalf("answer content: %+v", st)
	}
}

func TestConsumeSSEFirstContent(t *testing.T) {
	body := strings.Join([]string{
		`data: {"model":"kimi-k3","choices":[{"delta":{"role":"assistant"}}]}`,
		``,
		`data: {"choices":[{"delta":{"content":"你好"}}]}`,
		``,
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":4}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	t0 := time.Now().Add(-8 * time.Millisecond)
	st := consumeSSE(strings.NewReader(body), t0, false, nil, "kimi-k3")
	if !st.SawContent || st.ContentChars != 2 || st.FinishReason != "stop" || !st.SawDone {
		t.Fatalf("%+v", st)
	}
	if st.Usage.PromptTokens != 10 || st.Usage.CompletionTokens != 2 || st.Usage.CachedTokens != 4 {
		t.Fatalf("usage %+v", st.Usage)
	}
	if st.FirstSSE <= 0 || st.FirstContent <= 0 || st.FirstToken <= 0 {
		t.Fatalf("timings %+v", st)
	}
	if st.ResponseModel != "kimi-k3" {
		t.Fatalf("model %q", st.ResponseModel)
	}
	if st.ModelMissing < 1 {
		t.Fatalf("later chunks omit model, missing=%d present=%d", st.ModelMissing, st.ModelPresent)
	}
	issues := contractIssues(st, true, true)
	if !containsIssue(issues, issueModelMissing) {
		t.Fatalf("issues %v", issues)
	}
}

func TestClassifyHTTPExtractsJSONMessage(t *testing.T) {
	cat, msg := classifyHTTP(400, `{"error":{"message":"prompt is too long: 100001 tokens > 100000 maximum"}}`)
	if cat != "prompt_too_long" || !strings.Contains(msg, "prompt is too long") {
		t.Fatalf("%s %s", cat, msg)
	}
}

func TestClassifyHTTP(t *testing.T) {
	cat, _ := classifyHTTP(413, "Request Entity Too Large")
	if cat != "payload_413" {
		t.Fatalf("%s", cat)
	}
	cat, _ = classifyHTTP(400, `{"code":11115,"msg":"prompt is too long: 100001 tokens > 100000 maximum"}`)
	if cat != "prompt_too_long" {
		t.Fatalf("%s", cat)
	}
	cat, _ = classifyHTTP(400, "invalid temperature: value is not allowed for this model")
	if cat != "invalid_temperature" {
		t.Fatalf("%s", cat)
	}
	cat, _ = classifyHTTP(524, "error code 524")
	if cat != "cf_524" {
		t.Fatalf("%s", cat)
	}
}

func TestRunProgressIncludesEachResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"model\":\"kimi-k3\",\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"model\":\"kimi-k3\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":1}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	var live []int
	_, err := Run(t.Context(), Config{
		BaseURL:     srv.URL,
		APIKey:      "test-key",
		Profile:     "smoke",
		Concurrency: 1,
		Total:       3,
		MaxTokens:   8,
		Tools:       "off",
	}, func(p Progress) {
		if p.Result != nil {
			live = append(live, p.Result.Seq)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 3 {
		t.Fatalf("live results=%v", live)
	}
}

func TestRunOneStreamSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth")
		}
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "Go-http-client/") {
			t.Errorf("ua %q", ua)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Request-Id", "rid-1")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":9,\"completion_tokens\":1}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	cls := payloadClass{Name: "smoke", Stream: true, Bucket: "smoke", TargetTokens: 40, UniqueTokens: 40}
	res := runOne(t.Context(), srv.Client(), srv.URL+"/v1/chat/completions", map[string]string{
		"Authorization": "Bearer test-key",
		"Content-Type":  "application/json",
	}, cls, "kimi-k3", 1, 32, nil, "off", "", false, APIModeChatCompletions)
	if res.Outcome != "success" {
		t.Fatalf("%+v", res)
	}
	if res.RequestID != "rid-1" || res.ContentChars == 0 {
		t.Fatalf("%+v", res)
	}
	if res.RequestedModel != "kimi-k3" {
		t.Fatalf("requested %q", res.RequestedModel)
	}
	if !containsIssue(res.ContractIssues, issueModelMissing) {
		t.Fatalf("mock omitted model, issues=%v", res.ContractIssues)
	}
}

func TestInspectModelMissingEmptyMismatch(t *testing.T) {
	var st streamStats
	inspectJSONObject([]byte(`{"choices":[{"delta":{"content":"x"}}]}`), "glm-5.3", &st, true)
	if st.ModelMissing != 1 {
		t.Fatalf("missing=%d", st.ModelMissing)
	}
	inspectJSONObject([]byte(`{"model":"","choices":[{"delta":{"content":"y"}}]}`), "glm-5.3", &st, false)
	if st.ModelEmpty != 1 {
		t.Fatalf("empty=%d", st.ModelEmpty)
	}
	inspectJSONObject([]byte(`{"model":"other","choices":[{"delta":{"content":"z"}}]}`), "glm-5.3", &st, false)
	if st.ModelMismatch != 1 || st.MismatchValue != "other" {
		t.Fatalf("mismatch=%d val=%s", st.ModelMismatch, st.MismatchValue)
	}
	inspectJSONObject([]byte(`{"model":"glm-5.3","choices":[{"delta":{"content":"ok"}}]}`), "glm-5.3", &st, false)
	if st.ModelPresent != 2 {
		t.Fatalf("present=%d", st.ModelPresent)
	}
}

func TestParseModelList(t *testing.T) {
	got := parseModelList("kimi-k3", "glm-5.3, kimi-k3,glm-5.3")
	if len(got) != 2 || got[0] != "glm-5.3" || got[1] != "kimi-k3" {
		t.Fatalf("%v", got)
	}
}

func TestSLAClassIsStream(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	cls := pickClass(rng, "user363-sla", 0, 0, defaultCacheShare, 1)
	if !cls.Stream || cls.TargetTokens != 50000 || cls.MaxTokens != 200 || inputBand(cls.TargetTokens) != "50k" {
		t.Fatalf("%+v", cls)
	}
	p90 := pickClass(rng, "user363-sla", 0, 0, defaultCacheShare, 89)
	if p90.TargetTokens != 160000 || p90.MaxTokens != 1300 {
		t.Fatalf("p90 slot %+v", p90)
	}
	p99 := pickClass(rng, "user363-sla", 0, 0, defaultCacheShare, 99)
	if p99.TargetTokens != 380000 || p99.MaxTokens != 7000 {
		t.Fatalf("p99 slot %+v", p99)
	}
}

func TestSLASlotMatchesSheetPercentiles(t *testing.T) {
	var ins []int
	for seq := 1; seq <= 100; seq++ {
		in, _, _ := slaSlot(seq)
		ins = append(ins, in)
	}
	if percentile(ins, 0.5) != 50000 || percentile(ins, 0.9) != 160000 || percentile(ins, 0.99) != 380000 {
		t.Fatalf("p50=%d p90=%d p99=%d", percentile(ins, 0.5), percentile(ins, 0.9), percentile(ins, 0.99))
	}
}

func TestInputBandSLAUses50kCohort(t *testing.T) {
	rows := []Result{
		{Stream: true, TargetTokens: 50000, InputBand: "50k", FirstContentMs: 3000, TPOT: 80},
		{Stream: true, TargetTokens: 50000, InputBand: "50k", FirstContentMs: 3500, TPOT: 90},
		{Stream: true, TargetTokens: 380000, InputBand: "380k", FirstContentMs: 20000, TPOT: 50},
	}
	gates := evalInputBandSLA(rows)
	var p50 SLAVerdict
	for _, g := range gates {
		if g.Name == "TTFT p50 @ 50K in" {
			p50 = g
		}
	}
	if p50.Skip || !p50.Pass || p50.GotValue > 3500 {
		t.Fatalf("%+v", p50)
	}
}

func TestComputeTPOT(t *testing.T) {
	r := Result{FirstContentMs: 4000, DurationMs: 6000, CompletionTokens: 140}
	r.computeTPOT()
	if r.GenerationMs != 2000 {
		t.Fatalf("gen=%d", r.GenerationMs)
	}
	if r.TPOT < 69 || r.TPOT > 71 {
		t.Fatalf("tpot=%f", r.TPOT)
	}
}

func containsIssue(issues []string, code string) bool {
	for _, i := range issues {
		if i == code {
			return true
		}
	}
	return false
}

func TestTrimToTokens(t *testing.T) {
	s := makeFiller(2000, "STABLE_PREFIX user363-kimi-loadtest")
	got := trimToTokens(s, 400)
	n := estimateTokens(got)
	if n > 400 {
		t.Fatalf("trimmed tokens=%d", n)
	}
	if !strings.HasPrefix(s, got) {
		t.Fatal("trimmed value must remain a prefix")
	}
}

func TestBuildPayloadResponsesUsesInput(t *testing.T) {
	cls := payloadClass{Name: "smoke", Stream: true, Bucket: "smoke", TargetTokens: 80, UniqueTokens: 80}
	p, err := buildPayload(cls, "glm-5.3", 1, 64, nil, "off", "", APIModeResponses)
	if err != nil {
		t.Fatal(err)
	}
	body := string(p.Body)
	for _, needle := range []string{`"model":"glm-5.3"`, `"instructions"`, `"input"`, `"max_output_tokens":64`, `"stream":true`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("missing %s in %s", needle, body)
		}
	}
	if strings.Contains(body, `"messages"`) || strings.Contains(body, `"max_tokens"`) {
		t.Fatalf("responses payload should not use chat fields: %s", body[:min(len(body), 400)])
	}
}

func TestApplyStreamMode(t *testing.T) {
	cls := pickClass(rand.New(rand.NewSource(1)), "user363", defaultSyncRatio, 80000, defaultCacheShare, 1)
	if got := applyStreamMode(cls, StreamModeSync); got.Stream {
		t.Fatal("sync override")
	}
	if got := applyStreamMode(cls, StreamModeStream); !got.Stream {
		t.Fatal("stream override")
	}
}

func TestConsumeSSEResponsesDelta(t *testing.T) {
	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"model":"glm-5.3"}}`,
		``,
		`data: {"type":"response.output_text.delta","delta":"你好"}`,
		``,
		`data: {"type":"response.completed","response":{"model":"glm-5.3","usage":{"input_tokens":10,"output_tokens":2}}}`,
		``,
	}, "\n")
	st := consumeSSE(strings.NewReader(body), time.Now().Add(-8*time.Millisecond), false, nil, "glm-5.3")
	if !st.SawContent || st.ContentChars != 2 || st.FinishReason != "stop" {
		t.Fatalf("%+v", st)
	}
	if st.Usage.PromptTokens != 10 || st.Usage.CompletionTokens != 2 {
		t.Fatalf("usage %+v", st.Usage)
	}
	if st.ResponseModel != "glm-5.3" {
		t.Fatalf("model %q", st.ResponseModel)
	}
}

func TestNewClientRejectsInvalidProxy(t *testing.T) {
	_, err := newClient(1, time.Second, false, "ftp://proxy.example")
	if err == nil {
		t.Fatal("expected invalid proxy error")
	}
}

func TestNewClientAcceptsHTTPProxy(t *testing.T) {
	client, err := newClient(1, time.Second, false, "http://127.0.0.1:7890")
	if err != nil {
		t.Fatal(err)
	}
	if client == nil || client.Transport == nil {
		t.Fatal("missing transport")
	}
}

func TestPercentile(t *testing.T) {
	v := []int{1, 2, 3, 4, 5}
	if percentile(v, 0.5) != 3 {
		t.Fatalf("p50=%d", percentile(v, 0.5))
	}
	if percentile(v, 1) != 5 {
		t.Fatalf("max=%d", percentile(v, 1))
	}
}
