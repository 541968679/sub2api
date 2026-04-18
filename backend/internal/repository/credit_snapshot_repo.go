package repository

import (
	"context"
	"errors"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/aicreditsnapshot"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type creditSnapshotRepository struct {
	client *dbent.Client
}

func NewCreditSnapshotRepository(client *dbent.Client) service.CreditSnapshotRepository {
	return &creditSnapshotRepository{client: client}
}

func (r *creditSnapshotRepository) Insert(ctx context.Context, snap *service.CreditSnapshot) error {
	if snap == nil {
		return errors.New("nil snapshot")
	}
	client := clientFromContext(ctx, r.client)
	builder := client.AICreditSnapshot.Create().
		SetEmail(snap.Email).
		SetCreditType(snap.CreditType).
		SetAmount(snap.Amount)
	if !snap.CapturedAt.IsZero() {
		builder = builder.SetCapturedAt(snap.CapturedAt)
	}
	row, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	snap.ID = row.ID
	snap.CapturedAt = row.CapturedAt
	return nil
}

func (r *creditSnapshotRepository) ListInRange(ctx context.Context, start, end time.Time) (map[string][]service.CreditSnapshot, error) {
	rows, err := r.client.AICreditSnapshot.Query().
		Where(
			aicreditsnapshot.CapturedAtGTE(start),
			aicreditsnapshot.CapturedAtLTE(end),
		).
		Order(dbent.Asc(aicreditsnapshot.FieldCapturedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]service.CreditSnapshot, 16)
	for _, row := range rows {
		out[row.Email] = append(out[row.Email], service.CreditSnapshot{
			ID:         row.ID,
			Email:      row.Email,
			CreditType: row.CreditType,
			Amount:     row.Amount,
			CapturedAt: row.CapturedAt,
		})
	}
	return out, nil
}

func (r *creditSnapshotRepository) GetLatestBefore(ctx context.Context, email string, t time.Time) (*service.CreditSnapshot, error) {
	row, err := r.client.AICreditSnapshot.Query().
		Where(
			aicreditsnapshot.EmailEQ(email),
			aicreditsnapshot.CapturedAtLT(t),
		).
		Order(dbent.Desc(aicreditsnapshot.FieldCapturedAt)).
		First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &service.CreditSnapshot{
		ID:         row.ID,
		Email:      row.Email,
		CreditType: row.CreditType,
		Amount:     row.Amount,
		CapturedAt: row.CapturedAt,
	}, nil
}
