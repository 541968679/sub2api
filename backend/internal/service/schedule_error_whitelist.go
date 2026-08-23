package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	ScheduleErrorFamilyClientInvalidRequest = "client_invalid_request"
	ScheduleErrorFamilyClientWrapped400URF  = "client_wrapped_400_urf"
	ScheduleErrorFamilyClientContextTooLong = "client_context_too_long"
	ScheduleErrorFamilyPairConcurrency      = "pair_concurrency"
	ScheduleErrorFamilyGroupNoAccount       = "group_no_account"
	ScheduleErrorFamilyRoutingModelMiss     = "routing_model_miss"
	ScheduleErrorFamilyRoutingPoolEmpty     = "routing_pool_empty"
	ScheduleErrorFamilyProtocolMismatch     = "protocol_mismatch"

	ScheduleErrorFromErrorStructured = "structured"
	ScheduleErrorFromErrorMessage    = "message"

	MaxScheduleErrorCustomRules     = 50
	MaxScheduleErrorMessageContains = 200
	scheduleErrorWhitelistCacheTTL  = 3 * time.Second
)

// ScheduleErrorFamilyIDs is the admin-toggleable preset set.
// Custom rules live in ScheduleErrorWhitelist.Custom as literal fields
// (no free SQL / % _ wildcards). Legacy routing miss is hardcoded and
// is not in this list; empty config therefore matches pre-feature production.
var ScheduleErrorFamilyIDs = []string{
	ScheduleErrorFamilyClientInvalidRequest,
	ScheduleErrorFamilyClientWrapped400URF,
	ScheduleErrorFamilyClientContextTooLong,
	ScheduleErrorFamilyPairConcurrency,
	ScheduleErrorFamilyGroupNoAccount,
	ScheduleErrorFamilyRoutingPoolEmpty,
	ScheduleErrorFamilyProtocolMismatch,
}

// defaultScheduleErrorFamilyEnabled is the factory preset: all new families
// off. Missing key / {} / families:{} / all-false = no new excludes.
var defaultScheduleErrorFamilyEnabled = map[string]bool{
	ScheduleErrorFamilyClientInvalidRequest: false,
	ScheduleErrorFamilyClientWrapped400URF:  false,
	ScheduleErrorFamilyClientContextTooLong: false,
	ScheduleErrorFamilyPairConcurrency:      false,
	ScheduleErrorFamilyGroupNoAccount:       false,
	ScheduleErrorFamilyRoutingPoolEmpty:     false,
	ScheduleErrorFamilyProtocolMismatch:     false,
}

// ScheduleErrorCustomRule is one admin-defined exclude.
// Empty fields do not constrain. Enabled rules AND the filled fields.
type ScheduleErrorCustomRule struct {
	ID                string `json:"id"`
	Enabled           bool   `json:"enabled"`
	ErrorType         string `json:"error_type,omitempty"`
	Phase             string `json:"phase,omitempty"`
	StatusCode        int    `json:"status_code,omitempty"`
	ProviderErrorCode string `json:"provider_error_code,omitempty"`
	MessageContains   string `json:"message_contains,omitempty"`
}

// ScheduleErrorWhitelist is the Settings KV shape.
// families true = preset exclude. custom = extra literal rules.
type ScheduleErrorWhitelist struct {
	Families map[string]bool           `json:"families"`
	Custom   []ScheduleErrorCustomRule `json:"custom"`
}

func knownScheduleErrorFamily(id string) bool {
	if id == ScheduleErrorFamilyRoutingModelMiss {
		// Accepted on save for old payloads; ignored. Old miss is hardcoded.
		return true
	}
	_, ok := defaultScheduleErrorFamilyEnabled[id]
	return ok
}

// DefaultScheduleErrorWhitelist is the factory preset (all new families off,
// no custom rules).
func DefaultScheduleErrorWhitelist() ScheduleErrorWhitelist {
	families := make(map[string]bool, len(ScheduleErrorFamilyIDs))
	for _, id := range ScheduleErrorFamilyIDs {
		families[id] = defaultScheduleErrorFamilyEnabled[id]
	}
	return ScheduleErrorWhitelist{
		Families: families,
		Custom:   []ScheduleErrorCustomRule{},
	}
}

