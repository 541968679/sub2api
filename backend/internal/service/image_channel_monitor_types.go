package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	ImageChannelMonitorSourceCustom   = "custom"
	ImageChannelMonitorSourceAccount  = "account"
	ImageChannelMonitorManualGenerate = "generate"
	ImageChannelMonitorManualEdit     = "edit"

	imageMonitorDefaultModel           = "gpt-image-1"
	imageMonitorDefaultPrompt          = "Generate a simple health-check image with a clean geometric shape."
	imageMonitorDefaultQuality         = "auto"
	imageMonitorDefaultIntervalSeconds = 300
	imageMonitorDefaultTimeoutSeconds  = 300
	// b64_json 拿图时整张图内联在 JSON 里(16MB 图 base64 后约 21.4MB),上限需覆盖之。
	imageMonitorMaxResponseBytes     = 24 * 1024 * 1024
	imageMonitorMaxDownloadBytes     = 32 * 1024 * 1024
	imageMonitorMaxReturnedImageData = 16 * 1024 * 1024
	imageMonitorExitIPProbeURL       = "https://api.ipify.org?format=text"
	imageMonitorNetworkProbeTimeout  = 5 * time.Second
	imageMonitorRunnerConcurrency    = 3
	imageMonitorRunOneBuffer         = 15 * time.Second
	imageMonitorManualRunRetention   = 30 * time.Minute
	imageMonitorManualRunMax         = 200
	imageMonitorHistoryRetentionDays = 30
)

var (
	ErrImageChannelMonitorNotFound = infraerrors.NotFound(
		"IMAGE_CHANNEL_MONITOR_NOT_FOUND", "image channel monitor not found",
	)
	ErrImageChannelMonitorInvalidSource = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_INVALID_SOURCE", "source_type must be custom or account",
	)
	ErrImageChannelMonitorMissingAPIKey = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_MISSING_API_KEY", "api_key is required for custom image monitors",
	)
	ErrImageChannelMonitorMissingAccount = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_MISSING_ACCOUNT", "account_id is required for account image monitors",
	)
	ErrImageChannelMonitorUnsupportedAccount = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_UNSUPPORTED_ACCOUNT", "account source must be an OpenAI API key account",
	)
	ErrImageChannelMonitorMissingModel = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_MISSING_MODEL", "model is required",
	)
	ErrImageChannelMonitorMissingPrompt = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_MISSING_PROMPT", "prompt is required",
	)
	ErrImageChannelMonitorInvalidInterval = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_INVALID_INTERVAL", "interval_seconds must be in [15, 3600]",
	)
	ErrImageChannelMonitorInvalidTimeout = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_INVALID_TIMEOUT", "timeout_seconds must be in [30, 600]",
	)
	ErrImageChannelMonitorInvalidN = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_INVALID_N", "n must be in [1, 10]",
	)
	ErrImageChannelMonitorAPIKeyDecryptFailed = infraerrors.InternalServer(
		"IMAGE_CHANNEL_MONITOR_KEY_DECRYPT_FAILED", "api key decryption failed; please re-edit the monitor with a fresh key",
	)
	ErrImageChannelMonitorAlreadyRunning = infraerrors.Conflict(
		"IMAGE_CHANNEL_MONITOR_ALREADY_RUNNING", "image channel monitor check is already running",
	)
	ErrImageChannelMonitorInvalidManualMode = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_INVALID_MANUAL_MODE", "manual test mode must be generate or edit",
	)
	ErrImageChannelMonitorMissingInputImage = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_MISSING_INPUT_IMAGE", "input image is required for image edit tests",
	)
	ErrImageChannelMonitorInvalidInputImage = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_INVALID_INPUT_IMAGE", "input image must be a valid base64 image",
	)
	ErrImageChannelMonitorManualRunNotFound = infraerrors.NotFound(
		"IMAGE_CHANNEL_MONITOR_MANUAL_RUN_NOT_FOUND", "manual image test run not found",
	)
	ErrImageChannelMonitorInvalidWindow = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_INVALID_WINDOW", "window must be 24h, 7d or 30d",
	)
	ErrImageChannelMonitorInvalidResponseFormat = infraerrors.BadRequest(
		"IMAGE_CHANNEL_MONITOR_INVALID_RESPONSE_FORMAT", "response_format must be url, b64_json or empty",
	)
)

// 拿图方式:'url' / 'b64_json' / ”(不传参数,接受任意返回形式)。
const (
	ImageMonitorResponseFormatURL  = "url"
	ImageMonitorResponseFormatB64  = "b64_json"
	ImageMonitorResponseFormatOmit = ""
)

