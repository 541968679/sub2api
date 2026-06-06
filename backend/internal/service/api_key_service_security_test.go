//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type apiKeySecurityRepo struct {
	APIKeyRepository
	created *APIKey
	updated *APIKey
	key     *APIKey
}

func (r *apiKeySecurityRepo) Create(_ context.Context, key *APIKey) error {
	clone := *key
	r.created = &clone
	return nil
}

func (r *apiKeySecurityRepo) GetByID(_ context.Context, _ int64) (*APIKey, error) {
	if r.key == nil {
		return nil, ErrAPIKeyNotFound
	}
	clone := *r.key
	return &clone, nil
}

func (r *apiKeySecurityRepo) Update(_ context.Context, key *APIKey) error {
	clone := *key
	r.updated = &clone
	return nil
}

func (r *apiKeySecurityRepo) ExistsByKey(context.Context, string) (bool, error) {
	return false, nil
}

func (r *apiKeySecurityRepo) ListByUserID(context.Context, int64, pagination.PaginationParams, APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{}, nil
}

type apiKeySecurityUserRepo struct {
	UserRepository
	user *User
}

func (r *apiKeySecurityUserRepo) GetByID(context.Context, int64) (*User, error) {
	if r.user == nil {
		return nil, ErrUserNotFound
	}
	clone := *r.user
	return &clone, nil
}

func TestAPIKeyServiceEscapesAPIKeyNames(t *testing.T) {
	rawName := `<img src=x onerror=alert(1)>`
	escapedName := `&lt;img src=x onerror=alert(1)&gt;`
	repo := &apiKeySecurityRepo{}
	userRepo := &apiKeySecurityUserRepo{user: &User{ID: 42, Status: StatusActive}}
	svc := NewAPIKeyService(repo, userRepo, nil, nil, nil, nil, &config.Config{})

	created, err := svc.Create(context.Background(), 42, CreateAPIKeyRequest{Name: rawName})
	require.NoError(t, err)
	require.Equal(t, escapedName, created.Name)
	require.Equal(t, escapedName, repo.created.Name)

	repo.key = &APIKey{ID: 10, UserID: 42, Key: "sk-existing", Name: "old", Status: StatusActive}
	updated, err := svc.Update(context.Background(), 10, 42, UpdateAPIKeyRequest{Name: &rawName})
	require.NoError(t, err)
	require.Equal(t, escapedName, updated.Name)
	require.Equal(t, escapedName, repo.updated.Name)
}

func TestAPIKeyServiceEscapesDistributionAPIKeyNames(t *testing.T) {
	rawName := `<svg onload=alert(1)>`
	repo := &apiKeySecurityRepo{}
	userRepo := &apiKeySecurityUserRepo{user: &User{ID: 42, Status: StatusActive}}
	svc := NewAPIKeyService(repo, userRepo, nil, nil, nil, nil, &config.Config{})

	created, err := svc.CreateForDistribution(context.Background(), 42, CreateAPIKeyRequest{Name: rawName})
	require.NoError(t, err)
	require.Equal(t, `&lt;svg onload=alert(1)&gt;`, created.Name)
	require.Equal(t, created.Name, repo.created.Name)
}
