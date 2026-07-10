package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIImagesShouldPenalizeAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	active, _ := gin.CreateTestContext(httptest.NewRecorder())
	active.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	require.True(t, openAIImagesShouldPenalizeAccount(active, errors.New("upstream failed")))

	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()
	canceled, _ := gin.CreateTestContext(httptest.NewRecorder())
	canceled.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil).WithContext(canceledContext)
	require.False(t, openAIImagesShouldPenalizeAccount(canceled, context.Canceled))
}
