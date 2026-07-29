package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TestApplyCodexOAuthTransform_PreservesTextFormat proves the OAuth CC→Responses
// path does not strip text.format after conversion. This is the secondary
// drop risk after ChatCompletionsToResponses maps response_format.
func TestApplyCodexOAuthTransform_PreservesTextFormat(t *testing.T) {
	strict := true
	chatReq := &apicompat.ChatCompletionsRequest{
		Model: "gpt-5.6-sol",
		Messages: []apicompat.ChatMessage{
			{Role: "user", Content: json.RawMessage(`"schema probe"`)},
		},
		Stream: true,
		ResponseFormat: &apicompat.ChatResponseFormat{
			Type: "json_schema",
			JSONSchema: &apicompat.ChatResponseFormatJSONSchema{
				Name:   "hvoy_schema_probe",
				Strict: &strict,
				Schema: json.RawMessage(`{
					"type":"object",
					"properties":{
						"nonce":{"type":"string"},
						"result":{
							"type":"object",
							"properties":{
								"sum":{"type":"integer"},
								"scaled":{"type":"integer"},
								"difference":{"type":"integer"},
								"parity":{"type":"string"},
								"ordered":{"type":"array","items":{"type":"integer"}}
							},
							"required":["sum","scaled","difference","parity","ordered"],
							"additionalProperties":false
						}
					},
					"required":["nonce","result"],
					"additionalProperties":false
				}`),
			},
		},
	}

	// Same converter the gateway OAuth chat-completions path uses.
	responsesReq, err := apicompat.ChatCompletionsToResponses(chatReq)
	require.NoError(t, err)
	responsesReq.Model = "gpt-5.6-sol"

	bodyBytes, err := json.Marshal(responsesReq)
	require.NoError(t, err)

	var reqBody map[string]any
	require.NoError(t, json.Unmarshal(bodyBytes, &reqBody))

	// Precondition: conversion already placed text.format.
	require.Contains(t, reqBody, "text")

	applyCodexOAuthTransform(reqBody, false, false)

	// text.format must survive OAuth unsupported-field stripping.
	require.Contains(t, reqBody, "text", "codex OAuth transform must not strip text")
	text, ok := reqBody["text"].(map[string]any)
	require.True(t, ok)
	format, ok := text["format"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "json_schema", format["type"])
	require.Equal(t, "hvoy_schema_probe", format["name"])
	require.Equal(t, true, format["strict"])

	remarshaled, err := json.Marshal(reqBody)
	require.NoError(t, err)
	require.Equal(t, "json_schema", gjson.GetBytes(remarshaled, "text.format.type").String())
	require.Equal(t, "hvoy_schema_probe", gjson.GetBytes(remarshaled, "text.format.name").String())
	require.True(t, gjson.GetBytes(remarshaled, "text.format.strict").Bool())
	require.False(t, gjson.GetBytes(remarshaled, "text.format.schema.additionalProperties").Bool())
	require.Equal(t, "integer", gjson.GetBytes(remarshaled, "text.format.schema.properties.result.properties.sum.type").String())
	// Chat Completions key must not reappear after OAuth transform.
	require.False(t, gjson.GetBytes(remarshaled, "response_format").Exists())
}
