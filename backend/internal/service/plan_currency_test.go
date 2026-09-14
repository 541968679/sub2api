package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizePlanCurrency(t *testing.T) {
	require.Equal(t, "", NormalizePlanCurrency("  "))
	require.Equal(t, "USD", NormalizePlanCurrency(" usd "))
	require.Equal(t, "CNY", NormalizePlanCurrency("cny"))
}
