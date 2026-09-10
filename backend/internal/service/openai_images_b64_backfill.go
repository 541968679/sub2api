package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const AccountExtraImagesURLToB64JSON = "images_url_to_b64_json"

const openAIImageURLDownloadTimeout = 60 * time.Second

func ImagesURLToB64JSONEnabled(account *Account) bool {
	if account == nil || account.Extra == nil {
		return false
	}
	enabled, ok := account.Extra[AccountExtraImagesURLToB64JSON].(bool)
	return ok && enabled
}

func (s *OpenAIGatewayService) backfillOpenAIImagesB64JSON(
	ctx context.Context,
	account *Account,
	parsed *OpenAIImagesRequest,
	body []byte,
) []byte {
	if !ImagesURLToB64JSONEnabled(account) {
		return body
	}
	if parsed != nil && parsed.ResponseFormat == "url" {
		return body
	}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body
	}
	items := gjson.GetBytes(body, "data")
	if !items.IsArray() {
		return body
	}
	for index, item := range items.Array() {
		if !item.IsObject() {
			continue
		}
		if strings.TrimSpace(item.Get("b64_json").String()) != "" {
			continue
		}
		rawURL := strings.TrimSpace(item.Get("url").String())
		if rawURL == "" {
			continue
		}
		encoded, err := s.fetchOpenAIImageURLBase64(ctx, account, rawURL)
		if err != nil {
			logger.LegacyPrintf(
				"service.openai_gateway",
				"[OpenAI] Images b64_json backfill skipped account_id=%d index=%d err=%s",
				account.ID,
				index,
				sanitizeUpstreamErrorMessage(err.Error()),
			)
			continue
		}
		updated, err := sjson.SetBytes(body, fmt.Sprintf("data.%d.b64_json", index), encoded)
		if err != nil {
			logger.LegacyPrintf(
				"service.openai_gateway",
				"[OpenAI] Images b64_json backfill skipped account_id=%d index=%d err=%s",
				account.ID,
				index,
				sanitizeUpstreamErrorMessage(err.Error()),
			)
			continue
		}
		body = updated
	}
	return body
}

func (s *OpenAIGatewayService) fetchOpenAIImageURLBase64(ctx context.Context, account *Account, rawURL string) (string, error) {
	if strings.HasPrefix(strings.ToLower(rawURL), "data:") {
		if encoded := normalizeOpenAIImageBase64(rawURL); encoded != "" {
			return encoded, nil
		}
		return "", errors.New("data url payload is not valid base64")
	}
	if s == nil || s.httpUpstream == nil {
		return "", errors.New("http upstream is not configured")
	}
	downloadURL, err := s.validateOutboundURL(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid image url: %w", err)
	}
	if err := rejectPrivateImageHost(downloadURL); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(WithHTTPUpstreamPublicHostsOnly(ctx), openAIImageURLDownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("build image download request: %w", err)
	}
	req.Header.Set("Accept", "image/*,*/*;q=0.8")
	proxyURL := ""
	if account != nil && account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	accountID := int64(0)
	concurrency := 0
	if account != nil {
		accountID = account.ID
		concurrency = account.Concurrency
	}
	resp, err := s.httpUpstream.Do(req, proxyURL, accountID, concurrency)
	if err != nil {
		return "", fmt.Errorf("download image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("download image: unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, openAIImageMaxDownloadBytes+1))
	if err != nil {
		return "", fmt.Errorf("read image body: %w", err)
	}
	if int64(len(data)) > openAIImageMaxDownloadBytes {
		return "", fmt.Errorf("downloaded image exceeds %d bytes", openAIImageMaxDownloadBytes)
	}
	if len(data) == 0 {
		return "", errors.New("download image: empty body")
	}
	if !isBackfillImageContent(data) {
		return "", errors.New("download image: content is not an allowed image format")
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

func rejectPrivateImageHost(downloadURL string) error {
	parsed, err := url.Parse(downloadURL)
	if err != nil {
		return fmt.Errorf("invalid image url: %w", err)
	}
	if host := parsed.Hostname(); urlvalidator.IsBlockedHost(host) {
		return fmt.Errorf("image url host is not allowed: %s", host)
	}
	return nil
}

var openAIImageBackfillContentTypes = map[string]struct{}{
	"image/png":  {},
	"image/jpeg": {},
	"image/webp": {},
	"image/gif":  {},
}

func detectedImageContentType(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	ct := http.DetectContentType(data)
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = ct[:idx]
	}
	return strings.TrimSpace(strings.ToLower(ct))
}

func isBackfillImageContent(data []byte) bool {
	_, ok := openAIImageBackfillContentTypes[detectedImageContentType(data)]
	return ok
}

func (s *OpenAIGatewayService) validateOutboundURL(raw string) (string, error) {
	allowInsecure := false
	if s != nil && s.cfg != nil {
		allowInsecure = s.cfg.Security.URLAllowlist.AllowInsecureHTTP
	}
	if s == nil || s.cfg == nil || !s.cfg.Security.URLAllowlist.Enabled {
		return urlvalidator.ValidateURLFormat(raw, allowInsecure)
	}
	return urlvalidator.ValidateHTTPURL(raw, allowInsecure, urlvalidator.ValidationOptions{
		AllowedHosts:     s.cfg.Security.URLAllowlist.UpstreamHosts,
		RequireAllowlist: true,
		AllowPrivate:     s.cfg.Security.URLAllowlist.AllowPrivateHosts,
	})
}
