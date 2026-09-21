package main

import (
	"encoding/json"
	"strings"
)

const (
	issueModelMissing         = "model_missing"
	issueModelEmpty           = "model_empty"
	issueModelMismatch        = "model_mismatch"
	issueResponseModelMissing = "response_model_missing"
	issueUsageMissing         = "usage_missing"
	issueSilentRefusal        = "silent_refusal"
	issueRoleOnlyFirst        = "role_only_first"
)

func jsonStringField(root map[string]json.RawMessage, key string) (kind, value string) {
	raw, ok := root[key]
	if !ok {
		return "missing", ""
	}
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return "empty", ""
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil {
		return "present", strings.Trim(s, `"`)
	}
	if strings.TrimSpace(v) == "" {
		return "empty", ""
	}
	return "present", v
}

func skipContractInspect(root map[string]json.RawMessage) bool {
	if raw, ok := root["type"]; ok {
		var typ string
		_ = json.Unmarshal(raw, &typ)
		if strings.TrimSpace(typ) == "error" {
			return true
		}
	}
	errRaw, hasErr := root["error"]
	if !hasErr {
		return false
	}
	s := strings.TrimSpace(string(errRaw))
	if s == "" || s == "null" {
		return false
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(errRaw, &obj) != nil || obj == nil {
		return false
	}
	if _, ok := root["choices"]; ok {
		return false
	}
	if resp, ok := root["response"]; ok {
		var nested map[string]json.RawMessage
		if json.Unmarshal(resp, &nested) == nil && len(nested) > 0 {
			return false
		}
	}
	var object string
	if raw, ok := root["object"]; ok {
		_ = json.Unmarshal(raw, &object)
	}
	return !strings.HasPrefix(object, "chat.completion")
}

func inspectJSONObject(payload []byte, requested string, st *streamStats, firstJSON bool) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(payload, &root); err != nil {
		return
	}
	if skipContractInspect(root) {
		return
	}
	st.Inspected++
	kind, val := jsonStringField(root, "model")
	switch kind {
	case "missing":
		st.ModelMissing++
	case "empty":
		st.ModelEmpty++
	default:
		st.ModelPresent++
		if st.FirstModel == "" {
			st.FirstModel = val
		}
		st.LastModel = val
		if requested != "" && val != requested {
			st.ModelMismatch++
			st.MismatchValue = val
		}
	}
	if resp, ok := root["response"]; ok {
		var nested map[string]json.RawMessage
		if json.Unmarshal(resp, &nested) == nil && nested != nil {
			k2, v2 := jsonStringField(nested, "model")
			switch k2 {
			case "missing", "empty":
				st.ResponseModelMissing++
			default:
				if st.LastModel == "" {
					st.LastModel = v2
				}
			}
		}
	}
	if firstJSON {
		st.RoleOnlyFirst = firstChunkRoleOnly(root)
	}
}

func firstChunkRoleOnly(root map[string]json.RawMessage) bool {
	raw, ok := root["choices"]
	if !ok {
		return false
	}
	var choices []map[string]any
	if err := json.Unmarshal(raw, &choices); err != nil || len(choices) == 0 {
		return false
	}
	delta, _ := choices[0]["delta"].(map[string]any)
	if delta == nil {
		return false
	}
	role, _ := delta["role"].(string)
	if strings.TrimSpace(role) == "" {
		return false
	}
	if deltaContent(delta["content"]) != "" {
		return false
	}
	if _, ok := delta["tool_calls"]; ok {
		return false
	}
	return true
}

func contractIssues(st streamStats, stream, includeUsage bool) []string {
	var issues []string
	if st.ModelMissing > 0 {
		issues = append(issues, issueModelMissing)
	}
	if st.ModelEmpty > 0 {
		issues = append(issues, issueModelEmpty)
	}
	if st.ModelMismatch > 0 {
		issues = append(issues, issueModelMismatch)
	}
	if st.ResponseModelMissing > 0 {
		issues = append(issues, issueResponseModelMissing)
	}
	if includeUsage && stream && st.Usage.PromptTokens == 0 && st.Usage.CompletionTokens == 0 && st.SawContent {
		issues = append(issues, issueUsageMissing)
	}
	if st.FinishReason == "stop" && !st.SawContent && !st.SawToolCall {
		issues = append(issues, issueSilentRefusal)
	}
	if st.RoleOnlyFirst {
		issues = append(issues, issueRoleOnlyFirst)
	}
	return issues
}

func isStrictContractIssue(code string) bool {
	switch code {
	case issueModelMissing, issueModelEmpty, issueModelMismatch, issueResponseModelMissing:
		return true
	default:
		return false
	}
}

func parseModelList(model, models string) []string {
	raw := strings.TrimSpace(models)
	if raw == "" {
		raw = strings.TrimSpace(model)
	}
	var out []string
	seen := map[string]bool{}
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	if len(out) == 0 {
		return []string{"kimi-k3"}
	}
	return out
}
