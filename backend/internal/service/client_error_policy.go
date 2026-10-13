package service

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"github.com/tidwall/gjson"
)

// Client-facing upstream error templates. These are the sentences the gateway
// already returns. Admins pick one; they do not write a free-form body.
const (
	ClientErrorTemplate502Auth        = "502_auth"
	ClientErrorTemplate502Forbidden   = "502_forbidden"
	ClientErrorTemplate429            = "429"
	ClientErrorTemplate529            = "529"
	ClientErrorTemplate502Unavailable = "502_unavailable"
	ClientErrorTemplate502Failed      = "502_failed"
	ClientErrorTemplate413            = "413"

	ClientErrorProfileGateway = "gateway"
	ClientErrorProfileOpenAI  = "openai"
	ClientErrorProfileGemini  = "gemini"

	clientReplacementDefaultID = "status_default"
)

// ClientErrorTemplateIDs is the closed catalog shown in the error policy page.
var ClientErrorTemplateIDs = []string{
	ClientErrorTemplate429,
	ClientErrorTemplate413,
	ClientErrorTemplate502Auth,
	ClientErrorTemplate502Forbidden,
	ClientErrorTemplate502Unavailable,
	ClientErrorTemplate502Failed,
	ClientErrorTemplate529,
}

// ErrorReplacementRule is one builtin status mapping.
// Match fields are factory-owned. Admins change enabled, template, and
// schedule_exclude. The catch-all cannot be disabled and never excludes schedule.
type ErrorReplacementRule struct {
	ID              string `json:"id"`
	Enabled         bool   `json:"enabled"`
	Priority        int    `json:"priority"`
	StatusCodes     []int  `json:"status_codes,omitempty"`
	Template        string `json:"template"`
	ScheduleExclude bool   `json:"schedule_exclude"`
	Locked          bool   `json:"locked,omitempty"`
	Builtin         bool   `json:"builtin,omitempty"`
}

// KnownClientErrorTemplate reports whether id is in the closed catalog.
func KnownClientErrorTemplate(id string) bool {
	switch strings.TrimSpace(id) {
	case ClientErrorTemplate502Auth, ClientErrorTemplate502Forbidden, ClientErrorTemplate429,
		ClientErrorTemplate529, ClientErrorTemplate502Unavailable, ClientErrorTemplate502Failed,
		ClientErrorTemplate413:
		return true
	default:
		return false
	}
}

// FactoryErrorReplacements is the pre-feature mapUpstreamError table.
// Missing settings use this list, so a deploy without edits keeps today's client sentences.
func FactoryErrorReplacements() []ErrorReplacementRule {
	return []ErrorReplacementRule{
		{ID: "status_401", Enabled: true, Priority: 10, StatusCodes: []int{http.StatusUnauthorized}, Template: ClientErrorTemplate502Auth, Builtin: true},
		{ID: "status_403", Enabled: true, Priority: 20, StatusCodes: []int{http.StatusForbidden}, Template: ClientErrorTemplate502Forbidden, Builtin: true},
		{ID: "status_429", Enabled: true, Priority: 30, StatusCodes: []int{http.StatusTooManyRequests}, Template: ClientErrorTemplate429, Builtin: true},
		{ID: "status_529", Enabled: true, Priority: 40, StatusCodes: []int{529}, Template: ClientErrorTemplate529, Builtin: true},
		{ID: "status_5xx", Enabled: true, Priority: 50, StatusCodes: []int{500, 502, 503, 504}, Template: ClientErrorTemplate502Unavailable, Builtin: true},
		{ID: clientReplacementDefaultID, Enabled: true, Priority: 1000, Template: ClientErrorTemplate502Failed, Locked: true, Builtin: true},
	}
}

func knownClientReplacementID(id string) bool {
	for _, rule := range FactoryErrorReplacements() {
		if rule.ID == id {
			return true
		}
	}
	return false
}

// NormalizeErrorReplacements overlays admin fields onto the factory match table.
// Nil input stays nil so "never saved" keeps using the factory at read time.
func NormalizeErrorReplacements(in []ErrorReplacementRule) []ErrorReplacementRule {
	if in == nil {
		return nil
	}
	saved := make(map[string]ErrorReplacementRule, len(in))
	for _, raw := range in {
		id := strings.TrimSpace(raw.ID)
		if id == "" {
			continue
		}
		saved[id] = raw
	}
	out := FactoryErrorReplacements()
	for i := range out {
		over, ok := saved[out[i].ID]
		if !ok {
			continue
		}
		if out[i].Locked {
			out[i].Enabled = true
			out[i].ScheduleExclude = false
		} else {
			out[i].Enabled = over.Enabled
			out[i].ScheduleExclude = over.ScheduleExclude
		}
		if tpl := strings.TrimSpace(over.Template); KnownClientErrorTemplate(tpl) {
			out[i].Template = tpl
		}
	}
	return out
}

func effectiveClientReplacements(wl ScheduleErrorWhitelist) []ErrorReplacementRule {
	if len(wl.Replacements) == 0 {
		return FactoryErrorReplacements()
	}
	return NormalizeErrorReplacements(wl.Replacements)
}