// NormalizeScheduleErrorWhitelist copies known keys over factory defaults.
// Missing keys keep the factory default. Unknown family keys are dropped.
// Custom rules are trimmed; empty-condition rows are dropped.
func NormalizeScheduleErrorWhitelist(in ScheduleErrorWhitelist) ScheduleErrorWhitelist {
	out := DefaultScheduleErrorWhitelist()
	if in.Families != nil {
		for _, id := range ScheduleErrorFamilyIDs {
			if v, ok := in.Families[id]; ok {
				out.Families[id] = v
			}
		}
	}
	out.Custom = normalizeCustomRules(in.Custom)
	return out
}

// FamilyEnabled reports whether a preset family is on the whitelist.
// Missing key / empty config = factory default. Unknown id = false.
func (w ScheduleErrorWhitelist) FamilyEnabled(id string) bool {
	if w.Families != nil {
		if v, ok := w.Families[id]; ok {
			return v
		}
	}
	if v, ok := defaultScheduleErrorFamilyEnabled[id]; ok {
		return v
	}
	return false
}

func (r ScheduleErrorCustomRule) HasCondition() bool {
	return strings.TrimSpace(r.ErrorType) != "" ||
		strings.TrimSpace(r.Phase) != "" ||
		r.StatusCode > 0 ||
		strings.TrimSpace(r.ProviderErrorCode) != "" ||
		strings.TrimSpace(r.MessageContains) != ""
}

func (r ScheduleErrorCustomRule) Match(in ScheduleErrorMatchInput) bool {
	if !r.Enabled || !r.HasCondition() {
		return false
	}
	if t := strings.TrimSpace(r.ErrorType); t != "" && !strings.EqualFold(strings.TrimSpace(in.Type), t) {
		return false
	}
	if p := strings.TrimSpace(r.Phase); p != "" && !strings.EqualFold(strings.TrimSpace(in.Phase), p) {
		return false
	}
	if r.StatusCode > 0 && in.Status != r.StatusCode {
		return false
	}
	if c := strings.TrimSpace(r.ProviderErrorCode); c != "" && !strings.EqualFold(strings.TrimSpace(in.ProviderErrorCode), c) {
		return false
	}
	if needle := strings.TrimSpace(r.MessageContains); needle != "" && !customMessageContains(in, needle) {
		return false
	}
	return true
}

func customMessageContains(in ScheduleErrorMatchInput, needle string) bool {
	n := strings.ToLower(strings.TrimSpace(needle))
	if n == "" {
		return false
	}
	haystacks := []string{in.Message, in.Body, in.UpstreamErrorMessage, in.ProviderErrorCode}
	for _, h := range haystacks {
		if strings.Contains(strings.ToLower(h), n) {
			return true
		}
	}
	return false
}

func customRuleFingerprint(r ScheduleErrorCustomRule) string {
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(r.ErrorType)),
		strings.ToLower(strings.TrimSpace(r.Phase)),
		strconv.Itoa(r.StatusCode),
		strings.ToLower(strings.TrimSpace(r.ProviderErrorCode)),
		strings.ToLower(strings.TrimSpace(r.MessageContains)),
	}, "\x1f")
}

func normalizeCustomRule(in ScheduleErrorCustomRule) ScheduleErrorCustomRule {
	out := ScheduleErrorCustomRule{
		ID:                strings.TrimSpace(in.ID),
		Enabled:           in.Enabled,
		ErrorType:         strings.TrimSpace(in.ErrorType),
		Phase:             strings.TrimSpace(in.Phase),
		StatusCode:        in.StatusCode,
		ProviderErrorCode: strings.TrimSpace(in.ProviderErrorCode),
		MessageContains:   strings.TrimSpace(in.MessageContains),
	}
	if out.StatusCode < 0 {
		out.StatusCode = 0
	}
	if out.MessageContains != "" {
		out.MessageContains = truncateRunes(out.MessageContains, MaxScheduleErrorMessageContains)
	}
	if out.ID == "" && out.HasCondition() {
		out.ID = newScheduleErrorCustomRuleID()
	}
	return out
}

