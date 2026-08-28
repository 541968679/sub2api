package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// openAIWaitTimeoutSettingsOverride is test-only. Non-nil replaces KV reads.
var openAIWaitTimeoutSettingsOverride *OpenAIWaitTimeoutSettings

type openAIWaitCancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (b *openAIWaitCancelOnCloseBody) Close() error {
	if b == nil {
		return nil
	}
	b.once.Do(func() {
		if b.cancel != nil {
			b.cancel()
		}
	})
	if b.ReadCloser == nil {
		return nil
	}
	return b.ReadCloser.Close()
}

func IsOpenAIWaitTimeoutOpsError(message, upstreamMessage, body string) bool {
	blob := strings.ToLower(message + "\n" + upstreamMessage + "\n" + body)
	return strings.Contains(blob, OpenAIHeaderWaitTimeoutMarker) ||
		strings.Contains(blob, OpenAIFirstUsefulFrameTimeoutMarker)
}

func (s *OpenAIGatewayService) openAIWaitTimeoutSettings() OpenAIWaitTimeoutSettings {
	if openAIWaitTimeoutSettingsOverride != nil {
		return *openAIWaitTimeoutSettingsOverride
	}
	if s == nil || s.settingService == nil {
		return *DefaultOpenAIWaitTimeoutSettings()
	}
	return s.settingService.GetOpenAIWaitTimeoutSettingsCached(context.Background())
}

func (s *OpenAIGatewayService) openAIWaitTimeoutSettingsForAccount(account *Account) OpenAIWaitTimeoutSettings {
	if account != nil && strings.EqualFold(strings.TrimSpace(account.Platform), PlatformGrok) {
		return OpenAIWaitTimeoutSettings{}
	}
	return s.openAIWaitTimeoutSettings()
}

func beginOpenAIWaitTimer(d time.Duration) (*time.Timer, <-chan time.Time) {
	if d <= 0 {
		return nil, nil
	}
	timer := time.NewTimer(d)
	return timer, timer.C
}

func stopOpenAIWaitTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func openAIWaitTimeoutCanSilentFailover(c *gin.Context, clientOutputStarted bool) bool {
	if clientOutputStarted {
		return false
	}
	if IsResponseCommitted(c) {
		return false
	}
	if c != nil && c.Writer != nil && (c.Writer.Written() || c.Writer.Size() > 0) {
		return false
	}
	return true
}

func isOpenAIHeaderWaitClientCanceled(parent context.Context, err error) bool {
	if parent != nil && errors.Is(parent.Err(), context.Canceled) {
		return true
	}
	if err == nil || parent == nil {
		return false
	}
	return errors.Is(err, context.Canceled) && parent.Err() != nil
}

func openAIWaitTimeoutMessage(marker string, waited time.Duration) string {
	waitedMs := waited.Milliseconds()
	if waitedMs > 0 {
		return fmt.Sprintf("%s waited_ms=%d", marker, waitedMs)
	}
	return marker
}

func openAIResponsesStreamErrorEventPayload(reason string) string {
	return `{"type":"error","sequence_number":0,"error":{"type":"upstream_error","message":` + strconv.Quote(reason) + `,"code":` + strconv.Quote(reason) + `}}`
}

func writeOpenAIResponsesSSEErrorEvent(c *gin.Context, reason string) {
	if c == nil || c.Writer == nil || strings.TrimSpace(reason) == "" {
		return
	}
	_, _ = c.Writer.WriteString("data: " + openAIResponsesStreamErrorEventPayload(reason) + "\n\n")
	c.Writer.Flush()
}

func writeOpenAIChatStreamErrorEvent(c *gin.Context, message string) {
	if c == nil || c.Writer == nil {
		return
	}
	if message == "" {
		message = OpenAIFirstUsefulFrameTimeoutMarker
	}
	if !c.Writer.Written() {
		writeChatCompletionsError(c, http.StatusBadGateway, "upstream_error", message)
		return
	}
	errorPayload, _ := json.Marshal(gin.H{"error": gin.H{"type": "upstream_error", "message": message}})
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", errorPayload)
	c.Writer.Flush()
}

