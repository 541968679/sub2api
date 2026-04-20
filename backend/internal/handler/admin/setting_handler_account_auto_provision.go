package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"time"
)

func (h *SettingHandler) GetAccountAutoProvisionSettings(c *gin.Context) {
	settings, err := h.settingService.GetAccountAutoProvisionSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	state, err := h.settingService.GetAccountAutoProvisionState(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountAutoProvisionSettingsResponse{
		Config: mapAccountAutoProvisionSettingsDTO(settings),
		State:  mapAccountAutoProvisionStateDTO(state),
	})
}

type UpdateAccountAutoProvisionSettingsRequest struct {
	Enabled              bool                           `json:"enabled"`
	CheckIntervalSeconds int                            `json:"check_interval_seconds"`
	MaxActionsPerRun     int                            `json:"max_actions_per_run"`
	Rules                []dto.AccountAutoProvisionRule `json:"rules"`
}

func (h *SettingHandler) UpdateAccountAutoProvisionSettings(c *gin.Context) {
	var req UpdateAccountAutoProvisionSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.AccountAutoProvisionSettings{
		Enabled:              req.Enabled,
		CheckIntervalSeconds: req.CheckIntervalSeconds,
		MaxActionsPerRun:     req.MaxActionsPerRun,
		Rules:                make([]service.AccountAutoProvisionRule, 0, len(req.Rules)),
	}
	for _, rule := range req.Rules {
		settings.Rules = append(settings.Rules, service.AccountAutoProvisionRule{
			ID:                            rule.ID,
			Name:                          rule.Name,
			Enabled:                       rule.Enabled,
			GroupIDs:                      rule.GroupIDs,
			NormalAccountCountBelow:       rule.NormalAccountCountBelow,
			ConcurrencyUtilizationAbove:   rule.ConcurrencyUtilizationAbove,
			AICreditsBelow:                rule.AICreditsBelow,
			AICreditsCheckIntervalMinutes: rule.AICreditsCheckIntervalMinutes,
			CooldownMinutes:               rule.CooldownMinutes,
			ProvisionMode:                 rule.ProvisionMode,
			Template: service.AccountAutoProvisionTemplate{
				ProxyID:       rule.Template.ProxyID,
				Concurrency:   rule.Template.Concurrency,
				Priority:      rule.Template.Priority,
				LoadFactor:    rule.Template.LoadFactor,
				Schedulable:   rule.Template.Schedulable,
				AllowOverages: rule.Template.AllowOverages,
			},
		})
	}

	if err := h.settingService.SetAccountAutoProvisionSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	updatedSettings, err := h.settingService.GetAccountAutoProvisionSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	state, err := h.settingService.GetAccountAutoProvisionState(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AccountAutoProvisionSettingsResponse{
		Config: mapAccountAutoProvisionSettingsDTO(updatedSettings),
		State:  mapAccountAutoProvisionStateDTO(state),
	})
}

func mapAccountAutoProvisionSettingsDTO(settings *service.AccountAutoProvisionSettings) dto.AccountAutoProvisionSettings {
	if settings == nil {
		settings = service.DefaultAccountAutoProvisionSettings()
	}
	result := dto.AccountAutoProvisionSettings{
		Enabled:              settings.Enabled,
		CheckIntervalSeconds: settings.CheckIntervalSeconds,
		MaxActionsPerRun:     settings.MaxActionsPerRun,
		Rules:                make([]dto.AccountAutoProvisionRule, 0, len(settings.Rules)),
	}
	for _, rule := range settings.Rules {
		result.Rules = append(result.Rules, dto.AccountAutoProvisionRule{
			ID:                            rule.ID,
			Name:                          rule.Name,
			Enabled:                       rule.Enabled,
			GroupIDs:                      rule.GroupIDs,
			NormalAccountCountBelow:       rule.NormalAccountCountBelow,
			ConcurrencyUtilizationAbove:   rule.ConcurrencyUtilizationAbove,
			AICreditsBelow:                rule.AICreditsBelow,
			AICreditsCheckIntervalMinutes: rule.AICreditsCheckIntervalMinutes,
			CooldownMinutes:               rule.CooldownMinutes,
			ProvisionMode:                 rule.ProvisionMode,
			Template: dto.AccountAutoProvisionTemplate{
				ProxyID:       rule.Template.ProxyID,
				Concurrency:   rule.Template.Concurrency,
				Priority:      rule.Template.Priority,
				LoadFactor:    rule.Template.LoadFactor,
				Schedulable:   rule.Template.Schedulable,
				AllowOverages: rule.Template.AllowOverages,
			},
		})
	}
	return result
}

func mapAccountAutoProvisionStateDTO(state *service.AccountAutoProvisionState) dto.AccountAutoProvisionState {
	if state == nil {
		state = service.DefaultAccountAutoProvisionState()
	}
	result := dto.AccountAutoProvisionState{
		LastTriggered:        state.LastTriggered,
		LastHealthySnapshots: make(map[string]dto.AccountAutoProvisionSnapshot, len(state.LastHealthySnapshots)),
		RecentLogs:           make([]dto.AccountAutoProvisionLogEntry, 0, len(state.RecentLogs)),
	}
	for key, snapshot := range state.LastHealthySnapshots {
		result.LastHealthySnapshots[key] = dto.AccountAutoProvisionSnapshot{
			SourceAccountID:   snapshot.SourceAccountID,
			SourceAccountName: snapshot.SourceAccountName,
			CapturedAt:        snapshot.CapturedAt,
			Template: dto.AccountAutoProvisionTemplate{
				ProxyID:       snapshot.Template.ProxyID,
				Concurrency:   snapshot.Template.Concurrency,
				Priority:      snapshot.Template.Priority,
				LoadFactor:    snapshot.Template.LoadFactor,
				Schedulable:   snapshot.Template.Schedulable,
				AllowOverages: snapshot.Template.AllowOverages,
			},
		}
	}
	for _, entry := range state.RecentLogs {
		result.RecentLogs = append(result.RecentLogs, dto.AccountAutoProvisionLogEntry{
			OccurredAt:  entry.OccurredAt,
			Level:       entry.Level,
			Action:      entry.Action,
			RuleID:      entry.RuleID,
			RuleName:    entry.RuleName,
			GroupID:     entry.GroupID,
			GroupName:   entry.GroupName,
			AccountID:   entry.AccountID,
			AccountName: entry.AccountName,
			Message:     entry.Message,
		})
	}
	if result.LastTriggered == nil {
		result.LastTriggered = map[string]time.Time{}
	}
	if result.RecentLogs == nil {
		result.RecentLogs = []dto.AccountAutoProvisionLogEntry{}
	}
	return result
}
