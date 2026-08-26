package repository

import (
	"fmt"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestIsUndefinedColumnError(t *testing.T) {
	t.Parallel()
	require.True(t, isUndefinedColumnError(&pq.Error{Code: "42703"}))
	require.True(t, isUndefinedColumnError(fmt.Errorf("wrap: %w", &pq.Error{Code: "42703"})))
	require.False(t, isUndefinedColumnError(&pq.Error{Code: "42P01"}))
	require.False(t, isUndefinedColumnError(fmt.Errorf("column soft_cooldown does not exist")))
}
