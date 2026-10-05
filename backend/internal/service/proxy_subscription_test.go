package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseProxySubscriptionPlainAndURL(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		"# comment",
		"1.2.3.4:9000 美国-01",
		"2.2.2.2:3128:bob:secret",
		"alice:p:ass@3.3.3.3:1080",
		"socks5,4.4.4.4,1080,user,pw",
		"http://user:p%40ss@[2001:db8::10]:8080#node-a",
		"socks5h://proxy.example.com:1080",
		"vmess://abc",
		"not a proxy",
		"1.2.3.4:9000",
	}, "\n")

	got := ParseProxySubscription([]byte(body), "socks5")
	if got.Format != "plain" {
		t.Fatalf("format = %s", got.Format)
	}
	if got.Unsupported != 1 || len(got.SkippedProtocols) != 1 || got.SkippedProtocols[0] != "vmess" {
		t.Fatalf("unsupported = %+v", got)
	}
	if got.Invalid != 1 {
		t.Fatalf("invalid = %d", got.Invalid)
	}
	if got.Duplicate != 1 {
		t.Fatalf("duplicate = %d", got.Duplicate)
	}
	if len(got.Proxies) != 6 {
		t.Fatalf("proxies = %+v", got.Proxies)
	}

	assertNode(t, got.Proxies[0], "美国-01", "socks5", "1.2.3.4", 9000, "", "")
	assertNode(t, got.Proxies[1], "2.2.2.2:3128", "socks5", "2.2.2.2", 3128, "bob", "secret")
	assertNode(t, got.Proxies[2], "3.3.3.3:1080", "socks5", "3.3.3.3", 1080, "alice", "p:ass")
	assertNode(t, got.Proxies[3], "4.4.4.4:1080", "socks5", "4.4.4.4", 1080, "user", "pw")
	assertNode(t, got.Proxies[4], "node-a", "http", "2001:db8::10", 8080, "user", "p@ss")
	assertNode(t, got.Proxies[5], "proxy.example.com:1080", "socks5h", "proxy.example.com", 1080, "", "")
}

func TestParseProxySubscriptionClashAndJSON(t *testing.T) {
	t.Parallel()

	clash := []byte(`
proxies:
  - name: http-node
    type: http
    server: 1.2.3.4
    port: 8080
    username: alice
    password: "p@ss:word"
  - name: socks-node
    type: socks5
    server: proxy.example.com
    port: 1080
  - {name: tls-http, type: http, server: 8.8.8.8, port: 443, tls: true}
  - name: vmess-node
    type: vmess
    server: 9.9.9.9
    port: 443
    uuid: abc
`)
	got := ParseProxySubscription(clash, "http")
	if got.Format != "clash" || got.Unsupported != 1 || len(got.Proxies) != 3 {
		t.Fatalf("clash result = %+v", got)
	}
	assertNode(t, got.Proxies[0], "http-node", "http", "1.2.3.4", 8080, "alice", "p@ss:word")
	assertNode(t, got.Proxies[1], "socks-node", "socks5", "proxy.example.com", 1080, "", "")
	assertNode(t, got.Proxies[2], "tls-http", "https", "8.8.8.8", 443, "", "")

	encoded := base64.StdEncoding.EncodeToString(clash)
	decoded := ParseProxySubscription([]byte(encoded), "http")
	if decoded.Format != "clash-base64" || len(decoded.Proxies) != 3 {
		t.Fatalf("encoded clash = %+v", decoded)
	}

	payload := []byte(`{"code":0,"data":[{"ip":"3.3.3.3","port":8000,"username":"u","password":"p"},{"server":"4.4.4.4","server_port":8388,"method":"aes-256-gcm","remarks":"ss1"},"https://5.5.5.5:443#edge"]}`)
	jsonResult := ParseProxySubscription(payload, "http")
	if jsonResult.Format != "json" || jsonResult.Unsupported != 1 || len(jsonResult.Proxies) != 2 {
		t.Fatalf("json result = %+v", jsonResult)
	}
	assertNode(t, jsonResult.Proxies[0], "3.3.3.3:8000", "http", "3.3.3.3", 8000, "u", "p")
	assertNode(t, jsonResult.Proxies[1], "edge", "https", "5.5.5.5", 443, "", "")
}

