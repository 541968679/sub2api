package service

import (
	"fmt"
	"strconv"
	"strings"
)

// StickySessionBinding is the session→account pin plus the overflow lag flag.
// overflow=true means this pin was taken because a cheaper admitted peer was
// unavailable or full; the next cheaper eligible peer with headroom may steal
// the pin once. A plain integer Redis value is overflow=false.
type StickySessionBinding struct {
	AccountID int64
	Overflow  bool
}

const stickySessionOverflowSuffix = "|o"

func EncodeStickySessionValue(b StickySessionBinding) string {
	if b.AccountID <= 0 {
		return ""
	}
	raw := strconv.FormatInt(b.AccountID, 10)
	if b.Overflow {
		return raw + stickySessionOverflowSuffix
	}
	return raw
}

func ParseStickySessionValue(raw string) (StickySessionBinding, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return StickySessionBinding{}, fmt.Errorf("empty sticky session value")
	}
	overflow := false
	if id, rest, ok := strings.Cut(raw, "|"); ok {
		raw = id
		overflow = rest == "o"
	}
	accountID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return StickySessionBinding{}, err
	}
	if accountID <= 0 {
		return StickySessionBinding{}, fmt.Errorf("invalid sticky account id")
	}
	return StickySessionBinding{AccountID: accountID, Overflow: overflow}, nil
}
