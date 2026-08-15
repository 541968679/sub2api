package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRewriteCodexCompactV2Request_TriggerBecomesInstruction(t *testing.T) {
	body := []byte(`{"model":"m","stream":true,"tools":[{"type":"function","name":"exec"}],"input":[
		{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]},
		{"type":"compaction_trigger"}
	]}`)

	out, changed, err := rewriteCodexCompactV2Request(body)
	require.NoError(t, err)
	require.True(t, changed)

	input := gjson.GetBytes(out, "input").Array()
	require.Len(t, input, 2)
	require.Equal(t, "hello", input[0].Get("content.0.text").String())
	require.Equal(t, "message", input[1].Get("type").String())
	require.Equal(t, "user", input[1].Get("role").String())
	require.Equal(t, codexCompactV2SummaryPrompt, input[1].Get("content.0.text").String())
	require.Equal(t, "none", gjson.GetBytes(out, "tool_choice").String())
	require.True(t, gjson.GetBytes(out, "stream").Bool(), "native path keeps stream; CC path forces unary separately")
}

func TestRewriteCodexCompactV2Request_ReplaysPriorCompaction(t *testing.T) {
	body := []byte(`{"model":"m","input":[
		{"type":"compaction","summary":[{"type":"summary_text","text":"earlier work"}]},
		{"type":"message","role":"user","content":[{"type":"input_text","text":"next"}]},
		{"type":"compaction_trigger"}
	]}`)

	out, changed, err := rewriteCodexCompactV2Request(body)
	require.NoError(t, err)
	require.True(t, changed)
	input := gjson.GetBytes(out, "input").Array()
	require.Len(t, input, 3)
	require.Contains(t, input[0].Get("content.0.text").String(), "<"+codexCompactV2SummaryTag+">")
	require.Contains(t, input[0].Get("content.0.text").String(), "earlier work")
}

func TestRewriteCodexCompactV2Request_ReplaysPrefixedEncryptedContent(t *testing.T) {
	body := []byte(`{"model":"m","input":[
		{"type":"compaction","encrypted_content":"` + encodeCodexCompactV2EncryptedContent("kept via blob") + `"},
		{"type":"compaction_trigger"}
	]}`)

	out, changed, err := rewriteCodexCompactV2Request(body)
	require.NoError(t, err)
	require.True(t, changed)
	input := gjson.GetBytes(out, "input").Array()
	require.Len(t, input, 2)
	require.Contains(t, input[0].Get("content.0.text").String(), "kept via blob")
	require.Contains(t, input[0].Get("content.0.text").String(), "<"+codexCompactV2SummaryTag+">")
}

func TestRewriteCodexCompactV2Request_LeavesForeignEncryptedContent(t *testing.T) {
	body := []byte(`{"model":"m","input":[
		{"type":"compaction","encrypted_content":"gAAA-official-looking-blob"},
		{"type":"compaction_trigger"}
	]}`)

	out, changed, err := rewriteCodexCompactV2Request(body)
	require.NoError(t, err)
	require.True(t, changed)
	input := gjson.GetBytes(out, "input").Array()
	require.Len(t, input, 2)
	require.Equal(t, "compaction", input[0].Get("type").String())
	require.Equal(t, "gAAA-official-looking-blob", input[0].Get("encrypted_content").String())
}

func TestRewriteCodexCompactV2Request_NoTriggerUnchanged(t *testing.T) {
	body := []byte(`{"model":"m","stream":true,"input":[{"type":"message","role":"user","content":[]}]}`)
	out, changed, err := rewriteCodexCompactV2Request(body)
	require.NoError(t, err)
	require.False(t, changed)
	require.JSONEq(t, string(body), string(out))
}

func TestDecodeCodexCompactV2EncryptedContent(t *testing.T) {
	require.Equal(t, "kept", decodeCodexCompactV2EncryptedContent(encodeCodexCompactV2EncryptedContent("kept")))
	require.Empty(t, decodeCodexCompactV2EncryptedContent("gAAA-official-looking-blob"))
	require.Empty(t, decodeCodexCompactV2EncryptedContent(""))
	require.Empty(t, decodeCodexCompactV2EncryptedContent(codexCompactV2EncryptedPrefix+"   "))
}

