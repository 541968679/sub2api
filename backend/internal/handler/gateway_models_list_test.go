package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayHandlerModels_OpenAICuratedDiscoveryList(t *testing.T) {
	groupID := int64(1)
	apiKey := &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformOpenAI,
		},
	}

	ids := runGatewayModelsForTest(t, apiKey)
	require.Equal(t, []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5", "gpt-5.4", "gpt-5.4-mini"}, ids)
}

func TestGatewayHandlerModels_OpenAICuratedDiscoveryListIncludesCodexMetadata(t *testing.T) {
	groupID := int64(1)
	apiKey := &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformOpenAI,
		},
	}

	entries := runGatewayModelEntriesForTest(t, apiKey)
	require.NotEmpty(t, entries)

	first := entries[0]
	require.Equal(t, "gpt-5.6-sol", first.ID)
	require.ElementsMatch(t, []string{"openai-response", "openai", "openai-response-compact"}, first.SupportedEndpointTypes)
	require.ElementsMatch(t, []string{"chat_completions", "responses"}, first.SupportedSessionModes)
	require.Equal(t, "gpt-5.6-sol", first.ActualModelReturned["chat_completions"])
	require.Equal(t, "gpt-5.6-sol", first.ActualModelReturned["responses"])
	require.ElementsMatch(t, []string{"text", "image"}, first.InputModalities)
	require.ElementsMatch(t, []string{"text"}, first.OutputModalities)
	require.ElementsMatch(t, []string{"text", "image"}, first.SupportedModalities)
}

func TestGatewayHandlerModels_OpenAICuratedListCanBeNarrowedByCustomList(t *testing.T) {
	groupID := int64(1)
	apiKey := &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformOpenAI,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-image-2", "gpt-5.6-terra", "gpt-5.4-mini", "gpt-5.5"},
			},
		},
	}

	ids := runGatewayModelsForTest(t, apiKey)
	require.Equal(t, []string{"gpt-5.6-terra", "gpt-5.4-mini", "gpt-5.5"}, ids)
}

func TestGatewayHandlerModels_OpenAILegacyFullCustomListIncludesNewCuratedModels(t *testing.T) {
	groupID := int64(1)
	apiKey := &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformOpenAI,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"gpt-5.5", "gpt-5.4", "gpt-5.4-mini"},
			},
		},
	}

	ids := runGatewayModelsForTest(t, apiKey)
	require.Equal(t, []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.5", "gpt-5.4", "gpt-5.4-mini"}, ids)
}

func TestGatewayHandlerModels_AntigravityCuratedDiscoveryList(t *testing.T) {
	groupID := int64(2)
	apiKey := &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformAntigravity,
		},
	}

	ids := runGatewayModelsForTest(t, apiKey)
	require.Equal(t, []string{
		"claude-opus-4-8",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-haiku-4-5",
		"claude-sonnet-4-6",
	}, ids)
}

func TestGatewayHandlerAntigravityModels_CuratedDiscoveryList(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/antigravity/models", nil)

	h := &GatewayHandler{}
	h.AntigravityModels(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []string{
		"claude-opus-4-8",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-haiku-4-5",
		"claude-sonnet-4-6",
	}, decodeModelIDsForTest(t, recorder.Body.Bytes()))
}

func runGatewayModelsForTest(t *testing.T, apiKey *service.APIKey) []string {
	t.Helper()

	entries := runGatewayModelEntriesForTest(t, apiKey)
	ids := make([]string, 0, len(entries))
	for _, model := range entries {
		ids = append(ids, model.ID)
	}
	return ids
}

func runGatewayModelEntriesForTest(t *testing.T, apiKey *service.APIKey) []modelEntryForTest {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), apiKey)

	h := &GatewayHandler{}
	h.Models(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	return decodeModelEntriesForTest(t, recorder.Body.Bytes())
}

type modelEntryForTest struct {
	ID                     string            `json:"id"`
	SupportedEndpointTypes []string          `json:"supported_endpoint_types"`
	SupportedSessionModes  []string          `json:"supported_session_modes"`
	ActualModelReturned    map[string]string `json:"actual_model_returned"`
	InputModalities        []string          `json:"input_modalities"`
	OutputModalities       []string          `json:"output_modalities"`
	SupportedModalities    []string          `json:"supported_modalities"`
}

func decodeModelIDsForTest(t *testing.T, body []byte) []string {
	t.Helper()

	var response struct {
		Data []modelEntryForTest `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &response))
	ids := make([]string, 0, len(response.Data))
	for _, model := range response.Data {
		ids = append(ids, model.ID)
	}
	return ids
}

func decodeModelEntriesForTest(t *testing.T, body []byte) []modelEntryForTest {
	t.Helper()

	var response struct {
		Data []modelEntryForTest `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &response))
	return response.Data
}
