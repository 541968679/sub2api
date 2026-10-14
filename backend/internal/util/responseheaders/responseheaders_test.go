package responseheaders

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestFilterHeadersDisabledUsesDefaultAllowlist(t *testing.T) {
	src := http.Header{}
	src.Add("Content-Type", "application/json")
	src.Add("X-Request-Id", "req-123")
	src.Add("X-Test", "ok")
	src.Add("Connection", "keep-alive")
	src.Add("Content-Length", "123")

	cfg := config.ResponseHeaderConfig{
		Enabled:     false,
		ForceRemove: []string{"x-request-id"},
	}

	filtered := FilterHeaders(src, CompileHeaderFilter(cfg))
	if filtered.Get("Content-Type") != "application/json" {
		t.Fatalf("expected Content-Type passthrough, got %q", filtered.Get("Content-Type"))
	}
	if filtered.Get("X-Request-Id") != "" {
		t.Fatalf("expected X-Request-Id stripped, got %q", filtered.Get("X-Request-Id"))
	}
	if filtered.Get("X-Test") != "" {
		t.Fatalf("expected X-Test removed, got %q", filtered.Get("X-Test"))
	}
	if filtered.Get("Connection") != "" {
		t.Fatalf("expected Connection to be removed, got %q", filtered.Get("Connection"))
	}
	if filtered.Get("Content-Length") != "" {
		t.Fatalf("expected Content-Length to be removed, got %q", filtered.Get("Content-Length"))
	}
}

func TestFilterHeadersEnabledUsesAllowlist(t *testing.T) {
	src := http.Header{}
	src.Add("Content-Type", "application/json")
	src.Add("X-Extra", "ok")
	src.Add("X-Remove", "nope")
	src.Add("X-Blocked", "nope")

	cfg := config.ResponseHeaderConfig{
		Enabled:           true,
		AdditionalAllowed: []string{"x-extra"},
		ForceRemove:       []string{"x-remove"},
	}

	filtered := FilterHeaders(src, CompileHeaderFilter(cfg))
	if filtered.Get("Content-Type") != "application/json" {
		t.Fatalf("expected Content-Type allowed, got %q", filtered.Get("Content-Type"))
	}
	if filtered.Get("X-Extra") != "ok" {
		t.Fatalf("expected X-Extra allowed, got %q", filtered.Get("X-Extra"))
	}
	if filtered.Get("X-Remove") != "" {
		t.Fatalf("expected X-Remove removed, got %q", filtered.Get("X-Remove"))
	}
	if filtered.Get("X-Blocked") != "" {
		t.Fatalf("expected X-Blocked removed, got %q", filtered.Get("X-Blocked"))
	}
}

func TestFilterHeadersStripsIdentityAndAbsoluteRedirects(t *testing.T) {
	src := http.Header{}
	src.Add("X-Request-Id", "upstream-req")
	src.Add("X-Client-Request-Id", "upstream-client")
	src.Add("Location", "https://upstream.example/v1/files")
	src.Add("Location", "/v1/files/local")
	src.Add("Www-Authenticate", `Bearer realm="https://accounts.google.com/"`)
	src.Add("Www-Authenticate", `Bearer error="insufficient_scope"`)

	cfg := config.ResponseHeaderConfig{
		Enabled:           true,
		AdditionalAllowed: []string{"x-request-id", "x-client-request-id"},
	}
	filtered := FilterHeaders(src, CompileHeaderFilter(cfg))
	if filtered.Get("X-Request-Id") != "" || filtered.Get("X-Client-Request-Id") != "" {
		t.Fatalf("identity headers must stay off the client response: %v", filtered)
	}
	if got := filtered.Values("Location"); len(got) != 1 || got[0] != "/v1/files/local" {
		t.Fatalf("expected only the relative Location, got %v", got)
	}
	if got := filtered.Get("Www-Authenticate"); got != `Bearer error="insufficient_scope"` {
		t.Fatalf("expected host-free challenge, got %q", got)
	}
}
