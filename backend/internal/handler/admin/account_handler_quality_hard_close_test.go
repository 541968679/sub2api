//go:build unit

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

type qualityHardCloseSettingRepo struct {
	data map[string]string
}

func (r *qualityHardCloseSettingRepo) Get(_ context.Context, key string) (*service.Setting, error) {
	if r.data != nil {
		if value, ok := r.data[key]; ok {
			return &service.Setting{Key: key, Value: value}, nil
		}
	}
	return nil, service.ErrSettingNotFound
}

func (r *qualityHardCloseSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	if r.data != nil {
		if value, ok := r.data[key]; ok {
			return value, nil
		}
	}
	return "", nil
}

func (r *qualityHardCloseSettingRepo) Set(_ context.Context, key, value string) error {
	if r.data == nil {
		r.data = map[string]string{}
	}
	r.data[key] = value
	return nil
}

func (r *qualityHardCloseSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if r.data != nil {
			if value, ok := r.data[key]; ok {
				out[key] = value
			}
		}
	}
	return out, nil
}

func (r *qualityHardCloseSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	if r.data == nil {
		r.data = map[string]string{}
	}
	for key, value := range settings {
		r.data[key] = value
	}
	return nil
}

func (r *qualityHardCloseSettingRepo) GetAll(_ context.Context) (map[string]string, error) {
	out := make(map[string]string, len(r.data))
	for key, value := range r.data {
		out[key] = value
	}
	return out, nil
}

func (r *qualityHardCloseSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.data, key)
	return nil
}

func newQualityHardCloseAccountHandler(adminSvc *stubAdminService) *AccountHandler {
	if adminSvc == nil {
		adminSvc = newStubAdminService()
	}
	settings := service.NewSettingService(&qualityHardCloseSettingRepo{}, &config.Config{})
	return NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, settings, nil)
}

func TestAccountHandler_GetQualityHardClose_Defaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newQualityHardCloseAccountHandler(nil)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/12/quality-hard-close", nil)
	c.Params = gin.Params{{Key: "id", Value: "12"}}

	handler.GetQualityHardClose(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var body struct {
		Data service.AccountQualityHardCloseView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.False(t, body.Data.Overlay.Enabled)
	require.True(t, body.Data.Overlay.UseGlobal)
	require.False(t, body.Data.GlobalEnabled)
	require.False(t, body.Data.Resolved.Enabled)
	require.Equal(t, 30, body.Data.Resolved.PauseMinutes)
}

func TestAccountHandler_UpdateQualityHardClose_MergesOnlyOverlayKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	handler := newQualityHardCloseAccountHandler(adminSvc)

	body, err := json.Marshal(map[string]any{
		"enabled":    true,
		"use_global": true,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/12/quality-hard-close", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "12"}}

	handler.UpdateQualityHardClose(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, adminSvc.extraUpdates, 1)
	require.Equal(t, int64(12), adminSvc.extraUpdates[0].id)
	require.Len(t, adminSvc.extraUpdates[0].updates, 1)
	_, ok := adminSvc.extraUpdates[0].updates[service.AccountExtraQualityHardClose]
	require.True(t, ok)

	var resp struct {
		Data service.AccountQualityHardCloseView `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.True(t, resp.Data.Overlay.Enabled)
	require.True(t, resp.Data.Overlay.UseGlobal)
}

func TestAccountHandler_UpdateQualityHardClose_InvalidPause(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newQualityHardCloseAccountHandler(nil)
	body, err := json.Marshal(map[string]any{
		"enabled":       true,
		"use_global":    false,
		"pause_minutes": 0,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/accounts/12/quality-hard-close", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "12"}}

	handler.UpdateQualityHardClose(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAccountHandler_GetQualityHardClose_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newQualityHardCloseAccountHandler(nil)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/x/quality-hard-close", nil)
	c.Params = gin.Params{{Key: "id", Value: "x"}}
	handler.GetQualityHardClose(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}
