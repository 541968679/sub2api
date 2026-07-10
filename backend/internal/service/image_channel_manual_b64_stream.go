package service

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	imageManualB64JSONKey       = "b64_json"
	imageManualURLKey           = "url"
	imageManualStreamBufferSize = 32 << 10
)

type imageManualStreamCounts struct {
	b64JSON int
	url     int
}

// streamImageManualB64JSON finds b64_json values without materializing their
// JSON strings. The callback receives decoded image bytes for each occurrence.
func streamImageManualB64JSON(
	reader io.Reader,
	handle func(ordinal int, decoded io.Reader) error,
) (int, error) {
	ordinal := 0
	counts, err := streamImageManualImageValues(reader, func(key string, _ int, value io.Reader) error {
		if key != imageManualB64JSONKey || handle == nil {
			return nil
		}
		err := handle(ordinal, value)
		ordinal++
		return err
	})
	return counts.b64JSON, err
}

// streamImageManualImageValues scans the complete gateway response once so
// large b64_json and data URL values can be consumed without materializing
// their JSON strings in memory.
func streamImageManualImageValues(
	reader io.Reader,
	handle func(key string, dataIndex int, value io.Reader) error,
) (imageManualStreamCounts, error) {
	if reader == nil {
		return imageManualStreamCounts{}, errors.New("manual gateway response reader is nil")
	}
	parser := imageManualJSONStreamParser{
		reader: bufio.NewReaderSize(reader, imageManualStreamBufferSize),
		handle: handle,
	}
	if err := parser.parseRoot(); err != nil {
		return parser.counts, err
	}
	return parser.counts, nil
}

type imageManualJSONStreamParser struct {
	reader *bufio.Reader
	handle func(key string, dataIndex int, value io.Reader) error
	counts imageManualStreamCounts
}

func (p *imageManualJSONStreamParser) parseRoot() error {
	start, err := readImageManualNextNonSpace(p.reader)
	if err != nil {
		return fmt.Errorf("scan manual gateway response: %w", err)
	}
	if start != '{' {
		return errors.New("manual gateway response must be a JSON object")
	}
	seenData := false
	if err := p.parseObjectMembers(func(key string, valueStart byte) error {
		if key != "data" {
			return p.skipValue(valueStart, 0)
		}
		if seenData {
			return errors.New("manual gateway response contains duplicate data fields")
		}
		seenData = true
		return p.parseDataArray(valueStart)
	}); err != nil {
		return err
	}
	if trailing, err := readImageManualNextNonSpace(p.reader); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("scan manual gateway trailing data: %w", err)
		}
		return fmt.Errorf("manual gateway response contains trailing byte %q", trailing)
	}
	return nil
}

func (p *imageManualJSONStreamParser) parseDataArray(start byte) error {
	if start != '[' {
		return p.skipValue(start, 0)
	}
	next, err := readImageManualNextNonSpace(p.reader)
	if err != nil {
		return fmt.Errorf("scan manual gateway data array: %w", err)
	}
	if next == ']' {
		return nil
	}
	for index := 0; ; index++ {
		if next == '{' {
			if err := p.parseDataEntry(index); err != nil {
				return err
			}
		} else if err := p.skipValue(next, 0); err != nil {
			return err
		}
		separator, err := readImageManualNextNonSpace(p.reader)
		if err != nil {
			return fmt.Errorf("scan manual gateway data separator: %w", err)
		}
		switch separator {
		case ']':
			return nil
		case ',':
			next, err = readImageManualNextNonSpace(p.reader)
			if err != nil {
				return fmt.Errorf("scan manual gateway data entry: %w", err)
			}
		default:
			return fmt.Errorf("invalid manual gateway data separator %q", separator)
		}
	}
}

