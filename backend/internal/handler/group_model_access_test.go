package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestDisallowedResponsesImageToolModel(t *testing.T) {
	group := &service.Group{BlockedModels: []string{"gpt-image-*"}}
	body := []byte(`{
		"model": "gpt-5.4",
		"tools": [
			{"type": "web_search"},
			{"type": "image_generation", "model": "gpt-image-2"}
		]
	}`)

	if got := disallowedResponsesImageToolModel(group, body); got != "gpt-image-2" {
		t.Fatalf("disallowedResponsesImageToolModel() = %q, want gpt-image-2", got)
	}
}
