package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

type configFile struct {
	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		DBName   string `yaml:"dbname"`
		SSLMode  string `yaml:"sslmode"`
	} `yaml:"database"`
	Admin struct {
		Email    string `yaml:"email"`
		Password string `yaml:"password"`
	} `yaml:"admin"`
	Default struct {
		AdminEmail    string `yaml:"admin_email"`
		AdminPassword string `yaml:"admin_password"`
	} `yaml:"default"`
	JWT struct {
		Secret     string `yaml:"secret"`
		ExpireHour int    `yaml:"expire_hour"`
	} `yaml:"jwt"`
}

type report struct {
	StartedAt   string        `json:"started_at"`
	FinishedAt  string        `json:"finished_at"`
	BaseURL     string        `json:"base_url"`
	FrontendURL string        `json:"frontend_url"`
	Suites      []string      `json:"suites"`
	Checks      []checkResult `json:"checks"`
	Summary     summary       `json:"summary"`
}

type summary struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

type checkResult struct {
	Suite      string            `json:"suite"`
	Name       string            `json:"name"`
	Status     string            `json:"status"`
	DurationMS int64             `json:"duration_ms"`
	Method     string            `json:"method,omitempty"`
	URL        string            `json:"url,omitempty"`
	StatusCode int               `json:"status_code,omitempty"`
	Error      string            `json:"error,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
}

type fixtureUser struct {
	ID          int64
	Email       string
	Role        string
	Concurrency int
}

type fixtureAPIKey struct {
	ID      int64
	Key     string
	UserID  int64
	GroupID sql.NullInt64
}

type smokeRunner struct {
	baseURL      string
	frontendURL  string
	cfg          configFile
	db           *sql.DB
	client       *http.Client
	report       report
	adminToken   string
	userToken    string
	apiKey       *fixtureAPIKey
	openAIKey    *fixtureAPIKey
	imageKey     *fixtureAPIKey
	embeddingKey *fixtureAPIKey
	bridgeKey    *fixtureAPIKey
	bridgeModel  string
	adminUser    *fixtureUser
	normalUser   *fixtureUser
}

func main() {
	var suiteArg string
	flag.StringVar(&suiteArg, "suite", "quick", "comma-separated smoke suites: quick,custom,openai,images,bridge,embeddings,all")
	flag.Parse()

	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "smoke: %v\n", err)
		os.Exit(2)
	}
	loadLocalEnv(filepath.Join(root, "tmp", "smoke", "local.env"))
	cfg, err := readConfig(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "smoke: %v\n", err)
		os.Exit(2)
	}

	baseURL := strings.TrimRight(firstNonEmpty(os.Getenv("SUB2API_BASE_URL"), "http://127.0.0.1:18081"), "/")
	frontendURL := strings.TrimRight(firstNonEmpty(os.Getenv("SUB2API_FRONTEND_URL"), "http://127.0.0.1:15174"), "/")

	db, err := openDB(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "smoke: %v\n", err)
		os.Exit(2)
	}
	defer db.Close()

	suites := expandSuites(suiteArg)
	r := &smokeRunner{
		baseURL:     baseURL,
		frontendURL: frontendURL,
		cfg:         cfg,
		db:          db,
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
		report: report{
			StartedAt:   time.Now().UTC().Format(time.RFC3339Nano),
			BaseURL:     baseURL,
			FrontendURL: frontendURL,
			Suites:      suites,
		},
	}

	ctx := context.Background()
	r.loadFixtures(ctx)
	for _, suite := range suites {
		switch suite {
		case "quick":
			r.runQuick(ctx)
		case "custom":
			r.runCustom(ctx)
		case "openai":
			r.runOpenAI(ctx)
		case "images":
			r.runImages(ctx)
		case "bridge":
			r.runBridge(ctx)
		case "embeddings":
			r.runEmbeddings(ctx)
		default:
			r.add("unknown", suite, "failed", 0, "", "", 0, fmt.Errorf("unknown suite %q", suite), nil)
		}
	}
	r.finish()

	reportPath, writeErr := r.writeReport(root)
	if writeErr != nil {
		fmt.Fprintf(os.Stderr, "smoke: write report: %v\n", writeErr)
	}

	encoded, _ := json.MarshalIndent(r.report.Summary, "", "  ")
	fmt.Printf("smoke report: %s\nsummary: %s\n", reportPath, string(encoded))
	if r.report.Summary.Failed > 0 {
		os.Exit(1)
	}
}

func (r *smokeRunner) runQuick(ctx context.Context) {
	r.get(ctx, "quick", "backend health", r.baseURL+"/health", nil, expectStatus(200))
	r.get(ctx, "quick", "public settings", r.baseURL+"/api/v1/settings/public", nil, expectEnvelopeOK())

	if r.adminUser == nil {
		r.add("quick", "admin fixture", "failed", 0, "", "", 0, errors.New("no active admin user found in database"), nil)
		return
	}

	if token, ok := r.tryLogin(ctx, "quick", "admin login"); ok {
		r.adminToken = token
	}

	if r.adminToken != "" {
		r.get(ctx, "quick", "auth me", r.baseURL+"/api/v1/auth/me", bearer(r.adminToken), expectEnvelopeOK())
	}
}

func (r *smokeRunner) runCustom(ctx context.Context) {
	if r.adminToken == "" {
		if token, ok := r.tryLogin(ctx, "custom", "admin login"); ok {
			r.adminToken = token
		}
	}
	if r.userToken == "" {
		r.userToken = r.adminToken
	}

	r.get(ctx, "custom", "frontend key-usage page", r.frontendURL+"/key-usage", nil, expectStatus(200))

	if r.apiKey == nil {
		r.add("custom", "api key fixture", "failed", 0, "", "", 0, errors.New("no active API key found in database"), nil)
	} else {
		headers := apiKeyHeaders(r.apiKey.Key)
		r.get(ctx, "custom", "public usage summary", r.baseURL+"/v1/usage", headers, expectStatus(200))
		r.get(ctx, "custom", "public usage records", r.baseURL+"/v1/usage/records?page=1&page_size=1", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "public usage stats", r.baseURL+"/v1/usage/stats", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "public usage trend", r.baseURL+"/v1/usage/trend", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "gateway models list", r.baseURL+"/v1/models", headers, expectModelsList())
	}

	if r.userToken == "" {
		r.add("custom", "user jwt fixture", "failed", 0, "", "", 0, errors.New("no active user token available"), nil)
	} else {
		headers := bearer(r.userToken)
		r.get(ctx, "custom", "user distribution summary", r.baseURL+"/api/v1/distribution", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "user distribution ledger", r.baseURL+"/api/v1/distribution/ledger?page=1&page_size=1", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "user distribution assets", r.baseURL+"/api/v1/distribution/assets?page=1&page_size=1", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "user distribution api-key groups", r.baseURL+"/api/v1/distribution/api-key-groups", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "user pricing page", r.baseURL+"/api/v1/user/pricing-page", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "user announcements", r.baseURL+"/api/v1/announcements", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "user usage page api", r.baseURL+"/api/v1/usage?page=1&page_size=1", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "user usage error requests", r.baseURL+"/api/v1/usage/errors?page=1&page_size=1", headers, expectEnvelopeOK())
	}

	if r.adminToken == "" {
		r.add("custom", "admin jwt fixture", "failed", 0, "", "", 0, errors.New("no active admin token available"), nil)
	} else {
		headers := bearer(r.adminToken)
		r.get(ctx, "custom", "admin distribution settings", r.baseURL+"/api/v1/admin/distribution/settings", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "admin distribution applications", r.baseURL+"/api/v1/admin/distribution/applications?page=1&page_size=1", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "admin distribution wallets", r.baseURL+"/api/v1/admin/distribution/wallets?page=1&page_size=1", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "admin distribution ledger", r.baseURL+"/api/v1/admin/distribution/ledger?page=1&page_size=1", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "admin model pricing", r.baseURL+"/api/v1/admin/model-pricing?page=1&page_size=1", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "admin announcements", r.baseURL+"/api/v1/admin/announcements?page=1&page_size=1", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "admin usage", r.baseURL+"/api/v1/admin/usage?page=1&page_size=1", headers, expectEnvelopeOK())
		r.get(ctx, "custom", "admin settings", r.baseURL+"/api/v1/admin/settings", headers, expectEnvelopeOK())
		if r.apiKey != nil && r.apiKey.GroupID.Valid {
			r.get(ctx, "custom", "admin group models-list candidates", fmt.Sprintf("%s/api/v1/admin/groups/%d/models-list-candidates", r.baseURL, r.apiKey.GroupID.Int64), headers, expectModelsListCandidates())
		}
	}
}

func (r *smokeRunner) runOpenAI(ctx context.Context) {
	key := firstFixtureKey(r.openAIKey)
	if key == nil {
		r.add("openai", "openai fixture", "failed", 0, "", "", 0, errors.New("no OpenAI chat API key fixture found"), map[string]string{
			"required": "active downstream API key bound to a group with an active OpenAI upstream account",
		})
		return
	}
	body := map[string]any{
		"model":  "gpt-5.5",
		"input":  "Sub2API smoke test. Reply with ok.",
		"stream": false,
	}
	r.postJSON(ctx, "openai", "responses non-stream", r.baseURL+"/v1/responses", apiKeyHeaders(key.Key), body, expectStatus(200))
	chat := map[string]any{
		"model": "gpt-5.5",
		"messages": []map[string]string{
			{"role": "user", "content": "Sub2API smoke test. Reply with ok."},
		},
		"stream": false,
	}
	r.postJSON(ctx, "openai", "chat completions non-stream", r.baseURL+"/v1/chat/completions", apiKeyHeaders(key.Key), chat, expectStatus(200))
}

func (r *smokeRunner) runImages(ctx context.Context) {
	key := firstFixtureKey(r.imageKey)
	if key == nil {
		r.add("images", "image fixture", "failed", 0, "", "", 0, errors.New("no image-capable OpenAI API key fixture found"), map[string]string{
			"required": "active downstream API key bound to a group with allow_image_generation=true and an active OpenAI OAuth or API Key upstream account",
		})
		return
	}
	invalid := map[string]any{
		"model":  "gpt-image-2",
		"prompt": "Sub2API smoke test",
		"size":   "not-a-size",
	}
	r.postJSON(ctx, "images", "invalid size passthrough", r.baseURL+"/v1/images/generations", apiKeyHeaders(key.Key), invalid, expectStatus(400))
}

func (r *smokeRunner) runBridge(ctx context.Context) {
	key := firstFixtureKey(r.bridgeKey)
	if key == nil {
		r.add("bridge", "bridge fixture", "failed", 0, "", "", 0, errors.New("no Antigravity API key fixture found"), nil)
		return
	}
	model := strings.TrimSpace(r.bridgeModel)
	if model == "" {
		r.add("bridge", "bridge model fixture", "failed", 0, "", "", 0, errors.New("no Claude-GPT bridge model mapping found for Antigravity fixture group"), map[string]string{
			"required": "Antigravity downstream API key group bound to an active OpenAI bridge account with a claude-* => gpt-* model_mapping entry",
		})
		return
	}
	body := map[string]any{
		"model":      model,
		"max_tokens": 16,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": "Sub2API bridge smoke test. Reply ok."},
				},
			},
		},
	}
	r.postJSON(ctx, "bridge", "antigravity messages bridge/native", r.baseURL+"/antigravity/v1/messages", apiKeyHeaders(key.Key), body, expectStatusRange(200, 499))
}

func (r *smokeRunner) runEmbeddings(ctx context.Context) {
	key := firstFixtureKey(r.embeddingKey)
	if key == nil {
		r.add("embeddings", "embedding fixture", "failed", 0, "", "", 0, errors.New("no embeddings-capable OpenAI API key fixture found"), map[string]string{
			"required": "active downstream API key bound to a group with an active OpenAI API Key upstream account whose openai_capabilities include embeddings or are unset",
		})
		return
	}
	body := map[string]any{
		"model": "text-embedding-3-small",
		"input": "Sub2API smoke test",
	}
	r.postJSON(ctx, "embeddings", "openai embeddings", r.baseURL+"/v1/embeddings", apiKeyHeaders(key.Key), body, expectStatus(200))
}

func (r *smokeRunner) loadFixtures(ctx context.Context) {
	if err := r.ensureSmokeAdmin(ctx); err != nil {
		r.add("fixture", "ensure smoke admin", "failed", 0, "", "", 0, err, nil)
	}
	if err := r.ensureUsageErrorRequestsEnabled(ctx); err != nil {
		r.add("fixture", "enable user error requests", "failed", 0, "", "", 0, err, nil)
	}
	smokeAdminEmail := firstNonEmpty(os.Getenv("SUB2API_SMOKE_ADMIN_EMAIL"), "smoke-admin@sub2api.local")
	r.adminUser, _ = queryUser(ctx, r.db, "email = "+sqlQuote(smokeAdminEmail)+" AND role = 'admin'")
	if r.adminUser == nil {
		r.adminUser, _ = queryUser(ctx, r.db, "role = 'admin'")
	}
	if r.adminUser != nil {
		if err := r.ensureDistributionFixture(ctx, r.adminUser.ID); err != nil {
			r.add("fixture", "ensure distribution fixture", "failed", 0, "", "", 0, err, nil)
		}
	}
	r.normalUser, _ = queryUser(ctx, r.db, "role <> 'admin'")
	if r.normalUser == nil {
		r.normalUser = r.adminUser
	}
	r.apiKey, _ = queryAPIKey(ctx, r.db)
	r.openAIKey, _ = queryOpenAIChatAPIKey(ctx, r.db, os.Getenv("SUB2API_SMOKE_OPENAI_API_KEY"))
	r.imageKey, _ = queryOpenAIImageAPIKey(ctx, r.db, firstNonEmpty(os.Getenv("SUB2API_SMOKE_OPENAI_IMAGES_API_KEY"), os.Getenv("SUB2API_SMOKE_OPENAI_API_KEY")))
	r.embeddingKey, _ = queryOpenAIEmbeddingAPIKey(ctx, r.db, firstNonEmpty(os.Getenv("SUB2API_SMOKE_OPENAI_EMBEDDINGS_API_KEY"), os.Getenv("SUB2API_SMOKE_OPENAI_API_KEY")))
	r.bridgeKey, _ = queryAPIKeyByRawKey(ctx, r.db, os.Getenv("SUB2API_SMOKE_ANTIGRAVITY_API_KEY"))
	r.bridgeModel, _ = queryBridgeModel(ctx, r.db, r.bridgeKey)
}

func (r *smokeRunner) tryLogin(ctx context.Context, suite, name string) (string, bool) {
	email := firstNonEmpty(os.Getenv("SUB2API_SMOKE_ADMIN_EMAIL"), os.Getenv("SUB2API_ADMIN_EMAIL"), "smoke-admin@sub2api.local", r.cfg.Admin.Email, r.cfg.Default.AdminEmail)
	password := firstNonEmpty(os.Getenv("SUB2API_SMOKE_ADMIN_PASSWORD"), os.Getenv("SUB2API_ADMIN_PASSWORD"), "smoke-admin-123456", r.cfg.Admin.Password, r.cfg.Default.AdminPassword)
	if email == "" || password == "" {
		r.add(suite, name, "failed", 0, "POST", r.baseURL+"/api/v1/auth/login", 0, errors.New("admin email/password not configured"), nil)
		return "", false
	}
	payload := map[string]string{
		"email":    email,
		"password": password,
	}
	start := time.Now()
	resp, body, err := r.request(ctx, http.MethodPost, r.baseURL+"/api/v1/auth/login", nil, payload)
	duration := time.Since(start).Milliseconds()
	if err != nil {
		r.add(suite, name, "failed", duration, http.MethodPost, r.baseURL+"/api/v1/auth/login", 0, err, nil)
		return "", false
	}
	details := map[string]string{"email": email}
	if resp.StatusCode != http.StatusOK {
		details["body"] = truncate(string(body), 500)
		r.add(suite, name, "failed", duration, http.MethodPost, r.baseURL+"/api/v1/auth/login", resp.StatusCode, fmt.Errorf("expected 200"), details)
		return "", false
	}
	data := envelopeData(body)
	token, _ := data["access_token"].(string)
	if token == "" {
		details["body"] = truncate(string(body), 500)
		r.add(suite, name, "failed", duration, http.MethodPost, r.baseURL+"/api/v1/auth/login", resp.StatusCode, fmt.Errorf("missing access_token"), details)
		return "", false
	}
	r.add(suite, name, "passed", duration, http.MethodPost, r.baseURL+"/api/v1/auth/login", resp.StatusCode, nil, details)
	return token, true
}

func (r *smokeRunner) ensureSmokeAdmin(ctx context.Context) error {
	email := firstNonEmpty(os.Getenv("SUB2API_SMOKE_ADMIN_EMAIL"), "smoke-admin@sub2api.local")
	password := firstNonEmpty(os.Getenv("SUB2API_SMOKE_ADMIN_PASSWORD"), "smoke-admin-123456")
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	updateQuery := `
UPDATE users
SET
  password_hash = $2,
  role = 'admin',
  status = 'active',
  concurrency = GREATEST(concurrency, 20),
  balance = GREATEST(balance, 100000),
  updated_at = now()
WHERE email = $1 AND deleted_at IS NULL`
	result, err := r.db.ExecContext(ctx, updateQuery, email, string(hash))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected > 0 {
		return nil
	}

	insertQuery := `
INSERT INTO users (
  email,
  password_hash,
  role,
  balance,
  concurrency,
  status,
  username,
  notes,
  created_at,
  updated_at
)
VALUES ($1, $2, 'admin', 100000, 20, 'active', 'Smoke Admin', 'created by backend/tools/smoke', now(), now())
ON CONFLICT (email) WHERE deleted_at IS NULL DO NOTHING`
	result, err = r.db.ExecContext(ctx, insertQuery, email, string(hash))
	if err != nil {
		return err
	}
	affected, err = result.RowsAffected()
	if err == nil && affected > 0 {
		return nil
	}
	_, err = r.db.ExecContext(ctx, updateQuery, email, string(hash))
	return err
}

func (r *smokeRunner) ensureUsageErrorRequestsEnabled(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO settings (key, value, updated_at)
VALUES ('allow_user_view_error_requests', 'true', now())
ON CONFLICT (key)
DO UPDATE SET value = EXCLUDED.value, updated_at = now()`)
	return err
}

