//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminService_GetUserIncludeDeleted(t *testing.T) {
	repo := &userRepoStub{user: &User{ID: 7, Email: "deleted@example.com"}}
	service := &adminServiceImpl{userRepo: repo}

	got, err := service.GetUserIncludeDeleted(context.Background(), 7)
	require.NoError(t, err)
	require.Equal(t, int64(7), got.ID)
}