func TestAccountAllowsCodexCompactV2Fallback(t *testing.T) {
	require.False(t, accountAllowsCodexCompactV2Fallback(&Account{Type: AccountTypeOAuth, Platform: PlatformOpenAI}))
	require.False(t, accountAllowsCodexCompactV2Fallback(&Account{Type: AccountTypeAPIKey, Platform: PlatformGrok}))
	require.True(t, accountAllowsCodexCompactV2Fallback(&Account{Type: AccountTypeAPIKey, Platform: PlatformOpenAI}))
}

func TestBuildCodexCompactV2FromChat_SingleItem(t *testing.T) {
	content, err := json.Marshal("summary text")
	require.NoError(t, err)
	resp := &apicompat.ChatCompletionsResponse{
		ID: "chatcmpl-1",
		Choices: []apicompat.ChatChoice{{
			Message: apicompat.ChatMessage{Role: "assistant", Content: content},
		}},
		Usage: &apicompat.ChatUsage{PromptTokens: 10, CompletionTokens: 4, TotalTokens: 14},
	}
	out, err := buildCodexCompactV2FromChat(resp, "gpt-5.4")
	require.NoError(t, err)
	require.Len(t, out.Output, 1)
	require.Equal(t, "compaction", out.Output[0].Type)
	require.Equal(t, "summary text", out.Output[0].Summary[0].Text)
	require.Equal(t, encodeCodexCompactV2EncryptedContent("summary text"), out.Output[0].EncryptedContent)
	require.Equal(t, 10, out.Usage.InputTokens)
}

func TestBuildCodexCompactV2FromChat_EmptyIsError(t *testing.T) {
	_, err := buildCodexCompactV2FromChat(&apicompat.ChatCompletionsResponse{
		Choices: []apicompat.ChatChoice{{Message: apicompat.ChatMessage{Role: "assistant"}}},
	}, "m")
	require.Error(t, err)
}

func TestClassifyCodexCompactV2JSON_PassthroughRealCompaction(t *testing.T) {
	body := []byte(`{"id":"resp_1","output":[{"type":"compaction","encrypted_content":"blob","summary":[{"type":"summary_text","text":"x"}]}]}`)
	class, summary := classifyCodexCompactV2JSON(body)
	require.Equal(t, compactV2Passthrough, class)
	require.Empty(t, summary)
}

func TestClassifyCodexCompactV2JSON_SynthesizeReasoningAndMessage(t *testing.T) {
	body := []byte(`{"id":"resp_1","output":[
		{"type":"reasoning","summary":[{"type":"summary_text","text":"think"}]},
		{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}
	]}`)
	class, summary := classifyCodexCompactV2JSON(body)
	require.Equal(t, compactV2Synthesize, class)
	require.Contains(t, summary, "think")
	require.Contains(t, summary, "hello")
}

func TestClassifyCodexCompactV2SSE_GotZeroFromTwo(t *testing.T) {
	sse := []byte("" +
		"event: response.output_item.done\n" +
		"data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"reasoning\",\"summary\":[{\"type\":\"summary_text\",\"text\":\"think\"}]}}\n\n" +
		"event: response.output_item.done\n" +
		"data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"hi\"}]}}\n\n" +
		"event: response.completed\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"output\":[{\"type\":\"reasoning\"},{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"hi\"}]}]}}\n\n")
	class, summary, id := classifyCodexCompactV2SSE(sse)
	require.Equal(t, compactV2Synthesize, class)
	require.Contains(t, summary, "hi")
	require.Equal(t, "resp_1", id)

	payload, outClass, err := applyCodexCompactV2ToSSE(sse, "m")
	require.NoError(t, err)
	require.Equal(t, compactV2Synthesize, outClass)
	text := string(payload)
	require.Equal(t, 1, strings.Count(text, "event: response.output_item.done"))
	require.Contains(t, text, `"type":"compaction"`)
	require.Contains(t, text, `"encrypted_content":"`+codexCompactV2EncryptedPrefix)
	require.Contains(t, text, "event: response.completed")
	require.NotContains(t, text, `"type":"reasoning"`)
}

func TestClassifyCodexCompactV2SSE_PassthroughContextCompaction(t *testing.T) {
	sse := []byte("data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"context_compaction\"}}\n\n")
	class, _, _ := classifyCodexCompactV2SSE(sse)
	require.Equal(t, compactV2Passthrough, class)
}

