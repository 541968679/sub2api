package service

import (
	"context"
	"fmt"
	"strings"
)

func sqlOpsErrorCols(prefix string) (status, phase, typ, msg, body string) {
	return prefix + "status_code",
		prefix + "error_phase",
		prefix + "error_type",
		prefix + "error_message",
		prefix + "error_body"
}

func sqlLowerLike(col, needle string) string {
	return fmt.Sprintf("LOWER(COALESCE(%s, '')) LIKE '%%%s%%'", col, needle)
}

func sqlQuoteLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func sqlLikeLiteral(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return sqlQuoteLiteral(s)
}

func sqlLowerEq(col, value string) string {
	return fmt.Sprintf("LOWER(TRIM(COALESCE(%s, ''))) = '%s'", col, sqlQuoteLiteral(strings.ToLower(strings.TrimSpace(value))))
}

func sqlCustomMessageContainsPredicate(prefix, needle string) string {
	_, _, _, msg, body := sqlOpsErrorCols(prefix)
	code := prefix + "provider_error_code"
	up := prefix + "upstream_error_message"
	n := sqlLikeLiteral(strings.ToLower(strings.TrimSpace(needle)))
	like := func(col string) string {
		return fmt.Sprintf("LOWER(COALESCE(%s, '')) LIKE '%%%s%%' ESCAPE '\\'", col, n)
	}
	return "(" + like(msg) + " OR " + like(body) + " OR " + like(up) + " OR " + like(code) + ")"
}

// ScheduleErrorMatchInput is one ops_error_logs row for schedule-exclude.
type ScheduleErrorMatchInput struct {
	Status               int
	Phase                string
	Type                 string
	Message              string
	Body                 string
	ProviderErrorCode    string
	UpstreamErrorMessage string
}

func sqlCustomRulePredicate(prefix string, rule ScheduleErrorCustomRule) string {
	if !rule.Enabled || !rule.HasCondition() {
		return "FALSE"
	}
	status, phase, typ, _, _ := sqlOpsErrorCols(prefix)
	parts := make([]string, 0, 5)
	if t := strings.TrimSpace(rule.ErrorType); t != "" {
		parts = append(parts, sqlLowerEq(typ, t))
	}
	if p := strings.TrimSpace(rule.Phase); p != "" {
		parts = append(parts, sqlLowerEq(phase, p))
	}
	if rule.StatusCode > 0 {
		parts = append(parts, fmt.Sprintf("COALESCE(%s, 0) = %d", status, rule.StatusCode))
	}
	if c := strings.TrimSpace(rule.ProviderErrorCode); c != "" {
		parts = append(parts, sqlLowerEq(prefix+"provider_error_code", c))
	}
	if n := strings.TrimSpace(rule.MessageContains); n != "" {
		parts = append(parts, sqlCustomMessageContainsPredicate(prefix, n))
	}
	if len(parts) == 0 {
		return "FALSE"
	}
	return "(" + strings.Join(parts, " AND ") + ")"
}

func sqlMsgOrBodyLike(msg, body, needle string) string {
	return "(" + sqlLowerLike(msg, needle) + " OR " + sqlLowerLike(body, needle) + ")"
}

// IsGroupNoAccountForModel matches "this group has no account that accepts the model".
// Any phase/status — production often stores this as 502 upstream_error.
func IsGroupNoAccountForModel(message, body string) bool {
	msg := strings.ToLower(message)
	bod := strings.ToLower(body)
	return strings.Contains(msg, "not supported by any configured account") ||
		strings.Contains(bod, "not supported by any configured account") ||
		strings.Contains(msg, "supporting model:") ||
		strings.Contains(bod, "supporting model:") ||
		strings.Contains(msg, "no account supports") ||
		strings.Contains(bod, "no account supports")
}

func SQLGroupNoAccountForModelPredicate(prefix string) string {
	_, _, _, msg, body := sqlOpsErrorCols(prefix)
	return "(" +
		sqlMsgOrBodyLike(msg, body, "not supported by any configured account") +
		" OR " + sqlMsgOrBodyLike(msg, body, "supporting model:") +
		" OR " + sqlMsgOrBodyLike(msg, body, "no account supports") +
		")"
}

// IsScheduleProtocolMismatch is a request aimed at the wrong endpoint or content shape.
func IsScheduleProtocolMismatch(message string) bool {
	msg := strings.ToLower(message)
	return strings.Contains(msg, "not supported on the chat completions endpoint") ||
		strings.Contains(msg, "unsupported content type") ||
		strings.Contains(msg, "invalid url")
}

