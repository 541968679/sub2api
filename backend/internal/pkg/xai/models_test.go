//go:build unit

package xai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultModelsIncludesGrok46(t *testing.T) {
	t.Parallel()
	ids := DefaultModelIDs()
	require.Contains(t, ids, "grok-4.6")
	require.Equal(t, "grok-4.6", ResolveGrokTextResponsesModelID("grok-4.6"))
	require.Equal(t, "grok-4.6", ResolveGrokTextResponsesModelID("grok-4.6-latest"))
	require.Equal(t, DefaultTextModel, ResolveGrokTextResponsesModelID(""))
	require.Equal(t, "grok-4.6", ResolveGrokTextResponsesModelID("grok"))
	require.Equal(t, "grok-4.6", ResolveGrokTextResponsesModelID("grok-latest"))
	require.Equal(t, "grok-4.5", ResolveGrokTextResponsesModelID("grok-4.5"))
}