func (s *OpenAIGatewayService) noteOpenAIWaitTimeout(
	ctx context.Context,
	account *Account,
	model string,
	marker string,
	waited time.Duration,
) string {
	if s != nil && s.rateLimitService != nil {
		s.rateLimitService.HandleStreamTimeout(ctx, account, model)
	}
	waitedMs := waited.Milliseconds()
	logName := "openai.header_wait_timeout"
	if marker == OpenAIFirstUsefulFrameTimeoutMarker {
		logName = "openai.first_useful_frame_timeout"
	}
	accountID := int64(0)
	accountName := ""
	if account != nil {
		accountID = account.ID
		accountName = account.Name
	}
	logger.L().With(zap.String("component", "service.openai_gateway")).Warn(
		logName,
		zap.Int64("account_id", accountID),
		zap.String("account_name", accountName),
		zap.Int64("waited_ms", waitedMs),
		zap.String("marker", marker),
	)
	return openAIWaitTimeoutMessage(marker, waited)
}

func (s *OpenAIGatewayService) failOpenAIWaitTimeout(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	model string,
	passthrough bool,
	marker string,
	waited time.Duration,
) error {
	message := s.noteOpenAIWaitTimeout(ctx, account, model, marker, waited)
	return s.newOpenAIStreamFailoverError(c, account, passthrough, "", nil, message)
}

// abortOpenAIWaitTimeoutAfterCommit records the same timeout / Ops 502 as a
// silent failover, but returns a non-UpstreamFailoverError so the handler
// does not switch accounts after the downstream response is already committed.
func (s *OpenAIGatewayService) abortOpenAIWaitTimeoutAfterCommit(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	model string,
	passthrough bool,
	marker string,
	waited time.Duration,
) error {
	message := s.noteOpenAIWaitTimeout(ctx, account, model, marker, waited)
	_ = s.newOpenAIStreamFailoverError(c, account, passthrough, "", nil, message)
	return fmt.Errorf("upstream response failed: %s", message)
}

func (s *OpenAIGatewayService) openAIFirstUsefulFrameTimeoutErr(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	model string,
	passthrough bool,
	waited time.Duration,
	clientOutputStarted bool,
) (silentFailover bool, err error) {
	if openAIWaitTimeoutCanSilentFailover(c, clientOutputStarted) {
		return true, s.failOpenAIWaitTimeout(ctx, c, account, model, passthrough, OpenAIFirstUsefulFrameTimeoutMarker, waited)
	}
	return false, s.abortOpenAIWaitTimeoutAfterCommit(ctx, c, account, model, passthrough, OpenAIFirstUsefulFrameTimeoutMarker, waited)
}

func (s *OpenAIGatewayService) doOpenAIUpstreamWithHeaderWait(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	req *http.Request,
	proxyURL string,
	passthrough bool,
	model string,
) (*http.Response, error) {
	if s == nil || s.httpUpstream == nil {
		return nil, errors.New("openai upstream client is not configured")
	}
	accountID := int64(0)
	concurrency := 0
	if account != nil {
		accountID = account.ID
		concurrency = account.Concurrency
	}
	wait := s.openAIWaitTimeoutSettingsForAccount(account).HeaderWaitDuration()
	if wait <= 0 || req == nil {
		return s.httpUpstream.Do(req, proxyURL, accountID, concurrency)
	}

	parent := req.Context()
	if parent == nil {
		parent = context.Background()
	}
	waitCtx, cancelWait := context.WithCancel(parent)
	var mu sync.Mutex
	headersArrived := false
	timer := time.AfterFunc(wait, func() {
		mu.Lock()
		defer mu.Unlock()
		if !headersArrived {
			cancelWait()
		}
	})
	req2 := req.Clone(waitCtx)
	started := time.Now()
	resp, err := s.httpUpstream.Do(req2, proxyURL, accountID, concurrency)
	mu.Lock()
	headersArrived = true
	timer.Stop()
	mu.Unlock()
	if err != nil {
		cancelWait()
		if isOpenAIHeaderWaitClientCanceled(parent, err) {
			return nil, err
		}
		if waitCtx.Err() != nil && parent.Err() == nil {
			return nil, s.failOpenAIWaitTimeout(ctx, c, account, model, passthrough, OpenAIHeaderWaitTimeoutMarker, time.Since(started))
		}
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, passthrough)
	}
	if resp != nil && resp.Body != nil {
		resp.Body = &openAIWaitCancelOnCloseBody{ReadCloser: resp.Body, cancel: cancelWait}
	} else {
		cancelWait()
	}
	return resp, nil
}
