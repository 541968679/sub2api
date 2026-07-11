package httputil

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeLenientJSONRequestBody_EscapesControlBytesInsideStrings(t *testing.T) {
	body := []byte("{\"input\":\"hello\x00world\"}")
	got, err := NormalizeLenientJSONRequestBody(body, 1024)
	require.NoError(t, err)
	require.True(t, gjson.ValidBytes(got))
	require.Equal(t, "hello\x00world", gjson.GetBytes(got, "input").String())
}

func TestNormalizeLenientJSONRequestBody_RejectsNormalizedGrowthPastLimit(t *testing.T) {
	body := []byte("{\"x\":\"\x00\"}")
	_, err := NormalizeLenientJSONRequestBody(body, int64(len(body)))
	var maxErr *http.MaxBytesError
	require.True(t, errors.As(err, &maxErr))
}
