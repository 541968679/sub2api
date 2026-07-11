package openai

import (
	"regexp"
	"strings"
)

// CodexCLIUserAgentPrefixes matches Codex CLI User-Agent patterns
// Examples: "codex_vscode/1.0.0", "codex_cli_rs/0.1.2"
var CodexCLIUserAgentPrefixes = []string{
	"codex_vscode/",
	"codex_cli_rs/",
}

// CodexOfficialClientUserAgentPrefixes matches Codex 官方客户端家族 User-Agent 前缀。
// 该列表仅用于 OpenAI OAuth `codex_cli_only` 访问限制判定。
var CodexOfficialClientUserAgentPrefixes = []string{
	"codex_cli_rs/",
	"codex_vscode/",
	"codex_app/",
	"codex_chatgpt_desktop/",
	"codex_atlas/",
	"codex_exec/",
	"codex_sdk_ts/",
	"codex ",
}

// CodexOfficialClientOriginatorPrefixes matches Codex 官方客户端家族 originator 前缀。
// 说明：OpenAI 官方 Codex 客户端并不只使用固定的 codex_app 标识。
// 例如 codex_cli_rs、codex_vscode、codex_chatgpt_desktop、codex_atlas、codex_exec、codex_sdk_ts 等。
var CodexOfficialClientOriginatorPrefixes = []string{
	"codex_",
	"codex ",
}

var canonicalCodexOriginators = map[string]string{
	"codex_cli_rs":          "codex_cli_rs",
	"codex-tui":             "codex-tui",
	"codex_vscode":          "codex_vscode",
	"codex_vscode_copilot":  "codex_vscode_copilot",
	"codex_app":             "codex_app",
	"codex_chatgpt_desktop": "codex_chatgpt_desktop",
	"codex_atlas":           "codex_atlas",
	"codex_exec":            "codex_exec",
	"codex_sdk_ts":          "codex_sdk_ts",
}

// IsBrowserUserAgent 判断 User-Agent 是否来自浏览器（Chrome/Firefox/Safari/Edge/Opera 等）。
// 所有现代浏览器的 UA 均以 "Mozilla/" 作为前缀，CLI 工具（codex/claude/curl/postman/python-requests 等）不会。
// 该判定用于避免 Cloudflare 对浏览器型 UA 在 OpenAI 上游接口上触发 JS 质询。
func IsBrowserUserAgent(userAgent string) bool {
	ua := strings.TrimSpace(userAgent)
	if ua == "" {
		return false
	}
	return strings.HasPrefix(strings.ToLower(ua), "mozilla/")
}

// IsCodexCLIRequest checks if the User-Agent indicates a Codex CLI request
func IsCodexCLIRequest(userAgent string) bool {
	ua := normalizeCodexClientHeader(userAgent)
	if ua == "" {
		return false
	}
	return matchCodexClientHeaderPrefixes(ua, CodexCLIUserAgentPrefixes)
}

// IsCodexOfficialClientRequest checks if the User-Agent indicates a Codex 官方客户端请求。
// 与 IsCodexCLIRequest 解耦，避免影响历史兼容逻辑。
func IsCodexOfficialClientRequest(userAgent string) bool {
	ua := normalizeCodexClientHeader(userAgent)
	if ua == "" {
		return false
	}
	return matchCodexClientHeaderPrefixes(ua, CodexOfficialClientUserAgentPrefixes)
}

func IsCodexOfficialClientRequestStrict(userAgent string) bool {
	ua := normalizeCodexClientHeader(userAgent)
	if ua == "" {
		return false
	}
	for _, prefix := range CodexOfficialClientUserAgentPrefixes {
		if prefix == "codex " {
			if strings.HasPrefix(ua, prefix) {
				return true
			}
			continue
		}
		if normalized := normalizeCodexClientHeader(prefix); normalized != "" && strings.HasPrefix(ua, normalized) {
			return true
		}
	}
	if trailer := codexUATrailerName(ua); trailer != "" {
		return IsCodexOfficialClientOriginator(trailer)
	}
	return false
}

