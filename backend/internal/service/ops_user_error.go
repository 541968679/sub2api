package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type UserErrorRequest struct {
	ID              int64     `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	Model           string    `json:"model"`
	InboundEndpoint string    `json:"inbound_endpoint"`
	StatusCode      int       `json:"status_code"`
	Category        string    `json:"category"`
	Platform        string    `json:"platform"`
	Message         string    `json:"message"`
	KeyName         string    `json:"key_name"`
	KeyDeleted      bool      `json:"key_deleted"`
}

type UserErrorRequestList struct {
	Items    []*UserErrorRequest `json:"items"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

type UserErrorRequestDetail struct {
	UserErrorRequest
	ErrorBody          string `json:"error_body"`
	UpstreamStatusCode *int   `json:"upstream_status_code,omitempty"`
}

func MapUserErrorCategory(phase, errType string) string {
	switch phase {
	case "auth":
		return "auth"
	case "routing":
		return "service_unavailable"
	case "upstream", "network":
		return "upstream"
	case "internal":
		return "internal"
	case "request":
		switch errType {
		case "rate_limit_error":
			return "rate_limit"
		case "billing_error", "subscription_error":
			return "quota"
		case "invalid_request_error":
			return "invalid_request"
		case "cyber_policy":
			return "cyber"
		}
	}
	return "other"
}

func CategoryToFilter(category string) (phases []string, errorTypes []string) {
	switch category {
	case "auth":
		return []string{"auth"}, nil
	case "service_unavailable":
		return []string{"routing"}, nil
	case "upstream":
		return []string{"upstream", "network"}, nil
	case "internal":
		return []string{"internal"}, nil
	case "rate_limit":
		return nil, []string{"rate_limit_error"}
	case "quota":
		return nil, []string{"billing_error", "subscription_error"}
	case "invalid_request":
		return nil, []string{"invalid_request_error"}
	case "cyber":
		return []string{"request"}, []string{"cyber_policy"}
	default:
		return nil, nil
	}
}

func ToUserErrorRequest(e *OpsErrorLog) *UserErrorRequest {
	if e == nil {
		return nil
	}
	model := e.RequestedModel
	if model == "" {
		model = e.Model
	}
	return &UserErrorRequest{
		ID:              e.ID,
		CreatedAt:       e.CreatedAt,
		Model:           model,
		InboundEndpoint: e.InboundEndpoint,
		StatusCode:      e.StatusCode,
		Category:        MapUserErrorCategory(e.Phase, e.Type),
		Platform:        e.Platform,
		Message:         e.Message,
		KeyName:         e.APIKeyName,
		KeyDeleted:      e.APIKeyDeleted,
	}
}

func ToUserErrorRequestDetail(e *OpsErrorLogDetail) *UserErrorRequestDetail {
	if e == nil {
		return nil
	}
	base := ToUserErrorRequest(&e.OpsErrorLog)
	if base == nil {
		return nil
	}
	return &UserErrorRequestDetail{
		UserErrorRequest:   *base,
		ErrorBody:          e.ErrorBody,
		UpstreamStatusCode: e.UpstreamStatusCode,
	}
}

func (s *OpsService) ListUserErrorRequests(ctx context.Context, userID int64, filter *OpsErrorLogFilter) (*UserErrorRequestList, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER_ID", "invalid user id")
	}
	if s.opsRepo == nil {
		return &UserErrorRequestList{Items: []*UserErrorRequest{}, Total: 0, Page: 1, PageSize: 20}, nil
	}
	if filter == nil {
		filter = &OpsErrorLogFilter{}
	}
	filter.UserID = &userID
	filter.View = "all"
	result, err := s.opsRepo.ListErrorLogs(ctx, filter)
	if err != nil {
		return nil, err
	}
	items := make([]*UserErrorRequest, 0, len(result.Errors))
	for _, item := range result.Errors {
		if out := ToUserErrorRequest(item); out != nil {
			items = append(items, out)
		}
	}
	return &UserErrorRequestList{
		Items:    items,
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
	}, nil
}

func (s *OpsService) GetUserErrorRequestDetail(ctx context.Context, userID int64, id int64) (*UserErrorRequestDetail, error) {
	if err := s.RequireMonitoringEnabled(ctx); err != nil {
		return nil, err
	}
	if userID <= 0 || id <= 0 {
		return nil, infraerrors.BadRequest("INVALID_REQUEST", "invalid request")
	}
	if s.opsRepo == nil {
		return nil, infraerrors.NotFound("OPS_ERROR_NOT_FOUND", "ops error log not found")
	}
	detail, err := s.GetErrorLogByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if detail.UserID == nil || *detail.UserID != userID {
		return nil, infraerrors.NotFound("OPS_ERROR_NOT_FOUND", "ops error log not found")
	}
	return ToUserErrorRequestDetail(detail), nil
}
