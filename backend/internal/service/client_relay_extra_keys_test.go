//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeClientRelayExtraResponseKeys(t *testing.T) {
	got, err := NormalizeClientRelayExtraResponseKeys([]string{
		" cost_cny ",
		"cost_cny",
		"",
		"vendor_note",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"cost_cny", "vendor_note"}, got)

	_, err = NormalizeClientRelayExtraResponseKeys([]string{"reasoning_content"})
	require.Error(t, err)
	_, err = NormalizeClientRelayExtraResponseKeys([]string{"usage"})
	require.Error(t, err)
	_, err = NormalizeClientRelayExtraResponseKeys([]string{"prompt_tokens"})
	require.Error(t, err)
	_, err = NormalizeClientRelayExtraResponseKeys([]string{"choices.0.message"})
	require.Error(t, err)

	empty, err := NormalizeClientRelayExtraResponseKeys(nil)
	require.NoError(t, err)
	require.Empty(t, empty)

	tooMany := make([]string, 0, clientRelayExtraResponseKeyLimit+1)
	for i := 0; i < clientRelayExtraResponseKeyLimit+1; i++ {
		tooMany = append(tooMany, "field_"+itoa(i))
	}
	_, err = NormalizeClientRelayExtraResponseKeys(tooMany)
	require.Error(t, err)
}

func TestSettingService_ClientRelayExtraResponseKeysRoundTrip(t *testing.T) {
	InvalidateClientRelayExtraKeysCacheForTest()
	t.Cleanup(func() {
		clientRelayExtraKeysSource.Store(nil)
		InvalidateClientRelayExtraKeysCacheForTest()
	})

	repo := &gatewayHTTP1SettingRepo{data: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	got, err := svc.GetClientRelayExtraResponseKeys(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultClientRelayExtraResponseKeys(), got)
	require.Contains(t, got, "cost_cny")
	require.Contains(t, got, "reasoning_available")

	saved, err := svc.SetClientRelayExtraResponseKeys(context.Background(), []string{"cost_cny", " cost_cny ", "vendor_note"})
	require.NoError(t, err)
	require.Equal(t, []string{"cost_cny", "vendor_note"}, saved)

	body := []byte(`{"credit":1,"cost_cny":"0.01342000","vendor_note":"x","billing_pending":false,"choices":[{"message":{"content":"ok"}}]}`)
	stripped := sanitizeClientRelayPayload(body)
	require.False(t, gjson.GetBytes(stripped, "cost_cny").Exists())
	require.False(t, gjson.GetBytes(stripped, "vendor_note").Exists())
	require.True(t, gjson.GetBytes(stripped, "credit").Exists())
	require.True(t, gjson.GetBytes(stripped, "billing_pending").Exists())

	cleared, err := svc.SetClientRelayExtraResponseKeys(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, cleared)
	kept := sanitizeClientRelayPayload(body)
	require.Equal(t, "0.01342000", gjson.GetBytes(kept, "cost_cny").String())
}

func TestParseClientRelayExtraResponseKeysJSON_InvalidFallsBackOnRead(t *testing.T) {
	InvalidateClientRelayExtraKeysCacheForTest()
	t.Cleanup(func() {
		clientRelayExtraKeysSource.Store(nil)
		InvalidateClientRelayExtraKeysCacheForTest()
	})

	repo := &gatewayHTTP1SettingRepo{data: map[string]string{
		SettingKeyClientRelayExtraResponseKeys: "not-json",
	}}
	svc := NewSettingService(repo, &config.Config{})
	got, err := svc.GetClientRelayExtraResponseKeys(context.Background())
	require.NoError(t, err)
	require.Equal(t, DefaultClientRelayExtraResponseKeys(), got)
}