func SQLScheduleProtocolMismatchPredicate(prefix string) string {
	_, _, _, msg, _ := sqlOpsErrorCols(prefix)
	return "(" +
		sqlLowerLike(msg, "not supported on the chat completions endpoint") +
		" OR " + sqlLowerLike(msg, "unsupported content type") +
		" OR " + sqlLowerLike(msg, "invalid url") +
		")"
}

func isRoutingPoolEmpty(status int, phase string) bool {
	return status == 503 && strings.EqualFold(strings.TrimSpace(phase), "routing")
}

func SQLRoutingPoolEmptyPredicate(prefix string) string {
	status, phase, _, _, _ := sqlOpsErrorCols(prefix)
	return fmt.Sprintf("(COALESCE(%s, 0) = 503 AND LOWER(TRIM(COALESCE(%s, ''))) = 'routing')", status, phase)
}

func isClientInvalidRequest(phase, errorType string) bool {
	return strings.EqualFold(strings.TrimSpace(errorType), "invalid_request_error") &&
		strings.EqualFold(strings.TrimSpace(phase), "request")
}

func SQLClientInvalidRequestPredicate(prefix string) string {
	_, phase, typ, _, _ := sqlOpsErrorCols(prefix)
	return fmt.Sprintf(
		"(LOWER(TRIM(COALESCE(%s, ''))) = 'invalid_request_error' AND LOWER(TRIM(COALESCE(%s, ''))) = 'request')",
		typ, phase,
	)
}

func isClientWrapped400URF(status int, message string) bool {
	return status == 400 && strings.Contains(strings.ToLower(message), "upstream request failed")
}

func SQLClientWrapped400URFPredicate(prefix string) string {
	status, _, _, msg, _ := sqlOpsErrorCols(prefix)
	return "(COALESCE(" + status + ", 0) = 400 AND " + sqlLowerLike(msg, "upstream request failed") + ")"
}

func isClientContextTooLong(status int, message string) bool {
	if status == 413 {
		return true
	}
	msg := strings.ToLower(message)
	return strings.Contains(msg, "prompt is too long") ||
		strings.Contains(msg, "context window") ||
		strings.Contains(msg, "array too long")
}

func SQLClientContextTooLongPredicate(prefix string) string {
	status, _, _, msg, _ := sqlOpsErrorCols(prefix)
	return "(" +
		"COALESCE(" + status + ", 0) = 413" +
		" OR " + sqlLowerLike(msg, "prompt is too long") +
		" OR " + sqlLowerLike(msg, "context window") +
		" OR " + sqlLowerLike(msg, "array too long") +
		")"
}

func isPairConcurrency(status int, message string) bool {
	return status == 429 && strings.Contains(strings.ToLower(message), "concurrency limit exceeded for account")
}

func SQLPairConcurrencyPredicate(prefix string) string {
	status, _, _, msg, _ := sqlOpsErrorCols(prefix)
	return "(COALESCE(" + status + ", 0) = 429 AND " + sqlLowerLike(msg, "concurrency limit exceeded for account") + ")"
}

// isHardCountedUpstreamRequestFailed is the safety rail: 502 + that wording
// always counts toward schedule. Config cannot whitelist it away.
func isHardCountedUpstreamRequestFailed(status int, message string) bool {
	return status == 502 && strings.Contains(strings.ToLower(message), "upstream request failed")
}

func SQLHardCountedUpstreamRequestFailedPredicate(prefix string) string {
	status, _, _, msg, _ := sqlOpsErrorCols(prefix)
	return "(COALESCE(" + status + ", 0) = 502 AND " + sqlLowerLike(msg, "upstream request failed") + ")"
}

func SQLHardCountedOpenAIWaitTimeoutPredicate(prefix string) string {
	_, _, _, msg, body := sqlOpsErrorCols(prefix)
	up := prefix + "upstream_error_message"
	hit := func(needle string) string {
		return "(" + sqlLowerLike(msg, needle) + " OR " + sqlLowerLike(body, needle) + " OR " + sqlLowerLike(up, needle) + ")"
	}
	return "(" + hit(OpenAIHeaderWaitTimeoutMarker) + " OR " + hit(OpenAIFirstUsefulFrameTimeoutMarker) + ")"
}

// IsOpsAttentionError is the dedicated-ops family: group/model gap, routing miss,
// routing 503, protocol mismatch. Client noise is not attention.
// Attention does not follow the schedule whitelist.
func IsOpsAttentionError(status int, phase, errorType, message, body string) bool {
	if IsGroupNoAccountForModel(message, body) {
		return true
	}
	if IsAccountQualityRoutingModelMiss(status, phase, errorType, message, body) {
		return true
	}
	if isRoutingPoolEmpty(status, phase) {
		return true
	}
	return IsScheduleProtocolMismatch(message)
}

