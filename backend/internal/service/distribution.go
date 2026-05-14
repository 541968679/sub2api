package service

import (
	"context"
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

var (
	ErrDistributionAgentNotFound  = infraerrors.NotFound("DISTRIBUTION_AGENT_NOT_FOUND", "distribution agent not found")
	ErrDistributionAgentPending   = infraerrors.BadRequest("DISTRIBUTION_AGENT_PENDING", "distribution agent application is pending")
	ErrDistributionAgentRejected  = infraerrors.BadRequest("DISTRIBUTION_AGENT_REJECTED", "distribution agent application was rejected")
	ErrDistributionAgentFrozen    = infraerrors.Forbidden("DISTRIBUTION_AGENT_FROZEN", "distribution agent account is frozen")
	ErrDistributionAlreadyApplied = infraerrors.Conflict("DISTRIBUTION_ALREADY_APPLIED", "distribution application already exists")
	ErrDistributionWalletNotFound = infraerrors.NotFound("DISTRIBUTION_WALLET_NOT_FOUND", "distribution wallet not found")
	ErrDistributionWalletInactive = infraerrors.Forbidden("DISTRIBUTION_WALLET_INACTIVE", "distribution wallet is not active")
	ErrDistributionInvalidAmount  = infraerrors.BadRequest("DISTRIBUTION_INVALID_AMOUNT", "invalid amount")
	ErrDistributionInsufficient   = infraerrors.BadRequest("DISTRIBUTION_INSUFFICIENT_BALANCE", "insufficient distribution balance")
)

const (
	DistributionLedgerActionAdminAdjust          = "admin_adjust"
	DistributionLedgerActionGenerateRedeemCode   = "generate_redeem_code"
	DistributionLedgerActionGenerateSubscription = "generate_subscription"
	DistributionLedgerActionGenerateAPIKey       = "generate_api_key"
)

type DistributionAgentApplication struct {
	UserID     int64      `json:"user_id"`
	UserEmail  string     `json:"user_email,omitempty"`
	Username   string     `json:"username,omitempty"`
	Status     string     `json:"status"`
	Contact    string     `json:"contact"`
	Reason     string     `json:"reason"`
	AdminNote  string     `json:"admin_note"`
	ReviewedBy *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
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

type DistributionSummary struct {
	Application *DistributionAgentApplication `json:"application"`
	Wallet      *DistributionWallet           `json:"wallet"`
	Settings    DistributionSettings          `json:"settings"`
}

type DistributionSettings struct {
	RMBPerUSD            float64 `json:"rmb_per_usd"`
	SubscriptionDiscount float64 `json:"subscription_discount"`
}

type DistributionGeneratedRedeemCode struct {
	Code         string  `json:"code"`
	Type         string  `json:"type"`
	Value        float64 `json:"value"`
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
	CostRMB      float64    `json:"cost_rmb"`
	BalanceAfter float64    `json:"balance_after"`
}

type DistributionGenerateBalanceRedeemCodeInput struct {
	ValueUSD float64
	Note     string
}

type DistributionGenerateSubscriptionRedeemCodeInput struct {
	FaceValueRMB float64
	GroupID      int64
	ValidityDays int
	Note         string
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
	UpdateWalletStatus(ctx context.Context, userID int64, status string) (*DistributionWallet, error)
	AdjustWalletBalance(ctx context.Context, userID int64, amount float64, action, referenceType, referenceID, note string, createdBy int64) (*DistributionWallet, error)
	WithTx(ctx context.Context, fn func(txCtx context.Context) error) error
}

type DistributionService struct {
	repo        DistributionRepository
	settingRepo SettingRepository
	redeem      *RedeemService
	apiKey      *APIKeyService
	groupRepo   GroupRepository
}

func NewDistributionService(repo DistributionRepository, settingRepo SettingRepository, redeem *RedeemService, apiKey *APIKeyService, groupRepo GroupRepository) *DistributionService {
	return &DistributionService{repo: repo, settingRepo: settingRepo, redeem: redeem, apiKey: apiKey, groupRepo: groupRepo}
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
	settings, _ := s.GetSettings(ctx)
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

func (s *DistributionService) GetSettings(ctx context.Context) (DistributionSettings, error) {
	defaults := DistributionSettings{RMBPerUSD: 0.5, SubscriptionDiscount: 0.75}
	if s == nil || s.settingRepo == nil {
		return defaults, nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyDistributionRMBPerUSD, SettingKeyDistributionSubscriptionDiscount})
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
	return out, nil
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
	err := s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyDistributionRMBPerUSD:            strconv.FormatFloat(settings.RMBPerUSD, 'f', 8, 64),
		SettingKeyDistributionSubscriptionDiscount: strconv.FormatFloat(settings.SubscriptionDiscount, 'f', 8, 64),
	})
	if err != nil {
		return DistributionSettings{}, err
	}
	return s.GetSettings(ctx)
}

