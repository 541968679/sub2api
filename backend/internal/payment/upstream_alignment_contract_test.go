//go:build unit

package payment

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpstreamPaymentCurrencyContract(t *testing.T) {
	currency, err := NormalizePaymentCurrency(" kwd ")
	require.NoError(t, err)
	require.Equal(t, "KWD", currency)
	require.Equal(t, 3, CurrencyMinorUnit(currency))
	require.Equal(t, int64(1234), mustMinorUnit(t, "1.234", currency))
	require.Equal(t, "1.234", FormatAmountForCurrency(1.234, currency))

	_, err = AmountToMinorUnit("1.2345", currency)
	require.Error(t, err)
}

func TestUpstreamPaymentStatusAndProviderContract(t *testing.T) {
	require.Equal(t, PaymentType("airwallex"), TypeAirwallex)
	require.Equal(t, "REFUND_PENDING", OrderStatusRefundPending)
	require.Equal(t, TypeAirwallex, GetBasePaymentType(TypeAirwallex))
}

func mustMinorUnit(t *testing.T, amount, currency string) int64 {
	t.Helper()
	value, err := AmountToMinorUnit(amount, currency)
	require.NoError(t, err)
	return value
}
