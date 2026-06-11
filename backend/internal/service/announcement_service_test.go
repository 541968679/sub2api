//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type announcementRepoStub struct {
	item *Announcement
}

func (s *announcementRepoStub) Create(_ context.Context, a *Announcement) error {
	s.item = a
	return nil
}

func (s *announcementRepoStub) GetByID(_ context.Context, _ int64) (*Announcement, error) {
	if s.item == nil {
		return nil, ErrAnnouncementNotFound
	}
	return s.item, nil
}

func (s *announcementRepoStub) Update(_ context.Context, a *Announcement) error {
	s.item = a
	return nil
}

func (*announcementRepoStub) Delete(context.Context, int64) error {
	return nil
}

func (*announcementRepoStub) List(context.Context, pagination.PaginationParams, AnnouncementListFilters) ([]Announcement, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (*announcementRepoStub) ListActive(context.Context, time.Time, string) ([]Announcement, error) {
	return nil, nil
}

type announcementReadRepoStub struct {
	markReadCalls        int
	popupDismissCalls    int
	bannerDismissCalls   int
	lastAnnouncementID   int64
	lastUserID           int64
	lastReadStateRequest []int64
}

func (s *announcementReadRepoStub) MarkRead(_ context.Context, announcementID, userID int64, _ time.Time) error {
	s.markReadCalls++
	s.lastAnnouncementID = announcementID
	s.lastUserID = userID
	return nil
}

func (s *announcementReadRepoStub) MarkPopupDismissed(_ context.Context, announcementID, userID int64, _ time.Time) error {
	s.popupDismissCalls++
	s.lastAnnouncementID = announcementID
	s.lastUserID = userID
	return nil
}

func (s *announcementReadRepoStub) MarkBannerDismissed(_ context.Context, announcementID, userID int64, _ time.Time) error {
	s.bannerDismissCalls++
	s.lastAnnouncementID = announcementID
	s.lastUserID = userID
	return nil
}

func (s *announcementReadRepoStub) GetReadStateMapByUser(_ context.Context, _ int64, announcementIDs []int64) (map[int64]AnnouncementReadState, error) {
	s.lastReadStateRequest = append([]int64(nil), announcementIDs...)
	return map[int64]AnnouncementReadState{}, nil
}

func (*announcementReadRepoStub) GetReadMapByUsers(context.Context, int64, []int64) (map[int64]time.Time, error) {
	return nil, nil
}

func (*announcementReadRepoStub) CountByAnnouncementID(context.Context, int64) (int64, error) {
	return 0, nil
}

type announcementUserSubRepoStub struct {
	userSubRepoNoop
	active []UserSubscription
}

func (s *announcementUserSubRepoStub) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return s.active, nil
}

func TestAnnouncementServiceCreateRejectsEqualStartEndTimes(t *testing.T) {
	repo := &announcementRepoStub{}
	svc := NewAnnouncementService(repo, nil, nil, nil)
	now := time.Unix(1776790020, 0)

	_, err := svc.Create(context.Background(), &CreateAnnouncementInput{
		Title:      "公告",
		Content:    "内容",
		Status:     AnnouncementStatusActive,
		NotifyMode: AnnouncementNotifyModePopup,
		StartsAt:   &now,
		EndsAt:     &now,
	})
	require.ErrorIs(t, err, ErrAnnouncementInvalidSchedule)
}

func TestAnnouncementServiceUpdateRejectsEqualStartEndTimes(t *testing.T) {
	repo := &announcementRepoStub{
		item: &Announcement{
			ID:         1,
			Title:      "公告",
			Content:    "内容",
			Status:     AnnouncementStatusActive,
			NotifyMode: AnnouncementNotifyModePopup,
		},
	}
	svc := NewAnnouncementService(repo, nil, nil, nil)
	now := time.Unix(1776790020, 0)
	startsAt := &now
	endsAt := &now

	_, err := svc.Update(context.Background(), 1, &UpdateAnnouncementInput{
		StartsAt: &startsAt,
		EndsAt:   &endsAt,
	})
	require.ErrorIs(t, err, ErrAnnouncementInvalidSchedule)
}

func TestAnnouncementServiceCreateDefaultsAnnouncementDisplayFields(t *testing.T) {
	repo := &announcementRepoStub{}
	svc := NewAnnouncementService(repo, nil, nil, nil)

	created, err := svc.Create(context.Background(), &CreateAnnouncementInput{
		Title:      "announcement",
		Content:    "content",
		Status:     AnnouncementStatusActive,
		NotifyMode: AnnouncementNotifyModePopup,
	})

	require.NoError(t, err)
	require.Equal(t, AnnouncementSurfaceGeneral, created.Surface)
	require.Equal(t, AnnouncementPopupFrequencyOnce, created.PopupFrequency)
}

func TestAnnouncementServiceCreateRejectsInvalidDisplayFields(t *testing.T) {
	repo := &announcementRepoStub{}
	svc := NewAnnouncementService(repo, nil, nil, nil)

	_, err := svc.Create(context.Background(), &CreateAnnouncementInput{
		Title:      "announcement",
		Content:    "content",
		Status:     AnnouncementStatusActive,
		NotifyMode: AnnouncementNotifyModePopup,
		Surface:    "unknown",
	})
	require.ErrorIs(t, err, ErrAnnouncementInvalidSurface)

	_, err = svc.Create(context.Background(), &CreateAnnouncementInput{
		Title:          "announcement",
		Content:        "content",
		Status:         AnnouncementStatusActive,
		NotifyMode:     AnnouncementNotifyModePopup,
		PopupFrequency: "weekly",
	})
	require.ErrorIs(t, err, ErrAnnouncementInvalidPopupFrequency)
}

