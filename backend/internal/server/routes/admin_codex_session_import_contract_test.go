package routes

import (
	"os"
	"strings"
	"testing"
)

func TestAdminAccountRoutesRegisterCodexSessionImport(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin routes: %v", err)
	}
	registration := `accounts.POST("/import/codex-session", h.Admin.Account.ImportCodexSession)`
	if !strings.Contains(string(source), registration) {
		t.Fatalf("admin account routes missing %s", registration)
	}
}
