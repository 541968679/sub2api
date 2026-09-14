// Package securityaudit provides OpenAI-compatible prompt auditing.
// Integration is default-off so existing gateway behavior is unchanged
// until an admin enables observe/blocking.
package securityaudit

import "net/http"

// Mode controls audit evaluation.
type Mode string

const (
	ModeOff      Mode = "off"
	ModeObserve  Mode = "observe"
	ModeBlocking Mode = "blocking"
)

// Decision is the gateway-facing audit result.
type Decision struct {
	Kind           string
	AllowNextStage bool
	Message        string
	StatusCode     int
	Code           string
}

const (
	DecisionAllow = "allow"
	DecisionBlock = "block"
)

// Config is the runtime audit configuration snapshot.
type Config struct {
	Mode Mode
}

// DefaultConfig is off — Evaluate always allows.
func DefaultConfig() Config {
	return Config{Mode: ModeOff}
}

// Evaluate applies mode policy. flagged is the model/rule hit signal.
// off/observe always allow next stage; blocking only blocks when flagged.
func Evaluate(cfg Config, flagged bool) Decision {
	mode := cfg.Mode
	if mode == "" {
		mode = ModeOff
	}
	if mode == ModeOff || mode == ModeObserve || !flagged {
		return Decision{
			Kind:           DecisionAllow,
			AllowNextStage: true,
			StatusCode:     http.StatusOK,
		}
	}
	return Decision{
		Kind:           DecisionBlock,
		AllowNextStage: false,
		Message:        "content blocked by security audit policy",
		StatusCode:     http.StatusForbidden,
		Code:           "content_policy_violation",
	}
}

// EffectiveMode normalizes empty mode to off.
func EffectiveMode(cfg Config) Mode {
	if cfg.Mode == "" {
		return ModeOff
	}
	return cfg.Mode
}
