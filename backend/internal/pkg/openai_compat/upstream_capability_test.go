package openai_compat

import "testing"

func TestResolveResponsesSupport(t *testing.T) {
	tests := []struct {
		name  string
		extra map[string]any
		want  AccountResponsesSupport
	}{
		{"nil extra", nil, ResponsesSupportUnknown},
		{"empty extra", map[string]any{}, ResponsesSupportUnknown},
		{"key missing", map[string]any{"other": "value"}, ResponsesSupportUnknown},
		{"value true", map[string]any{ExtraKeyResponsesSupported: true}, ResponsesSupportYes},
		{"value false", map[string]any{ExtraKeyResponsesSupported: false}, ResponsesSupportNo},
		{"value wrong type string", map[string]any{ExtraKeyResponsesSupported: "true"}, ResponsesSupportUnknown},
		{"value wrong type number", map[string]any{ExtraKeyResponsesSupported: 1}, ResponsesSupportUnknown},
		{"value nil", map[string]any{ExtraKeyResponsesSupported: nil}, ResponsesSupportUnknown},
		{"force responses", map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeForceResponses)}, ResponsesSupportYes},
		{"force chat completions", map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeForceChatCompletions)}, ResponsesSupportNo},
		{"auto follows probe", map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeAuto), ExtraKeyResponsesSupported: false}, ResponsesSupportNo},
		{"invalid mode follows probe", map[string]any{ExtraKeyResponsesMode: "bogus", ExtraKeyResponsesSupported: true}, ResponsesSupportYes},
		{"force responses overrides probe false", map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeForceResponses), ExtraKeyResponsesSupported: false}, ResponsesSupportYes},
		{"force chat completions overrides probe true", map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeForceChatCompletions), ExtraKeyResponsesSupported: true}, ResponsesSupportNo},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveResponsesSupport(tc.extra)
			if got != tc.want {
				t.Errorf("ResolveResponsesSupport(%v) = %v, want %v", tc.extra, got, tc.want)
			}
		})
	}
}

func TestShouldUseResponsesAPI(t *testing.T) {
	tests := []struct {
		name  string
		extra map[string]any
		want  bool
	}{
		{"missing extra defaults to passthrough", nil, false},
		{"empty extra defaults to passthrough", map[string]any{}, false},
		{"invalid mode defaults to passthrough", map[string]any{ExtraKeyResponsesSupported: "yes"}, false},
		{"probe true without mode is passthrough", map[string]any{ExtraKeyResponsesSupported: true}, false},
		{"probe false without mode is passthrough", map[string]any{ExtraKeyResponsesSupported: false}, false},

		{"explicit auto unknown converts", map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeAuto)}, true},
		{"explicit auto supported converts", map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeAuto), ExtraKeyResponsesSupported: true}, true},
		{"explicit auto unsupported uses CC", map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeAuto), ExtraKeyResponsesSupported: false}, false},

		{"force responses overrides unsupported probe", map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeForceResponses), ExtraKeyResponsesSupported: false}, true},
		{"force chat completions overrides supported probe", map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeForceChatCompletions), ExtraKeyResponsesSupported: true}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ShouldUseResponsesAPI(tc.extra)
			if got != tc.want {
				t.Errorf("ShouldUseResponsesAPI(%v) = %v, want %v", tc.extra, got, tc.want)
			}
		})
	}
}

func TestNormalizeResponsesSupportMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want ResponsesSupportMode
	}{
		{"empty", "", ResponsesSupportModePassthrough},
		{"auto", "auto", ResponsesSupportModeAuto},
		{"force responses", "force_responses", ResponsesSupportModeForceResponses},
		{"force chat completions", "force_chat_completions", ResponsesSupportModeForceChatCompletions},
		{"passthrough", "passthrough", ResponsesSupportModePassthrough},
		{"invalid", "enabled", ResponsesSupportModePassthrough},
		{"native follows default passthrough", "native", ResponsesSupportModePassthrough},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeResponsesSupportMode(tc.mode)
			if got != tc.want {
				t.Errorf("NormalizeResponsesSupportMode(%q) = %q, want %q", tc.mode, got, tc.want)
			}
		})
	}
}

func TestResolveResponsesProbeSupportIgnoresForceMode(t *testing.T) {
	extra := map[string]any{
		ExtraKeyResponsesMode:      string(ResponsesSupportModeForceResponses),
		ExtraKeyResponsesSupported: false,
	}
	if got := ResolveResponsesProbeSupport(extra); got != ResponsesSupportNo {
		t.Errorf("ResolveResponsesProbeSupport() = %v, want No", got)
	}
	if got := ResolveResponsesSupport(extra); got != ResponsesSupportYes {
		t.Errorf("ResolveResponsesSupport still folds force_* = %v, want Yes", got)
	}
}

