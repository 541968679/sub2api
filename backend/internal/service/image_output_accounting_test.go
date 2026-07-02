package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIImageOutputCounterDataArrayCountsOnlyImageItems(t *testing.T) {
	body := []byte(`{
		"data": [
			{"url": "https://example.com/a.png", "size": "1024x1024"},
			{"b64_json": "YmFzZTY0", "size": "1536x1024"},
			{"type": "message", "text": "not an image"},
			{"size": "2048x2048"}
		]
	}`)

	require.Equal(t, 2, countOpenAIResponseImageOutputsFromJSONBytes(body))
	require.Equal(t, []string{"1024x1024", "1536x1024"}, collectOpenAIResponseImageOutputSizesFromJSONBytes(body))
}

func TestOpenAIImageOutputCounterCompletedEventRequiresResult(t *testing.T) {
	emptyResultBody := "data: {\"type\":\"image_generation.completed\",\"created_at\":1710000000}\n\n"
	require.Equal(t, 0, countOpenAIImageOutputsFromSSEBody(emptyResultBody))

	withResultBody := "data: {\"type\":\"image_generation.completed\",\"result\":\"YmFzZTY0\",\"size\":\"1024x1024\"}\n\n"
	require.Equal(t, 1, countOpenAIImageOutputsFromSSEBody(withResultBody))
	require.Equal(t, []string{"1024x1024"}, collectOpenAIImageOutputSizesFromSSEBody(withResultBody))
}
