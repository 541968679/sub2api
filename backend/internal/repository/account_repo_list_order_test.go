package repository

import "testing"

func TestAccountListColumnIsPrimary(t *testing.T) {
	t.Parallel()

	if !accountListColumnIsPrimary("upstream_rate_multiplier") {
		t.Fatal("upstream_rate_multiplier must sort as the primary key")
	}
	if !accountListColumnIsPrimary(" Upstream_Rate_Multiplier ") {
		t.Fatal("sort_by must be trimmed and case-insensitive")
	}
	if accountListColumnIsPrimary("priority") {
		t.Fatal("priority must keep pin-first ordering")
	}
	if accountListColumnIsPrimary("rate_multiplier") {
		t.Fatal("billing rate must keep pin-first ordering")
	}
	if accountListColumnIsPrimary("") {
		t.Fatal("default sort must keep pin-first ordering")
	}
}