func (p *imageManualJSONStreamParser) parseDataEntry(dataIndex int) error {
	seen := make(map[string]struct{}, 2)
	return p.parseObjectMembers(func(key string, valueStart byte) error {
		if key != imageManualB64JSONKey && key != imageManualURLKey {
			return p.skipValue(valueStart, 0)
		}
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("manual gateway data entry %d contains duplicate %s", dataIndex, key)
		}
		seen[key] = struct{}{}
		if valueStart != '"' {
			return p.skipValue(valueStart, 0)
		}
		encoded := &imageManualJSONStringReader{reader: p.reader}
		value := io.Reader(encoded)
		if key == imageManualB64JSONKey {
			value = base64.NewDecoder(base64.StdEncoding, encoded)
		}
		if p.handle != nil {
			if err := p.handle(key, dataIndex, value); err != nil {
				return err
			}
		}
		if _, err := io.CopyBuffer(io.Discard, value, make([]byte, imageManualStreamBufferSize)); err != nil {
			return fmt.Errorf("decode manual gateway %s: %w", key, err)
		}
		if err := encoded.finish(); err != nil {
			return fmt.Errorf("finish manual gateway %s string: %w", key, err)
		}
		if key == imageManualB64JSONKey {
			p.counts.b64JSON++
		} else {
			p.counts.url++
		}
		return nil
	})
}

func (p *imageManualJSONStreamParser) parseObjectMembers(handle func(key string, valueStart byte) error) error {
	next, err := readImageManualNextNonSpace(p.reader)
	if err != nil {
		return fmt.Errorf("scan manual gateway object: %w", err)
	}
	if next == '}' {
		return nil
	}
	for {
		if next != '"' {
			return fmt.Errorf("invalid manual gateway object key start %q", next)
		}
		key, err := readImageManualJSONStringCandidate(p.reader, len(imageManualB64JSONKey))
		if err != nil {
			return fmt.Errorf("scan manual gateway object key: %w", err)
		}
		colon, err := readImageManualNextNonSpace(p.reader)
		if err != nil || colon != ':' {
			if err != nil {
				return fmt.Errorf("scan manual gateway object colon: %w", err)
			}
			return fmt.Errorf("invalid manual gateway object colon %q", colon)
		}
		valueStart, err := readImageManualNextNonSpace(p.reader)
		if err != nil {
			return fmt.Errorf("scan manual gateway object value: %w", err)
		}
		if err := handle(key, valueStart); err != nil {
			return err
		}
		separator, err := readImageManualNextNonSpace(p.reader)
		if err != nil {
			return fmt.Errorf("scan manual gateway object separator: %w", err)
		}
		switch separator {
		case '}':
			return nil
		case ',':
			next, err = readImageManualNextNonSpace(p.reader)
			if err != nil {
				return fmt.Errorf("scan manual gateway object key: %w", err)
			}
		default:
			return fmt.Errorf("invalid manual gateway object separator %q", separator)
		}
	}
}

func (p *imageManualJSONStreamParser) skipValue(start byte, depth int) error {
	if depth > 256 {
		return errors.New("manual gateway response nesting is too deep")
	}
	switch start {
	case '"':
		return (&imageManualJSONStringReader{reader: p.reader}).finish()
	case '{':
		return p.parseObjectMembers(func(_ string, valueStart byte) error {
			return p.skipValue(valueStart, depth+1)
		})
	case '[':
		next, err := readImageManualNextNonSpace(p.reader)
		if err != nil {
			return err
		}
		if next == ']' {
			return nil
		}
		for {
			if err := p.skipValue(next, depth+1); err != nil {
				return err
			}
			separator, err := readImageManualNextNonSpace(p.reader)
			if err != nil {
				return err
			}
			switch separator {
			case ']':
				return nil
			case ',':
				next, err = readImageManualNextNonSpace(p.reader)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("invalid manual gateway array separator %q", separator)
			}
		}
	default:
		return p.skipPrimitive()
	}
}

func (p *imageManualJSONStreamParser) skipPrimitive() error {
	for {
		current, err := p.reader.ReadByte()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch current {
		case ',', ']', '}':
			return p.reader.UnreadByte()
		case ' ', '\t', '\r', '\n':
			return nil
		}
	}
}

