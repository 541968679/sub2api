package loadtest

import (
	"testing"
)

func TestPresetTiersSmokeAndSLA(t *testing.T) {
	smoke := PresetTiers("smoke")
	if len(smoke) != 1 || smoke[0].InputTokens != 80 || smoke[0].Count != 40 {
		t.Fatalf("smoke preset = %#v", smoke)
	}
	sla := PresetTiers("user363-sla")
	want := []Tier{
		{InputTokens: 50000, Count: 50},
		{InputTokens: 80000, Count: 38},
		{InputTokens: 160000, Count: 10},
		{InputTokens: 380000, Count: 2},
	}
	if len(sla) != len(want) {
		t.Fatalf("sla len=%d want %d", len(sla), len(want))
	}
	for i := range want {
		if sla[i] != want[i] {
			t.Fatalf("sla[%d]=%#v want %#v", i, sla[i], want[i])
		}
	}
}

func TestPresetTiersStreamSums100(t *testing.T) {
	tiers := PresetTiers("user363-stream")
	sum := 0
	for _, tier := range tiers {
		if tier.Count <= 0 || tier.InputTokens <= 0 {
			t.Fatalf("invalid tier %#v", tier)
		}
		sum += tier.Count
	}
	if sum != 100 {
		t.Fatalf("stream preset sum=%d want 100 (%#v)", sum, tiers)
	}
	again := PresetTiers("user363-stream")
	if len(again) != len(tiers) {
		t.Fatal("not deterministic length")
	}
	for i := range tiers {
		if again[i] != tiers[i] {
			t.Fatalf("not deterministic at %d", i)
		}
	}
}

func TestPresetTiersUser363MixedSums100(t *testing.T) {
	tiers := PresetTiers("user363")
	sum := 0
	for _, tier := range tiers {
		sum += tier.Count
	}
	if sum != 100 {
		t.Fatalf("mixed sum=%d want 100 (%#v)", sum, tiers)
	}
}

func TestFormatInputK(t *testing.T) {
	cases := map[int]string{80: "80", 4000: "4K", 50000: "50K", 80000: "80K", 160000: "160K"}
	for in, want := range cases {
		if got := FormatInputK(in); got != want {
			t.Fatalf("FormatInputK(%d)=%q want %q", in, got, want)
		}
	}
}
