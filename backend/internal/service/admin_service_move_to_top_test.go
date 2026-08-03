//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type moveToTopAccountRepo struct {
	mockAccountRepoForGemini
	account *Account
	updates map[string]any
}

func (r *moveToTopAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	if r.account == nil || r.account.ID != id {
		return nil, ErrAccountNotFound
	}
	cp := *r.account
	if r.account.Extra != nil {
		cp.Extra = make(map[string]any, len(r.account.Extra))
		for k, v := range r.account.Extra {
			cp.Extra[k] = v
		}
	}
	return &cp, nil
}

func (r *moveToTopAccountRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	if r.account == nil || r.account.ID != id {
		return ErrAccountNotFound
	}
	r.updates = updates
	if r.account.Extra == nil {
		r.account.Extra = map[string]any{}
	}
	for k, v := range updates {
		r.account.Extra[k] = v
	}
	return nil
}

func TestMoveAccountToTop_SetsListOrder(t *testing.T) {
	t.Parallel()
	repo := &moveToTopAccountRepo{
		account: &Account{ID: 42, Name: "demo", Extra: map[string]any{}},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	before := time.Now().UnixMilli()
	got, err := svc.MoveAccountToTop(context.Background(), 42)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, int64(42), got.ID)

	raw, ok := repo.updates[AccountListOrderExtraKey]
	require.True(t, ok)
	order, ok := raw.(int64)
	require.True(t, ok)
	require.GreaterOrEqual(t, order, before)

	require.NotNil(t, got.Extra)
	require.Equal(t, order, got.Extra[AccountListOrderExtraKey])
}
