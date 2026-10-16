//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestApplyInboundHost_UsesContextWhenUnset(t *testing.T) {
	log := &UsageLog{}
	ctx := context.WithValue(context.Background(), ctxkey.InboundHost, "zerocode.kaynlab.asia")
	applyInboundHost(ctx, log)
	require.NotNil(t, log.InboundHost)
	require.Equal(t, "zerocode.kaynlab.asia", *log.InboundHost)
}

func TestApplyInboundHost_KeepsExplicitValue(t *testing.T) {
	existing := "already.example"
	log := &UsageLog{InboundHost: &existing}
	ctx := context.WithValue(context.Background(), ctxkey.InboundHost, "zerocode.kaynlab.asia")
	applyInboundHost(ctx, log)
	require.Equal(t, "already.example", *log.InboundHost)
}