func SQLOpsAttentionPredicate(prefix string) string {
	return "(" +
		SQLGroupNoAccountForModelPredicate(prefix) +
		" OR (" + SQLAccountQualityRoutingModelMissPredicatePrefixed(prefix) + ")" +
		" OR " + SQLRoutingPoolEmptyPredicate(prefix) +
		" OR " + SQLScheduleProtocolMismatchPredicate(prefix) +
		")"
}

// IsScheduleClientNoise is a bad client/pair-concurrency row. Not hop failure.
// Attention wording wins so a 400 group-gap is still marked for ops.
// invalid_request_error only matches error_phase=request.
func IsScheduleClientNoise(status int, phase, errorType, message, body string) bool {
	if IsOpsAttentionError(status, phase, errorType, message, body) {
		return false
	}
	if isClientInvalidRequest(phase, errorType) {
		return true
	}
	if isClientWrapped400URF(status, message) {
		return true
	}
	if isClientContextTooLong(status, message) {
		return true
	}
	if isPairConcurrency(status, message) {
		return true
	}
	_ = body
	return false
}

func SQLScheduleClientNoisePredicate(prefix string) string {
	noise := "(" +
		SQLClientInvalidRequestPredicate(prefix) +
		" OR " + SQLClientWrapped400URFPredicate(prefix) +
		" OR " + SQLClientContextTooLongPredicate(prefix) +
		" OR " + SQLPairConcurrencyPredicate(prefix) +
		")"
	return "(" + noise + " AND NOT " + SQLOpsAttentionPredicate(prefix) + ")"
}

func matchScheduleErrorFamily(id string, status int, phase, errorType, message, body string) bool {
	switch id {
	case ScheduleErrorFamilyClientInvalidRequest:
		return isClientInvalidRequest(phase, errorType)
	case ScheduleErrorFamilyClientWrapped400URF:
		return isClientWrapped400URF(status, message)
	case ScheduleErrorFamilyClientContextTooLong:
		return isClientContextTooLong(status, message)
	case ScheduleErrorFamilyPairConcurrency:
		return isPairConcurrency(status, message)
	case ScheduleErrorFamilyGroupNoAccount:
		return IsGroupNoAccountForModel(message, body)
	case ScheduleErrorFamilyRoutingModelMiss:
		return IsAccountQualityRoutingModelMiss(status, phase, errorType, message, body)
	case ScheduleErrorFamilyRoutingPoolEmpty:
		return isRoutingPoolEmpty(status, phase)
	case ScheduleErrorFamilyProtocolMismatch:
		return IsScheduleProtocolMismatch(message)
	default:
		return false
	}
}

func sqlScheduleErrorFamilyPredicate(id, prefix string) string {
	switch id {
	case ScheduleErrorFamilyClientInvalidRequest:
		return SQLClientInvalidRequestPredicate(prefix)
	case ScheduleErrorFamilyClientWrapped400URF:
		return SQLClientWrapped400URFPredicate(prefix)
	case ScheduleErrorFamilyClientContextTooLong:
		return SQLClientContextTooLongPredicate(prefix)
	case ScheduleErrorFamilyPairConcurrency:
		return SQLPairConcurrencyPredicate(prefix)
	case ScheduleErrorFamilyGroupNoAccount:
		return SQLGroupNoAccountForModelPredicate(prefix)
	case ScheduleErrorFamilyRoutingModelMiss:
		return "(" + SQLAccountQualityRoutingModelMissPredicatePrefixed(prefix) + ")"
	case ScheduleErrorFamilyRoutingPoolEmpty:
		return SQLRoutingPoolEmptyPredicate(prefix)
	case ScheduleErrorFamilyProtocolMismatch:
		return SQLScheduleProtocolMismatchPredicate(prefix)
	default:
		return "FALSE"
	}
}

// IsScheduleQualityExcluded uses factory-default families (all new families off)
// plus the hardcoded pre-feature routing-miss exclude.
func IsScheduleQualityExcluded(status int, phase, errorType, message, body string) bool {
	return IsScheduleQualityExcludedWith(status, phase, errorType, message, body, DefaultScheduleErrorWhitelist())
}

// IsScheduleQualityExcludedWith excludes a row from schedule ErrorCount when
// the pre-feature routing miss matches or an enabled new family matches.
// 502 "Upstream request failed" is never excluded.
// Attention is independent of this whitelist.
func IsScheduleQualityExcludedWith(status int, phase, errorType, message, body string, wl ScheduleErrorWhitelist) bool {
	return IsScheduleQualityExcludedMatch(ScheduleErrorMatchInput{
		Status:  status,
		Phase:   phase,
		Type:    errorType,
		Message: message,
		Body:    body,
	}, wl)
}

