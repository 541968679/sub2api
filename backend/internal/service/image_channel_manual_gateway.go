package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/tidwall/gjson"
)

const imageManualGatewayUserAgent = "sub2api-image-manual-gateway/1.0"

type ImageManualGatewayRequest struct {
	Path        string
	APIKey      string
	ContentType string
	Body        []byte
	Timeout     time.Duration
	RequestID   string
}

type ImageManualGatewayResponse struct {
	StatusCode      int
	Header          http.Header
	Body            []byte
	ClientRequestID string
	RequestIDs      []string
	ErrorType       string
	ErrorCode       string
	ErrorMessage    string
	ErrorStage      string
	HeaderDuration  time.Duration
	TotalDuration   time.Duration
	spool           *openAIImagesResponseSpool
}

func (r *ImageManualGatewayResponse) Reader() (io.Reader, error) {
	if r == nil {
		return nil, errors.New("manual gateway response is nil")
	}
	if r.spool != nil {
		return r.spool.Reader()
	}
	return bytes.NewReader(r.Body), nil
}

func (r *ImageManualGatewayResponse) MetadataBytes() []byte {
	if r == nil {
		return nil
	}
	source := r.Body
	if len(source) == 0 && r.spool != nil {
		source = r.spool.MetadataBytes()
	}
	if len(source) == 0 {
		return nil
	}
	collector := newOpenAIImagesResponseMetadataCollector()
	collector.Add(source)
	return collector.Bytes()
}

func (r *ImageManualGatewayResponse) Close() error {
	if r == nil || r.spool == nil {
		return nil
	}
	err := r.spool.Close()
	r.spool = nil
	return err
}

func (r *ImageManualGatewayResponse) Size() int64 {
	if r == nil {
		return 0
	}
	if r.spool != nil {
		return r.spool.size
	}
	return int64(len(r.Body))
}

type imageManualGatewayDoer interface {
	Do(ctx context.Context, input ImageManualGatewayRequest) (*ImageManualGatewayResponse, error)
}

type imageManualGatewayClient struct {
	targetOrigin     string
	httpClient       *http.Client
	maxResponseBytes int64
}

func newImageManualGatewayClient(cfg *config.Config) *imageManualGatewayClient {
	port := 8080
	maxResponseBytes := defaultUpstreamResponseReadMaxBytes
	if cfg != nil {
		if cfg.Server.Port > 0 {
			port = cfg.Server.Port
		}
		maxResponseBytes = resolveUpstreamResponseReadLimit(cfg)
	}
	loopbackAddress := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, loopbackAddress)
		},
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	return &imageManualGatewayClient{
		targetOrigin: "http://" + loopbackAddress,
		httpClient: &http.Client{
			Transport: transport,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		maxResponseBytes: maxResponseBytes,
	}
}

func (c *imageManualGatewayClient) Do(ctx context.Context, input ImageManualGatewayRequest) (*ImageManualGatewayResponse, error) {
	if c == nil || c.httpClient == nil {
		return nil, errors.New("manual gateway client is not configured")
	}
	path := strings.TrimSpace(input.Path)
	if path != openAIImagesGenerationsEndpoint && path != openAIImagesEditsEndpoint {
		return nil, errors.New("manual gateway target must be a supported local images endpoint")
	}
	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" {
		return nil, errors.New("manual gateway API key is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if input.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, input.Timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.targetOrigin+path, bytes.NewReader(input.Body))
	if err != nil {
		return nil, errors.New("build local manual gateway request")
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if contentType := strings.TrimSpace(input.ContentType); contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("User-Agent", imageManualGatewayUserAgent)
	if requestID := strings.TrimSpace(input.RequestID); requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}

	startedAt := time.Now()
	resp, err := c.httpClient.Do(req)
	headerDuration := time.Since(startedAt)
	if err != nil {
		return nil, fmt.Errorf("local manual gateway transport failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	limit := c.maxResponseBytes
	if limit <= 0 {
		limit = defaultUpstreamResponseReadMaxBytes
	}
	spool, readErr := spoolOpenAIImagesResponse(
		resp.Body,
		limit,
		openAIImagesResponseSpoolMemoryThreshold,
		"",
	)
	result := &ImageManualGatewayResponse{
		StatusCode:      resp.StatusCode,
		Header:          resp.Header.Clone(),
		ClientRequestID: strings.TrimSpace(resp.Header.Get("X-Client-Request-ID")),
		RequestIDs:      append([]string(nil), resp.Header.Values("X-Request-ID")...),
		HeaderDuration:  headerDuration,
		TotalDuration:   time.Since(startedAt),
		spool:           spool,
	}
	if spool != nil && spool.file == nil {
		result.Body = spool.MetadataBytes()
	}
	parseImageManualGatewayError(result)
	if readErr != nil {
		return result, fmt.Errorf("read local manual gateway response: %w", readErr)
	}
	return result, nil
}

func parseImageManualGatewayError(response *ImageManualGatewayResponse) {
	if response == nil {
		return
	}
	body := response.Body
	if len(body) == 0 && response.spool != nil {
		body = response.spool.MetadataBytes()
	}
	if len(body) == 0 {
		return
	}
	root := gjson.ParseBytes(body)
	errorValue := root.Get("error")
	if errorValue.IsObject() {
		response.ErrorType = strings.TrimSpace(errorValue.Get("type").String())
		response.ErrorCode = strings.TrimSpace(errorValue.Get("code").String())
		response.ErrorMessage = strings.TrimSpace(errorValue.Get("message").String())
		response.ErrorStage = strings.TrimSpace(errorValue.Get("stage").String())
		return
	}
	response.ErrorType = strings.TrimSpace(root.Get("type").String())
	response.ErrorCode = strings.TrimSpace(root.Get("code").String())
	response.ErrorMessage = strings.TrimSpace(root.Get("message").String())
	response.ErrorStage = strings.TrimSpace(root.Get("stage").String())
}
