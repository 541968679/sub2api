//go:build unit

package repository

import "testing"

func TestOpenAIOAuth7dPriceModelSQLPrefersUpstream(t *testing.T) {
	t.Parallel()
	if openAI7dLiteLLMPriceModelSQL != `COALESCE(NULLIF(TRIM(upstream_model), ''), NULLIF(TRIM(requested_model), ''), model)` {
		t.Fatalf("unexpected price model SQL: %s", openAI7dLiteLLMPriceModelSQL)
	}
}
