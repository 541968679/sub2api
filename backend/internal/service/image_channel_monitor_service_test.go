package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type imageMonitorAccountReaderStub struct {
	account *Account
	err     error
}

func (s *imageMonitorAccountReaderStub) GetByID(context.Context, int64) (*Account, error) {
	return s.account, s.err
}

type imageMonitorProxyReaderStub struct {
	proxy *Proxy
	err   error
}

func (s *imageMonitorProxyReaderStub) GetByID(context.Context, int64) (*Proxy, error) {
	return s.proxy, s.err
}

type imageMonitorHTTPUpstreamRecorder struct {
	statusCode          int
	body                string
	req                 *http.Request
	requestBody         []byte
	proxyURL            string
	accountID           int64
	concurrency         int
	block               <-chan struct{}
	downloadBody        []byte
	downloadContentType string
	exitIPBody          string
}

func (r *imageMonitorHTTPUpstreamRecorder) Do(
	req *http.Request,
	proxyURL string,
	accountID int64,
	accountConcurrency int,
) (*http.Response, error) {
	return r.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (r *imageMonitorHTTPUpstreamRecorder) DoWithTLS(
	req *http.Request,
	proxyURL string,
	accountID int64,
	accountConcurrency int,
	_ *tlsfingerprint.Profile,
) (*http.Response, error) {
	if r.block != nil {
		<-r.block
	}
	r.req = req
	r.proxyURL = proxyURL
	r.accountID = accountID
	r.concurrency = accountConcurrency
	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		r.requestBody = body
	}
	body := []byte(r.body)
	header := make(http.Header)
	if req.URL != nil && req.URL.Hostname() == "api.ipify.org" && r.exitIPBody != "" {
		body = []byte(r.exitIPBody)
	} else if req.Method == http.MethodGet && r.downloadBody != nil {
		body = r.downloadBody
		if r.downloadContentType != "" {
			header.Set("Content-Type", r.downloadContentType)
		}
	}
	status := r.statusCode
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}, nil
}

type imageMonitorRepoStub struct {
	monitor             *ImageChannelMonitor
	getErr              error
	deleteHistoryBefore time.Time
	monitors            map[int64]*ImageChannelMonitor
	timelineBuckets     []*ImageMonitorTimelineBucket
	timelineErr         error
	windowStats         *ImageMonitorWindowStats
	windowStatsErr      error
	recentHistory       map[int64][]*ImageMonitorTimelinePoint
	publicMonitors      []*ImageChannelMonitor
	availability        map[int64]*ImageMonitorAvailability
	lastBucketSeconds   int
	lastSince           time.Time
}

func (r *imageMonitorRepoStub) Create(context.Context, *ImageChannelMonitor) error { return nil }

func (r *imageMonitorRepoStub) GetByID(_ context.Context, id int64) (*ImageChannelMonitor, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.monitors != nil {
		m, ok := r.monitors[id]
		if !ok {
			return nil, ErrImageChannelMonitorNotFound
		}
		return m, nil
	}
	return r.monitor, nil
}

func (r *imageMonitorRepoStub) Update(context.Context, *ImageChannelMonitor) error { return nil }
func (r *imageMonitorRepoStub) Delete(context.Context, int64) error                { return nil }

func (r *imageMonitorRepoStub) List(
	context.Context,
	ImageChannelMonitorListParams,
) ([]*ImageChannelMonitor, int64, error) {
	return nil, 0, nil
}

func (r *imageMonitorRepoStub) ListEnabled(context.Context) ([]*ImageChannelMonitor, error) {
	return nil, nil
}

func (r *imageMonitorRepoStub) MarkChecked(context.Context, int64, time.Time) error { return nil }

func (r *imageMonitorRepoStub) InsertHistory(context.Context, *ImageChannelMonitorHistoryRow) error {
	return nil
}

func (r *imageMonitorRepoStub) ListHistory(
	context.Context,
	int64,
	int,
) ([]*ImageChannelMonitorHistoryEntry, error) {
	return nil, nil
}

