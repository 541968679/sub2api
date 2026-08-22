package service

import (
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
	return fmt.Sprintf("(COALESCE(%s, 0) = 503 AND LOWER(COALESCE(%s, '')) = 'routing')", status, phase)
}

// IsOpsAttentionError is the dedicated-ops family: group/model gap, routing miss,
// routing 503, protocol mismatch. Client noise is not attention.
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
func IsScheduleClientNoise(status int, phase, errorType, message, body string) bool {
	if IsOpsAttentionError(status, phase, errorType, message, body) {
		return false
	}
	typ := strings.ToLower(strings.TrimSpace(errorType))
	msg := strings.ToLower(message)
	if typ == "invalid_request_error" {
		return true
	}
	if status == 400 && strings.Contains(msg, "upstream request failed") {
		return true
	}
	if status == 413 {
		return true
	}
	if strings.Contains(msg, "prompt is too long") ||
		strings.Contains(msg, "context window") ||
		strings.Contains(msg, "array too long") {
		return true
	}
	if status == 429 && strings.Contains(msg, "concurrency limit exceeded for account") {
		return true
	}
	_ = phase
	_ = body
	return false
}

func SQLScheduleClientNoisePredicate(prefix string) string {
	status, _, typ, msg, _ := sqlOpsErrorCols(prefix)
	noise := "(" +
		fmt.Sprintf("LOWER(COALESCE(%s, '')) = 'invalid_request_error'", typ) +
		" OR (COALESCE(" + status + ", 0) = 400 AND " + sqlLowerLike(msg, "upstream request failed") + ")" +
		" OR COALESCE(" + status + ", 0) = 413" +
		" OR " + sqlLowerLike(msg, "prompt is too long") +
		" OR " + sqlLowerLike(msg, "context window") +
		" OR " + sqlLowerLike(msg, "array too long") +
		" OR (COALESCE(" + status + ", 0) = 429 AND " + sqlLowerLike(msg, "concurrency limit exceeded for account") + ")" +
		")"
	return "(" + noise + " AND NOT " + SQLOpsAttentionPredicate(prefix) + ")"
}

func IsScheduleQualityExcluded(status int, phase, errorType, message, body string) bool {
	return IsOpsAttentionError(status, phase, errorType, message, body) ||
		IsScheduleClientNoise(status, phase, errorType, message, body) ||
		IsAccountQualityRoutingModelMiss(status, phase, errorType, message, body)
}

func SQLScheduleQualityExcludedPredicate(prefix string) string {
	return "(" +
		SQLOpsAttentionPredicate(prefix) +
		" OR " + SQLScheduleClientNoisePredicate(prefix) +
		" OR (" + SQLAccountQualityRoutingModelMissPredicatePrefixed(prefix) + ")" +
		")"
}

func SQLExcludeAccountQualityScheduleNoise(prefix string) string {
	return "NOT (" + SQLScheduleQualityExcludedPredicate(prefix) + ")"
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