func (r *smokeRunner) ensureDistributionFixture(ctx context.Context, userID int64) error {
	agentQuery := `
INSERT INTO distribution_agents (
  user_id,
  status,
  contact,
  reason,
  admin_note,
  reviewed_by,
  reviewed_at,
  created_at,
  updated_at
)
VALUES ($1, 'approved', 'backend/tools/smoke', 'smoke fixture', 'auto-approved smoke fixture', $1, now(), now(), now())
ON CONFLICT (user_id)
DO UPDATE SET
  status = 'approved',
  admin_note = 'auto-approved smoke fixture',
  reviewed_by = $1,
  reviewed_at = COALESCE(distribution_agents.reviewed_at, now()),
  updated_at = now()
RETURNING id`
	var agentID int64
	if err := r.db.QueryRowContext(ctx, agentQuery, userID).Scan(&agentID); err != nil {
		return err
	}

	walletQuery := `
INSERT INTO distribution_wallets (
  user_id,
  agent_id,
  balance,
  total_recharged,
  total_spent,
  total_rebate,
  status,
  created_at,
  updated_at
)
VALUES ($1, $2, 100000, 100000, 0, 0, 'active', now(), now())
ON CONFLICT (user_id)
DO UPDATE SET
  agent_id = EXCLUDED.agent_id,
  balance = GREATEST(distribution_wallets.balance, 100000),
  total_recharged = GREATEST(distribution_wallets.total_recharged, 100000),
  status = 'active',
  updated_at = now()`
	_, err := r.db.ExecContext(ctx, walletQuery, userID, agentID)
	return err
}

