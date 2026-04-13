package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
)

// GeminiOAuthClient performs Google OAuth token exchange/refresh for Gemini integration.
type GeminiOAuthClient interface {
	ExchangeCode(ctx context.Context, oauthType, code, codeVerifier, redirectURI, proxyURL string) (*geminicli.TokenResponse, error)
	RefreshToken(ctx context.Context, oauthType, refreshToken, proxyURL string) (*geminicli.TokenResponse, error)
	// GetUserInfo calls Google's userinfo endpoint with the given access token to fetch
	// the profile (email/name/picture). Requires the userinfo.email scope.
	GetUserInfo(ctx context.Context, accessToken, proxyURL string) (*geminicli.UserInfo, error)
}
