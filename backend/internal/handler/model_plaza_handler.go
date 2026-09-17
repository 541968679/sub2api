package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelPlazaHandler serves the public model plaza showcase.
// Anonymous users see only non-exclusive groups. Prices are display prices.
type ModelPlazaHandler struct {
	plazaService   *service.ModelPlazaService
	userService    *service.UserService
	settingService *service.SettingService
}

func NewModelPlazaHandler(
	plazaService *service.ModelPlazaService,
	userService *service.UserService,
	settingService *service.SettingService,
) *ModelPlazaHandler {
	return &ModelPlazaHandler{
		plazaService:   plazaService,
		userService:    userService,
		settingService: settingService,
	}
}

type modelPlazaOfficialPricing struct {
	InputPrice     *float64 `json:"input_price"`
	OutputPrice    *float64 `json:"output_price"`
	CacheReadPrice *float64 `json:"cache_read_price"`
}

type modelPlazaModelPricing struct {
	InputPrice     *float64 `json:"input_price"`
	OutputPrice    *float64 `json:"output_price"`
	CacheReadPrice *float64 `json:"cache_read_price"`
}

type modelPlazaModel struct {
	Name            string                     `json:"name"`
	Platform        string                     `json:"platform"`
	Pricing         *modelPlazaModelPricing    `json:"pricing"`
	OfficialPricing *modelPlazaOfficialPricing `json:"official_pricing"`
}

type modelPlazaGroup struct {
	ID               int64             `json:"id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Platform         string            `json:"platform"`
	SubscriptionType string            `json:"subscription_type"`
	RateMultiplier   float64           `json:"rate_multiplier"`
	IsExclusive      bool              `json:"is_exclusive"`
	Models           []modelPlazaModel `json:"models"`
}

type modelPlazaResponse struct {
	Description string            `json:"description"`
	Groups      []modelPlazaGroup `json:"groups"`
}

// Get GET /api/v1/model-plaza
func (h *ModelPlazaHandler) Get(c *gin.Context) {
	if h.settingService == nil {
		response.NotFound(c, "Model plaza is not enabled")
		return
	}
	rt := h.settingService.GetModelPlazaRuntime(c.Request.Context())
	if !rt.Enabled {
		response.NotFound(c, "Model plaza is not enabled")
		return
	}

	subject, authed := middleware.GetAuthSubjectFromContext(c)
	if rt.RequireAuth && !authed {
		response.Unauthorized(c, "Authentication required")
		return
	}

	groups, err := h.plazaService.ListGroups(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	var allowedGroups map[int64]struct{}
	if authed && h.userService != nil {
		user, userErr := h.userService.GetByID(c.Request.Context(), subject.UserID)
		if userErr != nil {
			response.ErrorFrom(c, userErr)
			return
		}
		allowedGroups = map[int64]struct{}{}
		if user != nil {
			for _, id := range user.AllowedGroups {
				allowedGroups[id] = struct{}{}
			}
		}
	}

	visible := filterPlazaVisibleGroups(groups, allowedGroups)
	out := make([]modelPlazaGroup, 0, len(visible))
	for i := range visible {
		out = append(out, toModelPlazaGroupDTO(&visible[i]))
	}
	response.Success(c, modelPlazaResponse{
		Description: rt.Description,
		Groups:      out,
	})
}

func filterPlazaVisibleGroups(groups []service.PlazaGroup, allowedGroups map[int64]struct{}) []service.PlazaGroup {
	visible := make([]service.PlazaGroup, 0, len(groups))
	for _, g := range groups {
		if g.IsExclusive {
			if allowedGroups == nil {
				continue
			}
			if _, ok := allowedGroups[g.ID]; !ok {
				continue
			}
		}
		visible = append(visible, g)
	}
	return visible
}

func toModelPlazaGroupDTO(g *service.PlazaGroup) modelPlazaGroup {
	models := make([]modelPlazaModel, 0, len(g.Models))
	for i := range g.Models {
		m := &g.Models[i]
		models = append(models, modelPlazaModel{
			Name:            m.Name,
			Platform:        m.Platform,
			Pricing:         toPlazaModelPricing(m.Pricing),
			OfficialPricing: toPlazaOfficialPricing(m.OfficialPricing),
		})
	}
	return modelPlazaGroup{
		ID:               g.ID,
		Name:             g.Name,
		Description:      g.Description,
		Platform:         g.Platform,
		SubscriptionType: g.SubscriptionType,
		RateMultiplier:   g.RateMultiplier,
		IsExclusive:      g.IsExclusive,
		Models:           models,
	}
}

func toPlazaModelPricing(p *service.PlazaModelPricing) *modelPlazaModelPricing {
	if p == nil {
		return nil
	}
	return &modelPlazaModelPricing{
		InputPrice:     p.InputPrice,
		OutputPrice:    p.OutputPrice,
		CacheReadPrice: p.CacheReadPrice,
	}
}

func toPlazaOfficialPricing(p *service.PlazaOfficialPricing) *modelPlazaOfficialPricing {
	if p == nil {
		return nil
	}
	return &modelPlazaOfficialPricing{
		InputPrice:     p.InputPrice,
		OutputPrice:    p.OutputPrice,
		CacheReadPrice: p.CacheReadPrice,
	}
}
