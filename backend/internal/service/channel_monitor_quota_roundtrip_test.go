//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type quotaRoundTripRepo struct {
	ChannelMonitorRepository
	byID map[int64]*ChannelMonitor
	next int64
}

func (r *quotaRoundTripRepo) Create(_ context.Context, m *ChannelMonitor) error {
	if r.byID == nil {
		r.byID = map[int64]*ChannelMonitor{}
	}
	r.next++
	clone := *m
	clone.ID = r.next
	if m.AccountID != nil {
		id := *m.AccountID
		clone.AccountID = &id
	}
	clone.CheckMode = defaultCheckMode(m.CheckMode)
	r.byID[clone.ID] = &clone
	m.ID = clone.ID
	return nil
}

func (r *quotaRoundTripRepo) GetByID(_ context.Context, id int64) (*ChannelMonitor, error) {
	m, ok := r.byID[id]
	if !ok {
		return nil, ErrChannelMonitorNotFound
	}
	clone := *m
	if m.AccountID != nil {
		idCopy := *m.AccountID
		clone.AccountID = &idCopy
	}
	return &clone, nil
}

func (r *quotaRoundTripRepo) Update(_ context.Context, m *ChannelMonitor) error {
	if r.byID == nil || r.byID[m.ID] == nil {
		return ErrChannelMonitorNotFound
	}
	clone := *m
	if m.AccountID != nil {
		id := *m.AccountID
		clone.AccountID = &id
	} else {
		clone.AccountID = nil
	}
	clone.CheckMode = defaultCheckMode(m.CheckMode)
	r.byID[m.ID] = &clone
	return nil
}

func TestChannelMonitorQuotaModeRoundTrip(t *testing.T) {
	repo := &quotaRoundTripRepo{}
	accountID := int64(42)
	created := &ChannelMonitor{
		Name:         "kimi-quota-roundtrip",
		Provider:     MonitorProviderKimi,
		CheckMode:    MonitorCheckModeQuota,
		AccountID:    &accountID,
		PrimaryModel: "quota",
	}
	require.NoError(t, repo.Create(context.Background(), created))

	loaded, err := repo.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, MonitorCheckModeQuota, loaded.CheckMode)
	require.NotNil(t, loaded.AccountID)
	require.Equal(t, accountID, *loaded.AccountID)

	loaded.CheckMode = MonitorCheckModeProbe
	loaded.AccountID = nil
	require.NoError(t, repo.Update(context.Background(), loaded))
	reloaded, err := repo.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, MonitorCheckModeProbe, reloaded.CheckMode)
	require.Nil(t, reloaded.AccountID)

	reloaded.CheckMode = MonitorCheckModeQuotaProbe
	reloaded.AccountID = &accountID
	require.NoError(t, repo.Update(context.Background(), reloaded))
	final, err := repo.GetByID(context.Background(), created.ID)
	require.NoError(t, err)
	require.Equal(t, MonitorCheckModeQuotaProbe, final.CheckMode)
	require.NotNil(t, final.AccountID)
	require.Equal(t, accountID, *final.AccountID)
}
