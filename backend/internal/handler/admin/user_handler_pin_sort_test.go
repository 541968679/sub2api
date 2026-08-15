//go:build unit

package admin

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSortUsersByCurrentConcurrency_PinnedFirstNewerAboveOlder(t *testing.T) {
	older := time.Date(2026, time.August, 1, 10, 0, 0, 0, time.UTC)
	newer := older.Add(2 * time.Hour)
	users := []service.User{
		{ID: 1, Email: "busy-unpinned@example.com"},
		{ID: 2, Email: "old-pin@example.com", PinnedAt: &older},
		{ID: 3, Email: "new-pin@example.com", PinnedAt: &newer},
		{ID: 4, Email: "idle-unpinned@example.com"},
	}
	loadInfo := map[int64]*service.UserLoadInfo{
		1: {CurrentConcurrency: 9},
		2: {CurrentConcurrency: 8},
		3: {CurrentConcurrency: 1},
		4: {CurrentConcurrency: 0},
	}

	sortUsersByCurrentConcurrency(users, loadInfo, pagination.SortOrderDesc)

	require.Equal(t, int64(3), users[0].ID)
	require.Equal(t, int64(2), users[1].ID)
	require.Equal(t, int64(1), users[2].ID)
	require.Equal(t, int64(4), users[3].ID)
}