func (r *imageMonitorRepoStub) DeleteHistoryBefore(_ context.Context, before time.Time) (int64, error) {
	r.deleteHistoryBefore = before
	return 0, nil
}

func (r *imageMonitorRepoStub) AggregateTimeline(_ context.Context, _ int64, bucketSeconds int, since time.Time) ([]*ImageMonitorTimelineBucket, error) {
	r.lastBucketSeconds = bucketSeconds
	r.lastSince = since
	return r.timelineBuckets, r.timelineErr
}

func (r *imageMonitorRepoStub) ComputeWindowStats(_ context.Context, _ int64, since time.Time) (*ImageMonitorWindowStats, error) {
	r.lastSince = since
	if r.windowStats == nil {
		return &ImageMonitorWindowStats{}, r.windowStatsErr
	}
	return r.windowStats, r.windowStatsErr
}

func (r *imageMonitorRepoStub) ListRecentHistoryForMonitors(context.Context, []int64, int) (map[int64][]*ImageMonitorTimelinePoint, error) {
	return r.recentHistory, nil
}

func (r *imageMonitorRepoStub) ListPublicVisible(context.Context) ([]*ImageChannelMonitor, error) {
	return r.publicMonitors, nil
}

func (r *imageMonitorRepoStub) ComputeAvailabilityForMonitors(context.Context, []int64) (map[int64]*ImageMonitorAvailability, error) {
	return r.availability, nil
}

type imageMonitorPlainEncryptor struct{}

func (e imageMonitorPlainEncryptor) Encrypt(plaintext string) (string, error) { return plaintext, nil }
func (e imageMonitorPlainEncryptor) Decrypt(ciphertext string) (string, error) {
	return ciphertext, nil
}

func TestImageChannelMonitorPublicFieldsCreateAndPartialUpdate(t *testing.T) {
	accountID := int64(7)
	repo := &imageMonitorRepoStub{}
	accounts := &imageMonitorAccountReaderStub{account: &Account{
		ID:       accountID,
		Name:     "openai-image",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "acct-key",
		},
	}}
	svc := NewImageChannelMonitorService(repo, accounts, nil, imageMonitorPlainEncryptor{}, nil, nil)

	created, err := svc.Create(context.Background(), ImageChannelMonitorCreateParams{
		Name:          "img-a",
		SourceType:    ImageChannelMonitorSourceAccount,
		AccountID:     &accountID,
		PublicVisible: true,
		PublicName:    "  生图通道A  ",
	})
	require.NoError(t, err)
	require.True(t, created.PublicVisible)
	require.Equal(t, "生图通道A", created.PublicName)

	repo.monitor = created
	newName := "通道A"
	updated, err := svc.Update(context.Background(), created.ID, ImageChannelMonitorUpdateParams{
		PublicName: &newName,
	})
	require.NoError(t, err)
	require.True(t, updated.PublicVisible, "partial update must not reset public_visible")
	require.Equal(t, "通道A", updated.PublicName)
}

func TestImageChannelMonitorRunDailyMaintenancePrunesOldHistory(t *testing.T) {
	repo := &imageMonitorRepoStub{}
	svc := NewImageChannelMonitorService(repo, nil, nil, nil, nil, nil)

	require.NoError(t, svc.RunDailyMaintenance(context.Background()))
	require.False(t, repo.deleteHistoryBefore.IsZero(), "DeleteHistoryBefore must be called")
	wantCutoff := time.Now().UTC().AddDate(0, 0, -30)
	require.WithinDuration(t, wantCutoff, repo.deleteHistoryBefore, time.Minute)
}

