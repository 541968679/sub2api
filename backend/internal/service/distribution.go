package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	DistributionAgentStatusPending  = "pending"
	DistributionAgentStatusApproved = "approved"
	DistributionAgentStatusRejected = "rejected"
	DistributionAgentStatusFrozen   = "frozen"

	DistributionWalletStatusActive = "active"
	DistributionWalletStatusFrozen = "frozen"
)

const (
	DistributionAssetTypeBalanceRedeemCode      = "balance_redeem_code"
	DistributionAssetTypeSubscriptionRedeemCode = "subscription_redeem_code"
	DistributionAssetTypeAPIKey                 = "api_key"

	DistributionAssetStatusActive   = "active"
	DistributionAssetStatusUsed     = "used"
	DistributionAssetStatusDisabled = "disabled"
	DistributionAssetStatusExpired  = "expired"
)

var (
	ErrDistributionAgentNotFound   = infraerrors.NotFound("DISTRIBUTION_AGENT_NOT_FOUND", "distribution agent not found")
	ErrDistributionAgentPending    = infraerrors.BadRequest("DISTRIBUTION_AGENT_PENDING", "distribution agent application is pending")
	ErrDistributionAgentRejected   = infraerrors.BadRequest("DISTRIBUTION_AGENT_REJECTED", "distribution agent application was rejected")
	ErrDistributionAgentFrozen     = infraerrors.Forbidden("DISTRIBUTION_AGENT_FROZEN", "distribution agent account is frozen")
	ErrDistributionAlreadyApplied  = infraerrors.Conflict("DISTRIBUTION_ALREADY_APPLIED", "distribution application already exists")
	ErrDistributionWalletNotFound  = infraerrors.NotFound("DISTRIBUTION_WALLET_NOT_FOUND", "distribution wallet not found")
	ErrDistributionWalletInactive  = infraerrors.Forbidden("DISTRIBUTION_WALLET_INACTIVE", "distribution wallet is not active")
	ErrDistributionInvalidAmount   = infraerrors.BadRequest("DISTRIBUTION_INVALID_AMOUNT", "invalid amount")
	ErrDistributionInsufficient    = infraerrors.BadRequest("DISTRIBUTION_INSUFFICIENT_BALANCE", "insufficient distribution balance")
	ErrDistributionGroupNotExposed = infraerrors.Forbidden("DISTRIBUTION_GROUP_NOT_EXPOSED", "api key group is not available for distribution")
)

const (
	DistributionLedgerActionAdminAdjust          = "admin_adjust"
	DistributionLedgerActionGenerateRedeemCode   = "generate_redeem_code"
	DistributionLedgerActionGenerateSubscription = "generate_subscription"
	DistributionLedgerActionGenerateAPIKey       = "generate_api_key"
	DistributionLedgerActionAssetRefund          = "asset_refund"
)

type DistributionAgentApplication struct {
	UserID                       int64      `json:"user_id"`
	UserEmail                    string     `json:"user_email,omitempty"`
	Username                     string     `json:"username,omitempty"`
	Status                       string     `json:"status"`
	Contact                      string     `json:"contact"`
	Reason                       string     `json:"reason"`
	AdminNote                    string     `json:"admin_note"`
	RMBPerUSDOverride            *float64   `json:"rmb_per_usd_override,omitempty"`
	SubscriptionDiscountOverride *float64   `json:"subscription_discount_override,omitempty"`
	ReviewedBy                   *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt                   *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt                    time.Time  `json:"created_at"`
	UpdatedAt                    time.Time  `json:"updated_at"`
}

type DistributionWallet struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	AgentID        int64     `json:"agent_id"`
	UserEmail      string    `json:"user_email,omitempty"`
	Username       string    `json:"username,omitempty"`
	Balance        float64   `json:"balance"`
	TotalRecharged float64   `json:"total_recharged"`
	TotalSpent     float64   `json:"total_spent"`
	TotalRebate    float64   `json:"total_rebate"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type DistributionWalletLedgerEntry struct {
	ID            int64     `json:"id"`
	WalletID      int64     `json:"wallet_id"`
	UserID        int64     `json:"user_id"`
	Action        string    `json:"action"`
	Amount        float64   `json:"amount"`
	BalanceAfter  float64   `json:"balance_after"`
	ReferenceType string    `json:"reference_type"`
	ReferenceID   string    `json:"reference_id"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
}