func normalizeFamilyReplace(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(ScheduleErrorFamilyIDs))
	for _, id := range ScheduleErrorFamilyIDs {
		tpl := strings.TrimSpace(in[id])
		if tpl == "" || !KnownClientErrorTemplate(tpl) {
			continue
		}
		out[id] = tpl
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ApplyClientErrorTemplate returns the status, error type, and sentence for a catalog id.
// Profile selects the 529 error type: the Claude gateway uses overloaded_error,
// OpenAI and Gemini paths use upstream_error. Gemini ignores the type.
func ApplyClientErrorTemplate(id, profile string) (status int, errType, message string, ok bool) {
	switch strings.TrimSpace(id) {
	case ClientErrorTemplate502Auth:
		return http.StatusBadGateway, "upstream_error", "Upstream authentication failed, please contact administrator", true
	case ClientErrorTemplate502Forbidden:
		return http.StatusBadGateway, "upstream_error", "Upstream access forbidden, please contact administrator", true
	case ClientErrorTemplate429:
		return http.StatusTooManyRequests, "rate_limit_error", "Upstream rate limit exceeded, please retry later", true
	case ClientErrorTemplate529:
		errType = "upstream_error"
		if profile == ClientErrorProfileGateway {
			errType = "overloaded_error"
		}
		return http.StatusServiceUnavailable, errType, "Upstream service overloaded, please retry later", true
	case ClientErrorTemplate502Unavailable:
		return http.StatusBadGateway, "upstream_error", "Upstream service temporarily unavailable", true
	case ClientErrorTemplate502Failed:
		return http.StatusBadGateway, "upstream_error", "Upstream request failed", true
	case ClientErrorTemplate413:
		return http.StatusRequestEntityTooLarge, "invalid_request_error", "Prompt is too long: this request exceeds the context window for the selected model.", true
	default:
		return 0, "", "", false
	}
}

func (r ErrorReplacementRule) matchesStatus(status int) bool {
	if !r.Enabled {
		return false
	}
	if len(r.StatusCodes) == 0 {
		return r.Locked
	}
	for _, code := range r.StatusCodes {
		if code == status {
			return true
		}
	}
	return false
}

// ResolveUpstreamClientError maps an upstream failure to the client sentence.
// Error-passthrough and the Anthropic 4xx body replay run before this.
// Wait-timeout markers always become the 502 unavailable sentence.
// Order: custom replace rules, then preset family replace, then builtin status rules.
func ResolveUpstreamClientError(ctx context.Context, profile, platform string, status int, body []byte) (int, string, string) {
	_ = platform
	wl := ResolveScheduleErrorWhitelist(ctx)
	msg := ExtractUpstreamErrorMessage(body)
	if IsOpenAIWaitTimeoutOpsError(msg, "", string(body)) {
		status, errType, message, _ := ApplyClientErrorTemplate(ClientErrorTemplate502Unavailable, profile)
		return status, errType, message
	}
	if status, errType, message, ok := matchCustomClientReplace(wl, profile, status, body); ok {
		return status, errType, message
	}
	if status, errType, message, ok := matchFamilyClientReplace(wl, profile, status, msg, string(body)); ok {
		return status, errType, message
	}
	rules := effectiveClientReplacements(wl)
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].Priority < rules[j].Priority })
	for _, rule := range rules {
		if !rule.matchesStatus(status) {
			continue
		}
		if status, errType, message, ok := ApplyClientErrorTemplate(rule.Template, profile); ok {
			return status, errType, message
		}
	}
	status, errType, message, _ := ApplyClientErrorTemplate(ClientErrorTemplate502Failed, profile)
	return status, errType, message
}

func matchCustomClientReplace(wl ScheduleErrorWhitelist, profile string, status int, body []byte) (int, string, string, bool) {
	msg := ExtractUpstreamErrorMessage(body)
	typ := upstreamBodyErrorType(body)
	code := ExtractUpstreamErrorCode(body)
	bodyText := string(body)
	for _, rule := range wl.Custom {
		if !rule.Enabled || strings.TrimSpace(rule.ReplaceWith) == "" {
			continue
		}
		if phase := strings.TrimSpace(rule.Phase); phase != "" && !strings.EqualFold(phase, "upstream") {
			continue
		}
		phase := ""
		if strings.TrimSpace(rule.Phase) != "" {
			phase = "upstream"
		}
		if !rule.Match(ScheduleErrorMatchInput{
			Status:               status,
			Phase:                phase,
			Type:                 typ,
			Message:              msg,
			Body:                 bodyText,
			ProviderErrorCode:    code,
			UpstreamErrorMessage: msg,
		}) {
			continue
		}
		if status, errType, message, ok := ApplyClientErrorTemplate(rule.ReplaceWith, profile); ok {
			return status, errType, message, true
		}
	}
	return 0, "", "", false
}

func matchFamilyClientReplace(wl ScheduleErrorWhitelist, profile string, status int, message, body string) (int, string, string, bool) {
	if len(wl.FamilyReplace) == 0 {
		return 0, "", "", false
	}
	for _, id := range ScheduleErrorFamilyIDs {
		tpl := strings.TrimSpace(wl.FamilyReplace[id])
		if !KnownClientErrorTemplate(tpl) {
			continue
		}
		if !matchScheduleErrorFamily(id, status, "upstream", "", message, body) {
			continue
		}
		if status, errType, message, ok := ApplyClientErrorTemplate(tpl, profile); ok {
			return status, errType, message, true
		}
	}
	return 0, "", "", false
}

func upstreamBodyErrorType(body []byte) string {
	if t := strings.TrimSpace(gjson.GetBytes(body, "error.type").String()); t != "" {
		return t
	}
	return strings.TrimSpace(gjson.GetBytes(body, "type").String())
}

func clientReplacementExcludesSchedule(rule ErrorReplacementRule, status int) bool {
	if !rule.Enabled || !rule.ScheduleExclude || rule.Locked || len(rule.StatusCodes) == 0 {
		return false
	}
	for _, code := range rule.StatusCodes {
		if code == status {
			return true
		}
	}
	return false
}
