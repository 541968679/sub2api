//go:build unit

package service

import (
	"context"
	"sort"
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
	require.Equal(t, true, repo.updates[AccountListPinnedExtraKey])
	require.Equal(t, true, got.Extra[AccountListPinnedExtraKey])
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

func (r *createAccountPinRepo) SyncScheduleUsers(_ context.Context, _ int64, _ AccountUserScheduleWrite) error {
	return nil
}

func (r *createAccountPinRepo) ListScheduleUserRefs(_ context.Context, _ []int64) ([]ScheduleUserRef, error) {
	return nil, nil
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
	_, pinned := repo.created.Extra[AccountListPinnedExtraKey]
	require.False(t, pinned, "unix-ms create list_order is not a manual pin")
}

type reorderAccountRepo struct {
	mockAccountRepoForGemini
	byID    map[int64]*Account
	updates map[int64]map[string]any
}

func (r *reorderAccountRepo) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	out := make([]*Account, 0, len(ids))
	for _, id := range ids {
		acc, ok := r.byID[id]
		if !ok {
			continue
		}
		cp := *acc
		if acc.Extra != nil {
			cp.Extra = make(map[string]any, len(acc.Extra))
			for k, v := range acc.Extra {
				cp.Extra[k] = v
			}
		}
		out = append(out, &cp)
	}
	return out, nil
}

func (r *reorderAccountRepo) UpdateExtra(_ context.Context, id int64, updates map[string]any) error {
	acc, ok := r.byID[id]
	if !ok {
		return ErrAccountNotFound
	}
	if r.updates == nil {
		r.updates = map[int64]map[string]any{}
	}
	r.updates[id] = updates
	if acc.Extra == nil {
		acc.Extra = map[string]any{}
	}
	for k, v := range updates {
		acc.Extra[k] = v
	}
	return nil
}

