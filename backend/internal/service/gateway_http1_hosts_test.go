//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNormalizeGatewayHTTP1Hosts(t *testing.T) {
	got, err := NormalizeGatewayHTTP1Hosts([]string{
		" https://API.InferHub.dev/v1 ",
		"api.inferhub.dev",
		"*.InferHub.dev",
		"",
		"  ",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"api.inferhub.dev", "*.inferhub.dev"}, got)

	_, err = NormalizeGatewayHTTP1Hosts([]string{"not a host"})
	require.Error(t, err)

	tooMany := make([]string, 0, gatewayHTTP1HostLimit+1)
	for i := 0; i < gatewayHTTP1HostLimit+1; i++ {
		tooMany = append(tooMany, "h"+itoa(i)+".example.com")
	}
	_, err = NormalizeGatewayHTTP1Hosts(tooMany)
	require.Error(t, err)
}

func TestGatewayHTTP1HostMatches(t *testing.T) {
	patterns := []string{"api.inferhub.dev", "*.example.com"}
	require.True(t, GatewayHTTP1HostMatches(patterns, "API.InferHub.dev"))
	require.False(t, GatewayHTTP1HostMatches(patterns, "files.inferhub.dev"))
	require.True(t, GatewayHTTP1HostMatches(patterns, "example.com"))
	require.True(t, GatewayHTTP1HostMatches(patterns, "a.example.com"))
	require.False(t, GatewayHTTP1HostMatches(patterns, "notexample.com"))
}

func TestSettingService_GatewayHTTP1HostsRoundTrip(t *testing.T) {
	InvalidateGatewayHTTP1HostsCacheForTest()
	t.Cleanup(InvalidateGatewayHTTP1HostsCacheForTest)

	repo := &gatewayHTTP1SettingRepo{data: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	got, err := svc.GetGatewayHTTP1Hosts(context.Background())
	require.NoError(t, err)
	require.Empty(t, got)
	require.False(t, svc.ForceHTTP1(context.Background(), "api.inferhub.dev"))

	saved, err := svc.SetGatewayHTTP1Hosts(context.Background(), []string{"https://api.inferhub.dev/v1"})
	require.NoError(t, err)
	require.Equal(t, []string{"api.inferhub.dev"}, saved)
	require.True(t, svc.ForceHTTP1(context.Background(), "api.inferhub.dev"))
	require.False(t, svc.ForceHTTP1(context.Background(), "api.openai.com"))
}

type gatewayHTTP1SettingRepo struct {
	data map[string]string
}

func (r *gatewayHTTP1SettingRepo) Get(_ context.Context, key string) (*Setting, error) {
	if value, ok := r.data[key]; ok {
		return &Setting{Key: key, Value: value}, nil
	}
	return nil, ErrSettingNotFound
}

func (r *gatewayHTTP1SettingRepo) GetValue(ctx context.Context, key string) (string, error) {
	setting, err := r.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (r *gatewayHTTP1SettingRepo) Set(_ context.Context, key, value string) error {
	if r.data == nil {
		r.data = map[string]string{}
	}
	r.data[key] = value
	return nil
}

func (r *gatewayHTTP1SettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if value, ok := r.data[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *gatewayHTTP1SettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		_ = r.Set(context.Background(), key, value)
	}
	return nil
}

func (r *gatewayHTTP1SettingRepo) GetAll(_ context.Context) (map[string]string, error) {
	out := map[string]string{}
	for key, value := range r.data {
		out[key] = value
	}
	return out, nil
}

func (r *gatewayHTTP1SettingRepo) Delete(_ context.Context, key string) error {
	delete(r.data, key)
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
