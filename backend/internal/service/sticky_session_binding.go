package service

import (
	"fmt"
	"net/url"
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
	// Model is set only for Seedance task pins. Ordinary sticky sessions leave it empty,
	// so their Redis value stays "id" or "id|o".
	Model string
}

const stickySessionOverflowSuffix = "|o"

func EncodeStickySessionValue(b StickySessionBinding) string {
	if b.AccountID <= 0 {
		return ""
	}
	raw := strconv.FormatInt(b.AccountID, 10)
	if b.Overflow {
		raw += stickySessionOverflowSuffix
	}
	if model := strings.TrimSpace(b.Model); model != "" {
		raw += "|m=" + url.QueryEscape(model)
	}
	return raw
}

func ParseStickySessionValue(raw string) (StickySessionBinding, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return StickySessionBinding{}, fmt.Errorf("empty sticky session value")
	}
	parts := strings.Split(raw, "|")
	accountID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return StickySessionBinding{}, err
	}
	if accountID <= 0 {
		return StickySessionBinding{}, fmt.Errorf("invalid sticky account id")
	}
	binding := StickySessionBinding{AccountID: accountID}
	for _, part := range parts[1:] {
		switch {
		case part == "o":
			binding.Overflow = true
		case strings.HasPrefix(part, "m="):
			model, unescapeErr := url.QueryUnescape(strings.TrimPrefix(part, "m="))
			if unescapeErr != nil {
				return StickySessionBinding{}, unescapeErr
			}
			binding.Model = model
		}
	}
	return binding, nil
}
