package loadtest

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"unicode"
)

// Production fingerprint for user 363 (78496398@qq.com, api_key 585, group 46)
// snapshot 2026-09-21 16:34 CST, last 48h of usage_logs + ops_error_logs.
//
// Transport: 100% User-Agent Go-http-client/2.0, inbound /v1/chat/completions.
// Model mix is ~97% kimi-k3. Stream vs sync is two distinct workloads, not a
// single average. Peak in-flight ~55 p95 / ~86 max (Little's law per minute).

const (
	fingerprintCapturedAt = "2026-09-21 16:34 CST"
	fingerprintUserID     = 363
	defaultSyncRatio      = 0.57 // 14500 sync / 25504 kimi-k3
	defaultCacheShare     = 0.62 // stream p50 cache 32408 / p50 ctx 51905

	APIModeChatCompletions = "chat_completions"
	APIModeResponses       = "responses"

	StreamModeAuto   = "auto"
	StreamModeStream = "stream"
	StreamModeSync   = "sync"
)

type sizeBucket struct {
	Name   string
	Weight int
	Tokens int
}

var streamBuckets = []sizeBucket{
	{Name: "0-8k", Weight: 1010, Tokens: 4000},
	{Name: "8-20k", Weight: 1299, Tokens: 14000},
	{Name: "20-50k", Weight: 3024, Tokens: 35000},
	{Name: "50-100k", Weight: 3484, Tokens: 70000},
	{Name: "100-200k", Weight: 1905, Tokens: 140000},
	{Name: "200k+", Weight: 282, Tokens: 220000},
}

var syncBuckets = []sizeBucket{
	{Name: "0-500", Weight: 9587, Tokens: 350},
	{Name: "500-2k", Weight: 4516, Tokens: 900},
	{Name: "2-8k", Weight: 158, Tokens: 4000},
	{Name: "8k+", Weight: 239, Tokens: 12000},
}

// generalBuckets mirrors PresetTiers("general") for empty-table sampling fallback.
var generalBuckets = []sizeBucket{
	{Name: "4k", Weight: 40, Tokens: 4000},
	{Name: "16k", Weight: 30, Tokens: 16000},
	{Name: "32k", Weight: 20, Tokens: 32000},
	{Name: "64k", Weight: 10, Tokens: 64000},
}

// Customer sheet mix (per 100 requests): p50=50K p90=160K p99=380K avg≈80K.
// Output is paired with the same percentile: 0.2K / 0.6K / 1.3K / 7K.

const (
	slaTTFTp50Ms = 4000
	slaTTFTp75Ms = 8000
	slaTTFTp90Ms = 12000
	slaTTFTp99Ms = 30000
	slaTPOTp50   = 70.0
	slaTPOTp99   = 40.0
)

const fillerBlock = "这是一段用于模拟长会话上下文的填充文本，混有中文说明、报错日志和代码片段。" +
	" func handle(ctx context.Context, req Request) error { if err := validate(req); err != nil { return fmt.Errorf(\"validate: %w\", err) }; return nil } " +
	"ERROR timeout upstream retry schedule account_id=1762 model=kimi-k3 stream=true. " +
	"Please keep the previous file tree, git diff and tool results in mind. "

var fillerBlockTokens = estimateTokens(fillerBlock)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatRequest struct {
	Model               string           `json:"model"`
	Messages            []chatMessage    `json:"messages"`
	Stream              bool             `json:"stream"`
	StreamOptions       *streamOptions   `json:"stream_options,omitempty"`
	MaxTokens           *int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int             `json:"max_completion_tokens,omitempty"`
	Temperature         *float64         `json:"temperature,omitempty"`
	Tools               []map[string]any `json:"tools,omitempty"`
	ToolChoice          any              `json:"tool_choice,omitempty"`
}

type responsesRequest struct {
	Model           string           `json:"model"`
	Instructions    string           `json:"instructions,omitempty"`
	Input           any              `json:"input"`
	Stream          bool             `json:"stream"`
	MaxOutputTokens *int             `json:"max_output_tokens,omitempty"`
	Temperature     *float64         `json:"temperature,omitempty"`
	Tools           []map[string]any `json:"tools,omitempty"`
	ToolChoice      any              `json:"tool_choice,omitempty"`
}

