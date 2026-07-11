package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildOpenAIModelsURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "version four base", base: "https://open.bigmodel.cn/api/coding/paas/v4", want: "https://open.bigmodel.cn/api/coding/paas/v4/models"},
		{name: "version two base", base: "https://gateway.example.com/openai/v2", want: "https://gateway.example.com/openai/v2/models"},
		{name: "version one base", base: "https://api.openai.com/v1", want: "https://api.openai.com/v1/models"},
		{name: "models URL unchanged", base: "https://api.openai.com/v1/models", want: "https://api.openai.com/v1/models"},
		{name: "host fallback uses v1", base: "https://api.openai.com", want: "https://api.openai.com/v1/models"},
		{name: "trailing slash on version four", base: "https://open.bigmodel.cn/api/coding/paas/v4/", want: "https://open.bigmodel.cn/api/coding/paas/v4/models"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, buildOpenAIModelsURL(tt.base))
		})
	}
}
