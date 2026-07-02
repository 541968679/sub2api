package claude

import "testing"

func TestDefaultModelIDsContainsSonnet5(t *testing.T) {
	t.Parallel()

	for _, id := range DefaultModelIDs() {
		if id == "claude-sonnet-5" {
			return
		}
	}

	t.Fatal("expected claude-sonnet-5 in default Claude model list")
}
