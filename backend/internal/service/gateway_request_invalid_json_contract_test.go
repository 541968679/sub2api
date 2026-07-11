//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDescribeInvalidJSON_ReportsStructureWithoutBodyContent(t *testing.T) {
	const secret = "secret-user-prompt-123"
	err := DescribeInvalidJSON([]byte(`{"input":"` + secret + `",}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "len=")
	require.Contains(t, err.Error(), "offset=")
	require.False(t, strings.Contains(err.Error(), secret))
}