type payloadClass struct {
	Name         string
	Stream       bool
	Bucket       string
	TargetTokens int
	CacheTokens  int
	UniqueTokens int
	MaxTokens    int
}

type builtPayload struct {
	Class     payloadClass
	Body      []byte
	ApproxTok int
}

func estimateTokens(s string) int {
	cjk := 0
	other := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			cjk++
			continue
		}
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		other++
	}
	return cjk + (other+3)/4
}

func trimToTokens(s string, n int) string {
	if n <= 0 || estimateTokens(s) <= n {
		return s
	}
	r := []rune(s)
	lo, hi := 0, len(r)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if estimateTokens(string(r[:mid])) <= n {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(r[:lo])
}

func makeFiller(tokens int, salt string) string {
	if tokens <= 0 {
		return salt
	}
	var b strings.Builder
	b.Grow(tokens * 2)
	if salt != "" {
		b.WriteString(salt)
		b.WriteByte('\n')
	}
	n := estimateTokens(b.String())
	if fillerBlockTokens <= 0 {
		for n < tokens {
			b.WriteString("填充")
			n++
		}
		return b.String()
	}
	for n < tokens {
		b.WriteString(fillerBlock)
		n += fillerBlockTokens
	}
	return b.String()
}

func pickBucket(rng *rand.Rand, buckets []sizeBucket, sizeCap int) sizeBucket {
	total := 0
	for _, b := range buckets {
		total += b.Weight
	}
	if total <= 0 {
		return buckets[0]
	}
	x := rng.Intn(total)
	acc := 0
	chosen := buckets[len(buckets)-1]
	for _, b := range buckets {
		acc += b.Weight
		if x < acc {
			chosen = b
			break
		}
	}
	if sizeCap > 0 && chosen.Tokens > sizeCap {
		chosen.Tokens = sizeCap
		chosen.Name = chosen.Name + fmt.Sprintf("@cap%d", sizeCap)
	}
	return chosen
}

func pickClass(rng *rand.Rand, profile string, syncRatio float64, sizeCap int, cacheShare float64, seq int) payloadClass {
	switch profile {
	case "smoke":
		return payloadClass{Name: "smoke", Stream: true, Bucket: "smoke", TargetTokens: 80, UniqueTokens: 80}
	case "general":
		b := pickBucket(rng, generalBuckets, sizeCap)
		return sizedStreamClass("stream-general", b.Name, b.Tokens, 0, cacheShare)
	case "user363-sync":
		b := pickBucket(rng, syncBuckets, sizeCap)
		return payloadClass{Name: "sync-short", Stream: false, Bucket: b.Name, TargetTokens: b.Tokens, UniqueTokens: b.Tokens}
	case "user363-stream":
		return streamClass(rng, sizeCap, cacheShare)
	case "user363-sla":
		return slaClassForSeq(seq, sizeCap, cacheShare)
	default: // user363 mixed
		if rng.Float64() < syncRatio {
			b := pickBucket(rng, syncBuckets, sizeCap)
			return payloadClass{Name: "sync-short", Stream: false, Bucket: b.Name, TargetTokens: b.Tokens, UniqueTokens: b.Tokens}
		}
		return streamClass(rng, sizeCap, cacheShare)
	}
}

func streamClass(rng *rand.Rand, sizeCap int, cacheShare float64) payloadClass {
	b := pickBucket(rng, streamBuckets, sizeCap)
	if cacheShare < 0 {
		cacheShare = 0
	}
	if cacheShare > 0.95 {
		cacheShare = 0.95
	}
	cacheTok := int(float64(b.Tokens) * cacheShare)
	uniqueTok := b.Tokens - cacheTok
	if uniqueTok < 64 {
		uniqueTok = 64
		if cacheTok+uniqueTok > b.Tokens && b.Tokens > 64 {
			cacheTok = b.Tokens - uniqueTok
		}
	}
	return payloadClass{
		Name:         "stream-ctx",
		Stream:       true,
		Bucket:       b.Name,
		TargetTokens: b.Tokens,
		CacheTokens:  cacheTok,
		UniqueTokens: uniqueTok,
	}
}

func applyStreamMode(cls payloadClass, mode string) payloadClass {
	switch mode {
	case StreamModeStream:
		cls.Stream = true
	case StreamModeSync:
		cls.Stream = false
	}
	return cls
}

func DefaultPathForAPIMode(apiMode string) string {
	if apiMode == APIModeResponses {
		return "/v1/responses"
	}
	return "/v1/chat/completions"
}

func defaultPathForAPIMode(apiMode string) string {
	return DefaultPathForAPIMode(apiMode)
}

func slaSlot(seq int) (inTok, outTok int, name string) {
	r := ((seq - 1) % 100) + 1
	switch {
	case r <= 50:
		return 50000, 200, "sla-p50"
	case r <= 88:
		return 80000, 600, "sla-avg"
	case r <= 98:
		return 160000, 1300, "sla-p90"
	default:
		return 380000, 7000, "sla-p99"
	}
}

func slaClassForSeq(seq, sizeCap int, cacheShare float64) payloadClass {
	inTok, outTok, name := slaSlot(seq)
	if sizeCap > 0 && inTok > sizeCap {
		inTok = sizeCap
		name += fmt.Sprintf("@cap%d", sizeCap)
	}
	return sizedStreamClass("stream-sla", name, inTok, outTok, cacheShare)
}

func sizedStreamClass(className, bucket string, inTok, outTok int, cacheShare float64) payloadClass {
	if cacheShare < 0 {
		cacheShare = 0
	}
	if cacheShare > 0.95 {
		cacheShare = 0.95
	}
	cacheTok := int(float64(inTok) * cacheShare)
	uniqueTok := inTok - cacheTok
	if uniqueTok < 64 {
		uniqueTok = 64
		if cacheTok+uniqueTok > inTok && inTok > 64 {
			cacheTok = inTok - uniqueTok
		}
	}
	return payloadClass{
		Name:         className,
		Stream:       true,
		Bucket:       bucket,
		TargetTokens: inTok,
		CacheTokens:  cacheTok,
		UniqueTokens: uniqueTok,
		MaxTokens:    outTok,
	}
}

func applyInputTokens(cls payloadClass, inputTokens int, cacheShare float64) payloadClass {
	if inputTokens <= 0 {
		return cls
	}
	outTok := cls.MaxTokens
	if outTok <= 0 {
		outTok = 200
	}
	return sizedStreamClass(cls.Name, fmt.Sprintf("fixed-%d", inputTokens), inputTokens, outTok, cacheShare)
}

const (
	maxTierRows        = 64
	maxTierInputTokens = 400000
	maxTierRequests    = 500
)

// Tier is one exact traffic row: this many requests at this input length.
type Tier struct {
	InputTokens int `json:"input_tokens"`
	Count       int `json:"count"`
}

// ValidateTiers accepts an empty list (preset sampling). A non-empty list is
// the whole run: each row is an absolute count, and the counts sum to 1–500.
func ValidateTiers(tiers []Tier) error {
	if len(tiers) == 0 {
		return nil
	}
	if len(tiers) > maxTierRows {
		return fmt.Errorf("tiers must be 1-%d rows", maxTierRows)
	}
	sum := 0
	for i, tier := range tiers {
		if tier.InputTokens < 1 || tier.InputTokens > maxTierInputTokens {
			return fmt.Errorf("tiers[%d].input_tokens must be 1-%d", i, maxTierInputTokens)
		}
		if tier.Count < 1 || tier.Count > maxTierRequests {
			return fmt.Errorf("tiers[%d].count must be 1-%d", i, maxTierRequests)
		}
		sum += tier.Count
		if sum > maxTierRequests {
			return fmt.Errorf("tier counts must sum to 1-%d", maxTierRequests)
		}
	}
	return nil
}

// expandTiers spreads rows across the run with weighted round-robin.
// Each row's count stays exact. A tie keeps the earlier row.
// Output max tokens stays 0 so the run-level max_tokens is used.
func expandTiers(tiers []Tier, cacheShare float64) []payloadClass {
	total := 0
	for _, tier := range tiers {
		total += tier.Count
	}
	if total <= 0 {
		return nil
	}
	acc := make([]int, len(tiers))
	out := make([]payloadClass, 0, total)
	for sent := 0; sent < total; sent++ {
		best := 0
		for i := range tiers {
			acc[i] += tiers[i].Count
			if acc[i] > acc[best] {
				best = i
			}
		}
		acc[best] -= total
		tier := tiers[best]
		out = append(out, sizedStreamClass("stream-tier", fmt.Sprintf("tier-%d", tier.InputTokens), tier.InputTokens, 0, cacheShare))
	}
	return out
}

func classForRequest(cfg Config, seq int, rng *rand.Rand, plan []payloadClass) payloadClass {
	if len(plan) > 0 {
		return applyStreamMode(plan[seq-1], cfg.StreamMode)
	}
	return applyInputTokens(applyStreamMode(pickClass(rng, cfg.Profile, cfg.SyncRatio, cfg.SizeCap, cfg.CacheShare, seq), cfg.StreamMode), cfg.InputTokens, cfg.CacheShare)
}

func inputBand(tokens int) string {
	switch {
	case tokens >= 40000 && tokens <= 60000:
		return "50k"
	case tokens >= 140000 && tokens <= 180000:
		return "160k"
	case tokens >= 300000 && tokens <= 420000:
		return "380k"
	default:
		return "other"
	}
}

func codingTools() []map[string]any {
	schema := func(name, desc string, props map[string]any, required []string) map[string]any {
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        name,
				"description": desc,
				"parameters": map[string]any{
					"type":                 "object",
					"properties":           props,
					"required":             required,
					"additionalProperties": false,
				},
			},
		}
	}
	return []map[string]any{
		schema("read_file", "Read a UTF-8 text file from the workspace.", map[string]any{
			"path":   map[string]any{"type": "string", "description": "Relative path"},
			"offset": map[string]any{"type": "integer"},
			"limit":  map[string]any{"type": "integer"},
		}, []string{"path"}),
		schema("write_file", "Write a UTF-8 text file into the workspace.", map[string]any{
			"path":    map[string]any{"type": "string"},
			"content": map[string]any{"type": "string"},
		}, []string{"path", "content"}),
		schema("run_terminal", "Run a shell command and return stdout/stderr.", map[string]any{
			"command":     map[string]any{"type": "string"},
			"working_dir": map[string]any{"type": "string"},
		}, []string{"command"}),
		schema("grep", "Search file contents with a regular expression.", map[string]any{
			"pattern": map[string]any{"type": "string"},
			"path":    map[string]any{"type": "string"},
			"glob":    map[string]any{"type": "string"},
		}, []string{"pattern"}),
	}
}

