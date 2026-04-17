package service

import "context"

// UserGroupRateEntry 分组下用户专属倍率条目
type UserGroupRateEntry struct {
	UserID                int64    `json:"user_id"`
	UserName              string   `json:"user_name"`
	UserEmail             string   `json:"user_email"`
	UserNotes             string   `json:"user_notes"`
	UserStatus            string   `json:"user_status"`
	RateMultiplier        *float64 `json:"rate_multiplier"`
	DisplayRateMultiplier *float64 `json:"display_rate_multiplier"`
}

// GroupRateMultiplierInput 批量设置分组倍率的输入条目
type GroupRateMultiplierInput struct {
	UserID                int64    `json:"user_id"`
	RateMultiplier        *float64 `json:"rate_multiplier"`
	DisplayRateMultiplier *float64 `json:"display_rate_multiplier"`
}

// UserGroupRateData 用户在某个分组的完整倍率数据
type UserGroupRateData struct {
	RateMultiplier        *float64 `json:"rate,omitempty"`
	DisplayRateMultiplier *float64 `json:"display_rate,omitempty"`
}

// UserGroupRateRepository 用户专属分组倍率仓储接口
// 允许管理员为特定用户设置分组的专属计费倍率，覆盖分组默认倍率
type UserGroupRateRepository interface {
	// GetByUserID 获取用户的所有专属分组倍率
	// 返回 map[groupID]rateMultiplier（仅真实倍率，NULL 行不返回）
	GetByUserID(ctx context.Context, userID int64) (map[int64]float64, error)

	// GetFullByUserID 获取用户的所有分组倍率数据（含展示倍率）
	// 返回 map[groupID]UserGroupRateData
	GetFullByUserID(ctx context.Context, userID int64) (map[int64]UserGroupRateData, error)

	// GetByUserAndGroup 获取用户在特定分组的专属倍率
	// 如果未设置专属倍率，返回 nil
	GetByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error)

	// GetDisplayRateByUserAndGroup 获取用户在特定分组的展示倍率
	// 如果未设置展示倍率，返回 nil
	GetDisplayRateByUserAndGroup(ctx context.Context, userID, groupID int64) (*float64, error)

	// GetByGroupID 获取指定分组下所有用户的专属倍率
	GetByGroupID(ctx context.Context, groupID int64) ([]UserGroupRateEntry, error)

	// SyncUserGroupRates 同步用户的分组专属倍率
	// rates: map[groupID]*rateMultiplier，nil 表示删除该分组的专属倍率
	SyncUserGroupRates(ctx context.Context, userID int64, rates map[int64]*float64) error

	// SyncUserGroupRatesFull 同步用户的分组专属倍率（含展示倍率）
	SyncUserGroupRatesFull(ctx context.Context, userID int64, rates map[int64]*UserGroupRateData) error

	// SyncGroupRateMultipliers 批量同步分组的用户专属倍率（替换整组数据）
	SyncGroupRateMultipliers(ctx context.Context, groupID int64, entries []GroupRateMultiplierInput) error

	// DeleteByGroupID 删除指定分组的所有用户专属倍率（分组删除时调用）
	DeleteByGroupID(ctx context.Context, groupID int64) error

	// DeleteByUserID 删除指定用户的所有专属倍率（用户删除时调用）
	DeleteByUserID(ctx context.Context, userID int64) error
}

// userGroupRateFullBatchReader 扩展接口，支持批量加载含展示倍率的数据
type UserGroupRateFullBatchReader interface {
	GetFullByUserIDs(ctx context.Context, userIDs []int64) (map[int64]map[int64]UserGroupRateData, error)
}
