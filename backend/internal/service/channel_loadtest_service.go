package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/loadtest"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/google/uuid"
)

const (
	loadtestMaxConcurrency = 80
	loadtestMaxTotal       = 500
	loadtestMaxDuration    = 10 * time.Minute
	loadtestMaxKeepRuns    = 8
	loadtestCostTokenWarn  = 2_000_000
)

type ChannelLoadtestStartInput struct {
	AccountID       *int64
	BaseURL         string
	APIKey          string
	ProxyID         *int64
	Path            string
	APIMode         string
	StreamMode      string
	Model           string
	Models          string
	Profile         string
	Concurrency     int
	Total           int
	DurationSec     int
	TimeoutSec      int
	MaxTokens       int
	SizeCap         int
	Tools           string
	ConfirmCost     bool
	AbortAfterFirst bool
	InputTokens     int
}

type ChannelLoadtestSnapshot struct {
	ID                 string                `json:"id"`
	Status             string                `json:"status"`
	Source             string                `json:"source"`
	AccountID          *int64                `json:"account_id,omitempty"`
	AccountName        string                `json:"account_name,omitempty"`
	ProxyID            *int64                `json:"proxy_id,omitempty"`
	ProxyName          string                `json:"proxy_name,omitempty"`
	BaseURL            string                `json:"base_url"`
	Path               string                `json:"path"`
	APIMode            string                `json:"api_mode"`
	StreamMode         string                `json:"stream_mode"`
	Models             []string              `json:"models"`
	Profile            string                `json:"profile"`
	Concurrency        int                   `json:"concurrency"`
	Total              int                   `json:"total"`
	DurationSec        int                   `json:"duration_sec,omitempty"`
	StartedAt          time.Time             `json:"started_at"`
	EndedAt            *time.Time            `json:"ended_at,omitempty"`
	Error              string                `json:"error,omitempty"`
	Inflight           int64                 `json:"inflight"`
	Peak               int64                 `json:"peak"`
	Done               int64                 `json:"done"`
	OK                 int64                 `json:"ok"`
	SuccessRate        float64               `json:"success_rate"`
	EstimatedInputTok  int                   `json:"estimated_input_tokens"`
	InputTokens        int                   `json:"input_tokens,omitempty"`
	DataProfile        loadtest.DataProfile  `json:"data_profile"`
	SLA                []loadtest.SLAVerdict `json:"sla"`
	SLAPass            bool                  `json:"sla_pass"`
	ModelMissingReqs   int                   `json:"model_missing_requests"`
	StrictContractHits int                   `json:"strict_contract_hits"`
	Results            []loadtest.Result     `json:"results"`
}

type channelLoadtestRun struct {
	id          string
	status      string
	source      string
	accountID   *int64
	accountName string
	proxyID     *int64
	proxyName   string
	baseURL     string
	cfg         loadtest.Config
	cancel      context.CancelFunc
	startedAt   time.Time
	endedAt     *time.Time
	err         string
	inflight    int64
	peak        int64
	done        int64
	ok          int64
	results     []loadtest.Result
}

type ChannelLoadtestService struct {
	accountRepo AccountRepository
	proxyRepo   ProxyRepository
	mu          sync.Mutex
	runningID   string
	runs        []*channelLoadtestRun
}

func NewChannelLoadtestService(accountRepo AccountRepository, proxyRepo ProxyRepository) *ChannelLoadtestService {
	return &ChannelLoadtestService{
		accountRepo: accountRepo,
		proxyRepo:   proxyRepo,
		runs:        make([]*channelLoadtestRun, 0, loadtestMaxKeepRuns),
	}
}

