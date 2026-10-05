package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

const (
	maxSubscriptionBytes      = 2 << 20
	maxSubscriptionURLRunes   = 4096
	maxSubscriptionRedirect   = 3
	subscriptionFetchTimeout  = 15 * time.Second
	subscriptionHeaderTimeout = 10 * time.Second
)

var (
	// ErrSubscriptionURLInvalid means the link is missing, not http(s), or targets a blocked host.
	ErrSubscriptionURLInvalid = errors.New("invalid subscription url")
	// ErrSubscriptionFetchFailed means the subscription host could not be reached.
	ErrSubscriptionFetchFailed = errors.New("failed to fetch subscription")
	// ErrSubscriptionTooLarge means the body exceeds the import limit.
	ErrSubscriptionTooLarge = errors.New("subscription body is too large")
	// ErrSubscriptionEmpty means the host returned no content.
	ErrSubscriptionEmpty = errors.New("subscription body is empty")
	// ErrSubscriptionNotList means the body is HTML or another non-list document.
	ErrSubscriptionNotList = errors.New("subscription body is not a proxy list")
)

// SubscriptionStatusError is returned when the subscription URL answers with a non-2xx status.
type SubscriptionStatusError struct {
	StatusCode int
}

func (e *SubscriptionStatusError) Error() string {
	if e == nil {
		return "subscription returned an error status"
	}
	return fmt.Sprintf("subscription returned HTTP %d", e.StatusCode)
}

// FetchProxySubscription downloads a public http(s) subscription.
// Private, loopback, and link-local targets are rejected before the request is sent.
func FetchProxySubscription(ctx context.Context, rawURL string) ([]byte, error) {
	if err := validateSubscriptionURL(rawURL); err != nil {
		return nil, err
	}
	shared, err := httpclient.GetClient(httpclient.Options{
		Timeout:               subscriptionFetchTimeout,
		ResponseHeaderTimeout: subscriptionHeaderTimeout,
		ValidateResolvedIP:    true,
	})
	if err != nil {
		return nil, ErrSubscriptionFetchFailed
	}
	// Copy the shared client before setting redirect policy. The pool reuses the original.
	client := *shared
	client.CheckRedirect = subscriptionRedirectCheck

	reqCtx, cancel := context.WithTimeout(ctx, subscriptionFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, strings.TrimSpace(rawURL), nil)
	if err != nil {
		return nil, ErrSubscriptionURLInvalid
	}
	req.Header.Set("User-Agent", "clash-meta/1.19.0")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, ErrSubscriptionFetchFailed
	}
	return readSubscriptionResponse(resp)
}

func validateSubscriptionURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || len(rawURL) > maxSubscriptionURLRunes {
		return ErrSubscriptionURLInvalid
	}
	if _, err := urlvalidator.ValidateHTTPURL(rawURL, true, urlvalidator.ValidationOptions{}); err != nil {
		return ErrSubscriptionURLInvalid
	}
	return nil
}

func subscriptionRedirectCheck(req *http.Request, via []*http.Request) error {
	if len(via) >= maxSubscriptionRedirect {
		return ErrSubscriptionFetchFailed
	}
	if req == nil || req.URL == nil {
		return ErrSubscriptionURLInvalid
	}
	if err := validateSubscriptionURL(req.URL.String()); err != nil {
		return err
	}
	return nil
}

func readSubscriptionResponse(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, ErrSubscriptionFetchFailed
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &SubscriptionStatusError{StatusCode: resp.StatusCode}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSubscriptionBytes+1))
	if err != nil {
		return nil, ErrSubscriptionFetchFailed
	}
	if len(body) > maxSubscriptionBytes {
		return nil, ErrSubscriptionTooLarge
	}
	if strings.TrimSpace(string(body)) == "" {
		return nil, ErrSubscriptionEmpty
	}
	return body, nil
}
