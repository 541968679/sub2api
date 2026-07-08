package service

import (
	"context"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

const (
	GatewayNetworkRetryMaxDefault = 2
	GatewayNetworkRetryMaxMin     = 0
	GatewayNetworkRetryMaxMax     = 10
)

type httpUpstreamNetworkRetryDisabledKey struct{}

type GatewayNetworkRetrySettingsReader interface {
	GetGatewayNetworkRetryMax(ctx context.Context) int
}

func ClampGatewayNetworkRetryMax(value int) int {
	if value < GatewayNetworkRetryMaxMin {
		return GatewayNetworkRetryMaxMin
	}
	if value > GatewayNetworkRetryMaxMax {
		return GatewayNetworkRetryMaxMax
	}
	return value
}

func WithHTTPUpstreamNetworkRetryDisabled(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, httpUpstreamNetworkRetryDisabledKey{}, true)
}

func HTTPUpstreamNetworkRetryDisabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	disabled, _ := ctx.Value(httpUpstreamNetworkRetryDisabledKey{}).(bool)
	return disabled
}

// HTTPUpstream 上游 HTTP 请求接口
// 用于向上游 API（Claude、OpenAI、Gemini 等）发送请求
type HTTPUpstream interface {
	// Do 执行 HTTP 请求（不启用 TLS 指纹）
	Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error)

	// DoWithTLS 执行带 TLS 指纹伪装的 HTTP 请求
	//
	// profile 参数:
	//   - nil: 不启用 TLS 指纹，行为与 Do 方法相同
	//   - non-nil: 使用指定的 Profile 进行 TLS 指纹伪装
	//
	// Profile 由调用方通过 TLSFingerprintProfileService 解析后传入，
	// 支持按账号绑定的数据库 profile 或内置默认 profile。
	DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error)
}
