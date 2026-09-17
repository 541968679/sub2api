package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

func RegisterModelPlazaRoutes(
	v1 *gin.RouterGroup,
	h *handler.Handlers,
	optionalJWT middleware.OptionalJWTAuthMiddleware,
	settingService *service.SettingService,
) {
	if h == nil || h.ModelPlaza == nil {
		return
	}
	plaza := v1.Group("/model-plaza")
	plaza.Use(gin.HandlerFunc(optionalJWT))
	plaza.Use(middleware.BackendModeUserGuard(settingService))
	{
		plaza.GET("", h.ModelPlaza.Get)
	}
}
