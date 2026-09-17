package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/stretchr/testify/require"
)

func TestNormalizeClientIPHeaderList_EmptyUsesDefaults(t *testing.T) {
	got := NormalizeClientIPHeaderList(nil)
	require.Equal(t, DefaultClientIPHeaderCandidates, got)
}

func TestNormalizeClientIPHeaderList_DedupesAndTrims(t *testing.T) {
	got := NormalizeClientIPHeaderList([]string{"  X-Real-IP ", "x-real-ip", "X-Forwarded-For", ""})
	require.Equal(t, []string{"X-Real-IP", "X-Forwarded-For"}, got)
}

func TestApplyClientIPHeaderSettings_PublishesToIPPackage(t *testing.T) {
	prev := ip.ClientIPHeaderOrder()
	t.Cleanup(func() { ip.SetClientIPHeaderOrder(prev) })

	ApplyClientIPHeaderSettings([]string{"True-Client-IP", "X-Real-IP"})
	require.Equal(t, []string{"True-Client-IP", "X-Real-IP"}, ip.ClientIPHeaderOrder())
}