func TestImageChannelMonitorGetAdminTimelineWindows(t *testing.T) {
	repo := &imageMonitorRepoStub{
		monitors:    map[int64]*ImageChannelMonitor{7: {ID: 7, Name: "img"}},
		windowStats: &ImageMonitorWindowStats{Total: 10, OK: 9, Availability: 90},
	}
	svc := NewImageChannelMonitorService(repo, nil, nil, nil, nil, nil)

	tl, err := svc.GetAdminTimeline(context.Background(), 7, "24h")
	require.NoError(t, err)
	require.Equal(t, "24h", tl.Window)
	require.Equal(t, 600, repo.lastBucketSeconds)
	require.WithinDuration(t, time.Now().UTC().Add(-24*time.Hour), repo.lastSince, time.Minute)
	require.Equal(t, 90.0, tl.Summary.Availability)

	_, err = svc.GetAdminTimeline(context.Background(), 7, "7d")
	require.NoError(t, err)
	require.Equal(t, 7200, repo.lastBucketSeconds)

	_, err = svc.GetAdminTimeline(context.Background(), 7, "30d")
	require.NoError(t, err)
	require.Equal(t, 86400, repo.lastBucketSeconds)

	_, err = svc.GetAdminTimeline(context.Background(), 7, "90d")
	require.ErrorIs(t, err, ErrImageChannelMonitorInvalidWindow)
}

func TestImageChannelMonitorListPublicViewMasksNameAndExtractsLatest(t *testing.T) {
	api1 := 18000
	dl1 := 2200
	repo := &imageMonitorRepoStub{
		publicMonitors: []*ImageChannelMonitor{
			{ID: 1, Name: "内部-adobe中转", PublicName: "生图通道A", PublicVisible: true, Model: "gpt-image-1"},
			{ID: 2, Name: "直连", PublicName: "", PublicVisible: true, Model: "gpt-image-1"},
		},
		recentHistory: map[int64][]*ImageMonitorTimelinePoint{
			1: {{Status: "operational", APITotalMs: &api1, ImageDownloadMs: &dl1, CheckedAt: time.Now()}},
		},
		availability: map[int64]*ImageMonitorAvailability{1: {D7: 99.5, D15: 98, D30: 97}},
	}
	svc := NewImageChannelMonitorService(repo, nil, nil, nil, nil, nil)

	views, err := svc.ListPublicView(context.Background())
	require.NoError(t, err)
	require.Len(t, views, 2)
	require.Equal(t, "生图通道A", views[0].Name)
	require.Equal(t, "operational", views[0].LatestStatus)
	require.Equal(t, 18000, *views[0].LatestAPIMs)
	require.Equal(t, 2200, *views[0].LatestDownloadMs)
	require.InDelta(t, 99.5, views[0].Availability.D7, 0.001)
	require.Equal(t, "直连", views[1].Name, "empty public_name falls back to monitor name")
	require.Equal(t, "empty", views[1].LatestStatus, "no history -> empty status")
	require.Zero(t, views[1].Availability.D7)
}

func TestImageChannelMonitorGetPublicDetailHidesPrivateMonitor(t *testing.T) {
	repo := &imageMonitorRepoStub{
		monitors: map[int64]*ImageChannelMonitor{3: {ID: 3, Name: "private", PublicVisible: false}},
	}
	svc := NewImageChannelMonitorService(repo, nil, nil, nil, nil, nil)
	_, err := svc.GetPublicDetail(context.Background(), 3)
	require.ErrorIs(t, err, ErrImageChannelMonitorNotFound)
}

func TestImageChannelMonitorGetPublicDetailReturnsThreeWindows(t *testing.T) {
	avgAPI := 15000
	repo := &imageMonitorRepoStub{
		monitors: map[int64]*ImageChannelMonitor{
			5: {ID: 5, Name: "内部名", PublicName: "对外名", PublicVisible: true, Model: "gpt-image-1"},
		},
		windowStats: &ImageMonitorWindowStats{Total: 4, OK: 4, Availability: 100, AvgAPITotalMs: &avgAPI},
	}
	svc := NewImageChannelMonitorService(repo, nil, nil, nil, nil, nil)

	detail, err := svc.GetPublicDetail(context.Background(), 5)
	require.NoError(t, err)
	require.Equal(t, "对外名", detail.Name)
	require.Len(t, detail.Windows, 3)
	require.Equal(t, []int{7, 15, 30}, []int{detail.Windows[0].WindowDays, detail.Windows[1].WindowDays, detail.Windows[2].WindowDays})
	require.Equal(t, 100.0, detail.Windows[0].Availability)
	require.Equal(t, 15000, *detail.Windows[0].AvgAPITotalMs)
}

