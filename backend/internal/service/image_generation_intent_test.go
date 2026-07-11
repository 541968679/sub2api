package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsImageGenerationIntentRecognizesImageGenNamespaceShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "top level image namespace",
			body: `{"model":"gpt-5.5","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}]}`,
			want: true,
		},
		{
			name: "responses lite image namespace",
			body: `{"model":"gpt-5.5","input":[{"type":"additional_tools","tools":[{"type":"namespace","name":"image_gen"}]}]}`,
			want: true,
		},
		{
			name: "image namespace tool choice",
			body: `{"model":"gpt-5.5","tool_choice":{"type":"namespace","name":"image_gen"}}`,
			want: true,
		},
		{
			name: "ordinary imagegen function is not image intent",
			body: `{"model":"gpt-5.5","tools":[{"type":"function","name":"imagegen"}],"tool_choice":{"type":"function","function":{"name":"imagegen"}}}`,
			want: false,
		},
		{
			name: "unrelated namespace is not image intent",
			body: `{"model":"gpt-5.5","tools":[{"type":"namespace","name":"code_tools","tools":[{"type":"function","name":"imagegen"}]}]}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsImageGenerationIntent("/v1/responses", "gpt-5.5", []byte(tt.body)))
		})
	}
}
