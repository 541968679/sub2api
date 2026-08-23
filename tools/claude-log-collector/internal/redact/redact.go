package redact

import (
	"bytes"
	"encoding/json"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

const Redacted = "***REDACTED***"

var (
	reSKAnt  = regexp.MustCompile(`(?i)\bsk-ant-[A-Za-z0-9_\-]{8,}`)
	reSK     = regexp.MustCompile(`(?i)\bsk-[A-Za-z0-9]{8,}`)
	reBearer = regexp.MustCompile(`(?i)\bBearer\s+\S+`)
	reURL    = regexp.MustCompile(`(?i)\bhttps?://[^\s"'\\]+`)
)

var secretField = map[string]bool{
	"apikey": true, "token": true, "accesstoken": true, "refreshtoken": true,
	"authorization": true, "auth": true,
	"password": true, "secret": true, "secretkey": true, "clientsecret": true,
	"cookie": true, "cookies": true,
	"anthropicapikey": true, "openaiapikey": true, "claudeapikey": true, "apitoken": true,
	"anthropicauthtoken": true, "xapikey": true, "primaryapikey": true, "oauthtoken": true,
}

var urlField = map[string]bool{
	"anthropicbaseurl": true, "baseurl": true,
	"url": true, "endpoint": true, "href": true,
}

func normField(k string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(k) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var excludeNames = map[string]bool{
	".credentials.json": true,
	"credentials.json":  true,
}

// ShouldExclude reports files that must never be copied into the zip.
func ShouldExclude(absPath string) bool {
	base := strings.ToLower(filepath.Base(absPath))
	if excludeNames[base] {
		return true
	}
	switch {
	case strings.HasSuffix(base, ".pem"), strings.HasSuffix(base, ".key"),
		strings.HasSuffix(base, ".p12"), strings.HasSuffix(base, ".pfx"):
		return true
	}
	rel := filepath.ToSlash(strings.ToLower(absPath))
	if strings.Contains(rel, "/.obsidian/plugins/cache/") || strings.HasSuffix(rel, "/.obsidian/plugins/cache") {
		return true
	}
	if base == "id_rsa" || base == "id_ed25519" || strings.HasPrefix(base, "id_rsa") {
		return true
	}
	return false
}

// LooksBinary treats NUL-containing buffers as non-text cache blobs.
func LooksBinary(data []byte) bool {
	return bytes.IndexByte(data, 0) >= 0
}

// ContainsSecret reports leftover plaintext key material after redaction.
func ContainsSecret(data []byte) bool {
	if reSKAnt.Match(data) || reBearer.Match(data) {
		return true
	}
	// reSK also matches sk-ant-; after ant is gone, leftover sk- still counts.
	return reSK.Match(data)
}

// RedactBytes redacts a copied buffer. Never write the original bytes to a zip.
func RedactBytes(name string, data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".jsonl"):
		data = redactJSONL(data)
	default:
		if out, ok := RedactJSON(data); ok {
			data = out
		}
	}
	// Always strip key tokens and URL userinfo/query, including leftover
	// strings inside JSON fields that are not in secretField/urlField.
	data = RedactText(data)
	if ContainsSecret(data) {
		return []byte(Redacted + "\n")
	}
	return data
}

// RedactJSON walks objects and redacts secret fields / key-shaped strings.
func RedactJSON(data []byte) ([]byte, bool) {
	trim := bytes.TrimSpace(data)
	if len(trim) == 0 || !json.Valid(trim) {
		return nil, false
	}
	var v any
	if err := json.Unmarshal(trim, &v); err != nil {
		return nil, false
	}
	redactValue("", v)
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, false
	}
	return out, true
}

func redactJSONL(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if out, ok := RedactJSON(line); ok {
			lines[i] = bytes.ReplaceAll(out, []byte("\n"), []byte(" "))
			continue
		}
		lines[i] = RedactText(line)
	}
	return bytes.Join(lines, []byte("\n"))
}

func redactValue(key string, v any) {
	switch node := v.(type) {
	case map[string]any:
		for k, child := range node {
			lk := normField(k)
			if secretField[lk] {
				node[k] = Redacted
				continue
			}
			if s, ok := child.(string); ok && urlField[lk] {
				node[k] = RedactURL(s)
				continue
			}
			redactValue(k, child)
		}
	case []any:
		for _, child := range node {
			redactValue(key, child)
		}
	}
}

// RedactText replaces key tokens, bearer tokens, and URL userinfo/query.
func RedactText(data []byte) []byte {
	s := string(data)
	s = reSKAnt.ReplaceAllString(s, Redacted)
	s = reSK.ReplaceAllString(s, Redacted)
	s = reBearer.ReplaceAllString(s, "Bearer "+Redacted)
	s = reURL.ReplaceAllStringFunc(s, RedactURL)
	return []byte(s)
}

// RedactURL keeps scheme + host (+ path), drops userinfo, query, and fragment.
func RedactURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return reSKAnt.ReplaceAllString(reSK.ReplaceAllString(raw, Redacted), Redacted)
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// HostnameOnly extracts the host from a URL-shaped value (env summary).
func HostnameOnly(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