func (r *smokeRunner) get(ctx context.Context, suite, name, rawURL string, headers map[string]string, expect expectation) {
	r.do(ctx, suite, name, http.MethodGet, rawURL, headers, nil, expect)
}

func (r *smokeRunner) postJSON(ctx context.Context, suite, name, rawURL string, headers map[string]string, payload any, expect expectation) {
	r.do(ctx, suite, name, http.MethodPost, rawURL, headers, payload, expect)
}

func (r *smokeRunner) do(ctx context.Context, suite, name, method, rawURL string, headers map[string]string, payload any, expect expectation) {
	start := time.Now()
	resp, body, err := r.request(ctx, method, rawURL, headers, payload)
	duration := time.Since(start).Milliseconds()
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	if err == nil {
		err = expect(resp, body)
	}
	status := "passed"
	details := map[string]string{}
	if err != nil {
		status = "failed"
		if len(body) > 0 {
			details["body"] = truncate(string(body), 500)
		}
	}
	r.add(suite, name, status, duration, method, rawURL, statusCode, err, details)
}

func (r *smokeRunner) request(ctx context.Context, method, rawURL string, headers map[string]string, payload any) (*http.Response, []byte, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return resp, data, err
}

func (r *smokeRunner) add(suite, name, status string, duration int64, method, rawURL string, statusCode int, err error, details map[string]string) {
	if details != nil && len(details) == 0 {
		details = nil
	}
	result := checkResult{
		Suite:      suite,
		Name:       name,
		Status:     status,
		DurationMS: duration,
		Method:     method,
		URL:        redactURL(rawURL),
		StatusCode: statusCode,
		Details:    details,
	}
	if err != nil {
		result.Error = err.Error()
	}
	r.report.Checks = append(r.report.Checks, result)
}

