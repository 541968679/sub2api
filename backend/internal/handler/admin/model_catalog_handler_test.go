package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type catalogSettingRepoStub struct {
	values map[string]string
}

func (s *catalogSettingRepoStub) Get(ctx context.Context, key string) (*service.Setting, error) {
	panic("unexpected Get")
}

func (s *catalogSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", service.ErrSettingNotFound
}

func (s *catalogSettingRepoStub) Set(ctx context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *catalogSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (s *catalogSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	return nil
}

func (s *catalogSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	return s.values, nil
}

func (s *catalogSettingRepoStub) Delete(ctx context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func TestModelPricingHandlerOpenAIModelCatalogGetSeed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewSettingService(&catalogSettingRepoStub{values: map[string]string{}}, &config.Config{})
	h := NewModelPricingHandler(nil, svc)
	router := gin.New()
	router.GET("/api/v1/admin/model-catalog/openai", h.GetOpenAIModelCatalog)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/model-catalog/openai", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data service.OpenAIModelCatalog `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, service.OpenAIDisplaySeed(), resp.Data.DisplayModels)
	require.Contains(t, resp.Data.WhitelistModels, service.OpenAIModelGPT6Astra)
	require.NotContains(t, resp.Data.WhitelistModels, "grok-4.5")
}

func TestModelPricingHandlerOpenAIModelCatalogPut(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &catalogSettingRepoStub{values: map[string]string{}}
	svc := service.NewSettingService(repo, &config.Config{})
	h := NewModelPricingHandler(nil, svc)
	router := gin.New()
	router.PUT("/api/v1/admin/model-catalog/openai", h.UpdateOpenAIModelCatalog)

	body, _ := json.Marshal(map[string]any{
		"display_models":   []string{"gpt-6-astra"},
		"whitelist_models": []string{"gpt-6-astra", "gpt-5.6-sol"},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/model-catalog/openai", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	got := svc.GetOpenAIModelCatalog(context.Background())
	require.Equal(t, []string{"gpt-6-astra"}, got.DisplayModels)
	require.Equal(t, []string{"gpt-6-astra", "gpt-5.6-sol"}, got.WhitelistModels)
}