func (s *ChannelLoadtestService) Start(ctx context.Context, in ChannelLoadtestStartInput) (*ChannelLoadtestSnapshot, error) {
	if in.Concurrency <= 0 {
		in.Concurrency = 20
	}
	if in.Concurrency > loadtestMaxConcurrency {
		return nil, fmt.Errorf("concurrency must be 1-%d", loadtestMaxConcurrency)
	}
	if in.Total <= 0 {
		in.Total = 40
	}
	if in.Total > loadtestMaxTotal {
		return nil, fmt.Errorf("total must be 1-%d", loadtestMaxTotal)
	}
	if in.DurationSec < 0 || time.Duration(in.DurationSec)*time.Second > loadtestMaxDuration {
		return nil, fmt.Errorf("duration_sec must be 0-%d", int(loadtestMaxDuration.Seconds()))
	}
	switch in.Profile {
	case "", "user363", "user363-stream", "user363-sync", "user363-sla", "smoke":
		if in.Profile == "" {
			in.Profile = "user363"
		}
	default:
		return nil, fmt.Errorf("unknown profile %q", in.Profile)
	}
	switch in.APIMode {
	case "", loadtest.APIModeChatCompletions, loadtest.APIModeResponses:
		if in.APIMode == "" {
			in.APIMode = loadtest.APIModeChatCompletions
		}
	default:
		return nil, fmt.Errorf("api_mode must be chat_completions or responses")
	}
	switch in.StreamMode {
	case "", loadtest.StreamModeAuto, loadtest.StreamModeStream, loadtest.StreamModeSync:
		if in.StreamMode == "" {
			in.StreamMode = loadtest.StreamModeAuto
		}
	default:
		return nil, fmt.Errorf("stream_mode must be auto, stream, or sync")
	}
	if in.Path == "" {
		in.Path = loadtest.DefaultPathForAPIMode(in.APIMode)
	}
	if in.Tools == "" {
		in.Tools = "auto"
	}
	if in.MaxTokens < 0 || in.MaxTokens > 8000 {
		return nil, fmt.Errorf("max_tokens must be 0-8000")
	}
	if in.SizeCap < 0 || in.SizeCap > 400000 {
		return nil, fmt.Errorf("size_cap must be 0-400000")
	}
	if in.InputTokens < 0 || in.InputTokens > 400000 {
		return nil, fmt.Errorf("input_tokens must be 0-400000")
	}
	if in.Profile == "user363-sla" && in.SizeCap == 80000 {
		in.SizeCap = 0
	}

	baseURL, apiKey, source, acc, err := s.resolveTarget(ctx, in)
	if err != nil {
		return nil, err
	}
	var accountID *int64
	accountName := ""
	if acc != nil {
		id := acc.ID
		accountID = &id
		accountName = acc.Name
	}
	proxyURL, proxyID, proxyName, err := s.resolveProxy(ctx, in.ProxyID, acc)
	if err != nil {
		return nil, err
	}

	models := loadtest.ParseModels(in.Model, in.Models)
	timeout := 180 * time.Second
	if in.TimeoutSec > 0 {
		timeout = time.Duration(in.TimeoutSec) * time.Second
	}
	cfg := loadtest.Config{
		BaseURL:           baseURL,
		Path:              in.Path,
		Models:            models,
		Profile:           in.Profile,
		Concurrency:       in.Concurrency,
		Total:             in.Total,
		Timeout:           timeout,
		MaxTokens:         in.MaxTokens,
		Tools:             in.Tools,
		SizeCap:           in.SizeCap,
		APIKey:            apiKey,
		AbortAfterContent: in.AbortAfterFirst,
		ProxyURL:          proxyURL,
		APIMode:           in.APIMode,
		StreamMode:        in.StreamMode,
		InputTokens:       in.InputTokens,
	}
	if in.DurationSec > 0 {
		cfg.Duration = time.Duration(in.DurationSec) * time.Second
	}
	est := loadtest.EstimateInputTokens(cfg)
	if est > loadtestCostTokenWarn && !in.ConfirmCost {
		return nil, fmt.Errorf("estimated input tokens ~%d; set confirm_cost=true to run", est)
	}

	s.mu.Lock()
	if s.runningID != "" {
		s.mu.Unlock()
		return nil, fmt.Errorf("a load test is already running")
	}
	runCtx, cancel := context.WithCancel(context.Background())
	run := &channelLoadtestRun{
		id:          uuid.NewString(),
		status:      "running",
		source:      source,
		accountID:   accountID,
		accountName: accountName,
		proxyID:     proxyID,
		proxyName:   proxyName,
		baseURL:     baseURL,
		cfg:         cfg,
		cancel:      cancel,
		startedAt:   time.Now(),
	}
	s.runningID = run.id
	s.runs = append(s.runs, run)
	if len(s.runs) > loadtestMaxKeepRuns {
		s.runs = s.runs[len(s.runs)-loadtestMaxKeepRuns:]
	}
	s.mu.Unlock()

	go s.execute(runCtx, run)

	return s.snapshot(run, false), nil
}

