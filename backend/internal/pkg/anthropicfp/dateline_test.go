package anthropicfp

import (
	"bytes"
	"testing"
)

func TestNormalizeDatelineCanonicalizesSupportedSystemVariants(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "ascii slash", body: `{"system":"Today's date is 2026/07/01.","messages":[]}`},
		{name: "right quote", body: `{"system":[{"type":"text","text":"Today’s date is 2026-07-01."}],"messages":[]}`},
		{name: "modifier apostrophe", body: `{"system":"Todayʼs date is 2026/07/01.","messages":[]}`},
		{name: "prime", body: `{"system":"Todayʹs date is 2026/07/01.","messages":[]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := []byte(tt.body)
			out, hits, changed := NormalizeDateline(in)
			if !changed || len(hits) != 1 {
				t.Fatalf("expected one rewrite, changed=%v hits=%v", changed, hits)
			}
			if !bytes.Contains(out, []byte("Today's date is 2026-07-01.")) {
				t.Fatalf("canonical dateline missing: %s", out)
			}
			if bytes.Equal(out, in) {
				t.Fatal("rewritten body must not alias unchanged content")
			}
		})
	}
}

func TestNormalizeDatelineOnlyTouchesSystemReminderInMessages(t *testing.T) {
	body := []byte(`{"messages":[` +
		`{"role":"user","content":"Today’s date is 2026/07/01. Keep this prose."},` +
		`{"role":"user","content":[{"type":"text","text":"<system-reminder>Todayʼs date is 2026/07/01.</system-reminder>"}]},` +
		`{"role":"assistant","content":[{"type":"tool_use","id":"x","name":"y","input":{"text":"Todayʹs date is 2026/07/01."}}]}` +
		`]}`)

	out, hits, changed := NormalizeDateline(body)
	if !changed || len(hits) != 1 {
		t.Fatalf("expected only reminder rewrite, changed=%v hits=%v", changed, hits)
	}
	if !bytes.Contains(out, []byte(`Today’s date is 2026/07/01. Keep this prose.`)) {
		t.Fatalf("user prose changed: %s", out)
	}
	if !bytes.Contains(out, []byte(`Todayʹs date is 2026/07/01.`)) {
		t.Fatalf("tool input changed: %s", out)
	}
	if !bytes.Contains(out, []byte(`<system-reminder>Today's date is 2026-07-01.</system-reminder>`)) {
		t.Fatalf("reminder was not normalized: %s", out)
	}
}

func TestNormalizeDatelineNoOpIsByteIdenticalAndIdempotent(t *testing.T) {
	canonical := []byte(`{"system":"Today's date is 2026-07-01.","messages":[{"role":"user","content":"hello"}]}`)
	out, hits, changed := NormalizeDateline(canonical)
	if changed || hits != nil || !bytes.Equal(out, canonical) {
		t.Fatalf("canonical body must be an identity transform: changed=%v hits=%v out=%s", changed, hits, out)
	}

	dirty := []byte(`{"system":"Today’s date is 2026/07/01.","messages":[]}`)
	first, _, changed := NormalizeDateline(dirty)
	if !changed {
		t.Fatal("first pass should rewrite")
	}
	second, hits, changed := NormalizeDateline(first)
	if changed || hits != nil || !bytes.Equal(first, second) {
		t.Fatalf("second pass must be identity: changed=%v hits=%v", changed, hits)
	}
}

func TestNormalizeDatelineRejectsMixedSeparatorsAndInvalidJSON(t *testing.T) {
	for _, body := range [][]byte{
		[]byte(`{"system":"Today’s date is 2026-07/01.","messages":[]}`),
		[]byte(`{"system":`),
		nil,
	} {
		out, hits, changed := NormalizeDateline(body)
		if changed || hits != nil || !bytes.Equal(out, body) {
			t.Fatalf("out-of-contract body must be unchanged: in=%q out=%q hits=%v", body, out, hits)
		}
	}
}
