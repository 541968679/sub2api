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

// Customer sheet (user 363): input p50=50K p90=160K p99=380K avg≈80K.
var slaInputBuckets = []sizeBucket{
	{Name: "sla-p50", Weight: 50, Tokens: 50000},
	{Name: "sla-mid", Weight: 30, Tokens: 65000},
	{Name: "sla-p90", Weight: 10, Tokens: 160000},
	{Name: "sla-p99body", Weight: 8, Tokens: 200000},
	{Name: "sla-p99", Weight: 2, Tokens: 380000},
}

// Customer sheet: output p50=0.2K p90=1.3K p99=7K avg≈0.6K.
var slaOutputBuckets = []sizeBucket{
	{Name: "out-p50", Weight: 50, Tokens: 200},
	{Name: "out-mid", Weight: 30, Tokens: 400},
	{Name: "out-p90", Weight: 10, Tokens: 1300},
	{Name: "out-p99body", Weight: 8, Tokens: 2000},
	{Name: "out-p99", Weight: 2, Tokens: 7000},
}

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
	Model         string           `json:"model"`
	Messages      []chatMessage    `json:"messages"`
	Stream        bool             `json:"stream"`
	StreamOptions *streamOptions   `json:"stream_options,omitempty"`
	MaxTokens     *int             `json:"max_tokens,omitempty"`
	Temperature   *float64         `json:"temperature,omitempty"`
	Tools         []map[string]any `json:"tools,omitempty"`
	ToolChoice    any              `json:"tool_choice,omitempty"`
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

func pickClass(rng *rand.Rand, profile string, syncRatio float64, sizeCap int, cacheShare float64) payloadClass {
	switch profile {
	case "smoke":
		return payloadClass{Name: "smoke", Stream: true, Bucket: "smoke", TargetTokens: 80, UniqueTokens: 80}
	case "user363-sync":
		b := pickBucket(rng, syncBuckets, sizeCap)
		return payloadClass{Name: "sync-short", Stream: false, Bucket: b.Name, TargetTokens: b.Tokens, UniqueTokens: b.Tokens}
	case "user363-stream":
		return streamClass(rng, sizeCap, cacheShare)
	case "user363-sla":
		return slaClass(rng, sizeCap, cacheShare)
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

func slaClass(rng *rand.Rand, sizeCap int, cacheShare float64) payloadClass {
	in := pickBucket(rng, slaInputBuckets, sizeCap)
	out := pickBucket(rng, slaOutputBuckets, 0)
	if cacheShare < 0 {
		cacheShare = 0
	}
	if cacheShare > 0.95 {
		cacheShare = 0.95
	}
	cacheTok := int(float64(in.Tokens) * cacheShare)
	uniqueTok := in.Tokens - cacheTok
	if uniqueTok < 64 {
		uniqueTok = 64
		if cacheTok+uniqueTok > in.Tokens && in.Tokens > 64 {
			cacheTok = in.Tokens - uniqueTok
		}
	}
	return payloadClass{
		Name:         "stream-sla",
		Stream:       true,
		Bucket:       in.Name + "/" + out.Name,
		TargetTokens: in.Tokens,
		CacheTokens:  cacheTok,
		UniqueTokens: uniqueTok,
		MaxTokens:    out.Tokens,
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

func buildConversation(cls payloadClass, seq int, cachePrefix string) (system string, msgs []chatMessage) {
	system = "你是一个谨慎的编程助手。根据仓库上下文回答，不要编造文件内容。用简体中文回复。"
	if cls.CacheTokens > 0 {
		prefix := cachePrefix
		if prefix == "" {
			prefix = makeFiller(cls.CacheTokens, "STABLE_PREFIX user363-kimi-loadtest")
		} else {
			prefix = trimToTokens(prefix, cls.CacheTokens)
		}
		msgs = append(msgs,
			chatMessage{Role: "user", Content: prefix},
			chatMessage{Role: "assistant", Content: "已记住当前仓库上下文，请继续给具体任务。"},
		)
	}
	uniqueN := cls.UniqueTokens
	if uniqueN <= 0 {
		uniqueN = cls.TargetTokens
	}
	userTurn := makeFiller(uniqueN, fmt.Sprintf("SEQ=%d CLASS=%s BUCKET=%s 请用三句话总结上面上下文里出现过的错误类型，并给出一条可执行的下一步。不要重复全文。", seq, cls.Name, cls.Bucket))
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
	system, msgs := buildConversation(cls, seq, cachePrefix)
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
			req.MaxTokens = &mt
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
