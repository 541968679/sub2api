package loadtest

import (
	"math"
	"sort"
)

// PresetTiers returns the absolute traffic-tier template for a named profile.
// Selecting a profile in Admin fills the tier table from this list.
func PresetTiers(profile string) []Tier {
	switch profile {
	case "smoke":
		return []Tier{{InputTokens: 80, Count: 40}}
	case "general":
		// Balanced short/mid/long mix for generic upstream soak (sum 100).
		return []Tier{
			{InputTokens: 4000, Count: 40},
			{InputTokens: 16000, Count: 30},
			{InputTokens: 32000, Count: 20},
			{InputTokens: 64000, Count: 10},
		}
	case "user363-sla":
		return []Tier{
			{InputTokens: 50000, Count: 50},
			{InputTokens: 80000, Count: 38},
			{InputTokens: 160000, Count: 10},
			{InputTokens: 380000, Count: 2},
		}
	case "user363-stream":
		return normalizeBuckets(streamBuckets, 100)
	case "user363-sync":
		return normalizeBuckets(syncBuckets, 100)
	case "user363":
		syncN := int(math.Round(defaultSyncRatio * 100))
		if syncN < 0 {
			syncN = 0
		}
		if syncN > 100 {
			syncN = 100
		}
		streamN := 100 - syncN
		return mergeTiers(normalizeBuckets(syncBuckets, syncN), normalizeBuckets(streamBuckets, streamN))
	default:
		return nil
	}
}

func normalizeBuckets(buckets []sizeBucket, total int) []Tier {
	if total <= 0 || len(buckets) == 0 {
		return nil
	}
	weightSum := 0
	for _, b := range buckets {
		if b.Weight > 0 {
			weightSum += b.Weight
		}
	}
	if weightSum <= 0 {
		return nil
	}

	type part struct {
		tokens int
		count  int
		frac   float64
		idx    int
	}
	parts := make([]part, 0, len(buckets))
	assigned := 0
	for i, b := range buckets {
		if b.Weight <= 0 || b.Tokens <= 0 {
			continue
		}
		raw := float64(b.Weight) / float64(weightSum) * float64(total)
		count := int(math.Floor(raw))
		parts = append(parts, part{tokens: b.Tokens, count: count, frac: raw - float64(count), idx: i})
		assigned += count
	}
	rem := total - assigned
	if rem > 0 {
		order := append([]part(nil), parts...)
		sort.SliceStable(order, func(i, j int) bool {
			if order[i].frac == order[j].frac {
				return order[i].idx < order[j].idx
			}
			return order[i].frac > order[j].frac
		})
		for i := 0; i < rem && i < len(order); i++ {
			for j := range parts {
				if parts[j].idx == order[i].idx {
					parts[j].count++
					break
				}
			}
		}
	}

	out := make([]Tier, 0, len(parts))
	for _, p := range parts {
		if p.count > 0 {
			out = append(out, Tier{InputTokens: p.tokens, Count: p.count})
		}
	}
	return out
}

func mergeTiers(groups ...[]Tier) []Tier {
	type agg struct {
		tokens int
		count  int
		order  int
	}
	byTok := map[int]*agg{}
	order := 0
	for _, group := range groups {
		for _, tier := range group {
			if tier.InputTokens <= 0 || tier.Count <= 0 {
				continue
			}
			if cur, ok := byTok[tier.InputTokens]; ok {
				cur.count += tier.Count
				continue
			}
			byTok[tier.InputTokens] = &agg{tokens: tier.InputTokens, count: tier.Count, order: order}
			order++
		}
	}
	list := make([]*agg, 0, len(byTok))
	for _, v := range byTok {
		list = append(list, v)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].order < list[j].order })
	out := make([]Tier, 0, len(list))
	for _, v := range list {
		out = append(out, Tier{InputTokens: v.tokens, Count: v.count})
	}
	return out
}
