package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Vault is one Obsidian library root.
type Vault struct {
	Path   string
	Source string // "obsidian.json" or "manual"
}

func findObsidianJSON(appDir string) string {
	if appDir == "" {
		return ""
	}
	for _, name := range []string{"obsidian.json", "Obsidian.json"} {
		p := filepath.Join(appDir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return filepath.Join(appDir, "obsidian.json")
}

// ParseObsidianJSON extracts vault filesystem paths from the official vault list.
func ParseObsidianJSON(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	var parsed struct {
		Vaults map[string]struct {
			Path string `json:"path"`
		} `json:"vaults"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil || parsed.Vaults == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, v := range parsed.Vaults {
		p := strings.TrimSpace(v.Path)
		if p == "" {
			continue
		}
		key := normKey(p)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}

func loadRegisteredVaults(obsidianAppDir string) []Vault {
	p := findObsidianJSON(obsidianAppDir)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var out []Vault
	for _, path := range ParseObsidianJSON(data) {
		out = append(out, Vault{Path: path, Source: "obsidian.json"})
	}
	return out
}

// PluginEligible reports whether an .obsidian/plugins/<id> folder is Claude-related.
// Name match is case-insensitive claude/anthropic. Copilot-like plugin ids are
// included only when their data.json mentions those same keywords.
func PluginEligible(pluginID string, dataJSON []byte) bool {
	id := strings.ToLower(strings.TrimSpace(pluginID))
	if id == "" || id == "cache" {
		return false
	}
	if NameMatchesClaudeOrAnthropic(id) {
		return true
	}
	if !strings.Contains(id, "copilot") {
		return false
	}
	blob := strings.ToLower(string(dataJSON))
	return strings.Contains(blob, "claude") || strings.Contains(blob, "anthropic")
}

// NameMatchesClaudeOrAnthropic is the bounded AppData directory-name rule.
func NameMatchesClaudeOrAnthropic(name string) bool {
	l := strings.ToLower(name)
	return strings.Contains(l, "claude") || strings.Contains(l, "anthropic")
}
