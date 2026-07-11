package service

import (
	"fmt"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
)

const CodexOfficialClientsOnlyMessage = "This account only allows Codex official clients"

const (
	CodexClientRestrictionReasonDisabled                   = "codex_cli_only_disabled"
	CodexClientRestrictionReasonMatchedUA                  = "official_client_user_agent_matched"
	CodexClientRestrictionReasonMatchedOriginator          = "official_client_originator_matched"
	CodexClientRestrictionReasonNotMatchedUA               = "official_client_user_agent_not_matched"
	CodexClientRestrictionReasonForceCodexCLI              = "force_codex_cli_enabled"
	CodexClientRestrictionReasonBlacklisted                = "blacklist_matched"
	CodexClientRestrictionReasonMatchedWhitelistClient     = "whitelist_client_matched"
	CodexClientRestrictionReasonVersionTooLow              = "codex_version_too_low"
	CodexClientRestrictionReasonMissingEngineFingerprint   = "missing_engine_fingerprint"
	CodexClientRestrictionReasonVersionUndetectable        = "codex_version_undetectable"
	CodexClientRestrictionReasonVersionTooHigh             = "codex_version_too_high"
	CodexClientRestrictionReasonMatchedAppServerClient     = "app_server_client_matched"
	CodexClientRestrictionReasonMatchedAllowedClient       = "allowed_client_matched"
	CodexClientRestrictionReasonMatchedGlobalAllowedClient = "global_allowed_client_matched"
)

type CodexRestrictionPolicy struct {
	Whitelist                []openai.AllowedClientEntry
	Blacklist                []openai.AllowedClientEntry
	MinCodexVersion          string
	MaxCodexVersion          string
	AllowAppServerClients    bool
	EngineFingerprintSignals []openai.EngineFingerprintSignal
}

type CodexClientRestrictionDetectionResult struct {
	Enabled         bool
	Matched         bool
	Reason          string
	DetectedVersion string
	MinCodexVersion string
	MaxCodexVersion string
}

type CodexClientRestrictionDetector interface {
	Detect(c *gin.Context, account *Account, globalAllowedClients []string) CodexClientRestrictionDetectionResult
}

type CodexClientRestrictionPolicyDetector interface {
	DetectWithPolicy(c *gin.Context, account *Account, policy CodexRestrictionPolicy, body []byte) CodexClientRestrictionDetectionResult
}

type OpenAICodexClientRestrictionDetector struct{ cfg *config.Config }

func NewOpenAICodexClientRestrictionDetector(cfg *config.Config) *OpenAICodexClientRestrictionDetector {
	return &OpenAICodexClientRestrictionDetector{cfg: cfg}
}

func (d *OpenAICodexClientRestrictionDetector) Detect(c *gin.Context, account *Account, globalAllowedClients []string) CodexClientRestrictionDetectionResult {
	if account == nil || !account.IsCodexCLIOnlyEnabled() {
		return CodexClientRestrictionDetectionResult{Reason: CodexClientRestrictionReasonDisabled}
	}
	if d != nil && d.cfg != nil && d.cfg.Gateway.ForceCodexCLI {
		return CodexClientRestrictionDetectionResult{Enabled: true, Matched: true, Reason: CodexClientRestrictionReasonForceCodexCLI}
	}
	var userAgent, originator string
	if c != nil {
		userAgent, originator = c.GetHeader("User-Agent"), c.GetHeader("originator")
	}
	switch {
	case openai.IsCodexOfficialClientRequest(userAgent):
		return CodexClientRestrictionDetectionResult{Enabled: true, Matched: true, Reason: CodexClientRestrictionReasonMatchedUA}
	case openai.IsCodexOfficialClientOriginator(originator):
		return CodexClientRestrictionDetectionResult{Enabled: true, Matched: true, Reason: CodexClientRestrictionReasonMatchedOriginator}
	case openai.MatchAllowedClients(userAgent, originator, account.GetCodexCLIOnlyAllowedClients()):
		return CodexClientRestrictionDetectionResult{Enabled: true, Matched: true, Reason: CodexClientRestrictionReasonMatchedAllowedClient}
	case openai.MatchAllowedClients(userAgent, originator, globalAllowedClients):
		return CodexClientRestrictionDetectionResult{Enabled: true, Matched: true, Reason: CodexClientRestrictionReasonMatchedGlobalAllowedClient}
	default:
		return CodexClientRestrictionDetectionResult{Enabled: true, Reason: CodexClientRestrictionReasonNotMatchedUA}
	}
}

func (d *OpenAICodexClientRestrictionDetector) DetectWithPolicy(c *gin.Context, account *Account, policy CodexRestrictionPolicy, body []byte) CodexClientRestrictionDetectionResult {
	return d.detect(c, account, policy, body, nil)
}

