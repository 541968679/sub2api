package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStreamImageManualB64JSONDecodesEntriesWithoutReadingResponseAhead(t *testing.T) {
	largeImage := bytes.Repeat([]byte{0xab}, 2<<20)
	secondImage := []byte{0xff, 0xff}
	encodedLarge := base64.StdEncoding.EncodeToString(largeImage)
	// JSON may legally escape slash characters inside a base64 string.
	encodedSecond := `\/\/8=`
	body := []byte(`{"created":1,"data":[{"b64_json":"` + encodedLarge + `"},{"url":"https://images.example/one.png"},{"b64_json":"` + encodedSecond + `"}]}`)
	source := &imageManualCountingReader{reader: bytes.NewReader(body)}

	want := [][]byte{largeImage, secondImage}
	seen := 0
	count, err := streamImageManualB64JSON(source, func(ordinal int, decoded io.Reader) error {
		require.Equal(t, seen, ordinal)
		require.Less(t, source.read, len(body), "the callback must run before the whole gateway response is buffered")
		hash := sha256.New()
		written, copyErr := io.CopyBuffer(hash, decoded, make([]byte, 32<<10))
		require.NoError(t, copyErr)
		require.Equal(t, int64(len(want[ordinal])), written)
		require.Equal(t, sha256.Sum256(want[ordinal]), [sha256.Size]byte(hash.Sum(nil)))
		seen++
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, 2, seen)
}

func TestStreamImageManualB64JSONPropagatesArtifactWriterError(t *testing.T) {
	wantErr := errors.New("artifact disk full")
	body := []byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`)

	count, err := streamImageManualB64JSON(bytes.NewReader(body), func(_ int, decoded io.Reader) error {
		buffer := make([]byte, 2)
		_, _ = decoded.Read(buffer)
		return wantErr
	})

	require.ErrorIs(t, err, wantErr)
	require.Zero(t, count)
}

func TestStreamImageManualB64JSONRejectsTruncatedJSONString(t *testing.T) {
	body := []byte(`{"data":[{"b64_json":"aW1hZ2U=`)

	count, err := streamImageManualB64JSON(bytes.NewReader(body), func(_ int, decoded io.Reader) error {
		_, readErr := io.Copy(io.Discard, decoded)
		return readErr
	})

	require.Error(t, err)
	require.Zero(t, count)
}

type imageManualCountingReader struct {
	reader *bytes.Reader
	read   int
}

func (r *imageManualCountingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.read += n
	return n, err
}
