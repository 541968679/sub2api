package service

import (
	"net/http"
	"strings"
)

const (
	anthropicAPIKeyAuthSchemeExtraKey = "anthropic_apikey_auth_scheme"

	AnthropicAPIKeyAuthSchemeXAPIKey             = "x_api_key"
	AnthropicAPIKeyAuthSchemeAuthorizationBearer = "authorization_bearer"
)

// GetAnthropicAPIKeyAuthScheme returns the upstream auth scheme for Anthropic
// API-key accounts. Missing, invalid, and out-of-scope values preserve x-api-key.
func (a *Account) GetAnthropicAPIKeyAuthScheme() string {
	if a == nil || a.Platform != PlatformAnthropic || a.Type != AccountTypeAPIKey {
		return AnthropicAPIKeyAuthSchemeXAPIKey
	}
	if strings.TrimSpace(a.GetExtraString(anthropicAPIKeyAuthSchemeExtraKey)) == AnthropicAPIKeyAuthSchemeAuthorizationBearer {
		return AnthropicAPIKeyAuthSchemeAuthorizationBearer
	}
	return AnthropicAPIKeyAuthSchemeXAPIKey
}

func setAnthropicAPIKeyAuthHeader(header http.Header, account *Account, token string) {
	deleteHeaderAllForms(header, "authorization")
	deleteHeaderAllForms(header, "x-api-key")
	if account.GetAnthropicAPIKeyAuthScheme() == AnthropicAPIKeyAuthSchemeAuthorizationBearer {
		setHeaderRaw(header, "authorization", "Bearer "+token)
		return
	}
	setHeaderRaw(header, "x-api-key", token)
}
