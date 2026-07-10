package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIImagesResponseSpoolSpillsToDiskReplaysAndCleansUp(t *testing.T) {
	tempDir := t.TempDir()
	largeImage := strings.Repeat("A", openAIImagesResponseSpoolMetadataStringMaxBytes+1024)
	body := []byte(fmt.Sprintf(
		`{"data":[{"b64_json":"%s","size":"1024x1024"}],"usage":{"input_tokens":11,"output_tokens":22,"output_tokens_details":{"image_tokens":20}}}`,
		largeImage,
	))

	spool, err := spoolOpenAIImagesResponse(bytes.NewReader(body), int64(len(body)+1), 32, tempDir)
	require.NoError(t, err)
	require.NotNil(t, spool)
	require.NotEmpty(t, spool.tempPath)
	_, err = os.Stat(spool.tempPath)
	require.NoError(t, err)

	var replay bytes.Buffer
	written, err := spool.WriteTo(&replay)
	require.NoError(t, err)
	require.Equal(t, int64(len(body)), written)
	require.Equal(t, body, replay.Bytes())

	metadata := spool.MetadataBytes()
	require.True(t, gjson.ValidBytes(metadata))
	require.Less(t, len(metadata), len(body)/2, "large image strings must not be duplicated in memory for accounting")
	usage, ok := extractOpenAIUsageFromJSONBytes(metadata)
	require.True(t, ok)
	require.Equal(t, 11, usage.InputTokens)
	require.Equal(t, 22, usage.OutputTokens)
	require.Equal(t, 20, usage.ImageOutputTokens)
	require.Equal(t, 1, extractOpenAIImageCountFromJSONBytes(metadata))
	require.Equal(t, []string{"1024x1024"}, collectOpenAIResponseImageOutputSizesFromJSONBytes(metadata))

	tempPath := spool.tempPath
	require.NoError(t, spool.Close())
	_, err = os.Stat(tempPath)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, spool.Close(), "cleanup must be idempotent")
}

func TestOpenAIImagesResponseMetadataCollectorTruncatesEscapedOnlyString(t *testing.T) {
	collector := newOpenAIImagesResponseMetadataCollector()
	escapedValue := strings.Repeat(`\\`, openAIImagesResponseSpoolMetadataStringMaxBytes/2+1)
	body := []byte(fmt.Sprintf(`{"data":[{"b64_json":"%s","size":"1024x1024"}]}`, escapedValue))

	collector.Add(body)

	metadata := collector.Bytes()
	require.True(t, gjson.ValidBytes(metadata))
	require.Less(t, len(metadata), len(body)/2, "escaped-only strings must obey the per-value metadata bound")
	require.Equal(t, "__spooled_value_1__", gjson.GetBytes(metadata, "data.0.b64_json").String())
	require.Equal(t, "1024x1024", gjson.GetBytes(metadata, "data.0.size").String())
}

