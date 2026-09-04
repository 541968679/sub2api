package service

import (
	"context"
	"errors"
	"testing"
)

type stubOpenAITestTokenProvider struct {
	token string
	err   error
	calls int
}

func (s *stubOpenAITestTokenProvider) GetAccessToken(context.Context, *Account) (string, error) {
	s.calls++
	return s.token, s.err
}

func TestResolveOpenAITestAccessTokenUsesProviderForOAuth(t *testing.T) {
	provider := &stubOpenAITestTokenProvider{token: "refreshed-at"}
	svc := &AccountTestService{}
	svc.SetOpenAITokenProvider(provider)
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "stale-at",
		},
	}

	got, err := svc.resolveOpenAITestAccessToken(context.Background(), account)
	if err != nil {
		t.Fatalf("resolveOpenAITestAccessToken error = %v", err)
	}
	if got != "refreshed-at" {
		t.Fatalf("token = %q, want refreshed-at", got)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
}

func TestResolveOpenAITestAccessTokenSkipsPATProvider(t *testing.T) {
	provider := &stubOpenAITestTokenProvider{token: "should-not-use"}
	svc := &AccountTestService{}
	svc.SetOpenAITokenProvider(provider)
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"auth_mode":    OpenAIAuthModePersonalAccessToken,
			"access_token": "pat-at",
		},
	}

	got, err := svc.resolveOpenAITestAccessToken(context.Background(), account)
	if err != nil {
		t.Fatalf("resolveOpenAITestAccessToken error = %v", err)
	}
	if got != "pat-at" {
		t.Fatalf("token = %q, want pat-at", got)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
}

func TestResolveOpenAITestAccessTokenProviderError(t *testing.T) {
	provider := &stubOpenAITestTokenProvider{err: errors.New("refresh failed")}
	svc := &AccountTestService{}
	svc.SetOpenAITokenProvider(provider)
	account := &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "stale-at"},
	}

	_, err := svc.resolveOpenAITestAccessToken(context.Background(), account)
	if err == nil || err.Error() != "refresh failed" {
		t.Fatalf("error = %v, want refresh failed", err)
	}
}
