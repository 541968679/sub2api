package domain

// GroupModelAllowlist is the group-level model allowlist config.
// Enabled filters /v1/models listing. Gateway request admission is a
// separate setting (group_model_allowlist_enforce, default off).
type GroupModelAllowlist struct {
	Enabled bool     `json:"enabled"`
	Models  []string `json:"models,omitempty"`
}
