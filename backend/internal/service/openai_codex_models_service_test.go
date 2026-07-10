package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func newCodexModelsTestAccount() *Account {
	return &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "test-access-token",
			"chatgpt_account_id": "acc-123",
		},
	}
}

func TestFetchCodexModelsManifestPassthrough(t *testing.T) {
	manifestBody := `{"models":[{"slug":"gpt-5.6-sol","display_name":"GPT-5.6-Sol"}]}`

	var gotAuth, gotAccountID, gotOriginator, gotVersionHeader, gotClientVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccountID = r.Header.Get("chatgpt-account-id")
		gotOriginator = r.Header.Get("Originator")
		gotVersionHeader = r.Header.Get("Version")
		gotClientVersion = r.URL.Query().Get("client_version")
		w.Header().Set("ETag", `W/"abc123"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(manifestBody))
	}))
	defer server.Close()

	original := chatgptCodexModelsURL
	chatgptCodexModelsURL = server.URL
	defer func() { chatgptCodexModelsURL = original }()

	s := &OpenAIGatewayService{}
	manifest, err := s.FetchCodexModelsManifest(context.Background(), newCodexModelsTestAccount(), "0.144.1", "")
	require.NoError(t, err)
	require.Equal(t, manifestBody, string(manifest.Body))
	require.Equal(t, `W/"abc123"`, manifest.ETag)
	require.Equal(t, "Bearer test-access-token", gotAuth)
	require.Equal(t, "acc-123", gotAccountID)
	require.Equal(t, "codex_cli_rs", gotOriginator)
	require.Equal(t, "0.144.1", gotVersionHeader)
	require.Equal(t, "0.144.1", gotClientVersion)
}

func TestFetchCodexModelsManifestDefaultClientVersion(t *testing.T) {
	var gotClientVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClientVersion = r.URL.Query().Get("client_version")
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	original := chatgptCodexModelsURL
	chatgptCodexModelsURL = server.URL
	defer func() { chatgptCodexModelsURL = original }()

	s := &OpenAIGatewayService{}
	_, err := s.FetchCodexModelsManifest(context.Background(), newCodexModelsTestAccount(), "", "")
	require.NoError(t, err)
	require.Equal(t, openAICodexProbeVersion, gotClientVersion)
}

func TestFetchCodexModelsManifestNotModified(t *testing.T) {
	var gotIfNoneMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIfNoneMatch = r.Header.Get("If-None-Match")
		w.Header().Set("ETag", `W/"abc123"`)
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	original := chatgptCodexModelsURL
	chatgptCodexModelsURL = server.URL
	defer func() { chatgptCodexModelsURL = original }()

	s := &OpenAIGatewayService{}
	manifest, err := s.FetchCodexModelsManifest(context.Background(), newCodexModelsTestAccount(), "0.144.1", `W/"abc123"`)
	require.NoError(t, err)
	require.True(t, manifest.NotModified)
	require.Equal(t, `W/"abc123"`, manifest.ETag)
	require.Equal(t, `W/"abc123"`, gotIfNoneMatch)
}

func TestFetchCodexModelsManifestUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"detail":"boom"}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	original := chatgptCodexModelsURL
	chatgptCodexModelsURL = server.URL
	defer func() { chatgptCodexModelsURL = original }()

	s := &OpenAIGatewayService{}
	_, err := s.FetchCodexModelsManifest(context.Background(), newCodexModelsTestAccount(), "0.144.1", "")
	require.Error(t, err)
}

func TestFetchCodexModelsManifestRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", int(codexModelsManifestBodyLimit)+1)))
	}))
	defer server.Close()

	original := chatgptCodexModelsURL
	chatgptCodexModelsURL = server.URL
	defer func() { chatgptCodexModelsURL = original }()

	s := &OpenAIGatewayService{}
	_, err := s.FetchCodexModelsManifest(context.Background(), newCodexModelsTestAccount(), "0.144.1", "")
	require.Error(t, err)
}

func TestFetchCodexModelsManifestRejectsMissingToken(t *testing.T) {
	account := newCodexModelsTestAccount()
	delete(account.Credentials, "access_token")

	s := &OpenAIGatewayService{}
	_, err := s.FetchCodexModelsManifest(context.Background(), account, "0.144.1", "")
	require.Error(t, err)
}

func TestFetchCodexModelsManifestRejectsAPIKeyAccount(t *testing.T) {
	account := &Account{
		ID:          2,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test-api-key"},
	}

	s := &OpenAIGatewayService{}
	_, err := s.FetchCodexModelsManifest(context.Background(), account, "0.144.1", "")
	require.Error(t, err)
}

func TestSelectAccountForCodexModelsSkipsAPIKeyAccounts(t *testing.T) {
	groupID := int64(1)
	apiKeyAccount := Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Priority:    0,
	}
	oauthAccount := *newCodexModelsTestAccount()
	oauthAccount.ID = 2
	oauthAccount.Status = StatusActive
	oauthAccount.Schedulable = true
	oauthAccount.Priority = 10

	s := &OpenAIGatewayService{
		accountRepo: stubOpenAIAccountRepo{accounts: []Account{apiKeyAccount, oauthAccount}},
	}
	selected, err := s.SelectAccountForCodexModels(context.Background(), &groupID)
	require.NoError(t, err)
	require.Equal(t, oauthAccount.ID, selected.ID)
}

func TestSelectAccountForCodexModelsRejectsAPIKeyOnlyGroup(t *testing.T) {
	groupID := int64(1)
	s := &OpenAIGatewayService{
		accountRepo: stubOpenAIAccountRepo{accounts: []Account{{
			ID:          1,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
		}}},
	}

	_, err := s.SelectAccountForCodexModels(context.Background(), &groupID)
	require.Error(t, err)
}
