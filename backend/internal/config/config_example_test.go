package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDeployConfigExampleUsesProductionImageSafeResponseLimit(t *testing.T) {
	path := filepath.Join("..", "..", "..", "deploy", "config.example.yaml")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var example struct {
		Gateway struct {
			UpstreamResponseReadMaxBytes int64 `yaml:"upstream_response_read_max_bytes"`
		} `yaml:"gateway"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &example))
	require.Equal(
		t,
		DefaultUpstreamResponseReadMaxBytes,
		example.Gateway.UpstreamResponseReadMaxBytes,
		"the deployment example must not override the image-safe code default with a smaller limit",
	)
}
