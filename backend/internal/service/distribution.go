package service

import (
	"context"
	"errors"
	"math"
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
}

type DistributionRepository interface {
	EnsureAgent(ctx context.Context, userID int64) (*DistributionAgentApplication, error)
	CreateAgentApplication(ctx context.Context, userID int64, contact, reason string) (*DistributionAgentApplication, error)
	GetAgentApplication(ctx context.Context, userID int64) (*DistributionAgentApplication, error)
	ListAgentApplications(ctx context.Context, page, pageSize int, search string) ([]DistributionAgentApplication, int64, error)
	ReviewAgentApplication(ctx context.Context, userID int64, approved bool, adminNote string, reviewedBy int64) (*DistributionAgentApplication, error)
	EnsureWallet(ctx context.Context, userID int64) (*DistributionWallet, error)
	GetWalletByUserID(ctx context.Context, userID int64) (*DistributionWallet, error)
	ListWalletLedger(ctx context.Context, userID int64, page, pageSize int) ([]DistributionWalletLedgerEntry, int64, error)
}

type DistributionService struct {
	repo DistributionRepository
}

func NewDistributionService(repo DistributionRepository, _ UserRepository, _ AdminService) *DistributionService {
	return &DistributionService{repo: repo}
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
	return &DistributionSummary{Application: agent, Wallet: wallet}, nil
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
	return &DistributionSummary{Application: app, Wallet: nil}, nil
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

func normalizeDistributionText(v string) string {
	return strings.TrimSpace(v)
}

func validateDistributionAmount(v float64) error {
	if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return ErrDistributionInvalidAmount
	}
	return nil
}