func TestImageChannelMonitorRunCheckUsesOpenAIAPIKeyAccountSource(t *testing.T) {
	accountID := int64(7)
	account := &Account{
		ID:          accountID,
		Name:        "openai-image",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 3,
		Credentials: map[string]any{
			"api_key":    "acct-key",
			"base_url":   "https://upstream.example/custom/v1",
			"user_agent": "custom-ua",
		},
	}
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body: `{"data":[{"url":"https://cdn.example/generated.png","revised_prompt":"ok"}]}`,
	}
	svc := NewImageChannelMonitorService(
		nil,
		&imageMonitorAccountReaderStub{account: account},
		nil,
		nil,
		upstream,
		nil,
	)

	result := svc.runCheck(context.Background(), &ImageChannelMonitor{
		ID:             12,
		SourceType:     ImageChannelMonitorSourceAccount,
		AccountID:      &accountID,
		Model:          "gpt-image-1",
		Prompt:         "draw a square",
		Size:           "1024x1024",
		Quality:        "auto",
		N:              1,
		DownloadImage:  false,
		TimeoutSeconds: 300,
	})

	require.Equal(t, MonitorStatusOperational, result.Status)
	require.Equal(t, "https://upstream.example/custom/v1/images/generations", upstream.req.URL.String())
	require.Equal(t, "Bearer acct-key", upstream.req.Header.Get("Authorization"))
	require.Equal(t, "custom-ua", upstream.req.Header.Get("User-Agent"))
	require.Equal(t, accountID, upstream.accountID)
	require.Equal(t, 3, upstream.concurrency)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(upstream.requestBody, &payload))
	require.Equal(t, "gpt-image-1", payload["model"])
	require.Equal(t, "draw a square", payload["prompt"])
	require.Equal(t, "url", payload["response_format"])
}

func TestImageChannelMonitorRunCheckMarksB64JSONAsFailedForURLMonitor(t *testing.T) {
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body: `{"data":[{"b64_json":"aGVhbHRoLWNoZWNr","revised_prompt":"ok"}]}`,
	}
	svc := NewImageChannelMonitorService(nil, nil, nil, nil, upstream, nil)

	result := svc.runCheck(context.Background(), &ImageChannelMonitor{
		ID:             13,
		SourceType:     ImageChannelMonitorSourceCustom,
		Endpoint:       "https://api.example.com",
		APIKey:         "custom-key",
		Model:          "gpt-image-1",
		Prompt:         "draw",
		Size:           "1024x1024",
		Quality:        "auto",
		N:              1,
		DownloadImage:  false,
		TimeoutSeconds: 300,
	})

	require.Equal(t, MonitorStatusFailed, result.Status)
	require.False(t, result.HasURL)
	require.True(t, result.HasB64JSON)
	require.Equal(t, "image_url", result.ErrorStage)
	require.Contains(t, result.Message, "b64_json")
	require.Equal(t, "Bearer custom-key", upstream.req.Header.Get("Authorization"))
}

func TestImageChannelMonitorRunCheckUsesCustomProxy(t *testing.T) {
	proxyID := int64(9)
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body: `{"data":[{"url":"https://cdn.example/generated.png","revised_prompt":"ok"}]}`,
	}
	svc := NewImageChannelMonitorService(
		nil,
		nil,
		&imageMonitorProxyReaderStub{proxy: &Proxy{
			ID:       proxyID,
			Name:     "image-proxy",
			Protocol: "http",
			Host:     "proxy.example",
			Port:     8080,
		}},
		nil,
		upstream,
		nil,
	)

	result := svc.runCheck(context.Background(), &ImageChannelMonitor{
		ID:             14,
		SourceType:     ImageChannelMonitorSourceCustom,
		Endpoint:       "https://api.example.com",
		APIKey:         "custom-key",
		ProxyID:        &proxyID,
		Model:          "gpt-image-1",
		Prompt:         "draw",
		Size:           "1024x1024",
		Quality:        "auto",
		N:              1,
		DownloadImage:  false,
		TimeoutSeconds: 300,
	})

	require.Equal(t, MonitorStatusOperational, result.Status)
	require.Equal(t, "http://proxy.example:8080", upstream.proxyURL)
}

