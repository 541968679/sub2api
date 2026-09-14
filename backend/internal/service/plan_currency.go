package service

import "strings"

// NormalizePlanCurrency trims and uppercases plan currency labels for storage/display.
// Empty input stays empty (historical UI path).
func NormalizePlanCurrency(currency string) string {
	return strings.ToUpper(strings.TrimSpace(currency))
}