// RequiresNativeChatCompletions reports models whose upstream only accepts
// POST /v1/chat/completions with a messages body. kimi-k3 is in this set:
// a Responses body (input/instructions) is rejected.
func RequiresNativeChatCompletions(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	return m == "kimi" || strings.HasPrefix(m, "kimi-")
}

func usesMaxCompletionTokens(model string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	return m == "kimi-k3" || strings.HasPrefix(m, "kimi-k3-")
}

// resolveLoadtestRequest picks the endpoint for one request. kimi models stay
// on /v1/chat/completions even when the run's api_mode is responses.
func resolveLoadtestRequest(apiMode, path, model string) (mode, reqPath string) {
	if RequiresNativeChatCompletions(model) {
		return APIModeChatCompletions, DefaultPathForAPIMode(APIModeChatCompletions)
	}
	if apiMode == "" {
		apiMode = APIModeChatCompletions
	}
	if path == "" {
		path = DefaultPathForAPIMode(apiMode)
	}
	return apiMode, path
}

func buildConversation(cls payloadClass, seq int, cachePrefix, model string) (system string, msgs []chatMessage) {
	system = "你是一个谨慎的编程助手。根据仓库上下文回答，不要编造文件内容。用简体中文回复。"
	uniqueN := cls.UniqueTokens
	if uniqueN <= 0 {
		uniqueN = cls.TargetTokens
	}
	userTurn := makeFiller(uniqueN, fmt.Sprintf("SEQ=%d CLASS=%s BUCKET=%s 请用三句话总结上面上下文里出现过的错误类型，并给出一条可执行的下一步。不要重复全文。", seq, cls.Name, cls.Bucket))
	if cls.CacheTokens > 0 {
		prefix := cachePrefix
		if prefix == "" {
			prefix = makeFiller(cls.CacheTokens, "STABLE_PREFIX user363-kimi-loadtest")
		} else {
			prefix = trimToTokens(prefix, cls.CacheTokens)
		}
		// kimi-k3 rejects a synthetic assistant turn that has no reasoning_content.
		// Keep the stable prefix as the leading user text so the body stays a
		// native chat-completions conversation.
		if RequiresNativeChatCompletions(model) {
			msgs = append(msgs, chatMessage{Role: "user", Content: prefix + "\n" + userTurn})
			return system, msgs
		}
		msgs = append(msgs,
			chatMessage{Role: "user", Content: prefix},
			chatMessage{Role: "assistant", Content: "已记住当前仓库上下文，请继续给具体任务。"},
		)
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: userTurn})
	return system, msgs
}

