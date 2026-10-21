package claude

import "testing"

func TestDefaultModelIDsContainsUpstream55Models(t *testing.T) {
	t.Parallel()
	ids := map[string]bool{}
	for _, id := range DefaultModelIDs() {
		ids[id] = true
	}
	for _, id := range []string{"claude-sonnet-5-5", "claude-haiku-5-5", "claude-opus-5", "gpt-5.5"} {
		if id == "gpt-5.5" {
			continue
		}
		if !ids[id] {
			t.Fatalf("missing %s", id)
		}
	}
}

func TestDefaultModelIDsContainsSonnet5(t *testing.T) {
	t.Parallel()

	for _, id := range DefaultModelIDs() {
		if id == "claude-sonnet-5" {
			return
		}
	}

	t.Fatal("expected claude-sonnet-5 in default Claude model list")
}

func TestDefaultModelIDsContainsOpus5(t *testing.T) {
	t.Parallel()

	for _, id := range DefaultModelIDs() {
		if id == "claude-opus-5" {
			return
		}
	}

	t.Fatal("expected claude-opus-5 in default Claude model list")
}

func TestRequiresForcedContext1M(t *testing.T) {
	t.Parallel()

	cases := []struct {
		model string
		want  bool
	}{
		{"claude-opus-5", true},
		{"claude-opus-5-5", true},
		{"claude-opus-5-5[1m]", true},
		{"anthropic.claude-opus-5-5", true},
		{"claude-opus-5[1m]", true},
		{"claude-opus-5[2m]", true},
		{"claude-opus-5-thinking", true},
		{"us.anthropic.claude-opus-5-v1", true},
		{"models/claude-opus-5", true},
		{"claude-opus-4-8", true},
		{"claude-opus-4-8[1m]", true},
		{"us.anthropic.claude-opus-4-8-v1", true},
		{"claude-opus-4-7", false},
		{"claude-opus-4-6", false},
		{"claude-sonnet-5", false},
		{"claude-opus-50", false},
		{"claude-fable-5", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := RequiresForcedContext1M(tc.model); got != tc.want {
			t.Fatalf("RequiresForcedContext1M(%q)=%v want %v", tc.model, got, tc.want)
		}
	}
}
