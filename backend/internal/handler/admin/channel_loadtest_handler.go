package admin

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/loadtest"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ChannelLoadtestHandler struct {
	svc *service.ChannelLoadtestService
}

func NewChannelLoadtestHandler(svc *service.ChannelLoadtestService) *ChannelLoadtestHandler {
	return &ChannelLoadtestHandler{svc: svc}
}

type channelLoadtestStartRequest struct {
	AccountID       *int64          `json:"account_id"`
	BaseURL         string          `json:"base_url"`
	APIKey          string          `json:"api_key"`
	ProxyID         *int64          `json:"proxy_id"`
	Path            string          `json:"path"`
	APIMode         string          `json:"api_mode"`
	StreamMode      string          `json:"stream_mode"`
	Model           string          `json:"model"`
	Models          string          `json:"models"`
	Profile         string          `json:"profile"`
	Concurrency     int             `json:"concurrency"`
	Total           int             `json:"total"`
	DurationSec     int             `json:"duration_sec"`
	TimeoutSec      int             `json:"timeout_sec"`
	MaxTokens       int             `json:"max_tokens"`
	SizeCap         int             `json:"size_cap"`
	Tools           string          `json:"tools"`
	ConfirmCost     bool            `json:"confirm_cost"`
	AbortAfterFirst bool            `json:"abort_after_first"`
	InputTokens     int             `json:"input_tokens"`
	Tiers           []loadtest.Tier `json:"tiers"`
}

func (h *ChannelLoadtestHandler) Start(c *gin.Context) {
	var req channelLoadtestStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	snap, err := h.svc.Start(c.Request.Context(), service.ChannelLoadtestStartInput{
		AccountID:       req.AccountID,
		BaseURL:         req.BaseURL,
		APIKey:          req.APIKey,
		ProxyID:         req.ProxyID,
		Path:            req.Path,
		APIMode:         req.APIMode,
		StreamMode:      req.StreamMode,
		Model:           req.Model,
		Models:          req.Models,
		Profile:         req.Profile,
		Concurrency:     req.Concurrency,
		Total:           req.Total,
		DurationSec:     req.DurationSec,
		TimeoutSec:      req.TimeoutSec,
		MaxTokens:       req.MaxTokens,
		SizeCap:         req.SizeCap,
		Tools:           req.Tools,
		ConfirmCost:     req.ConfirmCost,
		AbortAfterFirst: req.AbortAfterFirst,
		InputTokens:     req.InputTokens,
		Tiers:           req.Tiers,
	})
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "already running") {
			response.Error(c, http.StatusConflict, msg)
			return
		}
		if strings.Contains(msg, "not found") {
			response.NotFound(c, msg)
			return
		}
		response.BadRequest(c, msg)
		return
	}
	response.Accepted(c, snap)
}

func (h *ChannelLoadtestHandler) Latest(c *gin.Context) {
	snap := h.svc.Latest()
	if snap == nil {
		response.Success(c, gin.H{"run": nil})
		return
	}
	response.Success(c, snap)
}

func (h *ChannelLoadtestHandler) Get(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	snap, err := h.svc.Get(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, snap)
}

func (h *ChannelLoadtestHandler) Stop(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	snap, err := h.svc.Stop(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.Success(c, snap)
}

func (h *ChannelLoadtestHandler) Export(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	raw, filename, err := h.svc.ExportExcel(id)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "not found") {
			response.NotFound(c, msg)
			return
		}
		response.BadRequest(c, msg)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", raw)
}
