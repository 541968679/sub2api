package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	AnnouncementStatusDraft    = domain.AnnouncementStatusDraft
	AnnouncementStatusActive   = domain.AnnouncementStatusActive
	AnnouncementStatusArchived = domain.AnnouncementStatusArchived
)

const (
	AnnouncementNotifyModeSilent = domain.AnnouncementNotifyModeSilent
	AnnouncementNotifyModePopup  = domain.AnnouncementNotifyModePopup
)

const (
	AnnouncementSurfaceGeneral         = domain.AnnouncementSurfaceGeneral
	AnnouncementSurfaceDashboardBanner = domain.AnnouncementSurfaceDashboardBanner
	AnnouncementSurfaceAPIKeyRules     = domain.AnnouncementSurfaceAPIKeyRules
)

const (
	AnnouncementPopupFrequencyOnce  = domain.AnnouncementPopupFrequencyOnce
	AnnouncementPopupFrequencyDaily = domain.AnnouncementPopupFrequencyDaily
)

const (
	AnnouncementConditionTypeSubscription = domain.AnnouncementConditionTypeSubscription
	AnnouncementConditionTypeBalance      = domain.AnnouncementConditionTypeBalance
)

const (
	AnnouncementOperatorIn  = domain.AnnouncementOperatorIn
	AnnouncementOperatorGT  = domain.AnnouncementOperatorGT
	AnnouncementOperatorGTE = domain.AnnouncementOperatorGTE
	AnnouncementOperatorLT  = domain.AnnouncementOperatorLT
	AnnouncementOperatorLTE = domain.AnnouncementOperatorLTE
	AnnouncementOperatorEQ  = domain.AnnouncementOperatorEQ
)

var (
	ErrAnnouncementNotFound        = domain.ErrAnnouncementNotFound
	ErrAnnouncementInvalidTarget   = domain.ErrAnnouncementInvalidTarget
	ErrAnnouncementNilInput        = infraerrors.BadRequest("ANNOUNCEMENT_INPUT_REQUIRED", "announcement input is required")
	ErrAnnouncementInvalidTitle    = infraerrors.BadRequest("ANNOUNCEMENT_TITLE_INVALID", "announcement title is invalid")
	ErrAnnouncementContentRequired = infraerrors.BadRequest(
		"ANNOUNCEMENT_CONTENT_REQUIRED",
		"announcement content is required",
	)
	ErrAnnouncementInvalidStatus     = infraerrors.BadRequest("ANNOUNCEMENT_STATUS_INVALID", "announcement status is invalid")
	ErrAnnouncementInvalidNotifyMode = infraerrors.BadRequest(
		"ANNOUNCEMENT_NOTIFY_MODE_INVALID",
		"announcement notify_mode is invalid",
	)
	ErrAnnouncementInvalidSurface = infraerrors.BadRequest(
		"ANNOUNCEMENT_SURFACE_INVALID",
		"announcement surface is invalid",
	)
	ErrAnnouncementInvalidPopupFrequency = infraerrors.BadRequest(
		"ANNOUNCEMENT_POPUP_FREQUENCY_INVALID",
		"announcement popup_frequency is invalid",
	)
	ErrAnnouncementInvalidSchedule = infraerrors.BadRequest(
		"ANNOUNCEMENT_TIME_RANGE_INVALID",
		"starts_at must be before ends_at",
	)
)

type AnnouncementTargeting = domain.AnnouncementTargeting

type AnnouncementConditionGroup = domain.AnnouncementConditionGroup

type AnnouncementCondition = domain.AnnouncementCondition

type Announcement = domain.Announcement

type AnnouncementListFilters struct {
	Status  string
	Search  string
	Surface string
}

type AnnouncementReadState struct {
	ReadAt               *time.Time
	LastPopupDismissedAt *time.Time
	BannerDismissedAt    *time.Time
}

type AnnouncementRepository interface {
	Create(ctx context.Context, a *Announcement) error
	GetByID(ctx context.Context, id int64) (*Announcement, error)
	Update(ctx context.Context, a *Announcement) error
	Delete(ctx context.Context, id int64) error

	List(ctx context.Context, params pagination.PaginationParams, filters AnnouncementListFilters) ([]Announcement, *pagination.PaginationResult, error)
	ListActive(ctx context.Context, now time.Time, surface string) ([]Announcement, error)
}

type AnnouncementReadRepository interface {
	MarkRead(ctx context.Context, announcementID, userID int64, readAt time.Time) error
	MarkPopupDismissed(ctx context.Context, announcementID, userID int64, dismissedAt time.Time) error
	MarkBannerDismissed(ctx context.Context, announcementID, userID int64, dismissedAt time.Time) error
	GetReadStateMapByUser(ctx context.Context, userID int64, announcementIDs []int64) (map[int64]AnnouncementReadState, error)
	GetReadMapByUsers(ctx context.Context, announcementID int64, userIDs []int64) (map[int64]time.Time, error)
	CountByAnnouncementID(ctx context.Context, announcementID int64) (int64, error)
}
