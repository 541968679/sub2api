package securityaudit

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluate_DefaultOffAlwaysAllows(t *testing.T) {
	cfg := DefaultConfig()
	require.Equal(t, ModeOff, EffectiveMode(cfg))
	d := Evaluate(cfg, true)
	require.True(t, d.AllowNextStage)
	require.Equal(t, DecisionAllow, d.Kind)
}

func TestEvaluate_ObserveAllowsEvenWhenFlagged(t *testing.T) {
	d := Evaluate(Config{Mode: ModeObserve}, true)
	require.True(t, d.AllowNextStage)
}

func TestEvaluate_BlockingBlocksWhenFlagged(t *testing.T) {
	d := Evaluate(Config{Mode: ModeBlocking}, true)
	require.False(t, d.AllowNextStage)
	require.Equal(t, DecisionBlock, d.Kind)
	require.Equal(t, http.StatusForbidden, d.StatusCode)
	require.NotEmpty(t, d.Message)
}

func TestEvaluate_BlockingAllowsWhenNotFlagged(t *testing.T) {
	d := Evaluate(Config{Mode: ModeBlocking}, false)
	require.True(t, d.AllowNextStage)
}