func (r *smokeRunner) finish() {
	r.report.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	for _, check := range r.report.Checks {
		switch check.Status {
		case "passed":
			r.report.Summary.Passed++
		case "skipped":
			r.report.Summary.Skipped++
		default:
			r.report.Summary.Failed++
		}
	}
}

func (r *smokeRunner) writeReport(root string) (string, error) {
	dir := filepath.Join(root, "tmp", "smoke")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, time.Now().Format("20060102-150405")+".json")
	data, err := json.MarshalIndent(r.report, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

type expectation func(resp *http.Response, body []byte) error

func expectStatus(want int) expectation {
	return func(resp *http.Response, _ []byte) error {
		if resp.StatusCode != want {
			return fmt.Errorf("expected HTTP %d, got %d", want, resp.StatusCode)
		}
		return nil
	}
}

func expectStatusRange(min, max int) expectation {
	return func(resp *http.Response, _ []byte) error {
		if resp.StatusCode < min || resp.StatusCode > max {
			return fmt.Errorf("expected HTTP %d-%d, got %d", min, max, resp.StatusCode)
		}
		return nil
	}
}

func expectEnvelopeOK() expectation {
	return func(resp *http.Response, body []byte) error {
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("expected HTTP 200, got %d", resp.StatusCode)
		}
		var env struct {
			Code    int             `json:"code"`
			Message string          `json:"message"`
			Data    json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			return fmt.Errorf("invalid JSON envelope: %w", err)
		}
		if env.Code != 0 {
			return fmt.Errorf("expected envelope code 0, got %d (%s)", env.Code, env.Message)
		}
		return nil
	}
}

