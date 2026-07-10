package service

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	openAIImagesResponseSpoolMemoryThreshold          int64 = 8 << 20
	openAIImagesResponseSpoolMetadataMaxBytes               = 4 << 20
	openAIImagesResponseSpoolMetadataStringMaxBytes         = 64 << 10
	openAIImagesResponseSpoolBufferSize                     = 32 << 10
	openAIImagesResponseSpoolMaxConsecutiveEmptyReads       = 100
	openAIImagesResponseSpoolFilePrefix                     = "sub2api-openai-images-"
	openAIImagesResponseSpoolFileSuffix                     = ".spool"
	openAIImagesResponseSpoolOrphanMaxAge                   = 24 * time.Hour
)

var errOpenAIImagesResponseSpoolStorage = errors.New("local image response spool storage failed")

var errOpenAIImagesResponseDelivery = errors.New("local image response delivery failed")

func IsOpenAIImagesResponseDeliveryError(err error) bool {
	return errors.Is(err, errOpenAIImagesResponseDelivery)
}

func NewOpenAIImagesResponseDeliveryErrorForTest(cause error) error {
	return fmt.Errorf("%w: %v", errOpenAIImagesResponseDelivery, cause)
}

type openAIImagesResponseSpoolCreateTempFunc func(dir, pattern string) (*os.File, error)

type openAIImagesResponseSpool struct {
	memory   *bytes.Buffer
	file     *os.File
	tempPath string
	size     int64
	metadata *openAIImagesResponseMetadataCollector
}

func spoolOpenAIImagesResponse(reader io.Reader, maxBytes, memoryThreshold int64, tempDir string) (*openAIImagesResponseSpool, error) {
	return spoolOpenAIImagesResponseWithCreateTemp(reader, maxBytes, memoryThreshold, tempDir, os.CreateTemp)
}

func spoolOpenAIImagesResponseWithCreateTemp(
	reader io.Reader,
	maxBytes,
	memoryThreshold int64,
	tempDir string,
	createTemp openAIImagesResponseSpoolCreateTempFunc,
) (*openAIImagesResponseSpool, error) {
	if reader == nil {
		return nil, errors.New("response body is nil")
	}
	if createTemp == nil {
		return nil, fmt.Errorf("%w: temp-file creator is nil", errOpenAIImagesResponseSpoolStorage)
	}
	if maxBytes <= 0 {
		maxBytes = defaultUpstreamResponseReadMaxBytes
	}
	if memoryThreshold < 0 {
		memoryThreshold = 0
	}
	if memoryThreshold > maxBytes {
		memoryThreshold = maxBytes
	}

	spool := &openAIImagesResponseSpool{memory: &bytes.Buffer{}}
	buffer := make([]byte, openAIImagesResponseSpoolBufferSize)
	emptyReads := 0
	for {
		n, readErr := reader.Read(buffer)
		if n > 0 {
			emptyReads = 0
			if spool.size+int64(n) > maxBytes {
				return nil, spool.cleanupAfterError(fmt.Errorf("%w: limit=%d", ErrUpstreamResponseBodyTooLarge, maxBytes))
			}
			if err := spool.writeChunk(buffer[:n], memoryThreshold, tempDir, createTemp); err != nil {
				return nil, spool.cleanupAfterError(err)
			}
			spool.size += int64(n)
		} else if readErr == nil {
			emptyReads++
			if emptyReads >= openAIImagesResponseSpoolMaxConsecutiveEmptyReads {
				return nil, spool.cleanupAfterError(io.ErrNoProgress)
			}
		}

		if readErr == io.EOF {
			return spool, nil
		}
		if readErr != nil {
			return nil, spool.cleanupAfterError(readErr)
		}
	}
}

func (s *openAIImagesResponseSpool) writeChunk(
	chunk []byte,
	memoryThreshold int64,
	tempDir string,
	createTemp openAIImagesResponseSpoolCreateTempFunc,
) error {
	if s.file == nil && s.size+int64(len(chunk)) <= memoryThreshold {
		_, err := s.memory.Write(chunk)
		return err
	}
	if s.file == nil {
		file, err := createTemp(tempDir, openAIImagesResponseSpoolFilePrefix+"*"+openAIImagesResponseSpoolFileSuffix)
		if err != nil {
			return fmt.Errorf("%w: create image response spool: %v", errOpenAIImagesResponseSpoolStorage, err)
		}
		s.file = file
		s.tempPath = file.Name()
		s.metadata = newOpenAIImagesResponseMetadataCollector()
		if s.memory != nil && s.memory.Len() > 0 {
			memoryBytes := s.memory.Bytes()
			if err := writeAll(s.file, memoryBytes); err != nil {
				return fmt.Errorf("%w: write image response spool prefix: %v", errOpenAIImagesResponseSpoolStorage, err)
			}
			s.metadata.Add(memoryBytes)
		}
		s.memory = nil
	}
	if err := writeAll(s.file, chunk); err != nil {
		return fmt.Errorf("%w: write image response spool: %v", errOpenAIImagesResponseSpoolStorage, err)
	}
	s.metadata.Add(chunk)
	return nil
}

