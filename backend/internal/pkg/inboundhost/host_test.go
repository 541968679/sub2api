package inboundhost

import "testing"

func TestResolve_TrustedRelayUsesEntryHost(t *testing.T) {
	got := Resolve("zerocode.kaynlab.asia", EntryViaHKRelay, "zerocode.kaynlab.com")
	if got != "zerocode.kaynlab.asia" {
		t.Fatalf("got %q", got)
	}
}

func TestResolve_UntrustedEntryHostFallsBackToRequestHost(t *testing.T) {
	got := Resolve("zerocode.kaynlab.asia", "", "zerocode.kaynlab.com")
	if got != "zerocode.kaynlab.com" {
		t.Fatalf("spoofed entry host stored: %q", got)
	}
	got = Resolve("https://evil.example/path", "not-the-relay", "zerocode.kaynlab.com:443")
	if got != "zerocode.kaynlab.com" {
		t.Fatalf("got %q", got)
	}
}

func TestResolve_MissingEntryHostKeepsRequestHost(t *testing.T) {
	got := Resolve("", EntryViaHKRelay, "Zerocode.Kaynlab.COM")
	if got != "zerocode.kaynlab.com" {
		t.Fatalf("got %q", got)
	}
}

func TestSanitize_RejectsInjection(t *testing.T) {
	if got := Sanitize("evil.example\r\nX-Other: 1"); got != "" {
		t.Fatalf("got %q", got)
	}
}
