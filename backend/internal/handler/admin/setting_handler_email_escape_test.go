//go:build unit

package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildTestEmailBodyEscapesSiteName(t *testing.T) {
	body := buildTestEmailBody(`</h1><script>alert(1)</script><h1>`)

	assert.NotContains(t, body, "<script>")
	assert.Contains(t, body, "&lt;script&gt;")
}
