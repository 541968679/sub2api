package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// PlazaOfficialPricing is catalog/list reference (LiteLLM), not usage cost/tokens.
type PlazaOfficialPricing struct {
	InputPrice     *float64
	OutputPrice    *float64
	CacheReadPrice *float64
}

// PlazaModelPricing is the user-facing display-price column.
type PlazaModelPricing struct {
	InputPrice     *float64
	OutputPrice    *float64
	CacheReadPrice *float64
}

// PlazaModel is one model row in the plaza showcase.
type PlazaModel struct {
	Name            string
	Platform        string
	Pricing         *PlazaModelPricing
	OfficialPricing *PlazaOfficialPricing
}

// PlazaGroup is a group-centric plaza section.
type PlazaGroup struct {
	ID             int64
	Name           string
	Description    string
	Platform       string
	SubscriptionType string
	RateMultiplier float64
	IsExclusive    bool
	Models         []PlazaModel
}

type plazaGroupLister interface {
	ListActive(ctx context.Context) ([]Group, error)
}

type plazaDisplayCatalog interface {
	ListForPricingPage(ctx context.Context) ([]GlobalModelPricing, error)
}

// ModelPlazaService lists groups and models using the fork display-price chain.
type ModelPlazaService struct {
	groupRepo      plazaGroupLister
	modelPricing   plazaDisplayCatalog
	pricingService *PricingService
}

func NewModelPlazaService(
	groupRepo plazaGroupLister,
	modelPricing plazaDisplayCatalog,
	pricingService *PricingService,
) *ModelPlazaService {
	return &ModelPlazaService{
		groupRepo:      groupRepo,
		modelPricing:   modelPricing,
		pricingService: pricingService,
	}
}

func (s *ModelPlazaService) ListGroups(ctx context.Context) ([]PlazaGroup, error) {
	if s == nil || s.groupRepo == nil {
		return nil, fmt.Errorf("model plaza is not configured")
	}
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups: %w", err)
	}
	catalog := []GlobalModelPricing{}
	if s.modelPricing != nil {
		catalog, err = s.modelPricing.ListForPricingPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list display pricing catalog: %w", err)
		}
	}

	out := make([]PlazaGroup, 0, len(groups))
	for i := range groups {
		g := &groups[i]
		models := plazaModelsForGroup(g, catalog, s.pricingService)
		if len(models) == 0 {
			continue
		}
		out = append(out, PlazaGroup{
			ID:               g.ID,
			Name:             g.Name,
			Description:      g.Description,
			Platform:         g.Platform,
			SubscriptionType: g.SubscriptionType,
			RateMultiplier:   g.RateMultiplier,
			IsExclusive:      g.IsExclusive,
			Models:           models,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].RateMultiplier != out[j].RateMultiplier {
			return out[i].RateMultiplier < out[j].RateMultiplier
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func plazaModelsForGroup(g *Group, catalog []GlobalModelPricing, pricingService *PricingService) []PlazaModel {
	if g == nil {
		return nil
	}
	allow := map[string]struct{}{}
	for _, name := range g.AllowedModels {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			allow[name] = struct{}{}
		}
	}
	models := make([]PlazaModel, 0)
	seen := map[string]struct{}{}
	for i := range catalog {
		p := &catalog[i]
		if !plazaCatalogMatchesGroup(g, p) {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(p.Model))
		if key == "" {
			continue
		}
		if len(allow) > 0 {
			if _, ok := allow[key]; !ok {
				continue
			}
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		models = append(models, PlazaModel{
			Name:            p.Model,
			Platform:        firstNonEmpty(p.Provider, g.Platform),
			Pricing:         plazaDisplayPricing(p),
			OfficialPricing: plazaOfficialPricing(p.Model, pricingService),
		})
	}
	sort.SliceStable(models, func(i, j int) bool { return models[i].Name < models[j].Name })
	return models
}

func plazaCatalogMatchesGroup(g *Group, p *GlobalModelPricing) bool {
	if g == nil || p == nil {
		return false
	}
	if g.Platform == PlatformComposite || strings.TrimSpace(g.Platform) == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(p.Provider), strings.TrimSpace(g.Platform))
}

func plazaDisplayPricing(p *GlobalModelPricing) *PlazaModelPricing {
	if p == nil {
		return nil
	}
	in := firstNonNilFloat(p.DisplayInputPrice, p.InputPrice)
	out := firstNonNilFloat(p.DisplayOutputPrice, p.OutputPrice)
	cache := firstNonNilFloat(p.DisplayCacheReadPrice, p.CacheReadPrice)
	if in == nil && out == nil && cache == nil {
		return nil
	}
	return &PlazaModelPricing{InputPrice: in, OutputPrice: out, CacheReadPrice: cache}
}

func plazaOfficialPricing(model string, pricingService *PricingService) *PlazaOfficialPricing {
	if pricingService == nil {
		return nil
	}
	mp := pricingService.GetModelPricing(model)
	if mp == nil {
		return nil
	}
	official := &PlazaOfficialPricing{
		InputPrice:     plazaNonZeroPtr(mp.InputCostPerToken),
		OutputPrice:    plazaNonZeroPtr(mp.OutputCostPerToken),
		CacheReadPrice: plazaNonZeroPtr(mp.CacheReadInputTokenCost),
	}
	if official.InputPrice == nil && official.OutputPrice == nil && official.CacheReadPrice == nil {
		return nil
	}
	return official
}

func firstNonNilFloat(values ...*float64) *float64 {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func plazaNonZeroPtr(v float64) *float64 {
	if v == 0 {
		return nil
	}
	out := v
	return &out
}
