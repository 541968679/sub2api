package handler

import (
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"

	"github.com/gin-gonic/gin"
)

type TutorialPageHandler struct {
	settingRepo service.SettingRepository
}

func NewTutorialPageHandler(settingRepo service.SettingRepository) *TutorialPageHandler {
	return &TutorialPageHandler{settingRepo: settingRepo}
}

func (h *TutorialPageHandler) Get(c *gin.Context) {
	val, err := h.settingRepo.GetValue(c.Request.Context(), admin.SettingKeyTutorialContent)
	if err != nil {
		if infraerrors.IsNotFound(err) {
			response.Success(c, gin.H{"content": ""})
			return
		}
		response.ErrorFrom(c, infraerrors.InternalServer("LOAD_FAILED", err.Error()))
		return
	}
	response.Success(c, gin.H{"content": val})
}
