package service

import (
	"errors"
	"strings"
)

const (
	DomesticCategoryMixed    = "mixed"
	DomesticCategoryDeepseek = "deepseek"
	DomesticCategoryKimi     = "kimi"
	DomesticCategoryGLM      = "glm"
	DomesticCategoryQwen     = "qwen"
)

func NormalizeDomesticCategory(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", nil
	}
	switch value {
	case DomesticCategoryMixed, DomesticCategoryDeepseek, DomesticCategoryKimi, DomesticCategoryGLM, DomesticCategoryQwen:
		return value, nil
	default:
		return "", errors.New("invalid domestic_category")
	}
}
