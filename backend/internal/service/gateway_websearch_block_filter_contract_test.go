//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestFilterWebSearchHistoryBlocks_PreservesSummaryAndOtherTools(t *testing.T) {
	body := []byte(`{"model":"glm-4.7","messages":[{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"local","input":{}},{"type":"server_tool_use","id":"srvtoolu_ws_1","name":"web_search","input":{"query":"q"}},{"type":"web_search_tool_result","tool_use_id":"srvtoolu_ws_1","content":[]},{"type":"text","text":"search summary"}]}]}`)

	got := FilterWebSearchHistoryBlocks(body, "glm-4.7")
	require.True(t, gjson.ValidBytes(got))
	require.Contains(t, string(got), `"type":"tool_use"`)
	require.Contains(t, string(got), "search summary")
	require.NotContains(t, string(got), "server_tool_use")
	require.NotContains(t, string(got), "web_search_tool_result")
}

func TestFilterWebSearchHistoryBlocks_LeavesOrdinaryHistoryByteIdentical(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"user","content":"explain server_tool_use blocks"}]}`)
	require.Equal(t, body, FilterWebSearchHistoryBlocks(body, "claude-sonnet-4-6"))
}
