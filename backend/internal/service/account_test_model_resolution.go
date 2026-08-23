package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/gin-gonic/gin"
)

const (
	AccountTestMappingSourceAccount         = ModelMappingSourceAccount
	AccountTestMappingSourcePlatformDefault = ModelMappingSourcePlatformDefault
	AccountTestMappingSourcePrefix          = "prefix"
	AccountTestMappingSourceVertex          = "vertex"
	AccountTestMappingSourceNone            = "none"
)

type accountTestModelResolution struct {
	Selected string
	Mapped   string
	Source   string
}

func defaultAccountTestModelID(account *Account) string {
	if account == nil {
		return claude.DefaultTestModel
	}
	switch {
	case account.IsOpenAI():
		return openai.DefaultTestModel
	case account.IsGemini():
		return geminicli.DefaultTestModel
	case account.IsGrok():
		return grokDefaultResponsesModel
	case account.Platform == PlatformAntigravity:
		return "claude-sonnet-4-5"
	default:
		return claude.DefaultTestModel
	}
}

// resolveAccountTestModel mirrors the live gateway rewrite for the same account
// type. Anthropic API Key accounts are never given an extra NormalizeModelID.
func resolveAccountTestModel(account *Account, requested string) accountTestModelResolution {
	selected := strings.TrimSpace(requested)
	if selected == "" {
		selected = defaultAccountTestModelID(account)
	}
	resolution := accountTestModelResolution{
		Selected: selected,
		Mapped:   selected,
		Source:   AccountTestMappingSourceNone,
	}
	if account == nil {
		return resolution
	}

	if account.Platform == PlatformAnthropic && account.Type == AccountTypeServiceAccount {
		if mapped, matched := account.ResolveMappedModel(selected); matched {
			resolution.Mapped = mapped
			resolution.Source = AccountTestMappingSourceAccount
			return resolution
		}
		normalized := normalizeVertexAnthropicModelID(claude.NormalizeModelID(selected))
		if normalized != selected {
			resolution.Mapped = normalized
			resolution.Source = AccountTestMappingSourceVertex
		}
		return resolution
	}

	if account.Platform == PlatformAnthropic && account.Type != AccountTypeAPIKey {
		normalized := claude.NormalizeModelID(selected)
		if normalized != selected {
			resolution.Mapped = normalized
			resolution.Source = AccountTestMappingSourcePrefix
		}
		return resolution
	}

	detailed := account.ResolveMappedModelDetailed(selected)
	if mapped := strings.TrimSpace(detailed.MappedModel); mapped != "" {
		resolution.Mapped = mapped
	}
	if detailed.Matched && strings.TrimSpace(detailed.Source) != "" {
		resolution.Source = detailed.Source
	}
	return resolution
}

func (s *AccountTestService) sendTestStart(c *gin.Context, account *Account, resolution accountTestModelResolution) {
	accountID := int64(0)
	if account != nil {
		accountID = account.ID
	}
	logger.LegacyPrintf(
		"service.account_test",
		"account_test account_id=%d selected=%s mapped=%s source=%s",
		accountID,
		resolution.Selected,
		resolution.Mapped,
		resolution.Source,
	)
	s.sendEvent(c, TestEvent{
		Type:          "test_start",
		Model:         resolution.Mapped,
		SelectedModel: resolution.Selected,
		MappedModel:   resolution.Mapped,
		MappingSource: resolution.Source,
	})
}