func readImageManualJSONStringCandidate(reader *bufio.Reader, maxBytes int) (string, error) {
	buffer := make([]byte, 0, maxBytes)
	escaped := false
	validCandidate := true
	for {
		current, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		if escaped {
			escaped = false
			validCandidate = false
			continue
		}
		switch current {
		case '\\':
			escaped = true
			validCandidate = false
		case '"':
			if validCandidate {
				return string(buffer), nil
			}
			return "", nil
		default:
			if current < 0x20 {
				return "", errors.New("unescaped control character in JSON string")
			}
			if validCandidate && len(buffer) < maxBytes {
				buffer = append(buffer, current)
			} else if validCandidate {
				validCandidate = false
			}
		}
	}
}

func readImageManualNextNonSpace(reader *bufio.Reader) (byte, error) {
	for {
		current, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		switch current {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return current, nil
		}
	}
}

type imageManualJSONStringReader struct {
	reader  *bufio.Reader
	pending []byte
	done    bool
}

func (r *imageManualJSONStringReader) Read(target []byte) (int, error) {
	if len(target) == 0 {
		return 0, nil
	}
	if r.done && len(r.pending) == 0 {
		return 0, io.EOF
	}
	written := 0
	for written < len(target) {
		if len(r.pending) > 0 {
			copied := copy(target[written:], r.pending)
			r.pending = r.pending[copied:]
			written += copied
			continue
		}
		if r.done {
			break
		}

		current, err := r.reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			if written > 0 {
				return written, err
			}
			return 0, err
		}
		switch current {
		case '"':
			r.done = true
		case '\\':
			decoded, err := readImageManualJSONEscape(r.reader)
			if err != nil {
				if written > 0 {
					return written, err
				}
				return 0, err
			}
			r.pending = decoded
		default:
			if current < 0x20 {
				err := errors.New("unescaped control character in JSON string")
				if written > 0 {
					return written, err
				}
				return 0, err
			}
			target[written] = current
			written++
		}
	}
	if written > 0 {
		return written, nil
	}
	if r.done {
		return 0, io.EOF
	}
	return 0, nil
}

func (r *imageManualJSONStringReader) finish() error {
	if r == nil || (r.done && len(r.pending) == 0) {
		return nil
	}
	_, err := io.CopyBuffer(io.Discard, r, make([]byte, 4<<10))
	return err
}

func readImageManualJSONEscape(reader *bufio.Reader) ([]byte, error) {
	escape, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	switch escape {
	case '"', '\\', '/':
		return []byte{escape}, nil
	case 'b':
		return []byte{'\b'}, nil
	case 'f':
		return []byte{'\f'}, nil
	case 'n':
		return []byte{'\n'}, nil
	case 'r':
		return []byte{'\r'}, nil
	case 't':
		return []byte{'\t'}, nil
	case 'u':
		return readImageManualJSONUnicodeEscape(reader)
	default:
		return nil, fmt.Errorf("invalid JSON escape %q", escape)
	}
}

func readImageManualJSONUnicodeEscape(reader *bufio.Reader) ([]byte, error) {
	first, err := readImageManualJSONHexRune(reader)
	if err != nil {
		return nil, err
	}
	decoded := rune(first)
	if utf16.IsSurrogate(decoded) {
		prefix, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		marker, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if prefix != '\\' || marker != 'u' {
			return nil, errors.New("invalid JSON surrogate pair")
		}
		second, err := readImageManualJSONHexRune(reader)
		if err != nil {
			return nil, err
		}
		decoded = utf16.DecodeRune(decoded, rune(second))
		if decoded == utf8.RuneError {
			return nil, errors.New("invalid JSON surrogate pair")
		}
	}
	buffer := make([]byte, utf8.RuneLen(decoded))
	utf8.EncodeRune(buffer, decoded)
	return buffer, nil
}

func readImageManualJSONHexRune(reader *bufio.Reader) (uint16, error) {
	buffer := make([]byte, 4)
	if _, err := io.ReadFull(reader, buffer); err != nil {
		return 0, err
	}
	value, err := strconv.ParseUint(string(buffer), 16, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid JSON unicode escape: %w", err)
	}
	return uint16(value), nil
}
