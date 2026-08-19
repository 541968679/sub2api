package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type openAIRefreshTokenKind int

const (
	openAIRefreshTokenKindRaw openAIRefreshTokenKind = iota
	openAIRefreshTokenKindSessionJSON
	openAIRefreshTokenKindEmbeddedRT
	openAIRefreshTokenKindInvalidJSON
)

type classifiedOpenAIRefreshToken struct {
	Kind         openAIRefreshTokenKind
	RefreshToken string
	Object       map[string]any
}

func classifyOpenAIRefreshTokenInput(raw string) classifiedOpenAIRefreshToken {
	raw = strings.TrimSpace(raw)
	if raw == "" || !looksLikeJSON(raw) {
		return classifiedOpenAIRefreshToken{Kind: openAIRefreshTokenKindRaw, RefreshToken: raw}
	}
	if raw[0] != '{' {
		return classifiedOpenAIRefreshToken{Kind: openAIRefreshTokenKindInvalidJSON}
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return classifiedOpenAIRefreshToken{Kind: openAIRefreshTokenKindInvalidJSON}
	}
	accessToken := firstCodexString(obj, codexAccessTokenPaths...)
	refreshToken := firstCodexString(obj, codexRefreshTokenPaths...)
	if accessToken != "" {
		return classifiedOpenAIRefreshToken{
			Kind:         openAIRefreshTokenKindSessionJSON,
			RefreshToken: refreshToken,
			Object:       obj,
		}
	}
	if refreshToken != "" {
		return classifiedOpenAIRefreshToken{
			Kind:         openAIRefreshTokenKindEmbeddedRT,
			RefreshToken: refreshToken,
		}
	}
	return classifiedOpenAIRefreshToken{Kind: openAIRefreshTokenKindInvalidJSON}
}

type openAISessionJSONPlan struct {
	Item         *codexImportAccount
	RefreshToken string
	ErrorCode    string
	ErrorMessage string
}

func openAISessionIdentityMismatch(account *service.Account, incomingAccountID string) bool {
	if account == nil {
		return false
	}
	existing := strings.TrimSpace(account.GetCredential("chatgpt_account_id"))
	incoming := strings.TrimSpace(incomingAccountID)
	return existing != "" && incoming != "" && existing != incoming
}

func sessionJSONAccessTokenExpired(item *codexImportAccount, now time.Time) bool {
	if item == nil || item.TokenExpiresAt == nil {
		return false
	}
	return now.Unix() > item.TokenExpiresAt.Unix()+codexImportClockSkewSeconds
}

// planOpenAISessionJSONUpdate decides how a ChatGPT/Codex session object should be applied.
// Parse without rejecting expiry first so identity and an embedded refresh_token can still
// win: Codex auth.json often has a stale JWT plus a usable RT.
func planOpenAISessionJSONUpdate(account *service.Account, obj map[string]any, validate bool, now time.Time) openAISessionJSONPlan {
	if account != nil && account.IsOpenAIPersonalAccessToken() {
		return openAISessionJSONPlan{
			ErrorCode:    "OPENAI_SESSION_AUTH_MODE_MISMATCH",
			ErrorMessage: "cannot apply ChatGPT session JSON to a Codex personal access token account",
		}
	}

	item, err := normalizeCodexImportEntryAt(codexImportEntry{Index: 1, Value: obj}, now, false)
	if err != nil {
		return openAISessionJSONPlan{ErrorCode: "OPENAI_SESSION_JSON_INVALID", ErrorMessage: err.Error()}
	}
	if openAISessionIdentityMismatch(account, item.AccountID) {
		return openAISessionJSONPlan{
			ErrorCode:    "OPENAI_SESSION_IDENTITY_MISMATCH",
			ErrorMessage: "session JSON chatgpt_account_id does not match this account",
		}
	}
	if refreshToken := strings.TrimSpace(item.RefreshToken); refreshToken != "" {
		return openAISessionJSONPlan{Item: item, RefreshToken: refreshToken}
	}
	if validate && sessionJSONAccessTokenExpired(item, now) {
		return openAISessionJSONPlan{
			ErrorCode:    "OPENAI_SESSION_ACCESS_TOKEN_EXPIRED",
			ErrorMessage: fmt.Sprintf("access_token 已过期: %s", item.TokenExpiresAt.Format(time.RFC3339)),
		}
	}
	return openAISessionJSONPlan{Item: item}
}

// tryOpenAISessionJSONUpdate applies a ChatGPT/Codex session object to an OpenAI OAuth account.
// done=true means the HTTP response was already written.
// When the object also contains a refresh_token, it returns that token and done=false so the
// existing OAuth refresh-token path can validate and persist it.
func (h *AccountHandler) tryOpenAISessionJSONUpdate(
	c *gin.Context,
	account *service.Account,
	obj map[string]any,
	validate bool,
) (extractedRefreshToken string, done bool) {
	plan := planOpenAISessionJSONUpdate(account, obj, validate, time.Now().UTC())
	if plan.ErrorCode != "" {
		response.ErrorFrom(c, infraerrors.BadRequest(plan.ErrorCode, plan.ErrorMessage))
		return "", true
	}
	if plan.RefreshToken != "" {
		return plan.RefreshToken, false
	}

	h.persistOpenAISessionJSONUpdate(c, account, plan.Item, validate)
	return "", true
}

func (h *AccountHandler) persistOpenAISessionJSONUpdate(c *gin.Context, account *service.Account, item *codexImportAccount, validated bool) {
	mergedCredentials := mergeCodexImportCredentials(account.Credentials, item.Credentials, item)
	mergedExtra := mergeCodexImportMap(account.Extra, item.Extra)
	preserveRefresh := strings.TrimSpace(codexCredentialString(mergedCredentials, "refresh_token")) != ""

	input := &service.UpdateAccountInput{
		Credentials: mergedCredentials,
		Extra:       mergedExtra,
	}
	if !preserveRefresh && item.TokenExpiresAt != nil {
		expiresAtUnix := item.TokenExpiresAt.Unix()
		autoPause := true
		input.ExpiresAt = &expiresAtUnix
		input.AutoPauseOnExpired = &autoPause
	}

	ctx := c.Request.Context()
	if _, err := h.adminService.UpdateAccount(ctx, account.ID, input); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	cleared, err := h.adminService.ClearAccountError(ctx, account.ID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if h.tokenCacheInvalidator != nil {
		if invalidateErr := h.tokenCacheInvalidator.InvalidateToken(ctx, cleared); invalidateErr != nil {
			log.Printf("[WARN] Failed to invalidate token cache for account %d: %v", cleared.ID, invalidateErr)
		}
	}
	h.logRefreshTokenAudit(c, account, validated)
	response.Success(c, h.buildAccountResponseWithRuntime(ctx, cleared))
}