func TestApplyCodexCompactV2ToResponsesJSON_SynthesizeOneItem(t *testing.T) {
	body := []byte(`{"id":"resp_1","model":"m","output":[
		{"type":"reasoning","summary":[{"type":"summary_text","text":"think"}]},
		{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}
	]}`)
	out, class, err := applyCodexCompactV2ToResponsesJSON(body, "m", "")
	require.NoError(t, err)
	require.Equal(t, compactV2Synthesize, class)
	require.Equal(t, 1, len(gjson.GetBytes(out, "output").Array()))
	require.Equal(t, "compaction", gjson.GetBytes(out, "output.0.type").String())
	require.Contains(t, gjson.GetBytes(out, "output.0.summary.0.text").String(), "hello")
	enc := gjson.GetBytes(out, "output.0.encrypted_content").String()
	require.True(t, strings.HasPrefix(enc, codexCompactV2EncryptedPrefix), enc)
	require.Contains(t, enc, "hello")
}

func TestPrepareCodexCompactV2Fallback_Gates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{}
	body := []byte(`{"input":[{"type":"compaction_trigger"}]}`)

	oauthCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	oauthPrep := svc.prepareCodexCompactV2Fallback(context.Background(), oauthCtx, &Account{Type: AccountTypeOAuth, Platform: PlatformOpenAI}, body)
	require.False(t, oauthPrep.Applied)
	require.False(t, oauthPrep.SynthesizeResponse)
	require.False(t, isCodexCompactV2FallbackMarked(oauthCtx))

	grokCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	grokPrep := svc.prepareCodexCompactV2Fallback(context.Background(), grokCtx, &Account{Type: AccountTypeAPIKey, Platform: PlatformGrok}, body)
	require.False(t, grokPrep.Applied)
	require.False(t, isCodexCompactV2FallbackMarked(grokCtx))

	keyCtx, _ := gin.CreateTestContext(httptest.NewRecorder())
	keyPrep := svc.prepareCodexCompactV2Fallback(context.Background(), keyCtx, &Account{Type: AccountTypeAPIKey, Platform: PlatformOpenAI}, body)
	require.True(t, keyPrep.Applied)
	require.True(t, keyPrep.SynthesizeResponse)
	require.True(t, isCodexCompactV2FallbackMarked(keyCtx))
	require.Equal(t, "message", gjson.GetBytes(keyPrep.Body, "input.0.type").String())
}

func TestWriteMarkedCodexCompactV2Stream_SynthesizesOneItem(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	markCodexCompactV2Fallback(c)
	sse := []byte("" +
		"event: response.output_item.done\n" +
		"data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"reasoning\",\"summary\":[{\"type\":\"summary_text\",\"text\":\"think\"}]}}\n\n" +
		"event: response.output_item.done\n" +
		"data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"hi\"}]}}\n\n")
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(sse))}
	svc := &OpenAIGatewayService{}
	_, _, handled := svc.writeMarkedCodexCompactV2Stream(c, &Account{Type: AccountTypeAPIKey, Platform: PlatformOpenAI}, resp, "m", "test")
	require.True(t, handled)
	text := rec.Body.String()
	require.Equal(t, 1, strings.Count(text, "event: response.output_item.done"))
	require.Contains(t, text, `"type":"compaction"`)
	require.Contains(t, text, `"encrypted_content":"`+codexCompactV2EncryptedPrefix)
	require.NotContains(t, text, `"type":"reasoning"`)
}

func TestWriteMarkedCodexCompactV2Stream_UnmarkedPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`{"output":[]}`)))}
	svc := &OpenAIGatewayService{}
	_, _, handled := svc.writeMarkedCodexCompactV2Stream(c, &Account{Type: AccountTypeAPIKey}, resp, "m", "test")
	require.False(t, handled)
}

func TestHasCodexCompactV2InputSignal(t *testing.T) {
	hasTrigger, hasHistory := hasCodexCompactV2InputSignal([]byte(`{"input":[{"type":"compaction_trigger"}]}`))
	require.True(t, hasTrigger)
	require.False(t, hasHistory)
	hasTrigger, hasHistory = hasCodexCompactV2InputSignal([]byte(`{"input":[{"type":"compaction","summary":[]}]}`))
	require.False(t, hasTrigger)
	require.True(t, hasHistory)
}
