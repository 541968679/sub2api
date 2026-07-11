//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmailBodiesEscapeConfiguredValues(t *testing.T) {
	svc := &EmailService{}

	verifyBody := svc.buildVerifyCodeEmailBody("123456", `</h1><script>alert(1)</script><h1>`)
	assert.NotContains(t, verifyBody, "<script>")
	assert.Contains(t, verifyBody, "&lt;script&gt;")

	resetBody := svc.buildPasswordResetEmailBody(`https://example.com/reset?a=1&b=2`, `</h1><img src=x onerror=alert(1)>`)
	assert.NotContains(t, resetBody, "<img src=x")
	assert.Contains(t, resetBody, "&lt;img")
	assert.NotContains(t, resetBody, `href="https://example.com/reset?a=1&b=2"`)
	assert.Contains(t, resetBody, `href="https://example.com/reset?a=1&amp;b=2"`)
}