// IsScheduleQualityExcludedMatch is the full exclude (preset + custom).
func IsScheduleQualityExcludedMatch(in ScheduleErrorMatchInput, wl ScheduleErrorWhitelist) bool {
	if isHardCountedUpstreamRequestFailed(in.Status, in.Message) {
		return false
	}
	if IsOpenAIWaitTimeoutOpsError(in.Message, in.UpstreamErrorMessage, in.Body) {
		return false
	}
	// Pre-68060fbfb safety rail: keep excluding regardless of whitelist.
	if IsAccountQualityRoutingModelMiss(in.Status, in.Phase, in.Type, in.Message, in.Body) {
		return true
	}
	wl = NormalizeScheduleErrorWhitelist(wl)
	for _, id := range ScheduleErrorFamilyIDs {
		if wl.FamilyEnabled(id) && matchScheduleErrorFamily(id, in.Status, in.Phase, in.Type, in.Message, in.Body) {
			return true
		}
	}
	for _, rule := range wl.Custom {
		if rule.Match(in) {
			return true
		}
	}
	return false
}

func SQLScheduleQualityExcludedPredicate(prefix string) string {
	return SQLScheduleQualityExcludedPredicateWith(prefix, DefaultScheduleErrorWhitelist())
}

func SQLScheduleQualityExcludedPredicateWith(prefix string, wl ScheduleErrorWhitelist) string {
	wl = NormalizeScheduleErrorWhitelist(wl)
	parts := []string{"(" + SQLAccountQualityRoutingModelMissPredicatePrefixed(prefix) + ")"}
	for _, id := range ScheduleErrorFamilyIDs {
		if !wl.FamilyEnabled(id) {
			continue
		}
		parts = append(parts, sqlScheduleErrorFamilyPredicate(id, prefix))
	}
	for _, rule := range wl.Custom {
		if !rule.Enabled || !rule.HasCondition() {
			continue
		}
		parts = append(parts, sqlCustomRulePredicate(prefix, rule))
	}
	return "((" + strings.Join(parts, " OR ") + ") AND NOT " + SQLHardCountedUpstreamRequestFailedPredicate(prefix) + " AND NOT " + SQLHardCountedOpenAIWaitTimeoutPredicate(prefix) + ")"
}

func SQLExcludeAccountQualityScheduleNoise(prefix string) string {
	return SQLExcludeAccountQualityScheduleNoiseWith(prefix, DefaultScheduleErrorWhitelist())
}

func SQLExcludeAccountQualityScheduleNoiseWith(prefix string, wl ScheduleErrorWhitelist) string {
	return "NOT (" + SQLScheduleQualityExcludedPredicateWith(prefix, wl) + ")"
}

// SQLExcludeAccountQualityScheduleNoiseResolved builds the account-dimension
// 15m ErrorCount guard from the current Settings KV (short-cached).
func SQLExcludeAccountQualityScheduleNoiseResolved(ctx context.Context, prefix string) string {
	return SQLExcludeAccountQualityScheduleNoiseWith(prefix, ResolveScheduleErrorWhitelist(ctx))
}

// SQLAccountQualityRoutingModelMissPredicatePrefixed is the existing miss
// predicate with an optional table prefix ("" or "e.").
func SQLAccountQualityRoutingModelMissPredicatePrefixed(prefix string) string {
	status, phase, typ, msg, body := sqlOpsErrorCols(prefix)
	return fmt.Sprintf(
		"COALESCE(%s, 0) IN (400, 403, 404, 503)",
		status,
	) +
		fmt.Sprintf(" AND COALESCE(%s, '') <> 'upstream'", phase) +
		fmt.Sprintf(" AND LOWER(COALESCE(%s, '')) NOT IN ('upstream_error','overloaded_error','rate_limit_error')", typ) +
		" AND (" +
		fmt.Sprintf("LOWER(COALESCE(%s, '')) = 'model_not_found'", typ) +
		" OR " + sqlLowerLike(msg, "model_not_found") +
		" OR " + sqlLowerLike(body, "model_not_found") +
		" OR " + sqlLowerLike(msg, "unknown model") +
		" OR " + sqlLowerLike(msg, "model not found") +
		" OR " + sqlLowerLike(msg, "unsupported model") +
		" OR (LOWER(COALESCE(" + msg + ", '')) LIKE '%model%' AND LOWER(COALESCE(" + msg + ", '')) LIKE '%does not exist%')" +
		" OR " + sqlLowerLike(msg, "not supported by any configured account") +
		" OR " + sqlLowerLike(msg, "supporting model:") +
		" OR " + sqlLowerLike(msg, "no account supports") +
		" OR (LOWER(COALESCE(" + msg + ", '')) LIKE '%model%' AND LOWER(COALESCE(" + msg + ", '')) LIKE '%not in whitelist%')" +
		")"
}