func shouldSendTools(toolsMode string, stream bool) bool {
	switch toolsMode {
	case "coding":
		return true
	case "off":
		return false
	default:
		return stream
	}
}

func resolveMaxTokens(cls payloadClass, maxTokens int) int {
	if cls.MaxTokens > 0 {
		return cls.MaxTokens
	}
	return maxTokens
}

func buildPayload(cls payloadClass, model string, seq int, maxTokens int, temperature *float64, toolsMode, cachePrefix, apiMode string) (builtPayload, error) {
	if RequiresNativeChatCompletions(model) {
		apiMode = APIModeChatCompletions
	}
	system, msgs := buildConversation(cls, seq, cachePrefix, model)
	mt := resolveMaxTokens(cls, maxTokens)
	useTools := shouldSendTools(toolsMode, cls.Stream)

	var body []byte
	var err error
	if apiMode == APIModeResponses {
		req := responsesRequest{
			Model:        model,
			Instructions: system,
			Input:        msgs,
			Stream:       cls.Stream,
			Temperature:  temperature,
		}
		if mt > 0 {
			req.MaxOutputTokens = &mt
		}
		if useTools {
			req.Tools = codingTools()
			req.ToolChoice = "none"
		}
		body, err = json.Marshal(req)
	} else {
		req := chatRequest{
			Model:       model,
			Messages:    append([]chatMessage{{Role: "system", Content: system}}, msgs...),
			Stream:      cls.Stream,
			Temperature: temperature,
		}
		if cls.Stream {
			req.StreamOptions = &streamOptions{IncludeUsage: true}
		}
		if mt > 0 {
			if usesMaxCompletionTokens(model) {
				req.MaxCompletionTokens = &mt
			} else {
				req.MaxTokens = &mt
			}
		}
		if useTools {
			req.Tools = codingTools()
			req.ToolChoice = "none"
		}
		body, err = json.Marshal(req)
	}
	if err != nil {
		return builtPayload{}, err
	}
	approx := estimateTokens(system)
	for _, m := range msgs {
		approx += estimateTokens(m.Content)
	}
	return builtPayload{Class: cls, Body: body, ApproxTok: approx}, nil
}