func (s *ChannelLoadtestService) execute(ctx context.Context, run *channelLoadtestRun) {
	defer func() {
		s.mu.Lock()
		if s.runningID == run.id {
			s.runningID = ""
		}
		s.mu.Unlock()
	}()

	results, err := loadtest.Run(ctx, run.cfg, func(p loadtest.Progress) {
		s.mu.Lock()
		run.inflight = p.Inflight
		run.peak = p.Peak
		run.done = p.Done
		run.ok = p.OK
		if p.Result != nil {
			run.results = append(run.results, *p.Result)
		}
		s.mu.Unlock()
	})

	now := time.Now()
	s.mu.Lock()
	if len(results) > 0 {
		run.results = results
	}
	run.endedAt = &now
	run.inflight = 0
	if err != nil && ctx.Err() != nil {
		run.status = "stopped"
		run.err = "stopped"
	} else if err != nil {
		run.status = "failed"
		run.err = err.Error()
	} else {
		run.status = "done"
	}
	s.mu.Unlock()
}

func (s *ChannelLoadtestService) Stop(id string) (*ChannelLoadtestSnapshot, error) {
	s.mu.Lock()
	run := s.findLocked(id)
	if run == nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("run not found")
	}
	if run.status == "running" && run.cancel != nil {
		run.status = "stopping"
		run.cancel()
	}
	s.mu.Unlock()
	return s.snapshot(run, true), nil
}

func (s *ChannelLoadtestService) Get(id string) (*ChannelLoadtestSnapshot, error) {
	s.mu.Lock()
	run := s.findLocked(id)
	s.mu.Unlock()
	if run == nil {
		return nil, fmt.Errorf("run not found")
	}
	return s.snapshot(run, true), nil
}

func (s *ChannelLoadtestService) Latest() *ChannelLoadtestSnapshot {
	s.mu.Lock()
	if len(s.runs) == 0 {
		s.mu.Unlock()
		return nil
	}
	run := s.runs[len(s.runs)-1]
	s.mu.Unlock()
	return s.snapshot(run, true)
}

func (s *ChannelLoadtestService) findLocked(id string) *channelLoadtestRun {
	for i := len(s.runs) - 1; i >= 0; i-- {
		if s.runs[i].id == id {
			return s.runs[i]
		}
	}
	return nil
}