func sweepOpenAIImagesResponseSpoolOrphans(directory string, now time.Time) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		directory = os.TempDir()
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return
	}
	cutoff := now.Add(-openAIImagesResponseSpoolOrphanMaxAge)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), openAIImagesResponseSpoolFilePrefix) || !strings.HasSuffix(entry.Name(), openAIImagesResponseSpoolFileSuffix) {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(directory, entry.Name()))
	}
}

func (s *openAIImagesResponseSpool) WriteTo(writer io.Writer) (int64, error) {
	if s == nil {
		return 0, errors.New("image response spool is nil")
	}
	if writer == nil {
		return 0, errors.New("image response writer is nil")
	}
	if s.file == nil {
		if s.memory == nil {
			return 0, errors.New("image response spool is closed")
		}
		return io.Copy(writer, bytes.NewReader(s.memory.Bytes()))
	}
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return 0, fmt.Errorf("rewind image response spool: %w", err)
	}
	written, err := io.Copy(writer, io.LimitReader(s.file, s.size))
	if err != nil {
		return written, err
	}
	if written != s.size {
		return written, io.ErrUnexpectedEOF
	}
	return written, nil
}

func (s *openAIImagesResponseSpool) Reader() (io.Reader, error) {
	if s == nil {
		return nil, errors.New("image response spool is nil")
	}
	if s.file == nil {
		if s.memory == nil {
			return nil, errors.New("image response spool is closed")
		}
		return bytes.NewReader(s.memory.Bytes()), nil
	}
	if _, err := s.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind image response spool: %w", err)
	}
	return io.LimitReader(s.file, s.size), nil
}

func (s *openAIImagesResponseSpool) MetadataBytes() []byte {
	if s == nil {
		return nil
	}
	if s.file == nil {
		if s.memory == nil {
			return nil
		}
		return s.memory.Bytes()
	}
	if s.metadata == nil {
		return nil
	}
	return s.metadata.Bytes()
}

func (s *openAIImagesResponseSpool) Close() error {
	if s == nil {
		return nil
	}
	file := s.file
	tempPath := s.tempPath
	s.file = nil
	s.tempPath = ""
	s.memory = nil
	s.metadata = nil

	var closeErr error
	if file != nil {
		closeErr = file.Close()
	}
	var removeErr error
	if tempPath != "" {
		if err := os.Remove(tempPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			removeErr = err
		}
	}
	return errors.Join(closeErr, removeErr)
}

func (s *openAIImagesResponseSpool) cleanupAfterError(cause error) error {
	if cleanupErr := s.Close(); cleanupErr != nil {
		return errors.Join(cause, cleanupErr)
	}
	return cause
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

type openAIImagesResponseMetadataCollector struct {
	buffer          bytes.Buffer
	disabled        bool
	inString        bool
	escaped         bool
	stringTruncated bool
	stringStart     int
	stringBytes     int
	truncatedValues int
}

func newOpenAIImagesResponseMetadataCollector() *openAIImagesResponseMetadataCollector {
	return &openAIImagesResponseMetadataCollector{}
}

func (c *openAIImagesResponseMetadataCollector) Add(data []byte) {
	if c == nil || c.disabled {
		return
	}
	for _, b := range data {
		c.addByte(b)
		if c.disabled {
			return
		}
	}
}

func (c *openAIImagesResponseMetadataCollector) addByte(b byte) {
	if !c.inString {
		c.appendByte(b)
		if !c.disabled && b == '"' {
			c.inString = true
			c.escaped = false
			c.stringTruncated = false
			c.stringStart = c.buffer.Len()
			c.stringBytes = 0
		}
		return
	}

	if c.stringTruncated {
		if c.escaped {
			c.escaped = false
			return
		}
		switch b {
		case '\\':
			c.escaped = true
		case '"':
			c.appendByte(b)
			c.inString = false
			c.stringTruncated = false
		}
		return
	}

	c.appendByte(b)
	if c.disabled {
		return
	}
	if c.escaped {
		c.escaped = false
		c.stringBytes++
	} else if b == '\\' {
		c.escaped = true
		c.stringBytes++
	} else if b == '"' {
		c.inString = false
		return
	} else {
		c.stringBytes++
	}
	if c.stringBytes <= openAIImagesResponseSpoolMetadataStringMaxBytes {
		return
	}

	c.buffer.Truncate(c.stringStart)
	c.truncatedValues++
	c.appendString(fmt.Sprintf("__spooled_value_%d__", c.truncatedValues))
	c.stringTruncated = true
}

func (c *openAIImagesResponseMetadataCollector) appendByte(b byte) {
	if c.disabled {
		return
	}
	if c.buffer.Len()+1 > openAIImagesResponseSpoolMetadataMaxBytes {
		c.disable()
		return
	}
	_ = c.buffer.WriteByte(b)
}

func (c *openAIImagesResponseMetadataCollector) appendString(value string) {
	if c.disabled {
		return
	}
	if c.buffer.Len()+len(value) > openAIImagesResponseSpoolMetadataMaxBytes {
		c.disable()
		return
	}
	_, _ = c.buffer.WriteString(value)
}

func (c *openAIImagesResponseMetadataCollector) disable() {
	c.disabled = true
	c.buffer.Reset()
}

func (c *openAIImagesResponseMetadataCollector) Bytes() []byte {
	if c == nil || c.disabled {
		return nil
	}
	return c.buffer.Bytes()
}