func fingerprintText() string {
	var b strings.Builder
	b.WriteString("user 363 (78496398@qq.com) inbound fingerprint\n")
	b.WriteString("captured: " + fingerprintCapturedAt + "\n")
	b.WriteString("UA: Go-http-client/2.0 (stdlib HTTP/2 default)\n")
	b.WriteString("path: POST /v1/chat/completions\n")
	b.WriteString("model: kimi-k3 (~97% of 48h rows)\n")
	b.WriteString("sync 57%  p50 ctx 434 / p50 out 156 / p50 dur 8.1s\n")
	b.WriteString("stream 43% p50 ctx 51905 (cache 32408 + in 5075) / p50 out 223 / p50 TTFT 10.7s / p50 dur 17.1s\n")
	b.WriteString("in-flight p50 24 / p95 55 / peak 86\n")
	b.WriteString("error mix to watch: 503 routing, 413 >1MB, 502 empty/forbidden, 524, 400 prompt too long / invalid temperature\n")
	b.WriteString("customer SLA sheet: input p50/p90/p99/avg = 50K/160K/380K/80K; output 0.2K/1.3K/7K/0.6K\n")
	b.WriteString("customer SLA TTFT p50<4s p75<8s p90<12s p99<30s; TPOT p50>70 tok/s p99>40 tok/s\n")
	return b.String()
}
