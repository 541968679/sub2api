//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type modelNotFoundRateLimitCall struct {
	accountID int64
	scope     string
	resetAt   time.Time
}

type modelNotFoundAccountRepoStub struct {
	mockAccountRepoForGemini
	tempCalls           int
	modelRateLimitCalls []modelNotFoundRateLimitCall
	modelRateLimitErr   error
}

func (r *modelNotFoundAccountRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	r.tempCalls++
	return nil
}

func (r *modelNotFoundAccountRepoStub) SetModelRateLimit(ctx context.Context, id int64, scope string, resetAt time.Time) error {
	r.modelRateLimitCalls = append(r.modelRateLimitCalls, modelNotFoundRateLimitCall{
		accountID: id,
		scope:     scope,
		resetAt:   resetAt,
	})
	return r.modelRateLimitErr
}

func openAICodexPlanGatedOAuthAccount() *Account {
	return &Account{
		ID:          202,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{},
	}
}

func TestOpenAIImagesRejectedDriverDoesNotCoolImageModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SUB2API_IMAGES_MAIN_MODEL", "gpt-5.4-mini")
	for _, rejected := range []string{"gpt-5.4-mini", "gpt-image-2.5-flare"} {
		t.Run(rejected, func(t *testing.T) {
			repo := &modelNotFoundAccountRepoStub{}
			svc := &OpenAIGatewayService{rateLimitService: &RateLimitService{accountRepo: repo}}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, openAIImagesGenerationsEndpoint, nil)
			body := fmt.Sprintf(`{"error":{"message":"The '%s' model is not supported when using Codex with a ChatGPT account.","type":"invalid_request_error"}}`, rejected)
			resp := &http.Response{StatusCode: 400, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
			_, err := svc.handleOpenAIImagesErrorResponse(WithOpenAIImagesEndpoint(context.Background()), resp, c, openAICodexPlanGatedOAuthAccount(), "gpt-image-2.5-flare")
			require.Error(t, err)
			if rejected == "gpt-5.4-mini" {
				var upstreamErr *OpenAIImagesUpstreamError
				require.ErrorAs(t, err, &upstreamErr)
				require.Equal(t, 400, upstreamErr.StatusCode)
				require.Contains(t, upstreamErr.Message, rejected)
				require.Empty(t, repo.modelRateLimitCalls)
				require.Zero(t, repo.tempCalls)
			} else {
				require.Len(t, repo.modelRateLimitCalls, 1, "actual image-model rejection still needs bounded failover")
			}
		})
	}
}
