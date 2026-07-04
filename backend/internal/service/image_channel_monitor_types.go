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
	imageMonitorMaxResponseBytes       = 2 * 1024 * 1024
	imageMonitorMaxDownloadBytes       = 32 * 1024 * 1024
	imageMonitorRunnerConcurrency      = 3
	imageMonitorRunOneBuffer           = 15 * time.Second
	imageMonitorManualRunRetention     = 30 * time.Minute
	imageMonitorManualRunMax           = 200
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
	Enabled             bool
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
	Enabled         bool
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
	Enabled         *bool
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
