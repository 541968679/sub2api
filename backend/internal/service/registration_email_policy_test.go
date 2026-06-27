//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeRegistrationEmailSuffixWhitelist(t *testing.T) {
	got, err := NormalizeRegistrationEmailSuffixWhitelist([]string{"example.com", "@EXAMPLE.COM", " @foo.bar ", "*.EDU.cn", "@*.example.org"})
	require.NoError(t, err)
	require.Equal(t, []string{"@example.com", "@foo.bar", "@*.edu.cn", "@*.example.org"}, got)
}

func TestNormalizeRegistrationEmailSuffixWhitelist_Invalid(t *testing.T) {
	_, err := NormalizeRegistrationEmailSuffixWhitelist([]string{"@invalid_domain"})
	require.Error(t, err)
}

func TestParseRegistrationEmailSuffixWhitelist(t *testing.T) {
	got := ParseRegistrationEmailSuffixWhitelist(`["example.com","@foo.bar","*.edu.cn","@invalid_domain"]`)
	require.Equal(t, []string{"@example.com", "@foo.bar", "@*.edu.cn"}, got)
}

func TestIsRegistrationEmailSuffixAllowed(t *testing.T) {
	require.True(t, IsRegistrationEmailSuffixAllowed("user@example.com", []string{"@example.com"}))
	require.False(t, IsRegistrationEmailSuffixAllowed("user@sub.example.com", []string{"@example.com"}))
	require.True(t, IsRegistrationEmailSuffixAllowed("student@cs.edu.cn", []string{"@*.edu.cn"}))
	require.True(t, IsRegistrationEmailSuffixAllowed("student@deep.cs.edu.cn", []string{"@*.edu.cn"}))
	require.False(t, IsRegistrationEmailSuffixAllowed("student@edu.cn", []string{"@*.edu.cn"}))
	require.False(t, IsRegistrationEmailSuffixAllowed("student@badedu.cn", []string{"@*.edu.cn"}))
	require.True(t, IsRegistrationEmailSuffixAllowed("user@any.com", []string{}))
}