func TestResolveChatCompletionsProbeSupport(t *testing.T) {
	tests := []struct {
		name  string
		extra map[string]any
		want  AccountResponsesSupport
	}{
		{"nil extra", nil, ResponsesSupportUnknown},
		{"missing", map[string]any{ExtraKeyResponsesSupported: true}, ResponsesSupportUnknown},
		{"true", map[string]any{ExtraKeyChatCompletionsSupported: true}, ResponsesSupportYes},
		{"false", map[string]any{ExtraKeyChatCompletionsSupported: false}, ResponsesSupportNo},
		{"wrong type", map[string]any{ExtraKeyChatCompletionsSupported: "true"}, ResponsesSupportUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveChatCompletionsProbeSupport(tc.extra); got != tc.want {
				t.Errorf("ResolveChatCompletionsProbeSupport() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveUpstreamAPI(t *testing.T) {
	probeTrue := map[string]any{ExtraKeyResponsesSupported: true}
	autoTrue := map[string]any{
		ExtraKeyResponsesMode:      string(ResponsesSupportModeAuto),
		ExtraKeyResponsesSupported: true,
	}
	autoFalse := map[string]any{
		ExtraKeyResponsesMode:      string(ResponsesSupportModeAuto),
		ExtraKeyResponsesSupported: false,
	}
	autoBoth := map[string]any{
		ExtraKeyResponsesMode:              string(ResponsesSupportModeAuto),
		ExtraKeyResponsesSupported:         true,
		ExtraKeyChatCompletionsSupported:   true,
	}
	passthroughUnsupported := map[string]any{
		ExtraKeyResponsesMode:              string(ResponsesSupportModePassthrough),
		ExtraKeyResponsesSupported:         false,
		ExtraKeyChatCompletionsSupported:   false,
	}
	passthroughBoth := map[string]any{
		ExtraKeyResponsesMode:            string(ResponsesSupportModePassthrough),
		ExtraKeyResponsesSupported:       true,
		ExtraKeyChatCompletionsSupported: true,
	}

	tests := []struct {
		name    string
		inbound InboundEndpoint
		extra   map[string]any
		want    UpstreamEndpoint
	}{
		{"missing extra inbound CC", InboundChatCompletions, nil, UpstreamChatCompletions},
		{"missing extra inbound Responses", InboundResponses, nil, UpstreamResponses},
		{"invalid mode is passthrough", InboundChatCompletions, map[string]any{ExtraKeyResponsesMode: "native", ExtraKeyResponsesSupported: true}, UpstreamChatCompletions},
		{"probe true without mode inbound CC", InboundChatCompletions, probeTrue, UpstreamChatCompletions},
		{"probe true without mode inbound Responses", InboundResponses, probeTrue, UpstreamResponses},
		{"auto Rsupp true inbound CC", InboundChatCompletions, autoTrue, UpstreamResponses},
		{"auto Rsupp true inbound Responses", InboundResponses, autoTrue, UpstreamResponses},
		{"auto Rsupp false inbound CC", InboundChatCompletions, autoFalse, UpstreamChatCompletions},
		{"auto Rsupp false inbound Responses", InboundResponses, autoFalse, UpstreamChatCompletions},
		{"auto both true inbound CC stays Responses", InboundChatCompletions, autoBoth, UpstreamResponses},
		{"auto both true inbound Responses", InboundResponses, autoBoth, UpstreamResponses},
		{"auto CCsupp false does not change CC path", InboundChatCompletions, map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeAuto), ExtraKeyResponsesSupported: true, ExtraKeyChatCompletionsSupported: false}, UpstreamResponses},
		{"passthrough inbound CC", InboundChatCompletions, passthroughBoth, UpstreamChatCompletions},
		{"passthrough inbound Responses", InboundResponses, passthroughBoth, UpstreamResponses},
		{"passthrough ignores probe false inbound Responses", InboundResponses, passthroughUnsupported, UpstreamResponses},
		{"passthrough ignores probe false inbound CC", InboundChatCompletions, passthroughUnsupported, UpstreamChatCompletions},
		{"force responses inbound CC", InboundChatCompletions, map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeForceResponses), ExtraKeyResponsesSupported: false}, UpstreamResponses},
		{"force responses inbound Responses", InboundResponses, map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeForceResponses), ExtraKeyResponsesSupported: false}, UpstreamResponses},
		{"force chat inbound CC", InboundChatCompletions, map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeForceChatCompletions), ExtraKeyResponsesSupported: true}, UpstreamChatCompletions},
		{"force chat inbound Responses", InboundResponses, map[string]any{ExtraKeyResponsesMode: string(ResponsesSupportModeForceChatCompletions), ExtraKeyResponsesSupported: true}, UpstreamChatCompletions},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveUpstreamAPI(tc.inbound, tc.extra)
			if got != tc.want {
				t.Errorf("ResolveUpstreamAPI(%v, %v) = %v, want %v", tc.inbound, tc.extra, got, tc.want)
			}
		})
	}
}

func TestResponsesSupportModeFromExtra(t *testing.T) {
	if got := ResponsesSupportModeFromExtra(nil); got != ResponsesSupportModePassthrough {
		t.Errorf("nil extra = %q, want passthrough", got)
	}
	if got := ResponsesSupportModeFromExtra(map[string]any{}); got != ResponsesSupportModePassthrough {
		t.Errorf("empty extra = %q, want passthrough", got)
	}
	if got := ResponsesSupportModeFromExtra(map[string]any{ExtraKeyResponsesMode: "auto"}); got != ResponsesSupportModeAuto {
		t.Errorf("explicit auto = %q, want auto", got)
	}
}

func TestShouldUseResponsesAPI_PassthroughUsesInboundCC(t *testing.T) {
	extra := map[string]any{
		ExtraKeyResponsesMode:            string(ResponsesSupportModePassthrough),
		ExtraKeyResponsesSupported:       true,
		ExtraKeyChatCompletionsSupported: true,
	}
	if ShouldUseResponsesAPI(extra) {
		t.Fatal("passthrough inbound CC must not convert to Responses")
	}
}