func expectModelsList() expectation {
	return func(resp *http.Response, body []byte) error {
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("expected HTTP 200, got %d", resp.StatusCode)
		}
		var payload struct {
			Object string            `json:"object"`
			Data   []json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("invalid models list JSON: %w", err)
		}
		if payload.Object != "list" {
			return fmt.Errorf("expected object=list, got %q", payload.Object)
		}
		if payload.Data == nil {
			return errors.New("models list data is missing")
		}
		return nil
	}
}

func expectModelsListCandidates() expectation {
	return func(resp *http.Response, body []byte) error {
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("expected HTTP 200, got %d", resp.StatusCode)
		}
		var env struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				Models []string `json:"models"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &env); err != nil {
			return fmt.Errorf("invalid JSON envelope: %w", err)
		}
		if env.Code != 0 {
			return fmt.Errorf("expected envelope code 0, got %d (%s)", env.Code, env.Message)
		}
		if env.Data.Models == nil {
			return errors.New("models candidates are missing")
		}
		return nil
	}
}

func envelopeData(body []byte) map[string]any {
	var env struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal(body, &env)
	if env.Data == nil {
		return map[string]any{}
	}
	return env.Data
}

func queryUser(ctx context.Context, db *sql.DB, predicate string) (*fixtureUser, error) {
	query := `
SELECT id, email, role, concurrency
FROM users
WHERE status = 'active' AND deleted_at IS NULL AND ` + predicate + `
ORDER BY id ASC
LIMIT 1`
	row := db.QueryRowContext(ctx, query)
	var user fixtureUser
	if err := row.Scan(&user.ID, &user.Email, &user.Role, &user.Concurrency); err != nil {
		return nil, err
	}
	return &user, nil
}

func queryAPIKey(ctx context.Context, db *sql.DB) (*fixtureAPIKey, error) {
	query := `
SELECT ak.id, ak.key, ak.user_id, ak.group_id
FROM api_keys ak
JOIN users u ON u.id = ak.user_id
LEFT JOIN groups g ON g.id = ak.group_id
WHERE ak.status = 'active'
  AND ak.deleted_at IS NULL
  AND u.status = 'active'
  AND u.deleted_at IS NULL
  AND (ak.expires_at IS NULL OR ak.expires_at > now())
  AND (ak.quota <= 0 OR ak.quota_used < ak.quota)
  AND (ak.group_id IS NULL OR g.deleted_at IS NULL)
ORDER BY ak.last_used_at DESC NULLS LAST, ak.id ASC
LIMIT 1`
	row := db.QueryRowContext(ctx, query)
	var key fixtureAPIKey
	if err := row.Scan(&key.ID, &key.Key, &key.UserID, &key.GroupID); err != nil {
		return nil, err
	}
	return &key, nil
}

func queryAPIKeyByRawKey(ctx context.Context, db *sql.DB, raw string) (*fixtureAPIKey, error) {
	raw = normalizeRawAPIKey(raw)
	if raw == "" {
		return nil, nil
	}
	query := `
SELECT ak.id, ak.key, ak.user_id, ak.group_id
FROM api_keys ak
JOIN users u ON u.id = ak.user_id
LEFT JOIN groups g ON g.id = ak.group_id
WHERE ak.key = $1
  AND ak.status = 'active'
  AND ak.deleted_at IS NULL
  AND u.status = 'active'
  AND u.deleted_at IS NULL
  AND (ak.expires_at IS NULL OR ak.expires_at > now())
  AND (ak.quota <= 0 OR ak.quota_used < ak.quota)
  AND (ak.group_id IS NULL OR g.deleted_at IS NULL)
LIMIT 1`
	row := db.QueryRowContext(ctx, query, raw)
	var key fixtureAPIKey
	if err := row.Scan(&key.ID, &key.Key, &key.UserID, &key.GroupID); err != nil {
		return nil, err
	}
	return &key, nil
}

func queryOpenAIChatAPIKey(ctx context.Context, db *sql.DB, raw string) (*fixtureAPIKey, error) {
	return queryOpenAIFixtureAPIKey(ctx, db, raw, `
  AND a.type IN ('oauth', 'apikey')
  AND `+openAIEndpointCapabilitySQL("chat_completions"))
}

func queryOpenAIImageAPIKey(ctx context.Context, db *sql.DB, raw string) (*fixtureAPIKey, error) {
	return queryOpenAIFixtureAPIKey(ctx, db, raw, `
  AND g.allow_image_generation = TRUE
  AND a.type IN ('oauth', 'apikey')
  AND `+openAIImagesEndpointEnabledSQL())
}

func queryOpenAIEmbeddingAPIKey(ctx context.Context, db *sql.DB, raw string) (*fixtureAPIKey, error) {
	return queryOpenAIFixtureAPIKey(ctx, db, raw, `
  AND a.type = 'apikey'
  AND `+openAIEndpointCapabilitySQL("embeddings"))
}

func queryBridgeModel(ctx context.Context, db *sql.DB, key *fixtureAPIKey) (string, error) {
	if key == nil || !key.GroupID.Valid {
		return "", nil
	}
	query := `
SELECT mapping.key
FROM account_groups ag
JOIN accounts a ON a.id = ag.account_id
CROSS JOIN LATERAL jsonb_each_text(coalesce(a.credentials->'model_mapping', '{}'::jsonb)) AS mapping(key, value)
WHERE ag.group_id = $1
  AND a.platform = 'openai'
  AND a.status = 'active'
  AND a.schedulable = TRUE
  AND a.deleted_at IS NULL
  AND (a.auto_pause_on_expired = FALSE OR a.expires_at IS NULL OR a.expires_at > now())
  AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at <= now())
  AND (a.overload_until IS NULL OR a.overload_until <= now())
  AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= now())
  AND jsonb_typeof(a.extra->'openai_claude_gpt_bridge_enabled') = 'boolean'
  AND a.extra->>'openai_claude_gpt_bridge_enabled' = 'true'
  AND lower(mapping.key) LIKE 'claude-%'
  AND lower(mapping.value) LIKE 'gpt-%'
  AND mapping.key <> mapping.value