func normalizeCustomRules(in []ScheduleErrorCustomRule) []ScheduleErrorCustomRule {
	if len(in) == 0 {
		return []ScheduleErrorCustomRule{}
	}
	out := make([]ScheduleErrorCustomRule, 0, len(in))
	seen := make(map[string]int, len(in))
	for _, raw := range in {
		rule := normalizeCustomRule(raw)
		if !rule.HasCondition() {
			continue
		}
		fp := customRuleFingerprint(rule)
		if i, ok := seen[fp]; ok {
			if rule.Enabled {
				out[i].Enabled = true
			}
			continue
		}
		seen[fp] = len(out)
		out = append(out, rule)
	}
	return out
}

func newScheduleErrorCustomRuleID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("c_%d", time.Now().UnixNano())
	}
	return "c_" + hex.EncodeToString(b)
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i]
		}
		n++
	}
	return s
}

func customTextOK(s string) bool {
	for _, r := range s {
		if r == 0 || (unicode.IsControl(r) && r != '\t') {
			return false
		}
	}
	return true
}

// ParseScheduleErrorWhitelistJSON treats empty / invalid JSON as factory
// default (all new families off, no custom).
func ParseScheduleErrorWhitelistJSON(raw string) ScheduleErrorWhitelist {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultScheduleErrorWhitelist()
	}
	var parsed ScheduleErrorWhitelist
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return DefaultScheduleErrorWhitelist()
	}
	return NormalizeScheduleErrorWhitelist(parsed)
}

// ValidateScheduleErrorWhitelist accepts known family ids + valid custom rules.
func ValidateScheduleErrorWhitelist(settings *ScheduleErrorWhitelist) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}
	for id := range settings.Families {
		if !knownScheduleErrorFamily(id) {
			return fmt.Errorf("unknown schedule error whitelist family %q", id)
		}
	}
	if len(settings.Custom) > MaxScheduleErrorCustomRules {
		return fmt.Errorf("at most %d custom schedule error whitelist rules", MaxScheduleErrorCustomRules)
	}
	for i, raw := range settings.Custom {
		rule := normalizeCustomRule(raw)
		if !rule.HasCondition() {
			return fmt.Errorf("custom rule %d needs at least one match field", i)
		}
		if !customTextOK(rule.ErrorType) || !customTextOK(rule.Phase) ||
			!customTextOK(rule.ProviderErrorCode) || !customTextOK(rule.MessageContains) {
			return fmt.Errorf("custom rule %d contains invalid characters", i)
		}
		if len([]rune(rule.MessageContains)) > MaxScheduleErrorMessageContains {
			return fmt.Errorf("custom rule %d message_contains exceeds %d characters", i, MaxScheduleErrorMessageContains)
		}
	}
	return nil
}

func cloneScheduleErrorWhitelist(in ScheduleErrorWhitelist) ScheduleErrorWhitelist {
	out := ScheduleErrorWhitelist{
		Families: make(map[string]bool, len(in.Families)),
		Custom:   append([]ScheduleErrorCustomRule{}, in.Custom...),
	}
	for k, v := range in.Families {
		out.Families[k] = v
	}
	if out.Custom == nil {
		out.Custom = []ScheduleErrorCustomRule{}
	}
	return out
}

type scheduleErrorWhitelistCache struct {
	mu      sync.Mutex
	value   ScheduleErrorWhitelist
	loaded  time.Time
	ok      bool
	source  func(context.Context) ScheduleErrorWhitelist
	sourceM sync.RWMutex
}

var scheduleErrorWLCache = &scheduleErrorWhitelistCache{}

// RegisterScheduleErrorWhitelistSource wires SettingService into SQL generation.
func RegisterScheduleErrorWhitelistSource(fn func(context.Context) ScheduleErrorWhitelist) {
	scheduleErrorWLCache.sourceM.Lock()
	scheduleErrorWLCache.source = fn
	scheduleErrorWLCache.sourceM.Unlock()
	InvalidateScheduleErrorWhitelistCache()
}

