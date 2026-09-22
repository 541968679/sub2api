package service

import "testing"

func TestKimiRequiresNativeChatCompletions(t *testing.T) {
	if !kimiRequiresNativeChatCompletions(nil, "kimi-k3") {
		t.Fatal("kimi-k3")
	}
	if !kimiRequiresNativeChatCompletions(nil, "moonshot/kimi-k3") {
		t.Fatal("prefixed kimi-k3")
	}
	if !kimiRequiresNativeChatCompletions(&Account{Platform: PlatformKimi}, "gpt-5.4") {
		t.Fatal("kimi platform")
	}
	if kimiRequiresNativeChatCompletions(&Account{Platform: PlatformOpenAI}, "glm-5.3") {
		t.Fatal("glm must stay on the account route")
	}
}