func TestParseProxySubscriptionLimits(t *testing.T) {
	t.Parallel()

	lines := make([]string, 0, maxSubscriptionProxies+5)
	for i := 0; i < maxSubscriptionProxies+5; i++ {
		lines = append(lines, fmt.Sprintf("10.1.1.%d:%d", i%200, 2000+i))
	}
	got := ParseProxySubscription([]byte(strings.Join(lines, "\n")), "http")
	if !got.Truncated || len(got.Proxies) != maxSubscriptionProxies {
		t.Fatalf("truncated result len=%d truncated=%v duplicate=%d", len(got.Proxies), got.Truncated, got.Duplicate)
	}

	longPassword := "http://user:" + strings.Repeat("a", maxProxyCredentialRune+1) + "@1.2.3.4:8080"
	limited := ParseProxySubscription([]byte(longPassword), "http")
	if limited.Invalid != 1 || len(limited.Proxies) != 0 {
		t.Fatalf("long password result = %+v", limited)
	}
}

func TestNormalizeProxyName(t *testing.T) {
	t.Parallel()

	if got := NormalizeProxyName("  \n "); got != "default" {
		t.Fatalf("empty name = %q", got)
	}
	long := strings.Repeat("名", 150)
	got := NormalizeProxyName(long)
	if utf8.RuneCountInString(got) != maxProxyNameRunes {
		t.Fatalf("rune count = %d", utf8.RuneCountInString(got))
	}
	if NormalizeProxyName("a\nb") != "ab" {
		t.Fatalf("control characters were kept: %q", NormalizeProxyName("a\nb"))
	}
}

func TestFetchProxySubscriptionRejectsBlockedURL(t *testing.T) {
	t.Parallel()

	blocked := []string{
		"",
		"ftp://example.com/sub",
		"file:///etc/passwd",
		"http://127.0.0.1:8080/sub",
		"http://10.1.2.3/sub",
		"http://localhost/sub",
		"http://169.254.169.254/latest",
		"http://[::1]:8080/sub",
	}
	for _, rawURL := range blocked {
		_, err := FetchProxySubscription(context.Background(), rawURL)
		if !errors.Is(err, ErrSubscriptionURLInvalid) {
			t.Fatalf("%q error = %v", rawURL, err)
		}
	}
}

func TestSubscriptionResponseAndRedirect(t *testing.T) {
	t.Parallel()

	ok := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("1.2.3.4:8080\n")),
	}
	body, err := readSubscriptionResponse(ok)
	if err != nil || string(body) != "1.2.3.4:8080\n" {
		t.Fatalf("body=%q err=%v", body, err)
	}

	statusResp := &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("missing"))}
	_, err = readSubscriptionResponse(statusResp)
	var statusErr *SubscriptionStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusNotFound {
		t.Fatalf("status error = %v", err)
	}

	empty := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(" \n"))}
	if _, err = readSubscriptionResponse(empty); !errors.Is(err, ErrSubscriptionEmpty) {
		t.Fatalf("empty error = %v", err)
	}

	large := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(strings.Repeat("a", maxSubscriptionBytes+1))),
	}
	if _, err = readSubscriptionResponse(large); !errors.Is(err, ErrSubscriptionTooLarge) {
		t.Fatalf("large error = %v", err)
	}

	allowed, err := http.NewRequest(http.MethodGet, "https://example.com/sub?token=secret", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = subscriptionRedirectCheck(allowed, nil); err != nil {
		t.Fatal(err)
	}
	blocked, err := http.NewRequest(http.MethodGet, "http://169.254.169.254/latest", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = subscriptionRedirectCheck(blocked, nil); !errors.Is(err, ErrSubscriptionURLInvalid) {
		t.Fatalf("redirect error = %v", err)
	}
	via := []*http.Request{allowed, allowed, allowed}
	if err = subscriptionRedirectCheck(allowed, via); !errors.Is(err, ErrSubscriptionFetchFailed) {
		t.Fatalf("redirect limit error = %v", err)
	}

	if !SubscriptionLooksLikeHTML([]byte("<!DOCTYPE html><html></html>")) {
		t.Fatal("expected html detection")
	}
}

func assertNode(t *testing.T, got SubscriptionProxyNode, name, protocol, host string, port int, username, password string) {
	t.Helper()
	if got.Name != name || got.Protocol != protocol || got.Host != host || got.Port != port || got.Username != username || got.Password != password {
		t.Fatalf("node = %+v, want %s %s %s:%d %s %s", got, name, protocol, host, port, username, password)
	}
}
