package admin

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestCodexSessionImportParsesRawJSONAndArrayInputs(t *testing.T) {
	req := CodexSessionImportRequest{
		Content:  "raw-access-token",
		Contents: []string{`[{"access_token":"json-token"},{"tokens":{"access_token":"nested-token"}}]`},
	}

	entries, err := parseCodexSessionImportEntries(req)
	if err != nil {
		t.Fatalf("parseCodexSessionImportEntries() error = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entry count = %d, want 3", len(entries))
	}
}

func TestCodexSessionIdentityPrefersUserBeforeSharedAccount(t *testing.T) {
	keys := buildCodexImportIdentityKeys("shared-workspace", "member-42", "member@example.com", "access", "refresh")
	if len(keys) == 0 || keys[0] != "user:member-42" {
		t.Fatalf("identity keys = %v, want chatgpt_user_id first", keys)
	}
	if keys[len(keys)-1] != "account:shared-workspace" {
		t.Fatalf("identity keys = %v, want shared account fallback last", keys)
	}
}

func TestCodexAccessOnlyIdentityUsesTokenFingerprint(t *testing.T) {
	keys := buildCodexImportIdentityKeys("shared-workspace", "member-42", "member@example.com", "access-only", "")
	sum := sha256.Sum256([]byte("access-only"))
	want := "access:" + hex.EncodeToString(sum[:])
	if len(keys) != 1 || keys[0] != want {
		t.Fatalf("identity keys = %v, want [%s]", keys, want)
	}
}
