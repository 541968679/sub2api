package service

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// relayVendorExtraKeys are relay bookkeeping fields. They are removed only
// when present, and only at the top level or under usage. Message content,
// tool-call ids, and prompt_cache_hit_tokens stay.
var relayVendorExtraKeys = []string{
	"credit",
	"elapsed_ms",
	"first_chunk_at",
	"credit_usage",
	"credit_unit",
	"credit_unit_plural",
	"billing_usage",
	"provider",
}

var (
	clientErrorURLPattern         = regexp.MustCompile(`https?://[^\s"'\\<>]+`)
	clientErrorQuotedLabelPattern = regexp.MustCompile(`(?i)\b(group|account)\s+["'][^"']+["']`)
	clientErrorEmailPattern       = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
)

// sanitizeClientRelayPayload removes relay extras and vendor-shaped ids
// without rebuilding a successful Chat Completions or Responses object.
// A Responses id is left in place because clients send it back as
// previous_response_id. Tool-call ids are nested and are not touched.
func sanitizeClientRelayPayload(payload []byte) []byte {
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return payload
	}
	root := gjson.ParseBytes(payload)
	if !root.IsObject() {
		return payload
	}
	updated := payload
	changed := false
	for _, key := range relayVendorExtraKeys {
		updated, changed = deleteRelayExtra(updated, key, changed)
		updated, changed = deleteRelayExtra(updated, "usage."+key, changed)
	}
	var textChanged bool
	updated, textChanged = scrubRelayErrorMessages(updated)
	if textChanged {
		changed = true
	}
	updated, idChanged := maskClientRelayID(updated)
	if !changed && !idChanged {
		return payload
	}
	return updated
}

// scrubRelayErrorMessages removes hosts and quoted account or group labels
// from error sentences. Assistant content and tool arguments are not walked.
func scrubRelayErrorMessages(payload []byte) ([]byte, bool) {
	if !clientRelayErrorPayload(payload) {
		return payload, false
	}
	changed := false
	for _, path := range []string{"error.message", "message", "error.error.message"} {
		val := gjson.GetBytes(payload, path)
		if val.Type != gjson.String || val.Str == "" {
			continue
		}
		scrubbed := ScrubClientErrorDisclosure(val.Str)
		if scrubbed == val.Str {
			continue
		}
		next, err := sjson.SetBytes(payload, path, scrubbed)
		if err != nil {
			continue
		}
		payload = next
		changed = true
	}
	return payload, changed
}

func deleteRelayExtra(payload []byte, path string, changed bool) ([]byte, bool) {
	if !gjson.GetBytes(payload, path).Exists() {
		return payload, changed
	}
	next, err := sjson.DeleteBytes(payload, path)
	if err != nil {
		return payload, changed
	}
	return next, true
}

func maskClientRelayID(payload []byte) ([]byte, bool) {
	id := gjson.GetBytes(payload, "id")
	if !id.Exists() || id.Type != gjson.String {
		return payload, false
	}
	current := id.Str
	if current == "" {
		return payload, false
	}
	if strings.HasPrefix(current, "msg_bdrk_") {
		next, err := sjson.SetBytes(payload, "id", "msg_"+strings.TrimPrefix(current, "msg_bdrk_"))
		if err != nil {
			return payload, false
		}
		return next, true
	}
	if !clientRelayErrorPayload(payload) {
		return payload, false
	}
	object := gjson.GetBytes(payload, "object").String()
	if object == "response" || object == "chat.completion" || object == "chat.completion.chunk" {
		return payload, false
	}
	if strings.HasPrefix(current, "local-") {
		return payload, false
	}
	sum := sha256.Sum256([]byte(current))
	next, err := sjson.SetBytes(payload, "id", "local-"+hex.EncodeToString(sum[:8]))
	if err != nil {
		return payload, false
	}
	return next, true
}

func clientRelayErrorPayload(payload []byte) bool {
	if gjson.GetBytes(payload, "type").String() == "error" {
		return true
	}
	errVal := gjson.GetBytes(payload, "error")
	return errVal.Exists() && errVal.Type != gjson.Null
}

func sanitizeClientRelayLine(line string) string {
	if line == "" {
		return line
	}
	payload, prefix, found := splitClientModelDataLine(line)
	if !found {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "{") {
			return line
		}
		rewritten := sanitizeClientRelayPayload([]byte(trimmed))
		if string(rewritten) == trimmed {
			return line
		}
		return strings.Replace(line, trimmed, string(rewritten), 1)
	}
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" || trimmed == "[DONE]" {
		return line
	}
	rewritten := sanitizeClientRelayPayload([]byte(trimmed))
	if string(rewritten) == trimmed {
		return line
	}
	return prefix + string(rewritten)
}

// ScrubClientErrorDisclosure removes hosts and quoted group or account labels
// from a client-visible error sentence. Ops records keep the original body.
func ScrubClientErrorDisclosure(msg string) string {
	if strings.TrimSpace(msg) == "" {
		return msg
	}
	out := clientErrorURLPattern.ReplaceAllString(msg, "")
	out = clientErrorQuotedLabelPattern.ReplaceAllString(out, "$1")
	out = clientErrorEmailPattern.ReplaceAllString(out, "")
	out = strings.Join(strings.Fields(out), " ")
	return strings.TrimSpace(out)
}
