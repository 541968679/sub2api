package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

var errPickerUpstream = errors.New("upstream 502")

type pickerAccountRepoStub struct {
	service.AccountRepository
	accounts []service.Account
}

func (s *pickerAccountRepoStub) ListByGroup(_ context.Context, _ int64) ([]service.Account, error) {
	return append([]service.Account(nil), s.accounts...), nil
}

type pickerFetcherStub struct {
	byID map[int64][]string
	err  map[int64]error
}

func (s pickerFetcherStub) FetchUpstreamSupportedModels(_ context.Context, account *service.Account) ([]string, error) {
	if account == nil {
		return nil, errors.New("nil account")
	}
	if s.err != nil {
		if err, ok := s.err[account.ID]; ok {
			return nil, err
		}
	}
	if s.byID == nil {
		return nil, nil
	}
	return append([]string(nil), s.byID[account.ID]...), nil
}

func TestGatewayHandlerModels_PickerEnabledReturnsUpstreamUnionNotGPTSeed(t *testing.T) {
	groupID := int64(46)
	svc := service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, nil)
	svc.SetAccountRepo(&pickerAccountRepoStub{
		accounts: []service.Account{
			{ID: 10, Platform: service.PlatformOpenAI},
			{ID: 11, Platform: service.PlatformOpenAI},
		},
	})
	svc.SetUpstreamModelsFetcher(pickerFetcherStub{
		byID: map[int64][]string{
			10: {"kimi-k2.5", "glm-5.3"},
			11: {"MiniMax-M2.5"},
		},
	})

	apiKey := &service.APIKey{
		ID:      1,
		UserID:  9,
		GroupID: &groupID,
		Group: &service.Group{
			ID:                          groupID,
			Platform:                    service.PlatformOpenAI,
			CcsImportModelPickerEnabled: true,
			CcsImportDefaultModel:       "glm-5.3",
		},
	}

	ids := runGatewayModelsForTestWithService(t, apiKey, svc)
	require.Equal(t, []string{"glm-5.3", "kimi-k2.5", "MiniMax-M2.5"}, ids)
	require.NotEqual(t, service.OpenAIDisplaySeed(), ids)
}

func TestGatewayHandlerModels_PickerEnabledSkipsFailedAccountAndKeepsDefault(t *testing.T) {
	groupID := int64(46)
	svc := service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, nil)
	svc.SetAccountRepo(&pickerAccountRepoStub{
		accounts: []service.Account{
			{ID: 10, Platform: service.PlatformOpenAI},
			{ID: 11, Platform: service.PlatformOpenAI},
		},
	})
	svc.SetUpstreamModelsFetcher(pickerFetcherStub{
		byID: map[int64][]string{
			10: {"glm-5.3"},
		},
		err: map[int64]error{
			11: errPickerUpstream,
		},
	})

	apiKey := &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:                          groupID,
			Platform:                    service.PlatformOpenAI,
			CcsImportModelPickerEnabled: true,
			CcsImportDefaultModel:       "glm-5.3",
		},
	}

	ids := runGatewayModelsForTestWithService(t, apiKey, svc)
	require.Equal(t, []string{"glm-5.3"}, ids)
}

func TestGatewayHandlerModels_PickerEnabledWithoutFetcherKeepsDefaultNotGPTSeed(t *testing.T) {
	groupID := int64(46)
	apiKey := &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:                          groupID,
			Platform:                    service.PlatformOpenAI,
			CcsImportModelPickerEnabled: true,
			CcsImportDefaultModel:       "glm-5.3",
		},
	}

	ids := runGatewayModelsForTest(t, apiKey)
	require.Equal(t, []string{"glm-5.3"}, ids)
}

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
	require.Equal(t, service.OpenAIDisplaySeed(), ids)
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
	require.Equal(t, "gpt-6-sol", first.ID)
	require.ElementsMatch(t, []string{"openai-response", "openai", "openai-response-compact"}, first.SupportedEndpointTypes)
	require.ElementsMatch(t, []string{"chat_completions", "responses"}, first.SupportedSessionModes)
	require.Equal(t, "gpt-6-sol", first.ActualModelReturned["chat_completions"])
	require.Equal(t, "gpt-6-sol", first.ActualModelReturned["responses"])
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
	require.Equal(t, []string{"gpt-image-2", "gpt-5.6-terra", "gpt-5.4-mini", "gpt-5.5"}, ids)
}

func TestGatewayHandlerModels_OpenAICustomListKeepsDomesticModelIDs(t *testing.T) {
	groupID := int64(1)
	apiKey := &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformOpenAI,
			ModelsListConfig: service.GroupModelsListConfig{
				Enabled: true,
				Models:  []string{"glm-5.3", "kimi-k2.5", "deepseek-v4-flash"},
			},
		},
	}

	ids := runGatewayModelsForTest(t, apiKey)
	require.Equal(t, []string{"glm-5.3", "kimi-k2.5", "deepseek-v4-flash"}, ids)
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
	require.Equal(t, service.OpenAIDisplaySeed(), ids)
}

func TestGatewayHandlerModels_AnthropicCuratedDiscoveryList(t *testing.T) {
	groupID := int64(3)
	apiKey := &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformAnthropic,
		},
	}

	ids := runGatewayModelsForTest(t, apiKey)
	require.Equal(t, service.PlatformDisplaySeed(service.PlatformAnthropic), ids)
}

func TestGatewayHandlerModels_GeminiCuratedDiscoveryList(t *testing.T) {
	groupID := int64(4)
	apiKey := &service.APIKey{
		GroupID: &groupID,
		Group: &service.Group{
			ID:       groupID,
			Platform: service.PlatformGemini,
		},
	}

	ids := runGatewayModelsForTest(t, apiKey)
	require.Equal(t, service.PlatformDisplaySeed(service.PlatformGemini), ids)
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
		"claude-opus-5",
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
		"claude-opus-5",
		"claude-opus-4-8",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-haiku-4-5",
		"claude-sonnet-4-6",
	}, decodeModelIDsForTest(t, recorder.Body.Bytes()))
}

func runGatewayModelsForTest(t *testing.T, apiKey *service.APIKey) []string {
	t.Helper()
	return runGatewayModelsForTestWithService(t, apiKey, nil)
}

func runGatewayModelsForTestWithService(t *testing.T, apiKey *service.APIKey, apiKeyService *service.APIKeyService) []string {
	t.Helper()

	entries := runGatewayModelEntriesForTest(t, apiKey, apiKeyService)
	ids := make([]string, 0, len(entries))
	for _, model := range entries {
		ids = append(ids, model.ID)
	}
	return ids
}

func runGatewayModelEntriesForTest(t *testing.T, apiKey *service.APIKey, apiKeyService ...*service.APIKeyService) []modelEntryForTest {
	t.Helper()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set(string(middleware2.ContextKeyAPIKey), apiKey)

	h := &GatewayHandler{}
	if len(apiKeyService) > 0 {
		h.apiKeyService = apiKeyService[0]
	}
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
