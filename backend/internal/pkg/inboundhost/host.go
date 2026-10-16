// Package inboundhost records which public hostname accepted a gateway request.
// Hong Kong rewrites Host to the origin name, so Request.Host alone cannot tell
// zerocode.kaynlab.asia from a direct hit on zerocode.kaynlab.com.
package inboundhost

import (
	"net"
	"strings"
)

const (
	// HeaderEntryHost is set by the Hong Kong relay to the site the client used.
	HeaderEntryHost = "X-Sub2-Entry-Host"
	// HeaderEntryVia is set by origin Caddy only when the TCP peer is the relay.
	// A client-supplied entry host without this marker is ignored.
	HeaderEntryVia = "X-Sub2-Entry-Via"
	// EntryViaHKRelay is the only accepted marker value.
	EntryViaHKRelay = "hk-relay"
)

// Resolve returns the hostname to store. entryHost is used only when via is the
// relay marker origin Caddy injects for peer 191.40.32.186. Any other source,
// including a client that sends HeaderEntryHost by itself, keeps requestHost.
func Resolve(entryHost, via, requestHost string) string {
	request := Sanitize(requestHost)
	if strings.EqualFold(strings.TrimSpace(via), EntryViaHKRelay) {
		if entry := Sanitize(entryHost); entry != "" {
			return entry
		}
	}
	return request
}

// Sanitize keeps a hostname (or host:port) and drops schemes, paths, and
// header injection. An empty result means the value must not be stored.
func Sanitize(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
	}
	if i := strings.IndexAny(raw, "/?#"); i >= 0 {
		raw = raw[:i]
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 255 {
		return ""
	}
	for _, r := range raw {
		if r <= 32 || r == 127 || r == ',' || r == ';' {
			return ""
		}
	}
	host := raw
	if h, _, err := net.SplitHostPort(raw); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || len(host) > 255 || strings.Contains(host, " ") {
		return ""
	}
	return host
}
