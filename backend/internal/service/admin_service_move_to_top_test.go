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

type createAccountPinRepo struct {
	mockAccountRepoForGemini
	created *Account
}

func (r *createAccountPinRepo) Create(_ context.Context, account *Account) error {
	cp := *account
	if account.Extra != nil {
		cp.Extra = make(map[string]any, len(account.Extra))
		for k, v := range account.Extra {
			cp.Extra[k] = v
		}
	}
	cp.ID = 99
	r.created = &cp
	account.ID = 99
	return nil
}

func (r *createAccountPinRepo) BindGroups(_ context.Context, _ int64, _ []int64) error {
	return nil
}

func TestCreateAccount_PinsNewAccountToListTop(t *testing.T) {
	t.Parallel()
	repo := &createAccountPinRepo{}
	svc := &adminServiceImpl{
		accountRepo: repo,
	}

	before := time.Now().UnixMilli()
	got, err := svc.CreateAccount(context.Background(), &CreateAccountInput{
		Name:                 "new-account",
		Platform:             PlatformOpenAI,
		Type:                 AccountTypeAPIKey,
		Credentials:          map[string]any{"api_key": "sk-test"},
		Concurrency:          1,
		Priority:             1,
		SkipDefaultGroupBind: true,
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, repo.created)
	require.NotNil(t, repo.created.Extra)

	raw, ok := repo.created.Extra[AccountListOrderExtraKey]
	require.True(t, ok, "new account must set list_order so it appears above older pins")
	order, ok := raw.(int64)
	require.True(t, ok)
	require.GreaterOrEqual(t, order, before)
}
