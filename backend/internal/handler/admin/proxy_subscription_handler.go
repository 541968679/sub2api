package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type parseSubscriptionRequest struct {
	URL             string `json:"url"`
	DefaultProtocol string `json:"default_protocol"`
}

// ParseSubscription fetches a subscription link and returns the HTTP/SOCKS nodes it contains.
// POST /api/v1/admin/proxies/parse-subscription
func (h *ProxyHandler) ParseSubscription(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req parseSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	protocol := strings.ToLower(strings.TrimSpace(req.DefaultProtocol))
	switch protocol {
	case "", "http", "https", "socks5", "socks5h":
	default:
		response.BadRequest(c, "Invalid default protocol")
		return
	}

	body, err := service.FetchProxySubscription(c.Request.Context(), req.URL)
	if err != nil {
		writeSubscriptionError(c, err)
		return
	}
	result := service.ParseProxySubscription(body, protocol)
	if len(result.Proxies) == 0 && result.Unsupported == 0 && service.SubscriptionLooksLikeHTML(body) {
		response.ErrorWithDetails(c, http.StatusBadRequest, service.ErrSubscriptionNotList.Error(), "SUBSCRIPTION_NOT_LIST", nil)
		return
	}
	response.Success(c, result)
}

func writeSubscriptionError(c *gin.Context, err error) {
	var statusErr *service.SubscriptionStatusError
	switch {
	case errors.As(err, &statusErr):
		response.ErrorWithDetails(c, http.StatusBadRequest, statusErr.Error(), "SUBSCRIPTION_BAD_STATUS", map[string]string{
			"status": strconv.Itoa(statusErr.StatusCode),
		})
	case errors.Is(err, service.ErrSubscriptionURLInvalid):
		response.ErrorWithDetails(c, http.StatusBadRequest, err.Error(), "SUBSCRIPTION_URL_INVALID", nil)
	case errors.Is(err, service.ErrSubscriptionTooLarge):
		response.ErrorWithDetails(c, http.StatusBadRequest, err.Error(), "SUBSCRIPTION_TOO_LARGE", nil)
	case errors.Is(err, service.ErrSubscriptionEmpty):
		response.ErrorWithDetails(c, http.StatusBadRequest, err.Error(), "SUBSCRIPTION_EMPTY", nil)
	case errors.Is(err, service.ErrSubscriptionNotList):
		response.ErrorWithDetails(c, http.StatusBadRequest, err.Error(), "SUBSCRIPTION_NOT_LIST", nil)
	default:
		response.ErrorWithDetails(c, http.StatusBadRequest, service.ErrSubscriptionFetchFailed.Error(), "SUBSCRIPTION_FETCH_FAILED", nil)
	}
}
