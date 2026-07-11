package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

type EngineFingerprintSignal struct {
	Type     string   `json:"type"`
	Match    []string `json:"match"`
	Required bool     `json:"required"`
}

const (
	FingerprintSignalHeaderExact  = "header_exact"
	FingerprintSignalHeaderPrefix = "header_prefix"
	FingerprintSignalBodyPath     = "body_path"
)

var DefaultEngineFingerprintSignals = []EngineFingerprintSignal{
	{Type: FingerprintSignalHeaderPrefix, Match: []string{"x-codex-"}, Required: true},
	{Type: FingerprintSignalHeaderExact, Match: []string{"session-id", "session_id"}},
	{Type: FingerprintSignalHeaderExact, Match: []string{"thread-id", "thread_id"}},
	{Type: FingerprintSignalBodyPath, Match: []string{"client_metadata.x-codex-window-id", "client_metadata.x-codex-installation-id"}},
}

func EvaluateEngineFingerprint(headers http.Header, body []byte, signals []EngineFingerprintSignal) bool {
	for _, signal := range signals {
		if signal.Required && !engineSignalMatches(headers, body, signal) {
			return false
		}
	}
	return true
}

func engineSignalMatches(headers http.Header, body []byte, signal EngineFingerprintSignal) bool {
	switch signal.Type {
	case FingerprintSignalHeaderExact:
		for _, name := range signal.Match {
			if name = strings.TrimSpace(name); name != "" && headers != nil && strings.TrimSpace(headers.Get(name)) != "" {
				return true
			}
		}
	case FingerprintSignalHeaderPrefix:
		for name := range headers {
			for _, prefix := range signal.Match {
				if prefix = strings.ToLower(strings.TrimSpace(prefix)); prefix != "" && strings.HasPrefix(strings.ToLower(name), prefix) {
					return true
				}
			}
		}
	case FingerprintSignalBodyPath:
		for _, path := range signal.Match {
			if path = strings.TrimSpace(path); path != "" && gjson.GetBytes(body, path).Exists() {
				return true
			}
		}
	}
	return false
}

func ParseEngineFingerprintSignals(raw string) ([]EngineFingerprintSignal, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	var signals []EngineFingerprintSignal
	if json.Unmarshal([]byte(raw), &signals) != nil {
		return nil, false
	}
	return signals, true
}

func ValidateEngineFingerprintSignalsJSON(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var signals []EngineFingerprintSignal
	if err := json.Unmarshal([]byte(raw), &signals); err != nil {
		return fmt.Errorf("must be empty or a valid JSON array of {type, match[], required}")
	}
	for i, signal := range signals {
		switch signal.Type {
		case FingerprintSignalHeaderExact, FingerprintSignalHeaderPrefix, FingerprintSignalBodyPath:
		default:
			return fmt.Errorf("entry %d: invalid type", i)
		}
		valid := false
		for _, match := range signal.Match {
			valid = valid || strings.TrimSpace(match) != ""
		}
		if !valid {
			return fmt.Errorf("entry %d: match must contain at least one non-empty value", i)
		}
	}
	return nil
}

func DefaultEngineFingerprintSignalsJSON() string {
	data, _ := json.Marshal(DefaultEngineFingerprintSignals)
	return string(data)
}
