package loadtest

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyurl"
	"github.com/Wei-Shaw/sub2api/internal/pkg/proxyutil"
)

type Config struct {
	BaseURL           string
	Path              string
	Models            []string
	Profile           string
	Concurrency       int
	Total             int
	Duration          time.Duration
	Timeout           time.Duration
	HeaderTimeout     time.Duration
	MaxTokens         int
	Tools             string
	SizeCap           int
	CacheShare        float64
	SyncRatio         float64
	Seed              int64
	AbortAfterContent bool
	InsecureTLS       bool
	APIKey            string
	Temperature       *float64
	ProxyURL          string
	APIMode           string
	StreamMode        string
	InputTokens       int
}

type Progress struct {
	Inflight int64   `json:"inflight"`
	Peak     int64   `json:"peak"`
	Done     int64   `json:"done"`
	OK       int64   `json:"ok"`
	Result   *Result `json:"result,omitempty"`
}

func (c Config) normalized() Config {
	out := c
	if out.APIMode == "" {
		out.APIMode = APIModeChatCompletions
	}
	if out.StreamMode == "" {
		out.StreamMode = StreamModeAuto
	}
	if out.Path == "" {
		out.Path = defaultPathForAPIMode(out.APIMode)
	}
	if out.Concurrency <= 0 {
		out.Concurrency = 1
	}
	if out.Total <= 0 && out.Duration <= 0 {
		out.Total = 1
	}
	if out.Timeout <= 0 {
		out.Timeout = 180 * time.Second
	}
	if out.HeaderTimeout <= 0 {
		out.HeaderTimeout = out.Timeout
	}
	if len(out.Models) == 0 {
		out.Models = []string{"kimi-k3"}
	}
	if out.Profile == "" {
		out.Profile = "user363"
	}
	if out.SyncRatio < 0 {
		out.SyncRatio = defaultSyncRatio
	}
	if out.CacheShare <= 0 {
		out.CacheShare = defaultCacheShare
	}
	if out.Seed == 0 {
		out.Seed = 363
	}
	if out.Profile == "user363-sla" && out.SizeCap == 80000 {
		out.SizeCap = 0
	}
	return out
}

func EstimateInputTokens(cfg Config) int {
	c := cfg.normalized()
	n := c.Total
	if c.Duration > 0 {
		n = c.Concurrency * 8
	}
	if n > 400 {
		n = 400
	}
	rng := rand.New(rand.NewSource(c.Seed))
	sum := 0
	for i := 0; i < n; i++ {
		sum += applyInputTokens(pickClass(rng, c.Profile, c.SyncRatio, c.SizeCap, c.CacheShare, i+1), c.InputTokens, c.CacheShare).TargetTokens
	}
	return sum
}

func Run(ctx context.Context, cfg Config, onProgress func(Progress)) ([]Result, error) {
	c := cfg.normalized()
	if strings.TrimSpace(c.BaseURL) == "" {
		return nil, fmt.Errorf("base url is required")
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return nil, fmt.Errorf("api key is required")
	}
	client, err := newClient(c.Concurrency, c.HeaderTimeout, c.InsecureTLS, c.ProxyURL)
	if err != nil {
		return nil, err
	}
	endpoint := JoinURL(c.BaseURL, c.Path)
	headers := map[string]string{
		"Authorization": "Bearer " + c.APIKey,
		"Content-Type":  "application/json",
		"Accept":        "application/json",
	}
	rng := rand.New(rand.NewSource(c.Seed))

	var inflight, peak, doneCount, okCount atomic.Int64
	var resultsMu sync.Mutex
	var results []Result

	record := func(r Result) {
		resultsMu.Lock()
		results = append(results, r)
		resultsMu.Unlock()
		doneCount.Add(1)
		if r.Outcome == "success" {
			okCount.Add(1)
		}
		if onProgress != nil {
			copied := r
			onProgress(Progress{
				Inflight: inflight.Load(),
				Peak:     peak.Load(),
				Done:     doneCount.Load(),
				OK:       okCount.Load(),
				Result:   &copied,
			})
		}
	}

	started := time.Now()
	var stopLaunch time.Time
	if c.Duration > 0 {
		stopLaunch = started.Add(c.Duration)
	}
	sem := make(chan struct{}, c.Concurrency)
	var wg sync.WaitGroup
	seq := 0

	launch := func() bool {
		if ctx.Err() != nil {
			return false
		}
		if c.Duration > 0 {
			if time.Now().After(stopLaunch) {
				return false
			}
		} else if seq >= c.Total {
			return false
		}
		seq++
		curSeq := seq
		cls := applyInputTokens(applyStreamMode(pickClass(rng, c.Profile, c.SyncRatio, c.SizeCap, c.CacheShare, curSeq), c.StreamMode), c.InputTokens, c.CacheShare)
		modelName := c.Models[(curSeq-1)%len(c.Models)]
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
			reqCtx, cancel := context.WithTimeout(ctx, c.Timeout)
			defer cancel()
			record(runOne(reqCtx, client, endpoint, headers, cls, modelName, curSeq, c.MaxTokens, c.Temperature, c.Tools, "", c.AbortAfterContent, c.APIMode))
		}()
		return true
	}

	for launch() {
		if ctx.Err() != nil {
			break
		}
	}
	wg.Wait()
	resultsMu.Lock()
	out := append([]Result(nil), results...)
	resultsMu.Unlock()
	return out, ctx.Err()
}

func JoinURL(base, path string) string {
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

func newClient(concurrency int, headerTimeout time.Duration, insecure bool, proxyRaw string) (*http.Client, error) {
	transport := &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          concurrency * 4,
		MaxIdleConnsPerHost:   concurrency + 8,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: headerTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	_, parsed, err := proxyurl.Parse(proxyRaw)
	if err != nil {
		return nil, fmt.Errorf("proxy: %w", err)
	}
	if parsed != nil {
		if err := proxyutil.ConfigureTransportProxy(transport, parsed); err != nil {
			return nil, fmt.Errorf("proxy: %w", err)
		}
	}
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &http.Client{Transport: transport, Timeout: 0}, nil
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
	apiMode string,
) Result {
	started := time.Now()
	if apiMode == "" {
		apiMode = APIModeChatCompletions
	}
	res := Result{
		Seq:            seq,
		StartedAt:      started.UTC().Format(time.RFC3339Nano),
		Class:          cls.Name,
		Bucket:         cls.Bucket,
		Stream:         cls.Stream,
		RequestedModel: model,
		TargetTokens:   cls.TargetTokens,
		InputBand:      inputBand(cls.TargetTokens),
		APIMode:        apiMode,
	}
	payload, err := buildPayload(cls, model, seq, maxTokens, temperature, toolsMode, cachePrefix, apiMode)
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
	if cls.Stream {
		st := consumeSSE(resp.Body, started, abortAfterContent, func() { _ = resp.Body.Close() }, model)
		applyStreamStats(&res, st, true)
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

func PublicSLA(rows []Result) []SLAVerdict {
	return evalSLA(rows)
}

func SLAOK(rows []Result) bool {
	return slaPassed(evalSLA(rows))
}

func ParseModels(model, models string) []string {
	return parseModelList(model, models)
}

func StrictContractHit(rows []Result) int {
	n := 0
	for _, r := range rows {
		for _, iss := range r.ContractIssues {
			if isStrictContractIssue(iss) {
				n++
				break
			}
		}
	}
	return n
}

func ModelMissingRequests(rows []Result) int {
	n := 0
	for _, r := range rows {
		if r.ModelMissingN > 0 || r.ModelEmptyN > 0 {
			n++
		}
	}
	return n
}