func TestAnnouncementServiceListRejectsInvalidSurfaceFilter(t *testing.T) {
	repo := &announcementRepoStub{}
	svc := NewAnnouncementService(repo, nil, nil, nil)

	_, _, err := svc.List(context.Background(), pagination.PaginationParams{}, AnnouncementListFilters{
		Surface: "unknown",
	})

	require.ErrorIs(t, err, ErrAnnouncementInvalidSurface)
}

func TestAnnouncementServiceMarkPopupDismissedRecordsPopupDismissal(t *testing.T) {
	announcementID := int64(11)
	userID := int64(7)
	readRepo := &announcementReadRepoStub{}
	svc := NewAnnouncementService(
		&announcementRepoStub{item: &Announcement{
			ID:             announcementID,
			Title:          "popup",
			Content:        "content",
			Status:         AnnouncementStatusActive,
			NotifyMode:     AnnouncementNotifyModePopup,
			Surface:        AnnouncementSurfaceGeneral,
			PopupFrequency: AnnouncementPopupFrequencyOnce,
		}},
		readRepo,
		&userRepoStub{user: &User{ID: userID, Balance: 1}},
		&announcementUserSubRepoStub{},
	)

	err := svc.MarkPopupDismissed(context.Background(), userID, announcementID)

	require.NoError(t, err)
	require.Equal(t, 1, readRepo.popupDismissCalls)
	require.Equal(t, 0, readRepo.bannerDismissCalls)
	require.Equal(t, 0, readRepo.markReadCalls)
	require.Equal(t, announcementID, readRepo.lastAnnouncementID)
	require.Equal(t, userID, readRepo.lastUserID)
}

func TestAnnouncementServiceMarkBannerDismissedDoesNotMarkRead(t *testing.T) {
	announcementID := int64(12)
	userID := int64(7)
	readRepo := &announcementReadRepoStub{}
	svc := NewAnnouncementService(
		&announcementRepoStub{item: &Announcement{
			ID:         announcementID,
			Title:      "banner",
			Content:    "content",
			Status:     AnnouncementStatusActive,
			NotifyMode: AnnouncementNotifyModeSilent,
			Surface:    AnnouncementSurfaceDashboardBanner,
		}},
		readRepo,
		&userRepoStub{user: &User{ID: userID, Balance: 1}},
		&announcementUserSubRepoStub{},
	)

	err := svc.MarkBannerDismissed(context.Background(), userID, announcementID)

	require.NoError(t, err)
	require.Equal(t, 1, readRepo.bannerDismissCalls)
	require.Equal(t, 0, readRepo.popupDismissCalls)
	require.Equal(t, 0, readRepo.markReadCalls)
	require.Equal(t, announcementID, readRepo.lastAnnouncementID)
	require.Equal(t, userID, readRepo.lastUserID)
}

func TestShouldPopupAnnouncementOnce(t *testing.T) {
	now := time.Date(2026, 6, 5, 9, 0, 0, 0, time.Local)
	ann := Announcement{
		NotifyMode:     AnnouncementNotifyModePopup,
		Surface:        AnnouncementSurfaceGeneral,
		PopupFrequency: AnnouncementPopupFrequencyOnce,
	}

	require.True(t, shouldPopupAnnouncement(ann, AnnouncementReadState{}, now))

	dismissedAt := now.Add(-time.Hour)
	require.False(t, shouldPopupAnnouncement(ann, AnnouncementReadState{
		LastPopupDismissedAt: &dismissedAt,
	}, now))
}

func TestShouldPopupAnnouncementDaily(t *testing.T) {
	now := time.Date(2026, 6, 5, 9, 0, 0, 0, time.Local)
	ann := Announcement{
		NotifyMode:     AnnouncementNotifyModePopup,
		Surface:        AnnouncementSurfaceGeneral,
		PopupFrequency: AnnouncementPopupFrequencyDaily,
	}

	sameDay := time.Date(2026, 6, 5, 1, 0, 0, 0, time.Local)
	require.False(t, shouldPopupAnnouncement(ann, AnnouncementReadState{
		LastPopupDismissedAt: &sameDay,
	}, now))

	previousDay := time.Date(2026, 6, 4, 23, 59, 0, 0, time.Local)
	require.True(t, shouldPopupAnnouncement(ann, AnnouncementReadState{
		LastPopupDismissedAt: &previousDay,
	}, now))
}

func TestShouldPopupAnnouncementIgnoresNonGeneralSurface(t *testing.T) {
	now := time.Date(2026, 6, 5, 9, 0, 0, 0, time.Local)
	ann := Announcement{
		NotifyMode:     AnnouncementNotifyModePopup,
		Surface:        AnnouncementSurfaceDashboardBanner,
		PopupFrequency: AnnouncementPopupFrequencyDaily,
	}

	require.False(t, shouldPopupAnnouncement(ann, AnnouncementReadState{}, now))
}