func (s *DistributionService) GenerateBalanceRedeemCode(ctx context.Context, userID int64, input DistributionGenerateBalanceRedeemCodeInput) (*DistributionGeneratedRedeemCode, error) {
	if err := validateDistributionAmount(input.ValueUSD); err != nil {
		return nil, err
	}
	settings, err := s.GetSettings(ctx)
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
		out = &DistributionGeneratedRedeemCode{Code: redeemCode.Code, Type: redeemCode.Type, Value: redeemCode.Value, CostRMB: cost}
		return redeemCode.Code, nil
	})
	if err == nil && out != nil && wallet != nil {
		out.BalanceAfter = wallet.Balance
	}
	return out, err
}

func (s *DistributionService) GenerateSubscriptionRedeemCode(ctx context.Context, userID int64, input DistributionGenerateSubscriptionRedeemCodeInput) (*DistributionGeneratedRedeemCode, error) {
	if err := validateDistributionAmount(input.FaceValueRMB); err != nil {
		return nil, err
	}
	if input.GroupID <= 0 || input.ValidityDays <= 0 {
		return nil, infraerrors.BadRequest("DISTRIBUTION_INVALID_SUBSCRIPTION", "group_id and validity_days are required")
	}
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	cost := roundMoney(input.FaceValueRMB * settings.SubscriptionDiscount)
	var out *DistributionGeneratedRedeemCode
	wallet, err := s.runGenerationTx(ctx, userID, cost, DistributionLedgerActionGenerateSubscription, "redeem_code", normalizeDistributionText(input.Note), func(txCtx context.Context, wallet *DistributionWallet) (string, error) {
		group, err := s.groupRepo.GetByID(txCtx, input.GroupID)
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
			Value:        input.FaceValueRMB,
			Status:       StatusUnused,
			Notes:        distributionNote("distribution subscription redeem code", input.Note),
			GroupID:      &input.GroupID,
			ValidityDays: input.ValidityDays,
		}
		if err := s.redeem.CreateCode(txCtx, redeemCode); err != nil {
			return "", err
		}
		out = &DistributionGeneratedRedeemCode{Code: redeemCode.Code, Type: redeemCode.Type, Value: redeemCode.Value, GroupID: redeemCode.GroupID, ValidityDays: redeemCode.ValidityDays, CostRMB: cost}
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
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	name := normalizeDistributionText(input.Name)
	if name == "" {
		name = "Distribution package"
	}
	cost := roundMoney(input.QuotaUSD * settings.RMBPerUSD)
	var out *DistributionGeneratedAPIKey
	wallet, err := s.runGenerationTx(ctx, userID, cost, DistributionLedgerActionGenerateAPIKey, "api_key", name, func(txCtx context.Context, wallet *DistributionWallet) (string, error) {
		apiKey, err := s.apiKey.Create(txCtx, userID, CreateAPIKeyRequest{
			Name:          name,
			GroupID:       input.GroupID,
			Quota:         input.QuotaUSD,
			ExpiresInDays: input.ExpiresInDays,
		})
		if err != nil {
			return "", err
		}
		out = &DistributionGeneratedAPIKey{ID: apiKey.ID, Name: apiKey.Name, Key: apiKey.Key, Quota: apiKey.Quota, GroupID: apiKey.GroupID, ExpiresAt: apiKey.ExpiresAt, CostRMB: cost}
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

func distributionNote(prefix, note string) string {
	note = normalizeDistributionText(note)
	if note == "" {
		return prefix
	}
	return fmt.Sprintf("%s: %s", prefix, note)
}