func TestReorderAccounts_PreservesRankMultiset(t *testing.T) {
	t.Parallel()
	repo := &reorderAccountRepo{
		byID: map[int64]*Account{
			1: {ID: 1, Extra: map[string]any{AccountListOrderExtraKey: int64(100)}},
			2: {ID: 2, Extra: map[string]any{AccountListOrderExtraKey: int64(90)}},
			3: {ID: 3, Extra: map[string]any{AccountListOrderExtraKey: int64(80)}},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	// New top-to-bottom: 2, 1, 3 → slots [100, 90, 80]
	err := svc.ReorderAccounts(context.Background(), []int64{2, 1, 3})
	require.NoError(t, err)
	require.Equal(t, int64(100), repo.updates[2][AccountListOrderExtraKey])
	require.Equal(t, int64(90), repo.updates[1][AccountListOrderExtraKey])
	require.Equal(t, int64(80), repo.updates[3][AccountListOrderExtraKey])
	require.Equal(t, true, repo.updates[2][AccountListPinnedExtraKey])
	require.Equal(t, true, repo.updates[1][AccountListPinnedExtraKey])
	require.Equal(t, true, repo.updates[3][AccountListPinnedExtraKey])
}

func TestReorderAccounts_AllZeroUsesTimestampBase(t *testing.T) {
	t.Parallel()
	repo := &reorderAccountRepo{
		byID: map[int64]*Account{
			1: {ID: 1, Extra: map[string]any{}},
			2: {ID: 2, Extra: map[string]any{}},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	before := time.Now().UnixMilli()
	err := svc.ReorderAccounts(context.Background(), []int64{2, 1})
	require.NoError(t, err)
	top, ok := repo.updates[2][AccountListOrderExtraKey].(int64)
	require.True(t, ok)
	second, ok := repo.updates[1][AccountListOrderExtraKey].(int64)
	require.True(t, ok)
	require.GreaterOrEqual(t, top, before)
	require.Equal(t, top-1, second)
	require.Equal(t, true, repo.updates[2][AccountListPinnedExtraKey])
	require.Equal(t, true, repo.updates[1][AccountListPinnedExtraKey])
}

func TestComputeAccountListOrderSlots(t *testing.T) {
	t.Parallel()
	// Keep unique positive multiset.
	got := computeAccountListOrderSlots([]int64{100, 90, 80}, 999)
	require.Equal(t, []int64{100, 90, 80}, got)

	// Respread when zeros present under a positive max.
	got = computeAccountListOrderSlots([]int64{100, 0, 0}, 999)
	require.Equal(t, []int64{100, 99, 98}, got)

	// All zero → base down.
	got = computeAccountListOrderSlots([]int64{0, 0}, 5000)
	require.Equal(t, []int64{5000, 4999}, got)
}

func extraUpdateKeys(updates map[string]any) []string {
	keys := make([]string, 0, len(updates))
	for k := range updates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestReorderAccountsAutoSort_ReservedBand(t *testing.T) {
	t.Parallel()
	repo := &reorderAccountRepo{
		byID: map[int64]*Account{
			1: {ID: 1, Extra: map[string]any{AccountListOrderExtraKey: time.Now().UnixMilli()}},
			2: {ID: 2, Extra: map[string]any{AccountListOrderExtraKey: int64(90)}},
			3: {ID: 3, Extra: map[string]any{AccountListOrderExtraKey: int64(80)}},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	err := svc.ReorderAccountsAutoSort(context.Background(), []int64{3, 1, 2})
	require.NoError(t, err)
	require.Equal(t, int64(3), repo.updates[3][AccountListOrderExtraKey])
	require.Equal(t, int64(2), repo.updates[1][AccountListOrderExtraKey])
	require.Equal(t, int64(1), repo.updates[2][AccountListOrderExtraKey])
	require.Equal(t, []string{AccountListOrderExtraKey}, extraUpdateKeys(repo.updates[3]))
	require.Equal(t, []string{AccountListOrderExtraKey}, extraUpdateKeys(repo.updates[1]))
	require.Equal(t, []string{AccountListOrderExtraKey}, extraUpdateKeys(repo.updates[2]))
	require.NotContains(t, repo.updates[3], AccountListPinnedExtraKey)
	require.NotContains(t, repo.updates[1], AccountListPinnedExtraKey)
	require.NotContains(t, repo.updates[2], AccountListPinnedExtraKey)
	require.NotContains(t, repo.updates[3], "priority")
	require.NotContains(t, repo.updates[1], "priority")
	require.NotContains(t, repo.updates[2], "priority")
	require.Less(t, repo.updates[3][AccountListOrderExtraKey].(int64), int64(10_000))
	require.Less(t, repo.updates[1][AccountListOrderExtraKey].(int64), int64(10_000))
	require.Less(t, repo.updates[2][AccountListOrderExtraKey].(int64), int64(10_000))
}

func TestReorderAccountsAutoSort_SkipsPinned(t *testing.T) {
	t.Parallel()
	repo := &reorderAccountRepo{
		byID: map[int64]*Account{
			1: {ID: 1, Extra: map[string]any{AccountListOrderExtraKey: int64(10)}},
			2: {ID: 2, Extra: map[string]any{
				AccountListOrderExtraKey:  int64(9_000_000_000_000),
				AccountListPinnedExtraKey: true,
			}},
			3: {ID: 3, Extra: map[string]any{AccountListOrderExtraKey: int64(8)}},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	err := svc.ReorderAccountsAutoSort(context.Background(), []int64{1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, int64(2), repo.updates[1][AccountListOrderExtraKey])
	require.Equal(t, int64(1), repo.updates[3][AccountListOrderExtraKey])
	_, pinnedWritten := repo.updates[2]
	require.False(t, pinnedWritten, "pinned account list_order must stay untouched")
	require.Equal(t, int64(9_000_000_000_000), repo.byID[2].Extra[AccountListOrderExtraKey])
}

func TestReorderAccountsAutoSort_SkipsPinnedStringTrue(t *testing.T) {
	t.Parallel()
	repo := &reorderAccountRepo{
		byID: map[int64]*Account{
			1: {ID: 1, Extra: map[string]any{AccountListOrderExtraKey: int64(10)}},
			2: {ID: 2, Extra: map[string]any{
				AccountListOrderExtraKey:  int64(9_000_000_000_000),
				AccountListPinnedExtraKey: "true",
			}},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	err := svc.ReorderAccountsAutoSort(context.Background(), []int64{1, 2})
	require.NoError(t, err)
	require.Equal(t, int64(1), repo.updates[1][AccountListOrderExtraKey])
	_, pinnedWritten := repo.updates[2]
	require.False(t, pinnedWritten)
	require.Equal(t, int64(9_000_000_000_000), repo.byID[2].Extra[AccountListOrderExtraKey])
}

func TestReorderAccountsAutoSort_LeavesOtherAccountsUntouched(t *testing.T) {
	t.Parallel()
	outsideOrder := int64(1_700_000_000_000)
	repo := &reorderAccountRepo{
		byID: map[int64]*Account{
			1: {ID: 1, Extra: map[string]any{AccountListOrderExtraKey: int64(10)}},
			2: {ID: 2, Extra: map[string]any{AccountListOrderExtraKey: int64(9)}},
			9: {ID: 9, Extra: map[string]any{AccountListOrderExtraKey: outsideOrder}},
		},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	err := svc.ReorderAccountsAutoSort(context.Background(), []int64{2, 1})
	require.NoError(t, err)
	require.Equal(t, int64(2), repo.updates[2][AccountListOrderExtraKey])
	require.Equal(t, int64(1), repo.updates[1][AccountListOrderExtraKey])
	_, outsideWritten := repo.updates[9]
	require.False(t, outsideWritten)
	require.Equal(t, outsideOrder, repo.byID[9].Extra[AccountListOrderExtraKey])
}

func TestReorderAccountsAutoSort_RejectsOverCap(t *testing.T) {
	t.Parallel()
	ids := make([]int64, maxAccountAutoSortIDs+1)
	byID := make(map[int64]*Account, len(ids))
	for i := range ids {
		id := int64(i + 1)
		ids[i] = id
		byID[id] = &Account{ID: id, Extra: map[string]any{}}
	}
	repo := &reorderAccountRepo{byID: byID}
	svc := &adminServiceImpl{accountRepo: repo}

	err := svc.ReorderAccountsAutoSort(context.Background(), ids)
	require.Error(t, err)
	require.Empty(t, repo.updates)
}
