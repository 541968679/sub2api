package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubUpstreamModelsFetcher struct {
	byID map[int64][]string
	err  map[int64]error
}

func (s stubUpstreamModelsFetcher) FetchUpstreamSupportedModels(_ context.Context, account *Account) ([]string, error) {
	if account == nil {
		return nil, errors.New("nil account")
	}
	if s.err != nil {
		if err, ok := s.err[account.ID]; ok {
			return nil, err
		}
	}
	if s.byID == nil {
		return nil, nil
	}
	return append([]string(nil), s.byID[account.ID]...), nil
}

func TestCollectGroupAccountUpstreamModelIDsUnionsLiveLists(t *testing.T) {
	parent := int64(1)
	accounts := []Account{
		{ID: 10, Platform: PlatformOpenAI},
		{ID: 11, Platform: PlatformOpenAI},
		{ID: 12, Platform: PlatformOpenAI, ParentAccountID: &parent},
	}
	fetcher := stubUpstreamModelsFetcher{
		byID: map[int64][]string{
			10: {"glm-5.3", "glm-4.7"},
			11: {"kimi-k2.5", "glm-5.3"},
			12: {"should-not-appear"},
		},
		err: map[int64]error{
			99: errors.New("unused"),
		},
	}
	got := CollectGroupAccountUpstreamModelIDs(context.Background(), fetcher, accounts)
	require.Equal(t, []string{"glm-4.7", "glm-5.3", "kimi-k2.5"}, got)
}

func TestCollectGroupAccountUpstreamModelIDsSkipsFailedAccounts(t *testing.T) {
	accounts := []Account{
		{ID: 10, Platform: PlatformOpenAI},
		{ID: 11, Platform: PlatformOpenAI},
	}
	fetcher := stubUpstreamModelsFetcher{
		byID: map[int64][]string{
			10: {"MiniMax-M2.5"},
		},
		err: map[int64]error{
			11: errors.New("upstream 401"),
		},
	}
	got := CollectGroupAccountUpstreamModelIDs(context.Background(), fetcher, accounts)
	require.Equal(t, []string{"MiniMax-M2.5"}, got)
}

func TestMergeCcsImportPickerOptionsPinsDefault(t *testing.T) {
	got := mergeCcsImportPickerOptions("glm-5.3", []string{"kimi-k2.5", "glm-5.3"})
	require.Equal(t, []string{"glm-5.3", "kimi-k2.5"}, got)
}
