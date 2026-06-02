package service

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const openAIImagesRequestContextGinKey = "openai_images_request_context"

type OpenAIImagesRequestContext struct {
	Endpoint string
}

func BindOpenAIImagesRequestContext(c *gin.Context, parsed *OpenAIImagesRequest) {
	if c == nil || parsed == nil {
		return
	}
	endpoint := strings.TrimSpace(parsed.Endpoint)
	if endpoint == "" {
		return
	}
	c.Set(openAIImagesRequestContextGinKey, &OpenAIImagesRequestContext{
		Endpoint: endpoint,
	})
}

func OpenAIImagesRequestContextFromGin(c *gin.Context) *OpenAIImagesRequestContext {
	if c == nil {
		return nil
	}
	v, ok := c.Get(openAIImagesRequestContextGinKey)
	if !ok {
		return nil
	}
	ctx, _ := v.(*OpenAIImagesRequestContext)
	return ctx
}

func IsOpenAIImagesRequest(c *gin.Context) bool {
	ctx := OpenAIImagesRequestContextFromGin(c)
	if ctx == nil {
		return false
	}
	switch strings.TrimSpace(ctx.Endpoint) {
	case openAIImagesGenerationsEndpoint, openAIImagesEditsEndpoint:
		return true
	default:
		return false
	}
}
