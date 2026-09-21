// Command kimi-loadtest sends OpenAI Chat Completions load that matches
// production user 363 (78496398@qq.com) inbound shape: Go HTTP/2 client,
// POST /v1/chat/completions, kimi-k3, mixed short-sync + large-stream.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type headerFlags []string

func (h *headerFlags) String() string { return strings.Join(*h, ",") }
func (h *headerFlags) Set(v string) error {
	*h = append(*h, v)
	return nil
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(argv []string) int {
	fs := flag.NewFlagSet("kimi-loadtest", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	baseURL := fs.String("base-url", "", "Upstream origin, e.g. https://api.example.com")
	path := fs.String("path", "/v1/chat/completions", "Request path")
	model := fs.String("model", "kimi-k3", "single model id")
	modelsFlag := fs.String("models", "", "comma-separated models (round-robin). overrides -model. e.g. glm-5.3,kimi-k3")
	profile := fs.String("profile", "user363", "user363 | user363-stream | user363-sync | user363-sla | smoke")
	concurrency := fs.Int("concurrency", 50, "in-flight workers")
	total := fs.Int("total", 100, "requests to send; ignored when -duration > 0")
	duration := fs.Duration("duration", 0, "keep workers busy for this long (overrides -total)")
	timeout := fs.Duration("timeout", 180*time.Second, "per-request total timeout")
	headerTimeout := fs.Duration("header-timeout", 0, "response header timeout; 0 = same as -timeout")
	warmup := fs.Int("warmup", 2, "first N results excluded from summary")
	maxTokens := fs.Int("max-tokens", 256, "max_tokens; 0 omits the field")
	tempFlag := fs.String("temperature", "", "omit by default (kimi-k3 rejected some values in prod)")
	toolsMode := fs.String("tools", "auto", "auto | coding | off")
	sizeCap := fs.Int("size-cap", 80000, "cap sampled context tokens; 0 = faithful including 200k+")
	cacheShare := fs.Float64("cache-share", defaultCacheShare, "fraction of stream context reused as a stable prefix")
	syncRatio := fs.Float64("sync-ratio", -1, "override mix sync fraction; <0 uses profile default 0.57")
	seed := fs.Int64("seed", 363, "RNG seed")
	outDir := fs.String("out", "", "output directory")
	dryRun := fs.Bool("dry-run", false, "build payloads and print fingerprint, send nothing")
	printFP := fs.Bool("print-fingerprint", false, "print the baked-in user 363 fingerprint and exit")
	yes := fs.Bool("yes", false, "do not prompt on large estimated token volume")
	insecure := fs.Bool("insecure", false, "skip TLS verify")
	abortFirst := fs.Bool("abort-after-first-content", false, "cancel the stream after first useful delta (cheaper TTFT probe)")
	strictContract := fs.Bool("strict-contract", false, "exit 1 on model_missing / model_empty / model_mismatch")
	slaFlag := fs.Bool("sla", false, "evaluate customer TTFT/TPOT SLA (auto-on for user363-sla)")
	apiKeyFlag := fs.String("api-key", "", "prefer env LOADTEST_KEY / KIMI_LOADTEST_KEY / SUB2API_KEY")
	var extraHeaders headerFlags
	fs.Var(&extraHeaders, "H", "extra header Key:Value (repeatable)")

	if err := fs.Parse(argv); err != nil {
		return 2
	}
	if *printFP {
		fmt.Fprint(os.Stdout, fingerprintText())
		return 0
	}
	switch *profile {
	case "user363", "user363-stream", "user363-sync", "user363-sla", "smoke":
	default:
		fmt.Fprintf(os.Stderr, "unknown -profile %q\n", *profile)
		return 2
	}
	modelList := parseModelList(*model, *modelsFlag)
	evalSLAFlag := *slaFlag || *profile == "user363-sla"
	if *profile == "user363-sla" && *sizeCap == 80000 {
		*sizeCap = 0
		fmt.Fprintln(os.Stderr, "user363-sla: default -size-cap 80000 lifted so customer p99 380K can be sent; pass -size-cap to clamp")
	}
	if *concurrency <= 0 || *total <= 0 && *duration <= 0 {
		fmt.Fprintln(os.Stderr, "-concurrency must be > 0 and need -total or -duration")
		return 2
	}
	if *headerTimeout <= 0 {
		*headerTimeout = *timeout
	}
	ratio := *syncRatio
	if ratio < 0 {
		ratio = defaultSyncRatio
	}

	var temperature *float64
	if strings.TrimSpace(*tempFlag) != "" {
		var v float64
		if _, err := fmt.Sscanf(*tempFlag, "%f", &v); err != nil {
			fmt.Fprintf(os.Stderr, "invalid -temperature: %v\n", err)
			return 2
		}
		temperature = &v
	}

	if *baseURL == "" && !*dryRun {
		fmt.Fprintln(os.Stderr, "-base-url is required (or use -dry-run / -print-fingerprint)")
		return 2
	}

	apiKey := strings.TrimSpace(*apiKeyFlag)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("LOADTEST_KEY"))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("KIMI_LOADTEST_KEY"))
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("SUB2API_KEY"))
	}
	if apiKey == "" && !*dryRun {
		fmt.Fprintln(os.Stderr, "missing API key: set LOADTEST_KEY or KIMI_LOADTEST_KEY or SUB2API_KEY")
		return 2
	}

	out := *outDir
	if out == "" {
		out = defaultOutDir()
	}
	if !filepath.IsAbs(out) {
		cwd, _ := os.Getwd()
		out = filepath.Join(cwd, out)
	}

	rng := rand.New(rand.NewSource(*seed))
	cachePrefix := "" // buildPayload synthesizes a stable same-salt prefix per CacheTokens

	nPlan := *total
	if *duration > 0 {
		nPlan = *concurrency * 8
	}
	estInput := estimateRunTokens(rng, *profile, ratio, *sizeCap, *cacheShare, nPlan)
	estOut := 0
	if *profile == "user363-sla" {
		estOut = 600 * nPlan
	} else if *maxTokens > 0 {
		estOut = *maxTokens * nPlan
	}
	fmt.Fprintf(os.Stderr, "%s", fingerprintText())
	fmt.Fprintf(os.Stderr, "\nplan: profile=%s models=%s concurrency=%d total~%d duration=%s size-cap=%d sla=%t\n",
		*profile, strings.Join(modelList, ","), *concurrency, nPlan, *duration, *sizeCap, evalSLAFlag)
	fmt.Fprintf(os.Stderr, "estimated input tokens ~%d, max output tokens ~%d (cap via -max-tokens / SLA output mix)\n", estInput, estOut)

	if *dryRun {
		return runDry(*profile, modelList, *maxTokens, temperature, *toolsMode, *sizeCap, *cacheShare, ratio, rng, cachePrefix)
	}

	if estInput > 2_000_000 && !*yes {
		fmt.Fprintf(os.Stderr, "estimated input > 2M tokens. re-run with -yes to confirm, or lower -total/-size-cap/-concurrency.\n")
		return 2
	}

	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir out: %v\n", err)
		return 1
	}

	client := newClient(*concurrency, *headerTimeout, *insecure)
	endpoint := joinURL(*baseURL, *path)
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Content-Type":  "application/json",
		"Accept":        "application/json",
	}
	for _, h := range extraHeaders {
		k, v, ok := strings.Cut(h, ":")
		if !ok || strings.TrimSpace(k) == "" {
			fmt.Fprintf(os.Stderr, "invalid -H %q, want Key:Value\n", h)
			return 2
		}
		headers[http.CanonicalHeaderKey(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}

	cfgSnap := map[string]any{
		"base_url":            *baseURL,
		"path":                *path,
		"model":               strings.Join(modelList, ","),
		"profile":             *profile,
		"concurrency":         *concurrency,
		"total":               *total,
		"duration":            duration.String(),
		"timeout":             timeout.String(),
		"header_timeout":      headerTimeout.String(),
		"warmup":              *warmup,
		"max_tokens":          *maxTokens,
		"tools":               *toolsMode,
		"size_cap":            *sizeCap,
		"cache_share":         *cacheShare,
		"sync_ratio":          ratio,
		"seed":                *seed,
		"abort_after_content": *abortFirst,
		"strict_contract":     *strictContract,
		"sla":                 evalSLAFlag,
		"api_key_present":     apiKey != "",
		"fingerprint_at":      fingerprintCapturedAt,
		"user_id":             fingerprintUserID,
	}
	if raw, err := json.MarshalIndent(cfgSnap, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(out, "run.json"), raw, 0o644)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	jsonlPath := filepath.Join(out, "requests.jsonl")
	jsonl, err := os.Create(jsonlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create jsonl: %v\n", err)
		return 1
	}
	defer jsonl.Close()
	var jsonlMu sync.Mutex
	enc := json.NewEncoder(jsonl)
	enc.SetEscapeHTML(false)

	var inflight atomic.Int64
	var peak atomic.Int64
	var doneCount atomic.Int64
	var okCount atomic.Int64
	var resultsMu sync.Mutex
	var results []Result

	record := func(r Result) {
		jsonlMu.Lock()
		_ = enc.Encode(r)
		jsonl.Sync()
		jsonlMu.Unlock()
		resultsMu.Lock()
		results = append(results, r)
		resultsMu.Unlock()
		doneCount.Add(1)
		if r.Outcome == "success" {
			okCount.Add(1)
		}
	}

	started := time.Now()
	stopLaunch := time.Time{}
	if *duration > 0 {
		stopLaunch = started.Add(*duration)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, *concurrency)
	seq := 0
	launch := func() bool {
		if ctx.Err() != nil {
			return false
		}
		if *duration > 0 {
			if time.Now().After(stopLaunch) {
				return false
			}
		} else if seq >= *total {
			return false
		}
		seq++
		curSeq := seq
		cls := pickClass(rng, *profile, ratio, *sizeCap, *cacheShare)
		modelName := modelList[(curSeq-1)%len(modelList)]
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			n := inflight.Add(1)
			for {
				old := peak.Load()
				if n <= old || peak.CompareAndSwap(old, n) {
					break
				}
			}
			defer inflight.Add(-1)
			reqCtx, reqCancel := context.WithTimeout(ctx, *timeout)
			defer reqCancel()
			r := runOne(reqCtx, client, endpoint, headers, cls, modelName, curSeq, *maxTokens, temperature, *toolsMode, cachePrefix, *abortFirst)
			record(r)
		}()
		return true
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "[loadtest] inflight=%d peak=%d done=%d ok=%d elapsed=%s\n",
					inflight.Load(), peak.Load(), doneCount.Load(), okCount.Load(), time.Since(started).Truncate(time.Second))
			}
		}
	}()

	for launch() {
		select {
		case <-ctx.Done():
			break
		default:
		}
		if ctx.Err() != nil {
			break
		}
	}
	wg.Wait()
	ended := time.Now()

	resultsMu.Lock()
	all := append([]Result(nil), results...)
	resultsMu.Unlock()
	effective := all
	if *warmup > 0 && *warmup < len(all) {
		effective = all[*warmup:]
	}

	summaryPath := filepath.Join(out, "summary.md")
	if err := writeSummary(summaryPath, cfgSnap, effective, started, ended, int(peak.Load())); err != nil {
		fmt.Fprintf(os.Stderr, "write summary: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "\ndone. peak in-flight=%d  results=%s\n", peak.Load(), out)
	fmt.Fprintf(os.Stderr, "summary: %s\n", summaryPath)
	ok := 0
	for _, r := range effective {
		if r.Outcome == "success" {
			ok++
		}
	}
	if len(effective) == 0 {
		return 1
	}
	fmt.Fprintf(os.Stderr, "success %d/%d (%.1f%%)\n", ok, len(effective), float64(ok)*100/float64(len(effective)))

	missingModel := 0
	strictHit := 0
	for _, r := range effective {
		if r.ModelMissingN > 0 || r.ModelEmptyN > 0 {
			missingModel++
		}
		for _, iss := range r.ContractIssues {
			if isStrictContractIssue(iss) {
				strictHit++
				break
			}
		}
	}
	if missingModel > 0 {
		fmt.Fprintf(os.Stderr, "contract: model_missing/empty on %d/%d requests (Go clients see got \"\")\n", missingModel, len(effective))
	}
	slaFail := false
	if evalSLAFlag {
		verdicts := evalSLA(effective)
		if !slaPassed(verdicts) {
			slaFail = true
			fmt.Fprintln(os.Stderr, "SLA: FAIL (see summary 二、性能指标)")
		} else {
			fmt.Fprintln(os.Stderr, "SLA: PASS")
		}
	}
	exit := 0
	if ok*100/len(effective) < 98 {
		exit = 1
	}
	if evalSLAFlag && slaFail {
		exit = 1
	}
	if *strictContract && strictHit > 0 {
		fmt.Fprintf(os.Stderr, "strict-contract: %d requests with model/response.model issues\n", strictHit)
		exit = 1
	}
	return exit
}

func runDry(profile string, models []string, maxTokens int, temperature *float64, toolsMode string, sizeCap int, cacheShare, ratio float64, rng *rand.Rand, cachePrefix string) int {
	fmt.Fprintln(os.Stderr, "\ndry-run sample payloads:")
	for i := 1; i <= 8; i++ {
		cls := pickClass(rng, profile, ratio, sizeCap, cacheShare)
		modelName := models[(i-1)%len(models)]
		p, err := buildPayload(cls, modelName, i, maxTokens, temperature, toolsMode, cachePrefix)
		if err != nil {
			fmt.Fprintf(os.Stderr, "build payload: %v\n", err)
			return 1
		}
		mt := cls.MaxTokens
		if mt == 0 {
			mt = maxTokens
		}
		fmt.Fprintf(os.Stderr, "  #%d model=%s class=%s bucket=%s stream=%t target=%d max_tokens=%d approx=%d body_bytes=%d\n",
			i, modelName, cls.Name, cls.Bucket, cls.Stream, cls.TargetTokens, mt, p.ApproxTok, len(p.Body))
	}
	return 0
}

func estimateRunTokens(rng *rand.Rand, profile string, ratio float64, sizeCap int, cacheShare float64, n int) int {
	if n > 400 {
		n = 400
	}
	sum := 0
	r := rand.New(rand.NewSource(rng.Int63()))
	for i := 0; i < n; i++ {
		sum += pickClass(r, profile, ratio, sizeCap, cacheShare).TargetTokens
	}
	return sum
}

func newClient(concurrency int, headerTimeout time.Duration, insecure bool) *http.Client {
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          concurrency * 4,
		MaxIdleConnsPerHost:   concurrency + 8,
		MaxConnsPerHost:       0,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: headerTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &http.Client{
		Transport: transport,
		Timeout:   0,
	}
}

func joinURL(base, path string) string {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if path == "" {
		path = "/v1/chat/completions"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if strings.HasSuffix(base, "/v1") && strings.HasPrefix(path, "/v1/") {
		path = strings.TrimPrefix(path, "/v1")
	}
	return base + path
}

func runOne(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	headers map[string]string,
	cls payloadClass,
	model string,
	seq int,
	maxTokens int,
	temperature *float64,
	toolsMode string,
	cachePrefix string,
	abortAfterContent bool,
) Result {
	started := time.Now()
	res := Result{
		Seq:            seq,
		StartedAt:      started.UTC().Format(time.RFC3339Nano),
		Class:          cls.Name,
		Bucket:         cls.Bucket,
		Stream:         cls.Stream,
		RequestedModel: model,
		TargetTokens:   cls.TargetTokens,
	}
	payload, err := buildPayload(cls, model, seq, maxTokens, temperature, toolsMode, cachePrefix)
	if err != nil {
		res.Outcome = "build_error"
		res.ErrorCategory = "build_error"
		res.ErrorMessage = err.Error()
		res.DurationMs = int(time.Since(started).Milliseconds())
		return res
	}
	res.BodyBytes = len(payload.Body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload.Body))
	if err != nil {
		res.Outcome = "network_error"
		res.ErrorCategory = "network_error"
		res.ErrorMessage = err.Error()
		res.DurationMs = int(time.Since(started).Milliseconds())
		return res
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if cls.Stream {
		req.Header.Set("Accept", "text/event-stream")
	}

	resp, err := client.Do(req)
	if err != nil {
		res.DurationMs = int(time.Since(started).Milliseconds())
		if ctx.Err() != nil {
			res.Outcome = "timeout"
			res.ErrorCategory = "timeout"
			res.ErrorMessage = err.Error()
			return res
		}
		res.Outcome = "network_error"
		res.ErrorCategory = "network_error"
		res.ErrorMessage = err.Error()
		return res
	}
	defer resp.Body.Close()
	res.StatusCode = resp.StatusCode
	res.Proto = resp.Proto
	res.HeaderMs = int(time.Since(started).Milliseconds())
	res.RequestID = firstHeader(resp.Header, "X-Request-Id", "X-Request-ID", "Request-Id")

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		res.Outcome = "http_error"
		res.ErrorCategory, res.ErrorMessage = classifyHTTP(resp.StatusCode, string(body))
		res.DurationMs = int(time.Since(started).Milliseconds())
		return res
	}

	includeUsage := cls.Stream
	if cls.Stream {
		st := consumeSSE(resp.Body, started, abortAfterContent, func() {
			_ = resp.Body.Close()
		}, model)
		applyStreamStats(&res, st, includeUsage)
	} else {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		if err != nil {
			res.Outcome = "network_error"
			res.ErrorCategory = "network_error"
			res.ErrorMessage = err.Error()
			res.DurationMs = int(time.Since(started).Milliseconds())
			return res
		}
		st := parseJSONCompletion(body, model)
		applyStreamStats(&res, st, false)
		if res.FirstContentMs == 0 && st.SawContent {
			res.FirstContentMs = res.HeaderMs
		}
	}
	res.DurationMs = int(time.Since(started).Milliseconds())
	res.computeTPOT()
	switch {
	case res.ErrorCategory != "":
		if res.Outcome == "" {
			res.Outcome = res.ErrorCategory
		}
		return res
	case cls.Stream && !res.SawSuccess():
		if res.Chunks == 0 {
			res.Outcome = "empty_stream"
			res.ErrorCategory = "empty_stream"
			res.ErrorMessage = "HTTP 200 stream with no content/tool_calls"
		} else {
			res.Outcome = "truncated_stream"
			res.ErrorCategory = "truncated_stream"
			res.ErrorMessage = "stream ended without useful output"
		}
	case !cls.Stream && !res.SawSuccess():
		res.Outcome = "empty_response"
		res.ErrorCategory = "empty_response"
		res.ErrorMessage = "HTTP 200 JSON without content/tool_calls"
	default:
		res.Outcome = "success"
	}
	return res
}

func (r Result) SawSuccess() bool {
	return r.ContentChars > 0 || r.FinishReason == "tool_calls" || r.CompletionTokens > 0
}

func applyStreamStats(res *Result, st streamStats, includeUsage bool) {
	if st.FirstSSE > 0 {
		res.FirstSSEMs = int(st.FirstSSE.Milliseconds())
	}
	if st.FirstContent > 0 {
		res.FirstContentMs = int(st.FirstContent.Milliseconds())
	}
	res.Chunks = st.Chunks
	res.ContentChars = st.ContentChars
	res.FinishReason = st.FinishReason
	res.ResponseModel = st.ResponseModel
	res.PromptTokens = st.Usage.PromptTokens
	res.CompletionTokens = st.Usage.CompletionTokens
	res.CachedTokens = st.Usage.CachedTokens
	res.InspectedChunks = st.Inspected
	res.ModelMissingN = st.ModelMissing
	res.ModelEmptyN = st.ModelEmpty
	res.ModelPresentN = st.ModelPresent
	res.ModelMismatchN = st.ModelMismatch
	res.ContractIssues = contractIssues(st, res.Stream, includeUsage)
	if st.SawToolCall && res.FinishReason == "" {
		res.FinishReason = "tool_calls"
	}
	if st.Truncated && st.FinishReason == "" && !st.SawDone && !st.SawContent && !st.SawToolCall {
		res.ErrorCategory = "truncated_stream"
	}
}

func firstHeader(h http.Header, keys ...string) string {
	for _, k := range keys {
		if v := h.Get(k); v != "" {
			return v
		}
	}
	return ""
}
