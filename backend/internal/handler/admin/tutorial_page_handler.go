package admin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	SettingKeyTutorialContent = "tutorial_page.content"
	tutorialMaxContentBytes   = 1024 * 1024      // 1 MB markdown
	tutorialMaxImageBytes     = 10 * 1024 * 1024 // 10 MB per image
	tutorialImageDir          = "data/tutorial-images"
)

var allowedImageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true,
}

type TutorialPageHandler struct {
	settingRepo service.SettingRepository
}

func NewTutorialPageHandler(settingRepo service.SettingRepository) *TutorialPageHandler {
	return &TutorialPageHandler{settingRepo: settingRepo}
}

type tutorialContentDTO struct {
	Content string `json:"content"`
}

func (h *TutorialPageHandler) Get(c *gin.Context) {
	val, err := h.settingRepo.GetValue(c.Request.Context(), SettingKeyTutorialContent)
	if err != nil {
		if infraerrors.IsNotFound(err) {
			response.Success(c, tutorialContentDTO{Content: ""})
			return
		}
		response.ErrorFrom(c, infraerrors.InternalServer("LOAD_FAILED", err.Error()))
		return
	}
	response.Success(c, tutorialContentDTO{Content: val})
}

func (h *TutorialPageHandler) Update(c *gin.Context) {
	var req tutorialContentDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	content := req.Content
	if len(content) > tutorialMaxContentBytes {
		response.ErrorFrom(c, infraerrors.BadRequest("CONTENT_TOO_LARGE", "tutorial content exceeds 1MB limit"))
		return
	}

	if err := h.settingRepo.Set(c.Request.Context(), SettingKeyTutorialContent, content); err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("UPDATE_FAILED", err.Error()))
		return
	}
	response.Success(c, tutorialContentDTO{Content: content})
}

func (h *TutorialPageHandler) UploadImage(c *gin.Context) {
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("NO_FILE", "no image file in request"))
		return
	}
	defer func() { _ = file.Close() }()

	if header.Size > tutorialMaxImageBytes {
		response.ErrorFrom(c, infraerrors.BadRequest("FILE_TOO_LARGE", "image exceeds 10MB limit"))
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedImageExts[ext] {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_TYPE", "allowed types: png, jpg, jpeg, gif, webp, svg"))
		return
	}

	if err := os.MkdirAll(tutorialImageDir, 0o755); err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("MKDIR_FAILED", err.Error()))
		return
	}

	randBytes := make([]byte, 16)
	if _, err := rand.Read(randBytes); err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("RAND_FAILED", err.Error()))
		return
	}
	filename := hex.EncodeToString(randBytes) + ext
	destPath := filepath.Join(tutorialImageDir, filename)

	dst, err := os.Create(destPath)
	if err != nil {
		response.ErrorFrom(c, infraerrors.InternalServer("CREATE_FAILED", err.Error()))
		return
	}
	dstClosed := false
	defer func() {
		if !dstClosed {
			_ = dst.Close()
		}
	}()

	if _, err := io.Copy(dst, file); err != nil {
		closeErr := dst.Close()
		dstClosed = true
		if closeErr != nil {
			err = fmt.Errorf("%w; close upload destination: %v", err, closeErr)
		}
		_ = os.Remove(destPath)
		response.ErrorFrom(c, infraerrors.InternalServer("WRITE_FAILED", err.Error()))
		return
	}
	if err := dst.Close(); err != nil {
		dstClosed = true
		_ = os.Remove(destPath)
		response.ErrorFrom(c, infraerrors.InternalServer("WRITE_FAILED", err.Error()))
		return
	}
	dstClosed = true

	url := fmt.Sprintf("/assets/tutorial/%s", filename)
	c.JSON(http.StatusOK, gin.H{
		"url":      url,
		"filename": filename,
	})
}
