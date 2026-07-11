//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type roleUserRepoStub struct {
	*userRepoStub
	adminTotal int64
}

func (s *roleUserRepoStub) ListWithFilters(_ context.Context, params pagination.PaginationParams, filters UserListFilters) ([]User, *pagination.PaginationResult, error) {
	if filters.Role != RoleAdmin {
		panic("unexpected non-admin role filter")
	}
	return nil, &pagination.PaginationResult{Page: params.Page, PageSize: params.PageSize, Total: s.adminTotal}, nil
}

func TestAdminService_CreateUser_RoleContract(t *testing.T) {
	t.Run("defaults to user", func(t *testing.T) {
		repo := &userRepoStub{nextID: 1}
		created, err := (&adminServiceImpl{userRepo: repo}).CreateUser(context.Background(), &CreateUserInput{
			Email: "default@example.com", Password: "secret123",
		})
		require.NoError(t, err)
		require.Equal(t, RoleUser, created.Role)
	})

	t.Run("accepts admin", func(t *testing.T) {
		repo := &userRepoStub{nextID: 2}
		created, err := (&adminServiceImpl{userRepo: repo}).CreateUser(context.Background(), &CreateUserInput{
			Email: "admin@example.com", Password: "secret123", Role: RoleAdmin, ActorAdminID: 99,
		})
		require.NoError(t, err)
		require.Equal(t, RoleAdmin, created.Role)
	})

	t.Run("rejects unknown role", func(t *testing.T) {
		repo := &userRepoStub{nextID: 3}
		_, err := (&adminServiceImpl{userRepo: repo}).CreateUser(context.Background(), &CreateUserInput{
			Email: "bad@example.com", Password: "secret123", Role: "owner",
		})
		require.Error(t, err)
		require.Empty(t, repo.created)
	})
}

func TestAdminService_UpdateUser_RoleSafetyAndCache(t *testing.T) {
	t.Run("rejects self demotion", func(t *testing.T) {
		repo := &roleUserRepoStub{userRepoStub: &userRepoStub{user: &User{ID: 7, Email: "self@example.com", Role: RoleAdmin}}, adminTotal: 2}
		_, err := (&adminServiceImpl{userRepo: repo}).UpdateUser(context.Background(), 7, &UpdateUserInput{
			Role: RoleUser, ActorAdminID: 7,
		})
		require.Error(t, err)
		require.Empty(t, repo.updated)
	})

	t.Run("rejects demoting last admin", func(t *testing.T) {
		repo := &roleUserRepoStub{userRepoStub: &userRepoStub{user: &User{ID: 7, Email: "last@example.com", Role: RoleAdmin}}, adminTotal: 1}
		_, err := (&adminServiceImpl{userRepo: repo}).UpdateUser(context.Background(), 7, &UpdateUserInput{
			Role: RoleUser, ActorAdminID: 9,
		})
		require.Error(t, err)
		require.Empty(t, repo.updated)
	})

	t.Run("demotes another admin and invalidates auth cache", func(t *testing.T) {
		repo := &roleUserRepoStub{userRepoStub: &userRepoStub{user: &User{ID: 7, Email: "other@example.com", Role: RoleAdmin}}, adminTotal: 2}
		invalidator := &authCacheInvalidatorStub{}
		updated, err := (&adminServiceImpl{userRepo: repo, authCacheInvalidator: invalidator}).UpdateUser(context.Background(), 7, &UpdateUserInput{
			Role: RoleUser, ActorAdminID: 9,
		})
		require.NoError(t, err)
		require.Equal(t, RoleUser, updated.Role)
		require.Equal(t, []int64{7}, invalidator.userIDs)
	})
}
