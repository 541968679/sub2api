package repository

import (
	"context"
	"database/sql"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/announcementread"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type announcementReadRepository struct {
	client *dbent.Client
}

func NewAnnouncementReadRepository(client *dbent.Client) service.AnnouncementReadRepository {
	return &announcementReadRepository{client: client}
}

func (r *announcementReadRepository) MarkRead(ctx context.Context, announcementID, userID int64, readAt time.Time) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.ExecContext(ctx, `
		INSERT INTO announcement_reads (announcement_id, user_id, read_at, created_at)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (announcement_id, user_id)
		DO UPDATE SET read_at = COALESCE(announcement_reads.read_at, EXCLUDED.read_at)
	`, announcementID, userID, readAt)
	return err
}

func (r *announcementReadRepository) MarkPopupDismissed(ctx context.Context, announcementID, userID int64, dismissedAt time.Time) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.ExecContext(ctx, `
		INSERT INTO announcement_reads (
			announcement_id,
			user_id,
			read_at,
			last_popup_dismissed_at,
			created_at
		)
		VALUES ($1, $2, $3, $3, $3)
		ON CONFLICT (announcement_id, user_id)
		DO UPDATE SET
			read_at = COALESCE(announcement_reads.read_at, EXCLUDED.read_at),
			last_popup_dismissed_at = EXCLUDED.last_popup_dismissed_at
	`, announcementID, userID, dismissedAt)
	return err
}

func (r *announcementReadRepository) MarkBannerDismissed(ctx context.Context, announcementID, userID int64, dismissedAt time.Time) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.ExecContext(ctx, `
		INSERT INTO announcement_reads (
			announcement_id,
			user_id,
			read_at,
			banner_dismissed_at,
			created_at
		)
		VALUES ($1, $2, NULL, $3, $3)
		ON CONFLICT (announcement_id, user_id)
		DO UPDATE SET banner_dismissed_at = EXCLUDED.banner_dismissed_at
	`, announcementID, userID, dismissedAt)
	return err
}

func (r *announcementReadRepository) GetReadStateMapByUser(
	ctx context.Context,
	userID int64,
	announcementIDs []int64,
) (map[int64]service.AnnouncementReadState, error) {
	out := make(map[int64]service.AnnouncementReadState, len(announcementIDs))
	if len(announcementIDs) == 0 {
		return out, nil
	}

	client := clientFromContext(ctx, r.client)
	rows, err := client.AnnouncementRead.Query().
		Where(
			announcementread.UserIDEQ(userID),
			announcementread.AnnouncementIDIn(announcementIDs...),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}

	for i := range rows {
		out[rows[i].AnnouncementID] = service.AnnouncementReadState{
			ReadAt:               rows[i].ReadAt,
			LastPopupDismissedAt: rows[i].LastPopupDismissedAt,
			BannerDismissedAt:    rows[i].BannerDismissedAt,
		}
	}
	return out, nil
}

func (r *announcementReadRepository) GetReadMapByUsers(
	ctx context.Context,
	announcementID int64,
	userIDs []int64,
) (map[int64]time.Time, error) {
	out := make(map[int64]time.Time, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `
		SELECT user_id, read_at
		FROM announcement_reads
		WHERE announcement_id = $1
		  AND user_id = ANY($2)
		  AND read_at IS NOT NULL
	`, announcementID, pq.Array(userIDs))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var userID int64
		var readAt sql.NullTime
		if err := rows.Scan(&userID, &readAt); err != nil {
			return nil, err
		}
		if readAt.Valid {
			out[userID] = readAt.Time
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *announcementReadRepository) CountByAnnouncementID(ctx context.Context, announcementID int64) (int64, error) {
	client := clientFromContext(ctx, r.client)
	count, err := client.AnnouncementRead.Query().
		Where(
			announcementread.AnnouncementIDEQ(announcementID),
			announcementread.ReadAtNotNil(),
		).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}