func (d *OpenAICodexClientRestrictionDetector) detect(c *gin.Context, account *Account, policy CodexRestrictionPolicy, body []byte, legacyGlobalAllowedClients []string) CodexClientRestrictionDetectionResult {
	if account == nil || !account.IsCodexCLIOnlyEnabled() {
		return CodexClientRestrictionDetectionResult{Reason: CodexClientRestrictionReasonDisabled}
	}
	if d != nil && d.cfg != nil && d.cfg.Gateway.ForceCodexCLI {
		return CodexClientRestrictionDetectionResult{Enabled: true, Matched: true, Reason: CodexClientRestrictionReasonForceCodexCLI}
	}
	var userAgent, originator string
	var headers http.Header
	if c != nil {
		userAgent, originator = c.GetHeader("User-Agent"), c.GetHeader("originator")
		if c.Request != nil {
			headers = c.Request.Header
		}
	}
	if openai.MatchDenyEntries(userAgent, originator, policy.Blacklist) {
		return CodexClientRestrictionDetectionResult{Enabled: true, Reason: CodexClientRestrictionReasonBlacklisted}
	}
	reason, skipFingerprint := "", false
	switch {
	case openai.IsCodexOfficialClientRequestStrict(userAgent):
		reason = CodexClientRestrictionReasonMatchedUA
	case openai.IsCodexOfficialClientOriginator(originator):
		reason = CodexClientRestrictionReasonMatchedOriginator
	default:
		if openai.MatchAllowedClients(userAgent, originator, account.GetCodexCLIOnlyAllowedClients()) {
			reason = CodexClientRestrictionReasonMatchedAllowedClient
		} else if openai.MatchAllowedClients(userAgent, originator, legacyGlobalAllowedClients) {
			reason = CodexClientRestrictionReasonMatchedGlobalAllowedClient
		} else if entry, ok := openai.MatchClientEntry(userAgent, originator, policy.Whitelist); ok {
			reason, skipFingerprint = CodexClientRestrictionReasonMatchedWhitelistClient, entry.SkipEngineFingerprint
		} else if policy.AllowAppServerClients || account.IsCodexCLIOnlyAppServerAllowed() {
			reason = CodexClientRestrictionReasonMatchedAppServerClient
		}
	}
	if reason == "" {
		return CodexClientRestrictionDetectionResult{Enabled: true, Reason: CodexClientRestrictionReasonNotMatchedUA}
	}
	if reason == CodexClientRestrictionReasonMatchedUA || reason == CodexClientRestrictionReasonMatchedOriginator {
		version, ok := openai.ParseCodexEngineVersion(userAgent)
		if !ok {
			return CodexClientRestrictionDetectionResult{Enabled: true, Reason: CodexClientRestrictionReasonVersionUndetectable}
		}
		if policy.MinCodexVersion != "" && CompareVersions(version, policy.MinCodexVersion) < 0 {
			return CodexClientRestrictionDetectionResult{Enabled: true, Reason: CodexClientRestrictionReasonVersionTooLow, DetectedVersion: version, MinCodexVersion: policy.MinCodexVersion}
		}
		if policy.MaxCodexVersion != "" && CompareVersions(version, policy.MaxCodexVersion) > 0 {
			return CodexClientRestrictionDetectionResult{Enabled: true, Reason: CodexClientRestrictionReasonVersionTooHigh, DetectedVersion: version, MaxCodexVersion: policy.MaxCodexVersion}
		}
	}
	if !skipFingerprint && !openai.EvaluateEngineFingerprint(headers, body, policy.EngineFingerprintSignals) {
		return CodexClientRestrictionDetectionResult{Enabled: true, Reason: CodexClientRestrictionReasonMissingEngineFingerprint}
	}
	return CodexClientRestrictionDetectionResult{Enabled: true, Matched: true, Reason: reason}
}

func CodexClientRestrictionMessage(result CodexClientRestrictionDetectionResult) string {
	switch result.Reason {
	case CodexClientRestrictionReasonVersionTooLow:
		return fmt.Sprintf("Your Codex version (%s) is below the minimum required version (%s). Please update Codex.", result.DetectedVersion, result.MinCodexVersion)
	case CodexClientRestrictionReasonVersionTooHigh:
		return fmt.Sprintf("Your Codex version (%s) exceeds the maximum allowed version (%s). Please downgrade Codex to %s or lower.", result.DetectedVersion, result.MaxCodexVersion, result.MaxCodexVersion)
	default:
		return CodexOfficialClientsOnlyMessage
	}
}
