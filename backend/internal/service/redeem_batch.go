package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// RedeemCodeBatch is an aggregated view of codes created in one generation run.
// BatchKey is the stable identifier for list/detail APIs:
//   - real batch: the stored batch_id hex string
//   - legacy row without batch_id: "legacy:{id}"
type RedeemCodeBatch struct {
	BatchKey                string
	BatchID                 *string
	IsLegacy                bool
	Type                    string
	Value                   float64
	GroupID                 *int64
	GroupName               string
	ValidityDays            int
	BatchRedeemLimitPerUser bool
	ExpiresAt               *time.Time
	CreatedAt               time.Time
	TotalCount              int
	UnusedCount             int
	UsedCount               int
	ExpiredCount            int
}

// RedeemCodeBatchRepository optional repo capability for batch aggregation.
type RedeemCodeBatchRepository interface {
	ListBatches(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]RedeemCodeBatch, *pagination.PaginationResult, error)
	ListCodesByBatchKey(ctx context.Context, batchKey string) ([]RedeemCode, error)
	DeleteUnusedByBatchKey(ctx context.Context, batchKey string) (int64, error)
}