type ImageChannelMonitorRepository interface {
	Create(ctx context.Context, m *ImageChannelMonitor) error
	GetByID(ctx context.Context, id int64) (*ImageChannelMonitor, error)
	Update(ctx context.Context, m *ImageChannelMonitor) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, params ImageChannelMonitorListParams) ([]*ImageChannelMonitor, int64, error)
	ListEnabled(ctx context.Context) ([]*ImageChannelMonitor, error)
	MarkChecked(ctx context.Context, id int64, checkedAt time.Time) error
	InsertHistory(ctx context.Context, row *ImageChannelMonitorHistoryRow) error
	ListHistory(ctx context.Context, monitorID int64, limit int) ([]*ImageChannelMonitorHistoryEntry, error)
	DeleteHistoryBefore(ctx context.Context, before time.Time) (int64, error)
	AggregateTimeline(ctx context.Context, monitorID int64, bucketSeconds int, since time.Time) ([]*ImageMonitorTimelineBucket, error)
	ComputeWindowStats(ctx context.Context, monitorID int64, since time.Time) (*ImageMonitorWindowStats, error)
	ListRecentHistoryForMonitors(ctx context.Context, ids []int64, perMonitorLimit int) (map[int64][]*ImageMonitorTimelinePoint, error)
	ListPublicVisible(ctx context.Context) ([]*ImageChannelMonitor, error)
	ComputeAvailabilityForMonitors(ctx context.Context, ids []int64) (map[int64]*ImageMonitorAvailability, error)
}

// ImageMonitorTimelineBucket 时间桶聚合(管理端折线图数据点)。
type ImageMonitorTimelineBucket struct {
	BucketStart        time.Time
	Total              int
	Operational        int
	Degraded           int
	Failed             int
	Error              int
	AvgAPITotalMs      *int
	MaxAPITotalMs      *int
	AvgImageDownloadMs *int
}

// ImageMonitorWindowStats 单窗口汇总统计。
type ImageMonitorWindowStats struct {
	Total              int
	OK                 int
	Availability       float64
	AvgAPITotalMs      *int
	MaxAPITotalMs      *int
	AvgImageDownloadMs *int
}

// ImageMonitorTimelinePoint 单次检查的时间线点(状态条数据)。
type ImageMonitorTimelinePoint struct {
	Status          string
	APITotalMs      *int
	ImageDownloadMs *int
	CheckedAt       time.Time
}

// ImageMonitorAvailability 固定三窗口可用率(百分比 0-100)。
type ImageMonitorAvailability struct {
	D7  float64
	D15 float64
	D30 float64
}

// ImageMonitorPublicView 用户侧公开渠道卡片视图。Name 已按 public_name 掩名。
type ImageMonitorPublicView struct {
	ID               int64
	Name             string
	Model            string
	LatestStatus     string // 无历史时为 "empty"
	LatestAPIMs      *int
	LatestDownloadMs *int
	Availability     ImageMonitorAvailability
	Timeline         []*ImageMonitorTimelinePoint
}

// ImageMonitorPublicWindowStat 用户侧详情弹窗的单窗口统计。
type ImageMonitorPublicWindowStat struct {
	WindowDays    int
	Availability  float64
	AvgAPITotalMs *int
}

// ImageMonitorPublicDetail 用户侧公开渠道详情。
type ImageMonitorPublicDetail struct {
	ID      int64
	Name    string
	Model   string
	Windows []ImageMonitorPublicWindowStat
}

// ImageMonitorAdminTimeline 管理端时间线响应(汇总 + 分桶)。
type ImageMonitorAdminTimeline struct {
	Window  string
	Summary *ImageMonitorWindowStats
	Buckets []*ImageMonitorTimelineBucket
}

type imageChannelMonitorAccountReader interface {
	GetByID(ctx context.Context, id int64) (*Account, error)
}

type imageChannelMonitorProxyReader interface {
	GetByID(ctx context.Context, id int64) (*Proxy, error)
}

type ImageMonitorScheduler interface {
	Schedule(m *ImageChannelMonitor)
	Unschedule(id int64)
}

type ImageChannelMonitor struct {
	ID                  int64
	Name                string
	SourceType          string
	Endpoint            string
	APIKey              string
	AccountID           *int64
	AccountName         string
	ProxyID             *int64
	ProxyName           string
	Model               string
	Prompt              string
	Size                string
	Quality             string
	N                   int
	DownloadImage       bool
	ResponseFormat      string
	Enabled             bool
	PublicVisible       bool
	PublicName          string
	IntervalSeconds     int
	TimeoutSeconds      int
	LastCheckedAt       *time.Time
	CreatedBy           int64
	CreatedAt           time.Time
	UpdatedAt           time.Time
	APIKeyDecryptFailed bool
}

