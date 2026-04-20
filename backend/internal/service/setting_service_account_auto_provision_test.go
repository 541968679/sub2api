package service

import (
	"context"
	"testing"
)

type accountAutoProvisionSettingRepoStub struct {
	data map[string]string
}

func (s *accountAutoProvisionSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *accountAutoProvisionSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.data[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *accountAutoProvisionSettingRepoStub) Set(_ context.Context, key, value string) error {
	if s.data == nil {
		s.data = make(map[string]string)
	}
	s.data[key] = value
	return nil
}

func (s *accountAutoProvisionSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *accountAutoProvisionSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *accountAutoProvisionSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *accountAutoProvisionSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestGetAccountAutoProvisionSettings_Defaults(t *testing.T) {
	repo := &accountAutoProvisionSettingRepoStub{data: map[string]string{}}
	svc := NewSettingService(repo, nil)

	settings, err := svc.GetAccountAutoProvisionSettings(context.Background())
	if err != nil {
		t.Fatalf("GetAccountAutoProvisionSettings() error = %v", err)
	}
	if settings == nil {
		t.Fatal("GetAccountAutoProvisionSettings() returned nil settings")
	}
	if settings.Enabled {
		t.Fatal("default auto provision should be disabled")
	}
	if settings.CheckIntervalSeconds != 60 {
		t.Fatalf("CheckIntervalSeconds = %d, want 60", settings.CheckIntervalSeconds)
	}
}

func TestSetAccountAutoProvisionSettings_RoundTrip(t *testing.T) {
	repo := &accountAutoProvisionSettingRepoStub{data: map[string]string{}}
	svc := NewSettingService(repo, nil)

	settings := &AccountAutoProvisionSettings{
		Enabled:              true,
		CheckIntervalSeconds: 120,
		MaxActionsPerRun:     2,
		Rules: []AccountAutoProvisionRule{
			{
				ID:                            "rule-1",
				Name:                          "rule-1",
				Enabled:                       true,
				GroupIDs:                      []int64{1, 2},
				NormalAccountCountBelow:       1,
				ConcurrencyUtilizationAbove:   80,
				AICreditsBelow:                10,
				AICreditsCheckIntervalMinutes: 15,
				CooldownMinutes:               20,
				ProvisionMode:                 AccountAutoProvisionModeCloneLastHealth,
				Template: AccountAutoProvisionTemplate{
					Concurrency:   8,
					Schedulable:   true,
					AllowOverages: true,
				},
			},
		},
	}

	if err := svc.SetAccountAutoProvisionSettings(context.Background(), settings); err != nil {
		t.Fatalf("SetAccountAutoProvisionSettings() error = %v", err)
	}

	loaded, err := svc.GetAccountAutoProvisionSettings(context.Background())
	if err != nil {
		t.Fatalf("GetAccountAutoProvisionSettings() error = %v", err)
	}
	if !loaded.Enabled {
		t.Fatal("loaded settings should be enabled")
	}
	if len(loaded.Rules) != 1 {
		t.Fatalf("len(loaded.Rules) = %d, want 1", len(loaded.Rules))
	}
	if loaded.Rules[0].ProvisionMode != AccountAutoProvisionModeCloneLastHealth {
		t.Fatalf("ProvisionMode = %q", loaded.Rules[0].ProvisionMode)
	}
}

func TestSetAccountAutoProvisionSettings_RejectsRuleWithoutTrigger(t *testing.T) {
	repo := &accountAutoProvisionSettingRepoStub{data: map[string]string{}}
	svc := NewSettingService(repo, nil)

	err := svc.SetAccountAutoProvisionSettings(context.Background(), &AccountAutoProvisionSettings{
		Enabled:              true,
		CheckIntervalSeconds: 60,
		MaxActionsPerRun:     1,
		Rules: []AccountAutoProvisionRule{
			{
				ID:            "rule-1",
				Name:          "rule-1",
				Enabled:       true,
				GroupIDs:      []int64{1},
				ProvisionMode: AccountAutoProvisionModeTemplate,
				Template: AccountAutoProvisionTemplate{
					Concurrency:   8,
					Schedulable:   true,
					AllowOverages: false,
				},
			},
		},
	})
	if err == nil {
		t.Fatal("SetAccountAutoProvisionSettings() expected error for rule without trigger")
	}
}
