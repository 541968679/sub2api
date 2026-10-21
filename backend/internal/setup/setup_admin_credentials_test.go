package setup

import (
	"strings"
	"testing"
)

func TestPrepareAdminCredentialsGeneratesUnguessableEmail(t *testing.T) {
	admin := AdminConfig{}
	emailGenerated, passwordGenerated, err := prepareAdminCredentials(&admin)
	if err != nil {
		t.Fatal(err)
	}
	if !emailGenerated || !passwordGenerated {
		t.Fatalf("generated flags = %v %v", emailGenerated, passwordGenerated)
	}
	if admin.Email == "admin@sub2api.local" || !strings.HasPrefix(admin.Email, "admin-") || !strings.HasSuffix(admin.Email, "@sub2api.local") {
		t.Fatalf("email = %q", admin.Email)
	}
	if len(admin.Password) < 8 {
		t.Fatalf("password length = %d", len(admin.Password))
	}
}

func TestPrepareAdminCredentialsRejectsShortPassword(t *testing.T) {
	admin := AdminConfig{Email: "owner@example.com", Password: "short"}
	if _, _, err := prepareAdminCredentials(&admin); err == nil {
		t.Fatal("expected short password to fail")
	}
}

func TestPrepareAdminCredentialsKeepsValidSuppliedEmail(t *testing.T) {
	admin := AdminConfig{Email: "owner@example.com", Password: "long-enough-password"}
	emailGenerated, passwordGenerated, err := prepareAdminCredentials(&admin)
	if err != nil {
		t.Fatal(err)
	}
	if emailGenerated || passwordGenerated {
		t.Fatal("supplied credentials should not be replaced")
	}
	if admin.Email != "owner@example.com" {
		t.Fatalf("email = %q", admin.Email)
	}
}
