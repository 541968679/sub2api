package handler

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 用户侧图片监控 DTO 是安全白名单:新增字段必须显式加入本清单并通过净化评审。
// 绝不允许出现:内部渠道名、endpoint、host/IP、错误消息、error_stage、图片 URL、代理/账号信息。
func TestImageMonitorPublicListItemFieldWhitelist(t *testing.T) {
	api := 18000
	dl := 2100
	item := imageMonitorPublicViewToItem(&service.ImageMonitorPublicView{
		ID: 1, Name: "生图通道A", Model: "gpt-image-1",
		LatestStatus: "operational", LatestAPIMs: &api, LatestDownloadMs: &dl,
		Availability: service.ImageMonitorAvailability{D7: 99, D15: 98, D30: 97},
		Timeline: []*service.ImageMonitorTimelinePoint{
			{Status: "operational", APITotalMs: &api, ImageDownloadMs: &dl, CheckedAt: time.Now()},
		},
	})
	raw, err := json.Marshal(item)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	require.Equal(t, []string{
		"availability_15d", "availability_30d", "availability_7d",
		"id", "latest_api_ms", "latest_download_ms", "latest_status",
		"model", "name", "timeline",
	}, keys)

	tlRaw, err := json.Marshal(item.Timeline)
	require.NoError(t, err)
	var tl []map[string]any
	require.NoError(t, json.Unmarshal(tlRaw, &tl))
	require.Len(t, tl, 1)
	pointKeys := make([]string, 0, len(tl[0]))
	for k := range tl[0] {
		pointKeys = append(pointKeys, k)
	}
	sort.Strings(pointKeys)
	require.Equal(t, []string{"checked_at", "latency_ms", "status"}, pointKeys)
}
