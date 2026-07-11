package payment

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const DefaultPaymentCurrency = "CNY"

type paymentCurrencyAmountUnit struct {
	apiMinorUnit      int
	maxFractionDigits int
}

var paymentCurrencyAmountUnits = map[string]paymentCurrencyAmountUnit{
	"BIF": {0, 0}, "CLP": {0, 0}, "DJF": {0, 0}, "GNF": {0, 0},
	"JPY": {0, 0}, "KMF": {0, 0}, "KRW": {0, 0}, "MGA": {0, 0},
	"PYG": {0, 0}, "RWF": {0, 0}, "VND": {0, 0}, "VUV": {0, 0},
	"XAF": {0, 0}, "XOF": {0, 0}, "XPF": {0, 0},
	"ISK": {2, 0}, "UGX": {2, 0},
	"BHD": {3, 3}, "IQD": {3, 3}, "JOD": {3, 3}, "KWD": {3, 3},
	"LYD": {3, 3}, "OMR": {3, 3}, "TND": {3, 3},
}

func NormalizePaymentCurrency(raw string) (string, error) {
	currency := strings.ToUpper(strings.TrimSpace(raw))
	if currency == "" {
		return DefaultPaymentCurrency, nil
	}
	if len(currency) != 3 {
		return "", fmt.Errorf("payment currency must be a 3-letter ISO currency code")
	}
	for _, ch := range currency {
		if ch < 'A' || ch > 'Z' {
			return "", fmt.Errorf("payment currency must be a 3-letter ISO currency code")
		}
	}
	return currency, nil
}

func CurrencyMinorUnit(currency string) int {
	return paymentCurrencyAmountUnitFor(currency).apiMinorUnit
}

func CurrencyMaxFractionDigits(currency string) int {
	return paymentCurrencyAmountUnitFor(currency).maxFractionDigits
}

func FormatAmountForCurrency(amount float64, currency string) string {
	return strconv.FormatFloat(amount, 'f', CurrencyMaxFractionDigits(currency), 64)
}

func paymentCurrencyAmountUnitFor(currency string) paymentCurrencyAmountUnit {
	normalized, err := NormalizePaymentCurrency(currency)
	if err != nil {
		return paymentCurrencyAmountUnit{2, 2}
	}
	if amountUnit, ok := paymentCurrencyAmountUnits[normalized]; ok {
		return amountUnit
	}
	return paymentCurrencyAmountUnit{2, 2}
}

func AmountToMinorUnit(amountStr, currency string) (int64, error) {
	amount, err := strconv.ParseFloat(strings.TrimSpace(amountStr), 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, fmt.Errorf("invalid amount: %s", amountStr)
	}
	normalized, err := NormalizePaymentCurrency(currency)
	if err != nil {
		return 0, err
	}
	unit := paymentCurrencyAmountUnitFor(normalized)
	displayFactor := math.Pow10(unit.maxFractionDigits)
	if math.Abs(amount*displayFactor-math.Round(amount*displayFactor)) > 1e-8 {
		if unit.maxFractionDigits == 0 {
			return 0, fmt.Errorf("payment amount for %s must be a whole number", normalized)
		}
		return 0, fmt.Errorf("payment amount for %s must not have more than %d decimal places", normalized, unit.maxFractionDigits)
	}
	return int64(math.Round(amount * math.Pow10(unit.apiMinorUnit))), nil
}

func MinorUnitToAmount(value int64, currency string) float64 {
	return float64(value) / math.Pow10(CurrencyMinorUnit(currency))
}
