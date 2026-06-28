//go:build unit

package service

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIResponsesProbePayloadRequiresFunctionCall(t *testing.T) {
	body := openaiResponsesProbePayload("glm-4.6")

	require.Equal(t, "glm-4.6", gjson.GetBytes(body, "model").String())
	require.Equal(t, "required", gjson.GetBytes(body, "tool_choice").String())
	require.Equal(t, "function", gjson.GetBytes(body, "tools.0.type").String())
	require.Equal(t, "probe_ping", gjson.GetBytes(body, "tools.0.name").String())
	require.False(t, gjson.GetBytes(body, "instructions").Exists())
	require.False(t, gjson.GetBytes(body, "stream").Bool())
}

func TestSelectResponsesProbeModel(t *testing.T) {
	account := &Account{
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"z":     "z-model",
				"a":     "a-model",
				"wild":  "glm-*",
				"empty": "",
			},
		},
	}

	require.Equal(t, "a-model", selectResponsesProbeModel(account))
	require.Equal(t, openai.DefaultTestModel, selectResponsesProbeModel(&Account{}))
}

func TestDecideResponsesProbeSupport(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{name: "404 unsupported", status: http.StatusNotFound, body: `{}`, want: false},
		{name: "405 unsupported", status: http.StatusMethodNotAllowed, body: `{}`, want: false},
		{name: "400 endpoint exists", status: http.StatusBadRequest, body: `{}`, want: true},
		{name: "500 endpoint exists", status: http.StatusInternalServerError, body: `{}`, want: true},
		{
			name:   "2xx with function call",
			status: http.StatusOK,
			body:   `{"output":[{"type":"reasoning"},{"type":"function_call","name":"probe_ping"}]}`,
			want:   true,
		},
		{
			name:   "2xx reasoning only",
			status: http.StatusOK,
			body:   `{"output":[{"type":"reasoning"}]}`,
			want:   false,
		},
		{name: "2xx invalid body", status: http.StatusOK, body: `{}`, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, decideResponsesProbeSupport(tt.status, []byte(tt.body)))
		})
	}
}
