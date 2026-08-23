package redact

import (
	"path/filepath"
	"strings"
	"testing"
)

const (
	fixtureAntKey = "sk-ant-testfixture000000000000001"
	fixtureSK     = "sk-abcdefghijklmnop"
	fixtureBearer = "Bearer 0a1b2c3d4e5f67890123456789abcd"
)

func TestShouldExcludeCredentialsAndCache(t *testing.T) {
	if !ShouldExclude(filepath.Join("C:\\Users\\x\\.claude", ".credentials.json")) {
		t.Fatal("credentials")
	}
	if !ShouldExclude(filepath.Join("vault", ".obsidian", "plugins", "cache", "market.json")) {
		t.Fatal("plugin cache")
	}
	if ShouldExclude(filepath.Join("C:\\Users\\x\\.claude", "settings.json")) {
		t.Fatal("settings should be packed after redact")
	}
}

func TestRedactSettingsAndClaudeJSON_AC4(t *testing.T) {
	settings := []byte(`{
		"env": {
			"ANTHROPIC_BASE_URL": "https://user:secret@gw.example.com/v1?api_key=` + fixtureSK + `",
			"ANTHROPIC_API_KEY": "` + fixtureAntKey + `"
		},
		"apiKey": "` + fixtureAntKey + `",
		"note": "plain ` + fixtureSK + `"
	}`)
	out := RedactBytes("settings.json", settings)
	got := string(out)
	if ContainsSecret(out) {
		t.Fatalf("still contains secret:\n%s", got)
	}
	for _, leak := range []string{fixtureAntKey, fixtureSK, "user:secret", "api_key="} {
		if strings.Contains(got, leak) {
			t.Fatalf("leaked %q in:\n%s", leak, got)
		}
	}
	if !strings.Contains(got, "gw.example.com") {
		t.Fatalf("should keep hostname:\n%s", got)
	}
	if !strings.Contains(got, Redacted) {
		t.Fatal("expected redaction marker")
	}
}

func TestRedactBearerAndCredentialsNeverPacked(t *testing.T) {
	body := []byte(`authorization: ` + fixtureBearer + "\n")
	out := RedactBytes("debug.log", body)
	if strings.Contains(string(out), "0a1b2c3d4e5f") {
		t.Fatalf("bearer leaked: %s", out)
	}
	if !ShouldExclude(".credentials.json") {
		t.Fatal("exclude raw credentials")
	}
}

func TestRedactURL(t *testing.T) {
	got := RedactURL("https://ak:tok@api.anthropic.com/v1/messages?key=sk-abcd1234")
	if strings.Contains(got, "ak:") || strings.Contains(got, "tok") || strings.Contains(got, "key=") {
		t.Fatalf("url not cleaned: %s", got)
	}
	if !strings.Contains(got, "api.anthropic.com") {
		t.Fatalf("host dropped: %s", got)
	}
}

func TestRedactJSONL(t *testing.T) {
	in := []byte(`{"token":"` + fixtureAntKey + `"}` + "\n" + `text ` + fixtureSK + "\n")
	out := string(RedactBytes("history.jsonl", in))
	if strings.Contains(out, fixtureAntKey) || strings.Contains(out, fixtureSK) {
		t.Fatalf("jsonl leaked: %s", out)
	}
}

func TestHostnameOnly(t *testing.T) {
	if HostnameOnly("https://proxy.customer.example/v1") != "proxy.customer.example" {
		t.Fatal(HostnameOnly("https://proxy.customer.example/v1"))
	}
}

func TestRedactCamelCaseAndAuthTokenFields_AC4(t *testing.T) {
	in := []byte(`{
		"oauthToken": "oauth-plain-token-value-001",
		"primaryApiKey": "` + fixtureAntKey + `",
		"env": {
			"ANTHROPIC_AUTH_TOKEN": "oauth-plain-token-value-002",
			"X-Api-Key": "plain-header-token-003"
		},
		"proxy": "https://user:hunter2@relay.example.com/v1?key=1"
	}`)
	got := string(RedactBytes("settings.json", in))
	for _, leak := range []string{
		"oauth-plain-token-value-001", "oauth-plain-token-value-002",
		"plain-header-token-003", fixtureAntKey, "user:hunter2", "key=1",
	} {
		if strings.Contains(got, leak) {
			t.Fatalf("leaked %q in:\n%s", leak, got)
		}
	}
	if !strings.Contains(got, "relay.example.com") {
		t.Fatalf("should keep hostname:\n%s", got)
	}
}
