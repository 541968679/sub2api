package service

import "context"

type UserGroupRateEntry struct {
	UserID                int64    `json:"user_id"`
	UserName              string   `json:"user_name"`
	UserEmail             string   `json:"user_email"`
	UserNotes             string   `json:"user_notes"`
	UserStatus            string   `json:"user_status"`
	RateMultiplier        *float64 `json:"rate_multiplier,omitempty"`
	DisplayRateMultiplier *float64 `json:"display_rate_multiplier,omitempty"`
	RPMOverride           *int     `json:"rpm_override,omitempty"`
}

type GroupRateMultiplierInput struct {
	UserID                int64    `json:"user_id"`
	RateMultiplier        *float64 `json:"rate_multiplier"`
	DisplayRateMultiplier *float64 `json:"display_rate_multiplier"`
}

type UserGroupRateData struct {
	RateMultiplier        *float64 `json:"rate,omitempty"`
	DisplayRateMultiplier *float64 `json:"display_rate,omitempty"`
}

type GroupRPMOverrideInput struct {
	UserID      int64 `json:"user_id"`
	RPMOverride *int  `json:"rpm_override"`
}

type UserGroupRateRepository interface {
	GetByUserID(ctx context.Context, userID int64) (map[int64]float64, error)
	GetFullByUserID(ctx context.Context, userID int64) (map[int64]UserGroupRateData, error)
	GetByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error)
	GetDisplayRateByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error)
	GetRPMOverrideByUserAndGroup(ctx context.Context, userID, groupID int64) (*int, error)
	GetByGroupID(ctx context.Context, groupID int64) ([]UserGroupRateEntry, error)
	SyncUserGroupRates(ctx context.Context, userID int64, rates map[int64]*float64) error
	SyncUserGroupRatesFull(ctx context.Context, userID int64, rates map[int64]*UserGroupRateData) error
	SyncGroupRateMultipliers(ctx context.Context, groupID int64, entries []GroupRateMultiplierInput) error
	SyncGroupRPMOverrides(ctx context.Context, groupID int64, entries []GroupRPMOverrideInput) error
	ClearGroupRPMOverrides(ctx context.Context, groupID int64) error
	DeleteByGroupID(ctx context.Context, groupID int64) error
	DeleteByUserID(ctx context.Context, userID int64) error
}

type UserGroupRateFullBatchReader interface {
	GetFullByUserIDs(ctx context.Context, userIDs []int64) (map[int64]map[int64]UserGroupRateData, error)
}