type ImageChannelMonitorListParams struct {
	Page       int
	PageSize   int
	SourceType string
	Enabled    *bool
	Search     string
}

type ImageChannelMonitorCreateParams struct {
	Name            string
	SourceType      string
	Endpoint        string
	APIKey          string
	AccountID       *int64
	ProxyID         *int64
	Model           string
	Prompt          string
	Size            string
	Quality         string
	N               int
	DownloadImage   bool
	ResponseFormat  *string
	Enabled         bool
	PublicVisible   bool
	PublicName      string
	IntervalSeconds int
	TimeoutSeconds  int
	CreatedBy       int64
}

type ImageChannelMonitorUpdateParams struct {
	Name            *string
	SourceType      *string
	Endpoint        *string
	APIKey          *string
	AccountID       *int64
	ProxyID         *int64
	Model           *string
	Prompt          *string
	Size            *string
	Quality         *string
	N               *int
	DownloadImage   *bool
	ResponseFormat  *string
	Enabled         *bool
	PublicVisible   *bool
	PublicName      *string
	IntervalSeconds *int
	TimeoutSeconds  *int
}

type ImageChannelMonitorResult struct {
	MonitorID         int64
	Status            string
	HTTPStatus        *int
	APIHeaderMs       *int
	APIBodyMs         *int
	APITotalMs        *int
	JSONBytes         *int
	HasURL            bool
	HasB64JSON        bool
	ResponseFormat    string
	ImageURLHost      string
	ImageFirstByteMs  *int
	ImageDownloadMs   *int
	ImageBytes        *int64
	ImageContentType  string
	ImageWidth        *int
	ImageHeight       *int
	ErrorStage        string
	Message           string
	CheckedAt         time.Time
	RevisedPrompt     string
	ReturnedImageURL  string
	ReturnedImageData string
	ExitIP            string
	RequestTargetURL  string
	RequestTargetHost string
	RequestTargetIPs  []string
	ImageDownloadURL  string
	ImageDownloadHost string
	ImageDownloadIPs  []string
	StageEvents       []ImageChannelMonitorStageEvent
}

type ImageChannelMonitorStageEvent struct {
	Stage   string
	Message string
	At      time.Time
}

type ImageChannelMonitorManualTestParams struct {
	Mode           string
	Model          string
	Prompt         string
	Size           string
	Quality        string
	N              int
	DownloadImage  bool
	ResponseFormat string
	TimeoutSeconds int
	InputImageData string
	InputImageType string
	InputImageName string
}

type ImageChannelMonitorManualTestResult struct {
	Monitor *ImageChannelMonitor
	Mode    string
	Result  *ImageChannelMonitorResult
}

type ImageChannelMonitorManualRunStatus struct {
	RunID       string
	Monitor     *ImageChannelMonitor
	Mode        string
	Running     bool
	Canceled    bool
	Stage       string
	Message     string
	StartedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
	Result      *ImageChannelMonitorResult
}

type ImageChannelMonitorRuntimeStatus struct {
	MonitorID             int64
	Running               bool
	Stage                 string
	Message               string
	StartedAt             *time.Time
	UpdatedAt             *time.Time
	CompletedAt           *time.Time
	NextCheckAt           *time.Time
	SecondsUntilNextCheck *int
}

type ImageChannelMonitorHistoryRow struct {
	MonitorID        int64
	Status           string
	HTTPStatus       *int
	APIHeaderMs      *int
	APIBodyMs        *int
	APITotalMs       *int
	JSONBytes        *int
	HasURL           bool
	HasB64JSON       bool
	ResponseFormat   string
	ImageURLHost     string
	ImageFirstByteMs *int
	ImageDownloadMs  *int
	ImageBytes       *int64
	ImageContentType string
	ImageWidth       *int
	ImageHeight      *int
	ErrorStage       string
	Message          string
	CheckedAt        time.Time
}

type ImageChannelMonitorHistoryEntry struct {
	ID               int64
	MonitorID        int64
	Status           string
	HTTPStatus       *int
	APIHeaderMs      *int
	APIBodyMs        *int
	APITotalMs       *int
	JSONBytes        *int
	HasURL           bool
	HasB64JSON       bool
	ResponseFormat   string
	ImageURLHost     string
	ImageFirstByteMs *int
	ImageDownloadMs  *int
	ImageBytes       *int64
	ImageContentType string
	ImageWidth       *int
	ImageHeight      *int
	ErrorStage       string
	Message          string
	CheckedAt        time.Time
}