ORDER BY
  CASE
    WHEN lower(mapping.key) LIKE '%sonnet%' THEN 0
    WHEN lower(mapping.key) LIKE '%haiku%' THEN 1
    ELSE 2
  END,
  mapping.key
LIMIT 1`
	var model string
	if err := db.QueryRowContext(ctx, query, key.GroupID.Int64).Scan(&model); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return model, nil
}

func queryOpenAIFixtureAPIKey(ctx context.Context, db *sql.DB, raw string, accountWhere string) (*fixtureAPIKey, error) {
	raw = normalizeRawAPIKey(raw)
	if raw != "" {
		key, err := queryOpenAIFixtureAPIKeyOnce(ctx, db, raw, accountWhere)
		if err == nil || !errors.Is(err, sql.ErrNoRows) {
			return key, err
		}
	}
	key, err := queryOpenAIFixtureAPIKeyOnce(ctx, db, "", accountWhere)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return key, err
}

func queryOpenAIFixtureAPIKeyOnce(ctx context.Context, db *sql.DB, raw string, accountWhere string) (*fixtureAPIKey, error) {
	query := `
SELECT DISTINCT ak.id, ak.key, ak.user_id, ak.group_id
FROM api_keys ak
JOIN users u ON u.id = ak.user_id
JOIN groups g ON g.id = ak.group_id
JOIN account_groups ag ON ag.group_id = g.id
JOIN accounts a ON a.id = ag.account_id
WHERE ak.status = 'active'
  AND ak.deleted_at IS NULL
  AND u.status = 'active'
  AND u.deleted_at IS NULL
  AND g.status = 'active'
  AND g.deleted_at IS NULL
  AND (ak.expires_at IS NULL OR ak.expires_at > now())
  AND (ak.quota <= 0 OR ak.quota_used < ak.quota)
  AND a.platform = 'openai'
  AND a.status = 'active'
  AND a.schedulable = TRUE
  AND a.deleted_at IS NULL
  AND (a.auto_pause_on_expired = FALSE OR a.expires_at IS NULL OR a.expires_at > now())
  AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at <= now())
  AND (a.overload_until IS NULL OR a.overload_until <= now())
  AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= now())
