package service

import "testing"

func TestGroupIsModelAllowed(t *testing.T) {
	tests := []struct {
		name    string
		group   *Group
		model   string
		allowed bool
	}{
		{
			name:    "empty lists allow by default",
			group:   &Group{},
			model:   "gpt-5.4",
			allowed: true,
		},
		{
			name:    "blocked exact match denies",
			group:   &Group{BlockedModels: []string{"gpt-image-2"}},
			model:   "GPT-Image-2",
			allowed: false,
		},
		{
			name:    "blocked trailing wildcard denies",
			group:   &Group{BlockedModels: []string{"gpt-image-*"}},
			model:   "gpt-image-2",
			allowed: false,
		},
		{
			name:    "non-empty allow list denies misses",
			group:   &Group{AllowedModels: []string{"gpt-5*"}},
			model:   "gpt-image-2",
			allowed: false,
		},
		{
			name:    "allow list permits matches",
			group:   &Group{AllowedModels: []string{"gpt-5*"}},
			model:   "gpt-5.4-mini",
			allowed: true,
		},
		{
			name:    "block list wins over allow list",
			group:   &Group{BlockedModels: []string{"gpt-image-*"}, AllowedModels: []string{"gpt-image-2"}},
			model:   "gpt-image-2",
			allowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.group.IsModelAllowed(tt.model); got != tt.allowed {
				t.Fatalf("IsModelAllowed(%q) = %v, want %v", tt.model, got, tt.allowed)
			}
		})
	}
}