type DistributionAsset struct {
	ID             int64      `json:"id"`
	UserID         int64      `json:"user_id"`
	UserEmail      string     `json:"user_email,omitempty"`
	Username       string     `json:"username,omitempty"`
	WalletID       int64      `json:"wallet_id"`
	AssetType      string     `json:"asset_type"`
	ReferenceType  string     `json:"reference_type"`
	ReferenceID    string     `json:"reference_id"`
	DisplayValue   string     `json:"display_value"`
	PackageURL     string     `json:"package_url,omitempty"`
	FaceValue      float64    `json:"face_value"`
	CostRMB        float64    `json:"cost_rmb"`
	GroupID        *int64     `json:"group_id,omitempty"`
	GroupName      string     `json:"group_name,omitempty"`
	ValidityDays   int        `json:"validity_days,omitempty"`
	QuotaUSD       float64    `json:"quota_usd,omitempty"`
	Status         string     `json:"status"`
	CustomerUserID *int64     `json:"customer_user_id,omitempty"`
	CustomerEmail  string     `json:"customer_email,omitempty"`
	UsedAt         *time.Time `json:"used_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	RefundedAt     *time.Time `json:"refunded_at,omitempty"`
	RefundedRMB    float64    `json:"refunded_rmb"`
	RefundedBy     *int64     `json:"refunded_by,omitempty"`
	Note           string     `json:"note"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type DistributionSummary struct {
	Application *DistributionAgentApplication `json:"application"`
	Wallet      *DistributionWallet           `json:"wallet"`
	Settings    DistributionSettings          `json:"settings"`
}

type DistributionSettings struct {
	RMBPerUSD            float64 `json:"rmb_per_usd"`
	SubscriptionDiscount float64 `json:"subscription_discount"`
	APIKeyGroupIDs       []int64 `json:"api_key_group_ids"`
}

type DistributionAgentRateSettings struct {
	RMBPerUSDOverride            *float64 `json:"rmb_per_usd_override"`
	SubscriptionDiscountOverride *float64 `json:"subscription_discount_override"`
}

type DistributionGeneratedRedeemCode struct {
	Code         string  `json:"code"`
	Type         string  `json:"type"`
	Value        float64 `json:"value"`
	PlanID       *int64  `json:"plan_id,omitempty"`
	PlanName     string  `json:"plan_name,omitempty"`
	GroupID      *int64  `json:"group_id,omitempty"`
	ValidityDays int     `json:"validity_days,omitempty"`
	CostRMB      float64 `json:"cost_rmb"`
	BalanceAfter float64 `json:"balance_after"`
}

type DistributionGeneratedAPIKey struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Key          string     `json:"key"`
	Quota        float64    `json:"quota"`
	GroupID      *int64     `json:"group_id,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	BaseURL      string     `json:"base_url"`
	CostRMB      float64    `json:"cost_rmb"`
	BalanceAfter float64    `json:"balance_after"`
}

type DistributionCreateAssetInput struct {
	UserID         int64
	WalletID       int64
	AssetType      string
	ReferenceType  string
	ReferenceID    string
	DisplayValue   string
	PackageURL     string
	FaceValue      float64
	CostRMB        float64
	GroupID        *int64
	ValidityDays   int
	QuotaUSD       float64
	Status         string
	CustomerUserID *int64
	UsedAt         *time.Time
	ExpiresAt      *time.Time
	Note           string
}

type DistributionGenerateBalanceRedeemCodeInput struct {
	ValueUSD float64
	Note     string
}

type DistributionGenerateSubscriptionRedeemCodeInput struct {
	PlanID int64
	Note   string
}

type DistributionGenerateAPIKeyInput struct {
	Name          string
	QuotaUSD      float64
	GroupID       *int64
	ExpiresInDays *int
}

type DistributionAdminAdjustWalletInput struct {
	UserID  int64
	Amount  float64
	Note    string
	AdminID int64
}

type DistributionVoidAssetResult struct {
	Asset     *DistributionAsset  `json:"asset"`
	Wallet    *DistributionWallet `json:"wallet"`
	RefundRMB float64             `json:"refund_rmb"`
}

type DistributionRepository interface {
	EnsureAgent(ctx context.Context, userID int64) (*DistributionAgentApplication, error)
	CreateAgentApplication(ctx context.Context, userID int64, contact, reason string) (*DistributionAgentApplication, error)
	GetAgentApplication(ctx context.Context, userID int64) (*DistributionAgentApplication, error)
	ListAgentApplications(ctx context.Context, page, pageSize int, search string) ([]DistributionAgentApplication, int64, error)
	ReviewAgentApplication(ctx context.Context, userID int64, approved bool, adminNote string, reviewedBy int64) (*DistributionAgentApplication, error)
	EnsureWallet(ctx context.Context, userID int64) (*DistributionWallet, error)
	GetWalletByUserID(ctx context.Context, userID int64) (*DistributionWallet, error)
	ListWallets(ctx context.Context, page, pageSize int, search string) ([]DistributionWallet, int64, error)
	ListWalletLedger(ctx context.Context, userID int64, page, pageSize int) ([]DistributionWalletLedgerEntry, int64, error)
	ListAllWalletLedger(ctx context.Context, page, pageSize int, userID int64) ([]DistributionWalletLedgerEntry, int64, error)
	CreateAsset(ctx context.Context, input DistributionCreateAssetInput) (*DistributionAsset, error)
	GetAssetByID(ctx context.Context, id int64) (*DistributionAsset, error)
	ListAssets(ctx context.Context, page, pageSize int, userID int64, assetType, status, search string) ([]DistributionAsset, int64, error)
	UpdateAgentRates(ctx context.Context, userID int64, rates DistributionAgentRateSettings) (*DistributionAgentApplication, error)
	MarkAssetRefunded(ctx context.Context, assetID int64, status string, refundedBy int64) (*DistributionAsset, error)
	VoidAPIKeyAssetReference(ctx context.Context, id int64, userID int64) error
	UpdateWalletStatus(ctx context.Context, userID int64, status string) (*DistributionWallet, error)
	AdjustWalletBalance(ctx context.Context, userID int64, amount float64, action, referenceType, referenceID, note string, createdBy int64) (*DistributionWallet, error)
	WithTx(ctx context.Context, fn func(txCtx context.Context) error) error
}

type DistributionService struct {
	repo          DistributionRepository
	settingRepo   SettingRepository
	redeem        *RedeemService
	apiKey        *APIKeyService
	groupRepo     GroupRepository
	paymentConfig *PaymentConfigService
}

func NewDistributionService(repo DistributionRepository, settingRepo SettingRepository, redeem *RedeemService, apiKey *APIKeyService, groupRepo GroupRepository, paymentConfig *PaymentConfigService) *DistributionService {
	return &DistributionService{repo: repo, settingRepo: settingRepo, redeem: redeem, apiKey: apiKey, groupRepo: groupRepo, paymentConfig: paymentConfig}
}

func (s *DistributionService) GetCurrentUserSummary(ctx context.Context, userID int64) (*DistributionSummary, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	agent, err := s.repo.GetAgentApplication(ctx, userID)
	if err != nil && !errors.Is(err, ErrDistributionAgentNotFound) {
		return nil, err
	}
	var wallet *DistributionWallet
	if agent != nil && agent.Status == DistributionAgentStatusApproved {
		wallet, err = s.repo.EnsureWallet(ctx, userID)
		if err != nil {
			return nil, err
		}
	}
	settings, _ := s.GetEffectiveSettingsForUser(ctx, userID)
	return &DistributionSummary{Application: agent, Wallet: wallet, Settings: settings}, nil
}

func (s *DistributionService) ApplyForAgent(ctx context.Context, userID int64, contact, reason string) (*DistributionSummary, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	app, err := s.repo.CreateAgentApplication(ctx, userID, contact, reason)
	if err != nil {
		return nil, err
	}
	settings, _ := s.GetSettings(ctx)
	return &DistributionSummary{Application: app, Wallet: nil, Settings: settings}, nil
}

func (s *DistributionService) ReviewAgentApplication(ctx context.Context, userID int64, approved bool, adminNote string, reviewedBy int64) (*DistributionAgentApplication, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	app, err := s.repo.ReviewAgentApplication(ctx, userID, approved, adminNote, reviewedBy)
	if err != nil {
		return nil, err
	}
	if approved {
		if _, err := s.repo.EnsureWallet(ctx, userID); err != nil {
			return nil, err
		}
	}
	return app, nil
}

func (s *DistributionService) ListAgentApplications(ctx context.Context, page, pageSize int, search string) ([]DistributionAgentApplication, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	return s.repo.ListAgentApplications(ctx, page, pageSize, search)
}

func (s *DistributionService) ListWalletLedger(ctx context.Context, userID int64, page, pageSize int) ([]DistributionWalletLedgerEntry, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	if userID <= 0 {
		return nil, 0, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	return s.repo.ListWalletLedger(ctx, userID, page, pageSize)
}

func (s *DistributionService) ListAssets(ctx context.Context, userID int64, page, pageSize int, assetType, status, search string) ([]DistributionAsset, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	if userID <= 0 {
		return nil, 0, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	return s.repo.ListAssets(ctx, page, pageSize, userID, assetType, status, search)
}

func (s *DistributionService) ListAllAssets(ctx context.Context, page, pageSize int, userID int64, assetType, status, search string) ([]DistributionAsset, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	return s.repo.ListAssets(ctx, page, pageSize, userID, assetType, status, search)
}

func (s *DistributionService) GetSettings(ctx context.Context) (DistributionSettings, error) {
	defaults := DistributionSettings{RMBPerUSD: 0.5, SubscriptionDiscount: 0.75, APIKeyGroupIDs: []int64{}}
	if s == nil || s.settingRepo == nil {
		return defaults, nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyDistributionRMBPerUSD,
		SettingKeyDistributionSubscriptionDiscount,
		SettingKeyDistributionAPIKeyGroupIDs,
	})
	if err != nil {
		return defaults, err
	}
	out := defaults
	if v, err := strconv.ParseFloat(strings.TrimSpace(values[SettingKeyDistributionRMBPerUSD]), 64); err == nil && v > 0 {
		out.RMBPerUSD = v
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(values[SettingKeyDistributionSubscriptionDiscount]), 64); err == nil && v > 0 {
		out.SubscriptionDiscount = v
	}
	out.APIKeyGroupIDs = normalizeDistributionGroupIDs(parseDistributionGroupIDs(values[SettingKeyDistributionAPIKeyGroupIDs]))
	return out, nil
}

func (s *DistributionService) GetEffectiveSettingsForUser(ctx context.Context, userID int64) (DistributionSettings, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return settings, err
	}
	if s == nil || s.repo == nil || userID <= 0 {
		return settings, nil
	}
	agent, err := s.repo.GetAgentApplication(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrDistributionAgentNotFound) {
			return settings, nil
		}
		return settings, err
	}
	if agent.RMBPerUSDOverride != nil && *agent.RMBPerUSDOverride > 0 {
		settings.RMBPerUSD = *agent.RMBPerUSDOverride
	}
	if agent.SubscriptionDiscountOverride != nil && *agent.SubscriptionDiscountOverride > 0 {
		settings.SubscriptionDiscount = *agent.SubscriptionDiscountOverride
	}
	return settings, nil
}

func (s *DistributionService) UpdateSettings(ctx context.Context, settings DistributionSettings) (DistributionSettings, error) {
	if s == nil || s.settingRepo == nil {
		return DistributionSettings{}, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	if err := validateDistributionAmount(settings.RMBPerUSD); err != nil {
		return DistributionSettings{}, err
	}
	if err := validateDistributionAmount(settings.SubscriptionDiscount); err != nil {
		return DistributionSettings{}, err
	}
	if settings.SubscriptionDiscount > 1 {
		return DistributionSettings{}, infraerrors.BadRequest("DISTRIBUTION_INVALID_DISCOUNT", "subscription discount must be between 0 and 1")
	}
	settings.APIKeyGroupIDs = normalizeDistributionGroupIDs(settings.APIKeyGroupIDs)
	if err := s.validateDistributionAPIKeyGroups(ctx, settings.APIKeyGroupIDs); err != nil {
		return DistributionSettings{}, err
	}
	groupIDsJSON, err := json.Marshal(settings.APIKeyGroupIDs)
	if err != nil {
		return DistributionSettings{}, err
	}
	err = s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyDistributionRMBPerUSD:            strconv.FormatFloat(settings.RMBPerUSD, 'f', 8, 64),
		SettingKeyDistributionSubscriptionDiscount: strconv.FormatFloat(settings.SubscriptionDiscount, 'f', 8, 64),
		SettingKeyDistributionAPIKeyGroupIDs:       string(groupIDsJSON),
	})
	if err != nil {
		return DistributionSettings{}, err
	}
	return s.GetSettings(ctx)
}

func (s *DistributionService) ListAPIKeyGroups(ctx context.Context, userID int64) ([]Group, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.groupRepo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	if err := s.ensureActiveAgent(ctx, userID); err != nil {
		return nil, err
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	allowed := int64Set(settings.APIKeyGroupIDs)
	if len(allowed) == 0 {
		return []Group{}, nil
	}
	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups: %w", err)
	}
	out := make([]Group, 0, len(groups))
	for _, group := range groups {
		if allowed[group.ID] && !group.IsSubscriptionType() {
			out = append(out, group)
		}
	}
	return out, nil
}

func (s *DistributionService) UpdateAgentRates(ctx context.Context, userID int64, rates DistributionAgentRateSettings) (*DistributionAgentApplication, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if rates.RMBPerUSDOverride != nil {
		if *rates.RMBPerUSDOverride < 0 || math.IsNaN(*rates.RMBPerUSDOverride) || math.IsInf(*rates.RMBPerUSDOverride, 0) {
			return nil, ErrDistributionInvalidAmount
		}
		if *rates.RMBPerUSDOverride == 0 {
			rates.RMBPerUSDOverride = nil
		}
	}
	if rates.SubscriptionDiscountOverride != nil {
		if *rates.SubscriptionDiscountOverride < 0 || *rates.SubscriptionDiscountOverride > 1 || math.IsNaN(*rates.SubscriptionDiscountOverride) || math.IsInf(*rates.SubscriptionDiscountOverride, 0) {
			return nil, infraerrors.BadRequest("DISTRIBUTION_INVALID_DISCOUNT", "subscription discount must be between 0 and 1")
		}
		if *rates.SubscriptionDiscountOverride == 0 {
			rates.SubscriptionDiscountOverride = nil
		}
	}
	return s.repo.UpdateAgentRates(ctx, userID, rates)
}

func (s *DistributionService) getAPIBaseURL(ctx context.Context) string {
	if s == nil || s.settingRepo == nil {
		return ""
	}
	setting, err := s.settingRepo.Get(ctx, SettingKeyAPIBaseURL)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(setting.Value)
}

func (s *DistributionService) GenerateBalanceRedeemCode(ctx context.Context, userID int64, input DistributionGenerateBalanceRedeemCodeInput) (*DistributionGeneratedRedeemCode, error) {
	if err := validateDistributionAmount(input.ValueUSD); err != nil {
		return nil, err
	}
	settings, err := s.GetEffectiveSettingsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	cost := roundMoney(input.ValueUSD * settings.RMBPerUSD)
	var out *DistributionGeneratedRedeemCode
	wallet, err := s.runGenerationTx(ctx, userID, cost, DistributionLedgerActionGenerateRedeemCode, "redeem_code", normalizeDistributionText(input.Note), func(txCtx context.Context, wallet *DistributionWallet) (string, error) {
		code, err := s.redeem.GenerateRandomCode()
		if err != nil {
			return "", err
		}
		redeemCode := &RedeemCode{
			Code:         code,
			Type:         RedeemTypeBalance,
			Value:        input.ValueUSD,
			Status:       StatusUnused,
			Notes:        distributionNote("distribution balance redeem code", input.Note),
			ValidityDays: 30,
		}
		if err := s.redeem.CreateCode(txCtx, redeemCode); err != nil {
			return "", err
		}
		if _, err := s.repo.CreateAsset(txCtx, DistributionCreateAssetInput{
			UserID:        userID,
			WalletID:      wallet.ID,
			AssetType:     DistributionAssetTypeBalanceRedeemCode,
			ReferenceType: "redeem_code",
			ReferenceID:   redeemCode.Code,
			DisplayValue:  redeemCode.Code,
			FaceValue:     redeemCode.Value,
			CostRMB:       cost,
			Status:        DistributionAssetStatusActive,
			Note:          distributionNote("distribution balance redeem code", input.Note),
		}); err != nil {
			return "", err
		}
		out = &DistributionGeneratedRedeemCode{Code: redeemCode.Code, Type: redeemCode.Type, Value: redeemCode.Value, CostRMB: cost}
		return redeemCode.Code, nil
	})
	if err == nil && out != nil && wallet != nil {
		out.BalanceAfter = wallet.Balance
	}
	return out, err
}

func (s *DistributionService) GenerateSubscriptionRedeemCode(ctx context.Context, userID int64, input DistributionGenerateSubscriptionRedeemCodeInput) (*DistributionGeneratedRedeemCode, error) {
	if input.PlanID <= 0 {
		return nil, infraerrors.BadRequest("DISTRIBUTION_INVALID_SUBSCRIPTION", "subscription plan is required")
	}
	if s == nil || s.paymentConfig == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "payment config service unavailable")
	}
	plan, err := s.paymentConfig.GetPlan(ctx, input.PlanID)
	if err != nil || plan == nil || !plan.ForSale {
		return nil, infraerrors.NotFound("PLAN_NOT_AVAILABLE", "plan not found or not for sale")
	}
	if err := validateDistributionAmount(plan.Price); err != nil {
		return nil, err
	}
	validityDays := psComputeValidityDays(plan.ValidityDays, plan.ValidityUnit)
	if plan.GroupID <= 0 || validityDays <= 0 {
		return nil, infraerrors.BadRequest("DISTRIBUTION_INVALID_SUBSCRIPTION", "invalid subscription plan")
	}
	settings, err := s.GetEffectiveSettingsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	cost := roundMoney(plan.Price * settings.SubscriptionDiscount)
	var out *DistributionGeneratedRedeemCode
	wallet, err := s.runGenerationTx(ctx, userID, cost, DistributionLedgerActionGenerateSubscription, "redeem_code", normalizeDistributionText(input.Note), func(txCtx context.Context, wallet *DistributionWallet) (string, error) {
		group, err := s.groupRepo.GetByID(txCtx, plan.GroupID)
		if err != nil {
			return "", err
		}
		if !group.IsSubscriptionType() {
			return "", infraerrors.BadRequest("DISTRIBUTION_INVALID_GROUP", "group must be subscription type")
		}
		code, err := s.redeem.GenerateRandomCode()
		if err != nil {
			return "", err
		}
		redeemCode := &RedeemCode{
			Code:         code,
			Type:         RedeemTypeSubscription,
			Value:        plan.Price,
			Status:       StatusUnused,
			Notes:        distributionNote("distribution subscription redeem code", input.Note),
			GroupID:      &plan.GroupID,
			ValidityDays: validityDays,
		}
		if err := s.redeem.CreateCode(txCtx, redeemCode); err != nil {
			return "", err
		}
		if _, err := s.repo.CreateAsset(txCtx, DistributionCreateAssetInput{
			UserID:        userID,
			WalletID:      wallet.ID,
			AssetType:     DistributionAssetTypeSubscriptionRedeemCode,
			ReferenceType: "redeem_code",
			ReferenceID:   redeemCode.Code,
			DisplayValue:  redeemCode.Code,
			FaceValue:     redeemCode.Value,
			CostRMB:       cost,
			GroupID:       redeemCode.GroupID,
			ValidityDays:  redeemCode.ValidityDays,
			Status:        DistributionAssetStatusActive,
			Note:          distributionNote(fmt.Sprintf("distribution subscription redeem code: plan %d %s", plan.ID, plan.Name), input.Note),
		}); err != nil {
			return "", err
		}
		planID := int64(plan.ID)
		out = &DistributionGeneratedRedeemCode{Code: redeemCode.Code, Type: redeemCode.Type, Value: redeemCode.Value, PlanID: &planID, PlanName: plan.Name, GroupID: redeemCode.GroupID, ValidityDays: redeemCode.ValidityDays, CostRMB: cost}
		return redeemCode.Code, nil
	})
	if err == nil && out != nil && wallet != nil {
		out.BalanceAfter = wallet.Balance
	}
	return out, err
}

func (s *DistributionService) GenerateAPIKey(ctx context.Context, userID int64, input DistributionGenerateAPIKeyInput) (*DistributionGeneratedAPIKey, error) {
	if err := validateDistributionAmount(input.QuotaUSD); err != nil {
		return nil, err
	}
	if input.GroupID == nil || *input.GroupID <= 0 {
		return nil, infraerrors.BadRequest("DISTRIBUTION_GROUP_REQUIRED", "api key group is required")
	}
	if err := s.ensureAPIKeyGroupExposed(ctx, *input.GroupID); err != nil {
		return nil, err
	}
	settings, err := s.GetEffectiveSettingsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	name := normalizeDistributionText(input.Name)
	if name == "" {
		name = "Distribution package"
	}
	cost := roundMoney(input.QuotaUSD * settings.RMBPerUSD)
	baseURL := s.getAPIBaseURL(ctx)
	var out *DistributionGeneratedAPIKey
	wallet, err := s.runGenerationTx(ctx, userID, cost, DistributionLedgerActionGenerateAPIKey, "api_key", name, func(txCtx context.Context, wallet *DistributionWallet) (string, error) {
		apiKey, err := s.apiKey.CreateForDistribution(txCtx, userID, CreateAPIKeyRequest{
			Name:          name,
			GroupID:       input.GroupID,
			Quota:         input.QuotaUSD,
			ExpiresInDays: input.ExpiresInDays,
		})
		if err != nil {
			return "", err
		}
		if _, err := s.repo.CreateAsset(txCtx, DistributionCreateAssetInput{
			UserID:        userID,
			WalletID:      wallet.ID,
			AssetType:     DistributionAssetTypeAPIKey,
			ReferenceType: "api_key",
			ReferenceID:   strconv.FormatInt(apiKey.ID, 10),
			DisplayValue:  apiKey.Key,
			PackageURL:    baseURL,
			FaceValue:     input.QuotaUSD,
			CostRMB:       cost,
			GroupID:       apiKey.GroupID,
			QuotaUSD:      apiKey.Quota,
			Status:        DistributionAssetStatusActive,
			ExpiresAt:     apiKey.ExpiresAt,
			Note:          name,
		}); err != nil {
			return "", err
		}
		out = &DistributionGeneratedAPIKey{ID: apiKey.ID, Name: apiKey.Name, Key: apiKey.Key, Quota: apiKey.Quota, GroupID: apiKey.GroupID, ExpiresAt: apiKey.ExpiresAt, BaseURL: baseURL, CostRMB: cost}
		return strconv.FormatInt(apiKey.ID, 10), nil
	})
	if err == nil && out != nil && wallet != nil {
		out.BalanceAfter = wallet.Balance
	}
	return out, err
}

func (s *DistributionService) ListWallets(ctx context.Context, page, pageSize int, search string) ([]DistributionWallet, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	return s.repo.ListWallets(ctx, page, pageSize, search)
}

func (s *DistributionService) ListAllWalletLedger(ctx context.Context, page, pageSize int, userID int64) ([]DistributionWalletLedgerEntry, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	return s.repo.ListAllWalletLedger(ctx, page, pageSize, userID)
}

func (s *DistributionService) AdminAdjustWallet(ctx context.Context, input DistributionAdminAdjustWalletInput) (*DistributionWallet, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	if input.UserID <= 0 || input.Amount == 0 || math.IsNaN(input.Amount) || math.IsInf(input.Amount, 0) {
		return nil, ErrDistributionInvalidAmount
	}
	return s.repo.AdjustWalletBalance(ctx, input.UserID, roundMoney(input.Amount), DistributionLedgerActionAdminAdjust, "admin", strconv.FormatInt(input.AdminID, 10), normalizeDistributionText(input.Note), input.AdminID)
}

func (s *DistributionService) UpdateWalletStatus(ctx context.Context, userID int64, frozen bool) (*DistributionWallet, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	status := DistributionWalletStatusActive
	if frozen {
		status = DistributionWalletStatusFrozen
	}
	return s.repo.UpdateWalletStatus(ctx, userID, status)
}

func (s *DistributionService) VoidAsset(ctx context.Context, userID, assetID, operatorID int64, admin bool) (*DistributionVoidAssetResult, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	if assetID <= 0 {
		return nil, infraerrors.BadRequest("DISTRIBUTION_INVALID_ASSET", "invalid distribution asset")
	}
	var result *DistributionVoidAssetResult
	err := s.repo.WithTx(ctx, func(txCtx context.Context) error {
		asset, err := s.repo.GetAssetByID(txCtx, assetID)
		if err != nil {
			return err
		}
		if !admin && asset.UserID != userID {
			return ErrDistributionAgentNotFound
		}
		if asset.Status == DistributionAssetStatusUsed {
			return infraerrors.Conflict("DISTRIBUTION_ASSET_USED", "used distribution asset cannot be voided")
		}
		if asset.RefundedAt != nil || asset.RefundedRMB > 0 {
			return infraerrors.Conflict("DISTRIBUTION_ASSET_REFUNDED", "distribution asset has already been refunded")
		}

		nextStatus := DistributionAssetStatusDisabled
		switch asset.AssetType {
		case DistributionAssetTypeBalanceRedeemCode, DistributionAssetTypeSubscriptionRedeemCode:
			if s.redeem == nil {
				return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "redeem service unavailable")
			}
			code, err := s.redeem.GetByCode(txCtx, asset.ReferenceID)
			if err != nil {
				return err
			}
			if code.Status == StatusUsed {
				return infraerrors.Conflict("DISTRIBUTION_ASSET_USED", "used distribution asset cannot be voided")
			}
			code.Status = StatusExpired
			if err := s.redeem.UpdateCode(txCtx, code); err != nil {
				return err
			}
			nextStatus = DistributionAssetStatusExpired
		case DistributionAssetTypeAPIKey:
			refID, err := strconv.ParseInt(asset.ReferenceID, 10, 64)
			if err != nil || refID <= 0 {
				return infraerrors.BadRequest("DISTRIBUTION_INVALID_ASSET", "invalid api key asset")
			}
			if err := s.repo.VoidAPIKeyAssetReference(txCtx, refID, asset.UserID); err != nil {
				return err
			}
			if s.apiKey != nil {
				s.apiKey.InvalidateAuthCacheByUserID(txCtx, asset.UserID)
			}
			nextStatus = DistributionAssetStatusDisabled
		default:
			return infraerrors.BadRequest("DISTRIBUTION_INVALID_ASSET", "unsupported distribution asset type")
		}

		refunded, err := s.repo.MarkAssetRefunded(txCtx, asset.ID, nextStatus, operatorID)
		if err != nil {
			return err
		}
		wallet, err := s.repo.AdjustWalletBalance(txCtx, asset.UserID, asset.CostRMB, DistributionLedgerActionAssetRefund, "distribution_asset", strconv.FormatInt(asset.ID, 10), "void distribution asset refund", operatorID)
		if err != nil {
			return err
		}
		result = &DistributionVoidAssetResult{Asset: refunded, Wallet: wallet, RefundRMB: asset.CostRMB}
		return nil
	})
	return result, err
}

func (s *DistributionService) runGenerationTx(ctx context.Context, userID int64, cost float64, action, referenceType, note string, create func(txCtx context.Context, wallet *DistributionWallet) (string, error)) (*DistributionWallet, error) {
	if s == nil || s.repo == nil || s.redeem == nil || s.apiKey == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if err := validateDistributionAmount(cost); err != nil {
		return nil, err
	}
	var updatedWallet *DistributionWallet
	err := s.repo.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.ensureActiveAgent(txCtx, userID); err != nil {
			return err
		}
		wallet, err := s.repo.GetWalletByUserID(txCtx, userID)
		if err != nil {
			return err
		}
		if wallet.Status != DistributionWalletStatusActive {
			return ErrDistributionWalletInactive
		}
		if wallet.Balance+1e-9 < cost {
			return ErrDistributionInsufficient
		}
		refID, err := create(txCtx, wallet)
		if err != nil {
			return err
		}
		updated, err := s.repo.AdjustWalletBalance(txCtx, userID, -cost, action, referenceType, refID, note, userID)
		if err != nil {
			return err
		}
		updatedWallet = updated
		return nil
	})
	return updatedWallet, err
}

func (s *DistributionService) ensureActiveAgent(ctx context.Context, userID int64) error {
	agent, err := s.repo.GetAgentApplication(ctx, userID)
	if err != nil {
		return err
	}
	switch agent.Status {
	case DistributionAgentStatusApproved:
		return nil
	case DistributionAgentStatusPending:
		return ErrDistributionAgentPending
	case DistributionAgentStatusRejected:
		return ErrDistributionAgentRejected
	default:
		return ErrDistributionAgentFrozen
	}
}

func normalizeDistributionText(v string) string {
	return strings.TrimSpace(v)
}

func validateDistributionAmount(v float64) error {
	if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return ErrDistributionInvalidAmount
	}
	return nil
}

func roundMoney(v float64) float64 {
	return math.Round(v*1e8) / 1e8
}

func parseDistributionGroupIDs(raw string) []int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []int64{}
	}
	var ids []int64
	if err := json.Unmarshal([]byte(raw), &ids); err == nil {
		return ids
	}
	return []int64{}
}

func normalizeDistributionGroupIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return []int64{}
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func int64Set(ids []int64) map[int64]bool {
	out := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id > 0 {
			out[id] = true
		}
	}
	return out
}

func (s *DistributionService) validateDistributionAPIKeyGroups(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	if s == nil || s.groupRepo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	for _, id := range ids {
		group, err := s.groupRepo.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if !group.IsActive() || group.IsSubscriptionType() {
			return infraerrors.BadRequest("DISTRIBUTION_INVALID_GROUP", "distribution api key group must be an active standard group")
		}
	}
	return nil
}

func (s *DistributionService) ensureAPIKeyGroupExposed(ctx context.Context, groupID int64) error {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return err
	}
	if !int64Set(settings.APIKeyGroupIDs)[groupID] {
		return ErrDistributionGroupNotExposed
	}
	if s == nil || s.groupRepo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "distribution service unavailable")
	}
	group, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return err
	}
	if !group.IsActive() || group.IsSubscriptionType() {
		return ErrDistributionGroupNotExposed
	}
	return nil
}

func distributionNote(prefix, note string) string {
	note = normalizeDistributionText(note)
	if note == "" {
		return prefix
	}
	return fmt.Sprintf("%s: %s", prefix, note)
}