func TestBuildImageMonitorPayloadOmitSizeWhenBlank(t *testing.T) {
	payload := buildImageMonitorPayload(&ImageChannelMonitor{
		Model:   "gpt-image-1",
		Prompt:  "draw",
		Size:    " ",
		Quality: "auto",
		N:       1,
	})

	require.NotContains(t, payload, "size")
	require.Equal(t, "auto", payload["quality"])
}

func TestBuildImageMonitorPayloadPassesCustomSize(t *testing.T) {
	payload := buildImageMonitorPayload(&ImageChannelMonitor{
		Model:   "gpt-image-2",
		Prompt:  "draw",
		Size:    "3840x2160",
		Quality: "high",
		N:       1,
	})

	require.Equal(t, "3840x2160", payload["size"])
}

func TestImageChannelMonitorManualEditUsesEditsEndpointAndMultipart(t *testing.T) {
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body: `{"data":[{"url":"https://cdn.example/edited.png","revised_prompt":"edited"}]}`,
	}
	svc := NewImageChannelMonitorService(nil, nil, nil, nil, upstream, nil)

	result := svc.runManualCheck(context.Background(), &ImageChannelMonitor{
		ID:             15,
		SourceType:     ImageChannelMonitorSourceCustom,
		Endpoint:       "https://api.example.com",
		APIKey:         "custom-key",
		Model:          "gpt-image-1",
		Prompt:         "edit",
		Size:           "1024x1024",
		Quality:        "auto",
		N:              1,
		DownloadImage:  false,
		TimeoutSeconds: 300,
	}, ImageChannelMonitorManualEdit, ImageChannelMonitorManualTestParams{
		InputImageData: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=",
		InputImageName: "source.png",
	})

	require.Equal(t, MonitorStatusOperational, result.Status)
	require.Equal(t, "https://api.example.com/v1/images/edits", upstream.req.URL.String())
	mediaType, params, err := mime.ParseMediaType(upstream.req.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	form, err := multipart.NewReader(bytes.NewReader(upstream.requestBody), params["boundary"]).ReadForm(1 << 20)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-image-1"}, form.Value["model"])
	require.Equal(t, []string{"edit"}, form.Value["prompt"])
	require.Equal(t, []string{"url"}, form.Value["response_format"])
	require.Len(t, form.File["image"], 1)
}

func TestImageChannelMonitorManualGenerateAcceptsB64JSONPreview(t *testing.T) {
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body: `{"data":[{"b64_json":"aGVhbHRoLWNoZWNr","revised_prompt":"ok"}]}`,
	}
	svc := NewImageChannelMonitorService(nil, nil, nil, nil, upstream, nil)

	result := svc.runManualCheck(context.Background(), &ImageChannelMonitor{
		ID:             16,
		SourceType:     ImageChannelMonitorSourceCustom,
		Endpoint:       "https://api.example.com",
		APIKey:         "custom-key",
		Model:          "gpt-image-1",
		Prompt:         "draw",
		Quality:        "auto",
		N:              1,
		DownloadImage:  false,
		TimeoutSeconds: 300,
	}, ImageChannelMonitorManualGenerate, ImageChannelMonitorManualTestParams{})

	require.Equal(t, MonitorStatusOperational, result.Status)
	require.True(t, result.HasB64JSON)
	require.Equal(t, "data:image/png;base64,aGVhbHRoLWNoZWNr", result.ReturnedImageData)
}

