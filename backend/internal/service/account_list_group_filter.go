package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const maxAccountListGroupIDs = 200

type accountListGroupFilterKey struct{}

// AccountListGroupFilter is an admin account-list view filter.
// It does not change an account's stored platform.
type AccountListGroupFilter struct {
	Include   []int64
	Exclude   []int64
	MatchNone bool
}

// WithAccountListGroupFilter attaches a group-membership filter to a list query.
func WithAccountListGroupFilter(ctx context.Context, filter AccountListGroupFilter) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, accountListGroupFilterKey{}, filter)
}

// AccountListGroupFilterFrom returns the list filter stored by WithAccountListGroupFilter.
func AccountListGroupFilterFrom(ctx context.Context) (AccountListGroupFilter, bool) {
	if ctx == nil {
		return AccountListGroupFilter{}, false
	}
	filter, ok := ctx.Value(accountListGroupFilterKey{}).(AccountListGroupFilter)
	return filter, ok
}

// ParseAccountGroupIDList parses a comma-separated admin list of group ids.
// "0" is the explicit empty set used when a view has no matching groups.
func ParseAccountGroupIDList(raw string) (ids []int64, matchNone bool, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > maxAccountListGroupIDs {
		return nil, false, fmt.Errorf("too many group ids")
	}
	if len(parts) == 1 && strings.TrimSpace(parts[0]) == "0" {
		return nil, true, nil
	}
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		id, convErr := strconv.ParseInt(part, 10, 64)
		if convErr != nil || id <= 0 {
			return nil, false, fmt.Errorf("invalid group id")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, false, nil
}