func TestOpenAIImagesResponseSpoolReaderReplaysMemoryAndDisk(t *testing.T) {
	tempDir := t.TempDir()

	for _, tc := range []struct {
		name            string
		memoryThreshold int64
		wantMemory      bool
	}{
		{name: "memory", memoryThreshold: 1024, wantMemory: true},
		{name: "disk", memoryThreshold: 1, wantMemory: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`)
			spool, err := spoolOpenAIImagesResponse(bytes.NewReader(body), 1024, tc.memoryThreshold, tempDir)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, spool.Close()) })

			reader, err := spool.Reader()
			require.NoError(t, err)
			if tc.wantMemory {
				require.IsType(t, &bytes.Reader{}, reader)
			} else {
				require.IsType(t, &io.LimitedReader{}, reader)
			}
			replayed, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.Equal(t, body, replayed)

			reader, err = spool.Reader()
			require.NoError(t, err, "each Reader call must rewind the spool")
			replayed, err = io.ReadAll(reader)
			require.NoError(t, err)
			require.Equal(t, body, replayed)
		})
	}
}

func TestOpenAIImagesResponseSpoolReaderRejectsClosedSpool(t *testing.T) {
	spool, err := spoolOpenAIImagesResponse(strings.NewReader(`{"data":[]}`), 1024, 1024, "")
	require.NoError(t, err)
	require.NoError(t, spool.Close())

	reader, err := spool.Reader()
	require.Nil(t, reader)
	require.ErrorContains(t, err, "closed")
}

func TestOpenAIImagesResponseSpoolReadErrorCleansUpTempFile(t *testing.T) {
	tempDir := t.TempDir()
	expectedErr := errors.New("upstream body interrupted")
	reader := io.MultiReader(
		strings.NewReader(strings.Repeat("x", 128)),
		iotest.ErrReader(expectedErr),
	)

	spool, err := spoolOpenAIImagesResponse(reader, 1024, 16, tempDir)
	require.Nil(t, spool)
	require.ErrorIs(t, err, expectedErr)
	entries, readDirErr := os.ReadDir(tempDir)
	require.NoError(t, readDirErr)
	require.Empty(t, entries)
}

func TestOpenAIImagesResponseSpoolRejectsOverLimitAndCleansUpTempFile(t *testing.T) {
	tempDir := t.TempDir()

	spool, err := spoolOpenAIImagesResponse(strings.NewReader(strings.Repeat("x", 128)), 64, 16, tempDir)
	require.Nil(t, spool)
	require.ErrorIs(t, err, ErrUpstreamResponseBodyTooLarge)
	entries, readDirErr := os.ReadDir(tempDir)
	require.NoError(t, readDirErr)
	require.Empty(t, entries)
}

func TestOpenAIImagesResponseSpoolClassifiesCreateTempFailureAsLocalStorage(t *testing.T) {
	missingDir := filepath.Join(t.TempDir(), "missing", "spool")

	spool, err := spoolOpenAIImagesResponse(strings.NewReader("image response"), 1024, 0, missingDir)

	require.Nil(t, spool)
	require.ErrorIs(t, err, errOpenAIImagesResponseSpoolStorage)
}

func TestOpenAIImagesResponseSpoolClassifiesWriteFailureAsLocalStorage(t *testing.T) {
	directory := t.TempDir()
	readOnlyPath := filepath.Join(directory, "read-only.spool")
	require.NoError(t, os.WriteFile(readOnlyPath, nil, 0o600))

	spool, err := spoolOpenAIImagesResponseWithCreateTemp(
		strings.NewReader("image response"),
		1024,
		0,
		directory,
		func(string, string) (*os.File, error) {
			return os.Open(readOnlyPath)
		},
	)

	require.Nil(t, spool)
	require.ErrorIs(t, err, errOpenAIImagesResponseSpoolStorage)
}

func TestOpenAIImagesResponseSpoolConcurrentIsolationAndCleanup(t *testing.T) {
	tempDir := t.TempDir()
	const workers = 16

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			body := []byte(fmt.Sprintf(`{"worker":%d,"data":"%s"}`, worker, strings.Repeat(fmt.Sprintf("%02d", worker), 64)))
			spool, err := spoolOpenAIImagesResponse(bytes.NewReader(body), 4096, 8, tempDir)
			if err != nil {
				errs <- fmt.Errorf("worker %d create spool: %w", worker, err)
				return
			}
			var replay bytes.Buffer
			if _, err := spool.WriteTo(&replay); err != nil {
				_ = spool.Close()
				errs <- fmt.Errorf("worker %d replay: %w", worker, err)
				return
			}
			if !bytes.Equal(body, replay.Bytes()) {
				_ = spool.Close()
				errs <- fmt.Errorf("worker %d replay mismatch", worker)
				return
			}
			if err := spool.Close(); err != nil {
				errs <- fmt.Errorf("worker %d cleanup: %w", worker, err)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestOpenAIImagesResponseSpoolSweepRemovesOnlyStaleOwnedFiles(t *testing.T) {
	directory := t.TempDir()
	now := time.Now()
	stale := filepath.Join(directory, openAIImagesResponseSpoolFilePrefix+"stale"+openAIImagesResponseSpoolFileSuffix)
	fresh := filepath.Join(directory, openAIImagesResponseSpoolFilePrefix+"fresh"+openAIImagesResponseSpoolFileSuffix)
	unrelated := filepath.Join(directory, "unrelated.spool")
	for _, path := range []string{stale, fresh, unrelated} {
		require.NoError(t, os.WriteFile(path, []byte("temporary data"), 0o600))
	}
	staleAt := now.Add(-openAIImagesResponseSpoolOrphanMaxAge - time.Minute)
	require.NoError(t, os.Chtimes(stale, staleAt, staleAt))
	require.NoError(t, os.Chtimes(unrelated, staleAt, staleAt))

	sweepOpenAIImagesResponseSpoolOrphans(directory, now)

	require.NoFileExists(t, stale)
	require.FileExists(t, fresh)
	require.FileExists(t, unrelated)
}
