package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type CyberPolicyUsageInput struct {
	APIKey             *APIKey
	Account            *Account
	Subscription       *UserSubscription
	RequestID          string
	Model              string
	Stream             bool
	InputTokens        int
	OutputTokens       int
	InboundEndpoint    string
	UpstreamEndpoint   string
	UserAgent          string
	IPAddress          string
	RequestPayloadHash string
	APIKeyService      APIKeyQuotaUpdater
	ChannelUsageFields
}

func (s *OpenAIGatewayService) RecordCyberPolicyUsageLog(ctx context.Context, in CyberPolicyUsageInput) {
	if s == nil || in.APIKey == nil || in.APIKey.User == nil || in.Account == nil || strings.TrimSpace(in.Model) == "" {
		return
	}
	result := &OpenAIForwardResult{
		RequestID: in.RequestID,
		Model:     in.Model,
		Stream:    in.Stream,
		Usage:     OpenAIUsage{InputTokens: in.InputTokens, OutputTokens: in.OutputTokens},
	}
	if err := s.RecordUsage(ctx, &OpenAIRecordUsageInput{
		Result: result, APIKey: in.APIKey, User: in.APIKey.User, Account: in.Account,
		Subscription: in.Subscription, InboundEndpoint: in.InboundEndpoint,
		UpstreamEndpoint: in.UpstreamEndpoint, UserAgent: in.UserAgent,
		IPAddress: in.IPAddress, RequestPayloadHash: in.RequestPayloadHash,
		APIKeyService: in.APIKeyService, CyberBlocked: true,
		ChannelUsageFields: in.ChannelUsageFields,
	}); err != nil {
		logger.LegacyPrintf("service.openai_gateway", "cyber usage record failed: request_id=%s err=%v", in.RequestID, err)
	}
}