` + accountWhere
	args := []any{}
	if raw != "" {
		query += `
  AND ak.key = $1`
		args = append(args, raw)
	}
	query += `
ORDER BY ak.id ASC
LIMIT 1`
	row := db.QueryRowContext(ctx, query, args...)
	var key fixtureAPIKey
	if err := row.Scan(&key.ID, &key.Key, &key.UserID, &key.GroupID); err != nil {
		return nil, err
	}
	return &key, nil
}

func openAIEndpointCapabilitySQL(capability string) string {
	escaped := strings.ReplaceAll(capability, "'", "''")
	arrayJSON, _ := json.Marshal([]string{capability})
	return `(
    NOT (a.credentials ? 'openai_capabilities')
    OR a.credentials->'openai_capabilities' IS NULL
    OR (
      jsonb_typeof(a.credentials->'openai_capabilities') = 'array'
      AND a.credentials->'openai_capabilities' @> '` + string(arrayJSON) + `'::jsonb
    )
    OR (
      jsonb_typeof(a.credentials->'openai_capabilities') = 'object'
      AND a.credentials->'openai_capabilities'->>'` + escaped + `' = 'true'
    )
  )`
}

func openAIImagesEndpointEnabledSQL() string {
	return `(
    CASE
      WHEN jsonb_typeof(a.extra->'openai_images_endpoint_enabled') = 'boolean'
        THEN a.extra->>'openai_images_endpoint_enabled' <> 'false'
      WHEN jsonb_typeof(a.extra->'openai') = 'object'
        AND jsonb_typeof(a.extra->'openai'->'openai_images_endpoint_enabled') = 'boolean'
        THEN a.extra->'openai'->>'openai_images_endpoint_enabled' <> 'false'
      ELSE TRUE
    END
  )`
}

func normalizeRawAPIKey(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "Bearer ")
	raw = strings.TrimPrefix(raw, "bearer ")
	return strings.TrimSpace(raw)
}

func firstFixtureKey(keys ...*fixtureAPIKey) *fixtureAPIKey {
	for _, key := range keys {
		if key != nil && strings.TrimSpace(key.Key) != "" {
			return key
		}
	}
	return nil
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, ".git")); err == nil {
			return wd, nil
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", fmt.Errorf("could not find repository root")
		}
		wd = parent
	}
}

func readConfig(root string) (configFile, error) {
	var cfg configFile
	candidates := []string{}
	if explicit := strings.TrimSpace(os.Getenv("SUB2API_CONFIG")); explicit != "" {
		candidates = append(candidates, explicit)
	}
	candidates = append(candidates,
		filepath.Join(root, "backend", "config.yaml"),
		filepath.Join(root, "config.yaml"),
		"config.yaml",
	)
	var lastErr error
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = err
			continue
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
		return cfg, nil
	}
	return cfg, fmt.Errorf("read config: %w", lastErr)
}

func loadLocalEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		if len(value) >= 2 {
			quote := value[0]
			if (quote == '\'' || quote == '"') && value[len(value)-1] == quote {
				value = value[1 : len(value)-1]
			}
		}
		_ = os.Setenv(key, value)
	}
}

func openDB(cfg configFile) (*sql.DB, error) {
	host := firstNonEmpty(os.Getenv("SUB2API_DB_HOST"), cfg.Database.Host, "127.0.0.1")
	port := cfg.Database.Port
	if port == 0 {
		port = 5432
	}
	user := firstNonEmpty(os.Getenv("SUB2API_DB_USER"), cfg.Database.User)
	password := firstNonEmpty(os.Getenv("SUB2API_DB_PASSWORD"), cfg.Database.Password)
	dbname := firstNonEmpty(os.Getenv("SUB2API_DB_NAME"), cfg.Database.DBName)
	sslmode := firstNonEmpty(os.Getenv("SUB2API_DB_SSLMODE"), cfg.Database.SSLMode, "disable")
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, dbname, sslmode)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func expandSuites(raw string) []string {
	seen := map[string]bool{}
	var suites []string
	for _, part := range strings.Split(raw, ",") {
		suite := strings.ToLower(strings.TrimSpace(part))
		if suite == "" {
			continue
		}
		if suite == "all" {
			for _, s := range []string{"quick", "custom", "openai", "bridge", "images", "embeddings"} {
				if !seen[s] {
					seen[s] = true
					suites = append(suites, s)
				}
			}
			continue
		}
		if !seen[suite] {
			seen[suite] = true
			suites = append(suites, suite)
		}
	}
	sort.Strings(suites)
	if len(suites) == 0 {
		return []string{"quick"}
	}
	return suites
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func apiKeyHeaders(key string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + key}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func truncate(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen] + "..."
}

func redactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	for _, key := range []string{"key", "api_key", "token"} {
		if q.Has(key) {
			q.Set(key, "redacted")
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func sqlQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
