package service

import "testing"

func TestNormalizeDomesticCategory(t *testing.T) {
	got, err := NormalizeDomesticCategory("  GLM ")
	if err != nil || got != DomesticCategoryGLM {
		t.Fatalf("got %q %v", got, err)
	}
	got, err = NormalizeDomesticCategory("")
	if err != nil || got != "" {
		t.Fatalf("empty got %q %v", got, err)
	}
	if _, err := NormalizeDomesticCategory("openai"); err == nil {
		t.Fatal("expected invalid category")
	}
}