func TestImageChannelMonitorManualGenerateB64JSONRecordsImageInfo(t *testing.T) {
	const onePixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII="
	decoded, err := base64.StdEncoding.DecodeString(onePixelPNG)
	require.NoError(t, err)
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body: `{"data":[{"b64_json":"` + onePixelPNG + `","revised_prompt":"ok"}]}`,
	}
	svc := NewImageChannelMonitorService(nil, nil, nil, nil, upstream, nil)

	result := svc.runManualCheck(context.Background(), &ImageChannelMonitor{
		ID:             18,
		SourceType:     ImageChannelMonitorSourceCustom,
		Endpoint:       "https://api.example.com",
		APIKey:         "custom-key",
		Model:          "gpt-image-1",
		Prompt:         "draw",
		Quality:        "auto",
		N:              1,
		DownloadImage:  false,
		TimeoutSeconds: 300,
	}, ImageChannelMonitorManualGenerate, ImageChannelMonitorManualTestParams{})

	require.Equal(t, MonitorStatusOperational, result.Status)
	require.NotNil(t, result.ImageBytes)
	require.Equal(t, int64(len(decoded)), *result.ImageBytes)
	require.Equal(t, "image/png", result.ImageContentType)
	require.NotNil(t, result.ImageWidth)
	require.NotNil(t, result.ImageHeight)
	require.Equal(t, 1, *result.ImageWidth)
	require.Equal(t, 1, *result.ImageHeight)
}

func TestImageChannelMonitorManualGenerateCapturesDownloadedURLPreview(t *testing.T) {
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body:                `{"data":[{"url":"https://cdn.example/generated.png","revised_prompt":"ok"}]}`,
		downloadBody:        []byte("png-bytes"),
		downloadContentType: "image/png",
	}
	svc := NewImageChannelMonitorService(nil, nil, nil, nil, upstream, nil)

	result := svc.runManualCheck(context.Background(), &ImageChannelMonitor{
		ID:             17,
		SourceType:     ImageChannelMonitorSourceCustom,
		Endpoint:       "https://api.example.com",
		APIKey:         "custom-key",
		Model:          "gpt-image-1",
		Prompt:         "draw",
		Quality:        "auto",
		N:              1,
		DownloadImage:  true,
		TimeoutSeconds: 300,
	}, ImageChannelMonitorManualGenerate, ImageChannelMonitorManualTestParams{})

	require.Equal(t, MonitorStatusOperational, result.Status)
	require.Equal(t, "https://cdn.example/generated.png", result.ReturnedImageURL)
	require.Equal(t, "data:image/png;base64,cG5nLWJ5dGVz", result.ReturnedImageData)
}

func TestImageChannelMonitorManualGenerateRecordsNetworkInfo(t *testing.T) {
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body:                `{"data":[{"url":"https://127.0.0.1:9443/generated.png","revised_prompt":"ok"}]}`,
		downloadBody:        []byte("png-bytes"),
		downloadContentType: "image/png",
		exitIPBody:          "203.0.113.5",
	}
	svc := NewImageChannelMonitorService(nil, nil, nil, nil, upstream, nil)

	result := svc.runManualCheck(context.Background(), &ImageChannelMonitor{
		ID:             18,
		SourceType:     ImageChannelMonitorSourceCustom,
		Endpoint:       "https://127.0.0.1:8443",
		APIKey:         "custom-key",
		Model:          "gpt-image-1",
		Prompt:         "draw",
		Quality:        "auto",
		N:              1,
		DownloadImage:  true,
		TimeoutSeconds: 300,
	}, ImageChannelMonitorManualGenerate, ImageChannelMonitorManualTestParams{})

	require.Equal(t, MonitorStatusOperational, result.Status)
	require.Equal(t, "203.0.113.5", result.ExitIP)
	require.Equal(t, "https://127.0.0.1:8443/v1/images/generations", result.RequestTargetURL)
	require.Equal(t, "127.0.0.1", result.RequestTargetHost)
	require.Equal(t, []string{"127.0.0.1"}, result.RequestTargetIPs)
	require.Equal(t, "https://127.0.0.1:9443/generated.png", result.ImageDownloadURL)
	require.Equal(t, "127.0.0.1", result.ImageDownloadHost)
	require.Equal(t, []string{"127.0.0.1"}, result.ImageDownloadIPs)
}