func (s *ChannelLoadtestService) snapshot(run *channelLoadtestRun, includeResults bool) *ChannelLoadtestSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := run.results
	done := run.done
	ok := run.ok
	if done == 0 && len(rows) > 0 {
		done = int64(len(rows))
	}
	rate := 0.0
	if done > 0 {
		rate = float64(ok) * 100 / float64(done)
	}
	out := &ChannelLoadtestSnapshot{
		ID:                 run.id,
		Status:             run.status,
		Source:             run.source,
		AccountID:          run.accountID,
		AccountName:        run.accountName,
		ProxyID:            run.proxyID,
		ProxyName:          run.proxyName,
		BaseURL:            run.baseURL,
		Path:               run.cfg.Path,
		APIMode:            run.cfg.APIMode,
		StreamMode:         run.cfg.StreamMode,
		Models:             append([]string(nil), run.cfg.Models...),
		Profile:            run.cfg.Profile,
		Concurrency:        run.cfg.Concurrency,
		Total:              run.cfg.Total,
		DurationSec:        int(run.cfg.Duration.Seconds()),
		StartedAt:          run.startedAt,
		EndedAt:            run.endedAt,
		Error:              run.err,
		Inflight:           run.inflight,
		Peak:               run.peak,
		Done:               done,
		OK:                 ok,
		SuccessRate:        rate,
		InputTokens:        run.cfg.InputTokens,
		EstimatedInputTok:  loadtest.EstimateInputTokens(run.cfg),
		DataProfile:        loadtest.BuildDataProfile(rows),
		SLA:                loadtest.PublicSLA(rows),
		SLAPass:            loadtest.SLAOK(rows),
		ModelMissingReqs:   loadtest.ModelMissingRequests(rows),
		StrictContractHits: loadtest.StrictContractHit(rows),
	}
	if includeResults && len(rows) > 0 {
		out.Results = append([]loadtest.Result(nil), rows...)
	}
	return out
}

func (s *ChannelLoadtestService) resolveTarget(ctx context.Context, in ChannelLoadtestStartInput) (baseURL, apiKey, source string, acc *Account, err error) {
	if in.AccountID != nil && *in.AccountID > 0 {
		acc, getErr := s.accountRepo.GetByID(ctx, *in.AccountID)
		if getErr != nil || acc == nil {
			return "", "", "", nil, fmt.Errorf("account not found")
		}
		baseURL = strings.TrimSpace(acc.GetOpenAIBaseURL())
		apiKey = strings.TrimSpace(acc.GetCredential("api_key"))
		if apiKey == "" {
			apiKey = strings.TrimSpace(acc.GetOpenAIApiKey())
		}
		if baseURL == "" || apiKey == "" {
			return "", "", "", nil, fmt.Errorf("account %d has no OpenAI-compatible base_url/api_key", acc.ID)
		}
		return baseURL, apiKey, "account", acc, nil
	}
	baseURL = strings.TrimSpace(in.BaseURL)
	apiKey = strings.TrimSpace(in.APIKey)
	if baseURL == "" || apiKey == "" {
		return "", "", "", nil, fmt.Errorf("provide account_id or base_url+api_key")
	}
	cleaned, valErr := urlvalidator.ValidateHTTPURL(baseURL, true, urlvalidator.ValidationOptions{AllowPrivate: true})
	if valErr != nil {
		return "", "", "", nil, fmt.Errorf("invalid base_url: %w", valErr)
	}
	return cleaned, apiKey, "manual", nil, nil
}

func (s *ChannelLoadtestService) resolveProxy(ctx context.Context, requested *int64, acc *Account) (proxyURL string, proxyID *int64, proxyName string, err error) {
	if requested != nil {
		if *requested <= 0 {
			return "", nil, "", nil
		}
		return s.lookupProxy(ctx, *requested)
	}
	if acc != nil && acc.ProxyID != nil && *acc.ProxyID > 0 {
		if acc.Proxy != nil {
			id := *acc.ProxyID
			return acc.Proxy.URL(), &id, acc.Proxy.Name, nil
		}
		return s.lookupProxy(ctx, *acc.ProxyID)
	}
	return "", nil, "", nil
}

func (s *ChannelLoadtestService) lookupProxy(ctx context.Context, id int64) (string, *int64, string, error) {
	if s.proxyRepo == nil {
		return "", nil, "", fmt.Errorf("proxy %d not found", id)
	}
	p, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil || p == nil {
		return "", nil, "", fmt.Errorf("proxy %d not found", id)
	}
	if !p.IsActive() {
		return "", nil, "", fmt.Errorf("proxy %d is not active", id)
	}
	pid := p.ID
	return p.URL(), &pid, p.Name, nil
}