// InvalidateScheduleErrorWhitelistCache drops the short TTL after a save.
func InvalidateScheduleErrorWhitelistCache() {
	scheduleErrorWLCache.mu.Lock()
	scheduleErrorWLCache.ok = false
	scheduleErrorWLCache.loaded = time.Time{}
	scheduleErrorWLCache.mu.Unlock()
}

// ResolveScheduleErrorWhitelist returns the current KV (or factory default).
// Classify callers that already loaded settings should pass that copy instead
// so Go and SQL stay on the same snapshot.
func ResolveScheduleErrorWhitelist(ctx context.Context) ScheduleErrorWhitelist {
	scheduleErrorWLCache.mu.Lock()
	if scheduleErrorWLCache.ok && time.Since(scheduleErrorWLCache.loaded) < scheduleErrorWhitelistCacheTTL {
		cached := cloneScheduleErrorWhitelist(scheduleErrorWLCache.value)
		scheduleErrorWLCache.mu.Unlock()
		return cached
	}
	scheduleErrorWLCache.mu.Unlock()

	scheduleErrorWLCache.sourceM.RLock()
	fn := scheduleErrorWLCache.source
	scheduleErrorWLCache.sourceM.RUnlock()

	wl := DefaultScheduleErrorWhitelist()
	if fn != nil {
		wl = NormalizeScheduleErrorWhitelist(fn(ctx))
	}

	scheduleErrorWLCache.mu.Lock()
	scheduleErrorWLCache.value = cloneScheduleErrorWhitelist(wl)
	scheduleErrorWLCache.loaded = time.Now()
	scheduleErrorWLCache.ok = true
	scheduleErrorWLCache.mu.Unlock()
	return wl
}

func registerScheduleErrorWhitelistSource(s *SettingService) {
	RegisterScheduleErrorWhitelistSource(func(ctx context.Context) ScheduleErrorWhitelist {
		if s == nil {
			return DefaultScheduleErrorWhitelist()
		}
		wl, err := s.GetScheduleErrorWhitelist(ctx)
		if err != nil || wl == nil {
			return DefaultScheduleErrorWhitelist()
		}
		return *wl
	})
}

// GetScheduleErrorWhitelist returns the admin-editable whitelist. Missing /
// invalid JSON yields factory defaults.
func (s *SettingService) GetScheduleErrorWhitelist(ctx context.Context) (*ScheduleErrorWhitelist, error) {
	if s == nil || s.settingRepo == nil {
		wl := DefaultScheduleErrorWhitelist()
		return &wl, nil
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyScheduleErrorWhitelist)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			wl := DefaultScheduleErrorWhitelist()
			return &wl, nil
		}
		return nil, fmt.Errorf("get schedule error whitelist: %w", err)
	}
	wl := ParseScheduleErrorWhitelistJSON(value)
	return &wl, nil
}

// SetScheduleErrorWhitelist persists families + custom, then invalidates
// the short cache so Classify / SQL pick up the new snapshot.
func (s *SettingService) SetScheduleErrorWhitelist(ctx context.Context, settings *ScheduleErrorWhitelist) error {
	if s == nil || s.settingRepo == nil {
		return fmt.Errorf("settings service not ready")
	}
	if err := ValidateScheduleErrorWhitelist(settings); err != nil {
		return err
	}
	normalized := NormalizeScheduleErrorWhitelist(*settings)
	data, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal schedule error whitelist: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyScheduleErrorWhitelist, string(data)); err != nil {
		return err
	}
	InvalidateScheduleErrorWhitelistCache()
	return nil
}