func TestImageChannelMonitorStartManualCheckRunsAsyncAndPollsResult(t *testing.T) {
	release := make(chan struct{})
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body:  `{"data":[{"b64_json":"aGVhbHRoLWNoZWNr","revised_prompt":"ok"}]}`,
		block: release,
	}
	svc := NewImageChannelMonitorService(
		&imageMonitorRepoStub{monitor: &ImageChannelMonitor{
			ID:              21,
			SourceType:      ImageChannelMonitorSourceCustom,
			Endpoint:        "https://api.example.com",
			APIKey:          "custom-key",
			Model:           "gpt-image-1",
			Prompt:          "draw",
			Quality:         "auto",
			N:               1,
			DownloadImage:   false,
			IntervalSeconds: 300,
			TimeoutSeconds:  300,
		}},
		nil,
		nil,
		imageMonitorPlainEncryptor{},
		upstream,
		nil,
	)

	status, err := svc.StartManualCheck(context.Background(), 21, ImageChannelMonitorManualTestParams{
		Mode:          ImageChannelMonitorManualGenerate,
		DownloadImage: false,
	})
	require.NoError(t, err)
	require.NotEmpty(t, status.RunID)
	require.True(t, status.Running)
	require.Nil(t, status.Result)

	close(release)

	require.Eventually(t, func() bool {
		current, err := svc.GetManualCheckStatus(context.Background(), status.RunID)
		if err != nil || current.Running || current.Result == nil {
			return false
		}
		status = current
		return true
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, MonitorStatusOperational, status.Result.Status)
	require.Equal(t, "data:image/png;base64,aGVhbHRoLWNoZWNr", status.Result.ReturnedImageData)
	require.NotNil(t, status.CompletedAt)
}

func TestImageChannelMonitorCancelManualCheckKeepsCanceledStatus(t *testing.T) {
	release := make(chan struct{})
	upstream := &imageMonitorHTTPUpstreamRecorder{
		body:  `{"data":[{"b64_json":"aGVhbHRoLWNoZWNr","revised_prompt":"ok"}]}`,
		block: release,
	}
	svc := NewImageChannelMonitorService(
		&imageMonitorRepoStub{monitor: &ImageChannelMonitor{
			ID:              22,
			SourceType:      ImageChannelMonitorSourceCustom,
			Endpoint:        "https://api.example.com",
			APIKey:          "custom-key",
			Model:           "gpt-image-1",
			Prompt:          "draw",
			Quality:         "auto",
			N:               1,
			DownloadImage:   false,
			IntervalSeconds: 300,
			TimeoutSeconds:  300,
		}},
		nil,
		nil,
		imageMonitorPlainEncryptor{},
		upstream,
		nil,
	)

	status, err := svc.StartManualCheck(context.Background(), 22, ImageChannelMonitorManualTestParams{
		Mode:          ImageChannelMonitorManualGenerate,
		DownloadImage: false,
	})
	require.NoError(t, err)
	require.True(t, status.Running)

	status, err = svc.CancelManualCheck(context.Background(), status.RunID)
	require.NoError(t, err)
	require.False(t, status.Running)
	require.True(t, status.Canceled)
	require.Equal(t, "canceled", status.Stage)
	require.Nil(t, status.Result)
	require.NotNil(t, status.CompletedAt)

	close(release)
	require.Eventually(t, func() bool {
		current, err := svc.GetManualCheckStatus(context.Background(), status.RunID)
		if err != nil {
			return false
		}
		status = current
		return status.Canceled && !status.Running && status.Result == nil
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, "canceled", status.Stage)
}