// IsCodexOfficialClientOriginator checks if originator indicates a Codex 官方客户端请求。
func IsCodexOfficialClientOriginator(originator string) bool {
	v := normalizeCodexClientHeader(originator)
	if v == "" {
		return false
	}
	return matchCodexClientHeaderPrefixes(v, CodexOfficialClientOriginatorPrefixes)
}

// IsCodexOfficialClientByHeaders checks whether the request headers indicate an
// official Codex client family request.
func IsCodexOfficialClientByHeaders(userAgent, originator string) bool {
	return IsCodexOfficialClientRequest(userAgent) || IsCodexOfficialClientOriginator(originator)
}

func normalizeCodexClientHeader(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func matchCodexClientHeaderPrefixes(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		normalizedPrefix := normalizeCodexClientHeader(prefix)
		if normalizedPrefix == "" {
			continue
		}
		// 优先前缀匹配；若 UA/Originator 被网关拼接为复合字符串时，退化为包含匹配。
		if strings.HasPrefix(value, normalizedPrefix) || strings.Contains(value, normalizedPrefix) {
			return true
		}
	}
	return false
}

// PairCodexClientIdentity derives an official originator from the final
// outbound User-Agent and normalizes its leading client name when necessary.
func PairCodexClientIdentity(userAgent string) (originator string, pairedUA string, ok bool) {
	ua := strings.TrimSpace(userAgent)
	slash := strings.IndexByte(ua, '/')
	if slash <= 0 {
		return "", "", false
	}
	if leading := strings.TrimSpace(ua[:slash]); isSaneCodexOriginator(leading) {
		if canonical, found := canonicalCodexOriginator(leading); found {
			return canonical, canonical + ua[slash:], true
		}
	}
	if trailer := codexUATrailerName(ua); trailer != "" && !strings.ContainsRune(trailer, '/') && isSaneCodexOriginator(trailer) {
		if canonical, found := canonicalCodexOriginator(trailer); found {
			return canonical, canonical + ua[slash:], true
		}
	}
	return "", "", false
}

const codexOriginatorMaxLen = 64

func isSaneCodexOriginator(name string) bool {
	if name == "" || len(name) > codexOriginatorMaxLen {
		return false
	}
	for i := 0; i < len(name); i++ {
		if name[i] < 0x20 || name[i] > 0x7e {
			return false
		}
	}
	return true
}

func canonicalCodexOriginator(name string) (string, bool) {
	if canonical, ok := canonicalCodexOriginators[normalizeCodexClientHeader(name)]; ok {
		return canonical, true
	}
	if strings.HasPrefix(name, "Codex ") {
		return name, true
	}
	return "", false
}

func codexUATrailerName(ua string) string {
	last := strings.LastIndex(ua, "(")
	if last < 0 {
		return ""
	}
	rest := ua[last+1:]
	closeIndex := strings.Index(rest, ")")
	if closeIndex < 0 {
		return ""
	}
	inner := strings.TrimSpace(rest[:closeIndex])
	if semicolon := strings.Index(inner, ";"); semicolon >= 0 {
		inner = strings.TrimSpace(inner[:semicolon])
	}
	return inner
}

var codexEngineVersionPattern = regexp.MustCompile(`^(\d+\.\d+\.\d+)`)

func ParseCodexEngineVersion(userAgent string) (string, bool) {
	userAgent = strings.TrimSpace(userAgent)
	slash := strings.IndexByte(userAgent, '/')
	if slash < 0 {
		return "", false
	}
	rest := userAgent[slash+1:]
	end := len(rest)
	for i := 0; i < len(rest); i++ {
		if rest[i] == ' ' || rest[i] == '(' {
			end = i
			break
		}
	}
	version := codexEngineVersionPattern.FindString(strings.TrimSpace(rest[:end]))
	return version, version != ""
}