// UpsertScheduleErrorCustomRule enables an existing fingerprint or appends.
func (s *SettingService) UpsertScheduleErrorCustomRule(ctx context.Context, rule ScheduleErrorCustomRule) (*ScheduleErrorWhitelist, error) {
	rule = normalizeCustomRule(rule)
	rule.Enabled = true
	if err := ValidateScheduleErrorWhitelist(&ScheduleErrorWhitelist{
		Families: map[string]bool{},
		Custom:   []ScheduleErrorCustomRule{rule},
	}); err != nil {
		return nil, err
	}
	current, err := s.GetScheduleErrorWhitelist(ctx)
	if err != nil {
		return nil, err
	}
	fp := customRuleFingerprint(rule)
	for i := range current.Custom {
		if customRuleFingerprint(current.Custom[i]) == fp {
			current.Custom[i].Enabled = true
			if err := s.SetScheduleErrorWhitelist(ctx, current); err != nil {
				return nil, err
			}
			return s.GetScheduleErrorWhitelist(ctx)
		}
	}
	if len(current.Custom) >= MaxScheduleErrorCustomRules {
		return nil, fmt.Errorf("at most %d custom schedule error whitelist rules", MaxScheduleErrorCustomRules)
	}
	if rule.ID == "" {
		rule.ID = newScheduleErrorCustomRuleID()
	}
	current.Custom = append(current.Custom, rule)
	if err := s.SetScheduleErrorWhitelist(ctx, current); err != nil {
		return nil, err
	}
	return s.GetScheduleErrorWhitelist(ctx)
}

func scheduleErrorListPrimary(code, upstream, message string) string {
	code = strings.TrimSpace(code)
	upstream = strings.TrimSpace(upstream)
	message = strings.TrimSpace(message)
	switch {
	case code != "" && upstream != "":
		return code + " " + upstream
	case upstream != "":
		return upstream
	case code != "":
		return code
	default:
		return message
	}
}

func scheduleErrorLogStatus(log *OpsErrorLog) int {
	if log == nil {
		return 0
	}
	if log.ClientStatusCode > 0 {
		return log.ClientStatusCode
	}
	return log.StatusCode
}

// BuildScheduleErrorCustomRuleFromLog builds D5 structured or D6 message rule.
// 502 + upstream request failed is rejected.
func BuildScheduleErrorCustomRuleFromLog(log *OpsErrorLog, mode string) (ScheduleErrorCustomRule, error) {
	if log == nil {
		return ScheduleErrorCustomRule{}, fmt.Errorf("error log is required")
	}
	status := scheduleErrorLogStatus(log)
	if isHardCountedUpstreamRequestFailed(status, log.Message) {
		return ScheduleErrorCustomRule{}, fmt.Errorf("502 upstream request failed cannot be whitelisted")
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = ScheduleErrorFromErrorStructured
	}
	switch mode {
	case ScheduleErrorFromErrorStructured:
		rule := ScheduleErrorCustomRule{
			Enabled:           true,
			ErrorType:         strings.TrimSpace(log.Type),
			Phase:             strings.TrimSpace(log.Phase),
			StatusCode:        status,
			ProviderErrorCode: strings.TrimSpace(log.ProviderErrorCode),
		}
		if !rule.HasCondition() {
			return ScheduleErrorCustomRule{}, fmt.Errorf("error log has no structured match fields")
		}
		return normalizeCustomRule(rule), nil
	case ScheduleErrorFromErrorMessage:
		needle := scheduleErrorListPrimary(log.ProviderErrorCode, log.UpstreamErrorMessage, log.Message)
		needle = truncateRunes(needle, MaxScheduleErrorMessageContains)
		if strings.TrimSpace(needle) == "" {
			return ScheduleErrorCustomRule{}, fmt.Errorf("error log has no message to whitelist")
		}
		return normalizeCustomRule(ScheduleErrorCustomRule{
			Enabled:         true,
			MessageContains: needle,
		}), nil
	default:
		return ScheduleErrorCustomRule{}, fmt.Errorf("unknown from-error mode %q", mode)
	}
}

func loadScheduleErrorWhitelistFromRepo(ctx context.Context, repo SettingRepository) ScheduleErrorWhitelist {
	if repo == nil {
		return DefaultScheduleErrorWhitelist()
	}
	value, err := repo.GetValue(ctx, SettingKeyScheduleErrorWhitelist)
	if err != nil || value == "" {
		return DefaultScheduleErrorWhitelist()
	}
	return ParseScheduleErrorWhitelistJSON(value)
}
