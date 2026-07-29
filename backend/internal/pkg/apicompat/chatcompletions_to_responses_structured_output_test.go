package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestChatCompletionsToResponses_JSONObjectResponseFormat(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-5.6-sol",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"return json"`)},
		},
		ResponseFormat: &ChatResponseFormat{Type: "json_object"},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Text)
	require.NotNil(t, resp.Text.Format)
	assert.Equal(t, "json_object", resp.Text.Format.Type)
	assert.Empty(t, resp.Text.Format.Name)
	assert.Nil(t, resp.Text.Format.Schema)

	body, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.Equal(t, "json_object", gjson.GetBytes(body, "text.format.type").String())
	assert.False(t, gjson.GetBytes(body, "text.format.schema").Exists())
	assert.False(t, gjson.GetBytes(body, "response_format").Exists(), "Responses body must not keep Chat Completions response_format key")
}

func TestChatCompletionsToResponses_JSONSchemaResponseFormat(t *testing.T) {
	strict := true
	schema := json.RawMessage(`{"type":"object","properties":{"answer":{"type":"integer"}},"required":["answer"],"additionalProperties":false}`)
	req := &ChatCompletionsRequest{
		Model: "gpt-5.6-sol",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"42"`)},
		},
		Stream: true,
		ResponseFormat: &ChatResponseFormat{
			Type: "json_schema",
			JSONSchema: &ChatResponseFormatJSONSchema{
				Name:        "math_answer",
				Description: "A single integer answer",
				Schema:      schema,
				Strict:      &strict,
			},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Text)
	require.NotNil(t, resp.Text.Format)
	assert.Equal(t, "json_schema", resp.Text.Format.Type)
	assert.Equal(t, "math_answer", resp.Text.Format.Name)
	assert.Equal(t, "A single integer answer", resp.Text.Format.Description)
	require.NotNil(t, resp.Text.Format.Strict)
	assert.True(t, *resp.Text.Format.Strict)
	require.JSONEq(t, string(schema), string(resp.Text.Format.Schema))

	// Schema bytes must be a defensive copy (mutation of source must not leak).
	schema[2] = 'X'
	require.JSONEq(t, `{"type":"object","properties":{"answer":{"type":"integer"}},"required":["answer"],"additionalProperties":false}`, string(resp.Text.Format.Schema))

	body, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.Equal(t, "json_schema", gjson.GetBytes(body, "text.format.type").String())
	assert.Equal(t, "math_answer", gjson.GetBytes(body, "text.format.name").String())
	assert.True(t, gjson.GetBytes(body, "text.format.strict").Bool())
	assert.Equal(t, "object", gjson.GetBytes(body, "text.format.schema.type").String())
	assert.Equal(t, "integer", gjson.GetBytes(body, "text.format.schema.properties.answer.type").String())
	assert.False(t, gjson.GetBytes(body, "text.format.schema.additionalProperties").Bool())
	// Flat Responses shape: no nested json_schema key under format.
	assert.False(t, gjson.GetBytes(body, "text.format.json_schema").Exists())
}

func TestChatCompletionsToResponses_JSONSchemaFlatFallback(t *testing.T) {
	strict := true
	req := &ChatCompletionsRequest{
		Model: "gpt-5.6-terra",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
		ResponseFormat: &ChatResponseFormat{
			Type:   "json_schema",
			Name:   "flat_schema",
			Schema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}},"required":["ok"],"additionalProperties":false}`),
			Strict: &strict,
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Text)
	require.NotNil(t, resp.Text.Format)
	assert.Equal(t, "json_schema", resp.Text.Format.Type)
	assert.Equal(t, "flat_schema", resp.Text.Format.Name)
	require.NotNil(t, resp.Text.Format.Strict)
	assert.True(t, *resp.Text.Format.Strict)
	assert.Equal(t, "boolean", gjson.GetBytes(resp.Text.Format.Schema, "properties.ok.type").String())
}

func TestChatCompletionsToResponses_JSONSchemaNestedWinsOverFlat(t *testing.T) {
	strictNested := true
	strictFlat := false
	req := &ChatCompletionsRequest{
		Model: "gpt-5.6-sol",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"x"`)},
		},
		ResponseFormat: &ChatResponseFormat{
			Type:   "json_schema",
			Name:   "flat_name",
			Schema: json.RawMessage(`{"type":"object"}`),
			Strict: &strictFlat,
			JSONSchema: &ChatResponseFormatJSONSchema{
				Name:   "nested_name",
				Schema: json.RawMessage(`{"type":"object","properties":{"v":{"type":"string"}},"required":["v"],"additionalProperties":false}`),
				Strict: &strictNested,
			},
		},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	require.NotNil(t, resp.Text.Format)
	assert.Equal(t, "nested_name", resp.Text.Format.Name)
	require.NotNil(t, resp.Text.Format.Strict)
	assert.True(t, *resp.Text.Format.Strict)
	assert.Equal(t, "string", gjson.GetBytes(resp.Text.Format.Schema, "properties.v.type").String())
}

func TestChatCompletionsToResponses_TextResponseFormatOmitsTextFormat(t *testing.T) {
	req := &ChatCompletionsRequest{
		Model: "gpt-5.6-sol",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"hi"`)},
		},
		ResponseFormat: &ChatResponseFormat{Type: "text"},
	}

	resp, err := ChatCompletionsToResponses(req)
	require.NoError(t, err)
	assert.Nil(t, resp.Text)
}

func TestChatCompletionsToResponses_JSONSchemaMissingNameOrSchema(t *testing.T) {
	t.Run("missing name", func(t *testing.T) {
		_, err := ChatCompletionsToResponses(&ChatCompletionsRequest{
			Model: "gpt-5.6-sol",
			Messages: []ChatMessage{
				{Role: "user", Content: json.RawMessage(`"x"`)},
			},
			ResponseFormat: &ChatResponseFormat{
				Type: "json_schema",
				JSONSchema: &ChatResponseFormatJSONSchema{
					Schema: json.RawMessage(`{"type":"object"}`),
				},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name")
	})

	t.Run("missing schema", func(t *testing.T) {
		_, err := ChatCompletionsToResponses(&ChatCompletionsRequest{
			Model: "gpt-5.6-sol",
			Messages: []ChatMessage{
				{Role: "user", Content: json.RawMessage(`"x"`)},
			},
			ResponseFormat: &ChatResponseFormat{
				Type: "json_schema",
				JSONSchema: &ChatResponseFormatJSONSchema{
					Name: "only_name",
				},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "schema")
	})
}

func TestChatCompletionsToResponses_UnsupportedResponseFormatType(t *testing.T) {
	_, err := ChatCompletionsToResponses(&ChatCompletionsRequest{
		Model: "gpt-5.6-sol",
		Messages: []ChatMessage{
			{Role: "user", Content: json.RawMessage(`"x"`)},
		},
		ResponseFormat: &ChatResponseFormat{Type: "yaml_schema"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported response_format.type")
}

// TestChatCompletionsToResponses_HvoySimSchemaProbe exercises a hvoy-like
// gpt56-schema probe: strict JSON object with exact keys nonce + nested
// result (sum/scaled/difference/parity/ordered), additionalProperties false.
// It drives the real ChatCompletionsToResponses converter (OAuth CC→Responses
// path shared converter) and asserts the constraint is preserved in
// marshaled text.format — the failure mode before the fix was dropping
// response_format entirely.
func TestChatCompletionsToResponses_HvoySimSchemaProbe(t *testing.T) {
	// Mirrors the shape hvoy validates with Qe(): exact key sets only.
	schemaDoc := json.RawMessage(`{
		"type": "object",
		"properties": {
			"nonce": { "type": "string" },
			"result": {
				"type": "object",
				"properties": {
					"sum": { "type": "integer" },
					"scaled": { "type": "integer" },
					"difference": { "type": "integer" },
					"parity": { "type": "string" },
					"ordered": {
						"type": "array",
						"items": { "type": "integer" },
						"minItems": 2,
						"maxItems": 2
					}
				},
				"required": ["sum", "scaled", "difference", "parity", "ordered"],
				"additionalProperties": false
			}
		},
		"required": ["nonce", "result"],
		"additionalProperties": false
	}`)
	strict := true

	// Raw client body as hvoy-style OpenAI Chat Completions would send it
	// (stream + stream_options + response_format nested json_schema).
	rawClientBody := []byte(`{
		"model": "gpt-5.6-sol",
		"stream": true,
		"stream_options": { "include_usage": true },
		"max_completion_tokens": 2048,
		"messages": [
			{"role": "user", "content": "Compute the validation record for nonce=abc and return only JSON."}
		],
		"response_format": {
			"type": "json_schema",
			"json_schema": {
				"name": "hvoy_schema_probe",
				"strict": true,
				"schema": ` + string(schemaDoc) + `
			}
		}
	}`)

	// Unmarshal through the same ChatCompletionsRequest type the gateway uses.
	var chatReq ChatCompletionsRequest
	require.NoError(t, json.Unmarshal(rawClientBody, &chatReq))
	require.NotNil(t, chatReq.ResponseFormat, "precondition: response_format must survive unmarshal into ChatCompletionsRequest")
	assert.Equal(t, "json_schema", chatReq.ResponseFormat.Type)
	require.NotNil(t, chatReq.ResponseFormat.JSONSchema)
	assert.Equal(t, "hvoy_schema_probe", chatReq.ResponseFormat.JSONSchema.Name)
	require.NotNil(t, chatReq.ResponseFormat.JSONSchema.Strict)
	assert.True(t, *chatReq.ResponseFormat.JSONSchema.Strict)

	// Real conversion path used by OAuth ForwardAsChatCompletions.
	responsesReq, err := ChatCompletionsToResponses(&chatReq)
	require.NoError(t, err)
	require.NotNil(t, responsesReq.Text, "text.format must be set so schema is not unconstrained free-form")
	require.NotNil(t, responsesReq.Text.Format)
	assert.Equal(t, "json_schema", responsesReq.Text.Format.Type)
	assert.Equal(t, "hvoy_schema_probe", responsesReq.Text.Format.Name)
	require.NotNil(t, responsesReq.Text.Format.Strict)
	assert.True(t, *responsesReq.Text.Format.Strict)
	assert.Equal(t, strict, *responsesReq.Text.Format.Strict)

	// Marshal as the gateway does before Codex OAuth transform / upstream send.
	upstreamBody, err := json.Marshal(responsesReq)
	require.NoError(t, err)

	// Assertions that would fail before the fix (missing text.format entirely).
	assert.True(t, gjson.GetBytes(upstreamBody, "text.format").Exists(), "converted body must include text.format")
	assert.Equal(t, "json_schema", gjson.GetBytes(upstreamBody, "text.format.type").String())
	assert.Equal(t, "hvoy_schema_probe", gjson.GetBytes(upstreamBody, "text.format.name").String())
	assert.True(t, gjson.GetBytes(upstreamBody, "text.format.strict").Bool())
	assert.False(t, gjson.GetBytes(upstreamBody, "response_format").Exists())

	// Schema payload intact for hvoy-style exact key-set constraints.
	formatSchema := gjson.GetBytes(upstreamBody, "text.format.schema")
	require.True(t, formatSchema.Exists())
	assert.Equal(t, "object", formatSchema.Get("type").String())
	assert.False(t, formatSchema.Get("additionalProperties").Bool())
	assert.Equal(t, `["nonce","result"]`, formatSchema.Get("required").Raw)

	// properties.result
	resultProps := formatSchema.Get("properties.result")
	require.True(t, resultProps.Exists())
	assert.Equal(t, "object", resultProps.Get("type").String())
	assert.False(t, resultProps.Get("additionalProperties").Bool())
	for _, key := range []string{"sum", "scaled", "difference", "parity", "ordered"} {
		assert.Truef(t, resultProps.Get("properties."+key).Exists(), "result.properties.%s must be preserved", key)
	}
	required := resultProps.Get("required").Array()
	require.Len(t, required, 5)
	gotRequired := make([]string, 0, len(required))
	for _, r := range required {
		gotRequired = append(gotRequired, r.String())
	}
	assert.ElementsMatch(t, []string{"sum", "scaled", "difference", "parity", "ordered"}, gotRequired)

	// Simulated pure-JSON adherence check that hvoy runs on the model reply:
	// given a model output matching the schema, parse + exact key-set validation.
	// This does not call the network; it documents the probe contract and proves
	// the converted constraint matches what that contract expects.
	simModelOutput := `{"nonce":"abc","result":{"sum":10,"scaled":20,"difference":2,"parity":"even","ordered":[1,9]}}`
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(simModelOutput), &parsed), "hvoy requires pure JSON body")
	assert.ElementsMatch(t, []string{"nonce", "result"}, mapKeys(parsed))
	result, ok := parsed["result"].(map[string]any)
	require.True(t, ok)
	assert.ElementsMatch(t, []string{"sum", "scaled", "difference", "parity", "ordered"}, mapKeys(result))
	assert.Equal(t, "abc", parsed["nonce"])
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestChatCompletionsRequest_ResponseFormatUnmarshalsFromClientJSON ensures
// the gateway's first hop (json.Unmarshal into ChatCompletionsRequest) no
// longer silently discards response_format — the root cause of schema probe
// failures before this fix.
func TestChatCompletionsRequest_ResponseFormatUnmarshalsFromClientJSON(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-5.6-sol",
		"messages":[{"role":"user","content":"hi"}],
		"response_format":{
			"type":"json_schema",
			"json_schema":{
				"name":"probe",
				"strict":true,
				"schema":{"type":"object","properties":{"n":{"type":"integer"}},"required":["n"],"additionalProperties":false}
			}
		}
	}`)
	var req ChatCompletionsRequest
	require.NoError(t, json.Unmarshal(raw, &req))
	require.NotNil(t, req.ResponseFormat)
	assert.Equal(t, "json_schema", req.ResponseFormat.Type)
	require.NotNil(t, req.ResponseFormat.JSONSchema)
	assert.Equal(t, "probe", req.ResponseFormat.JSONSchema.Name)
	require.NotNil(t, req.ResponseFormat.JSONSchema.Strict)
	assert.True(t, *req.ResponseFormat.JSONSchema.Strict)
	assert.Equal(t, "integer", gjson.GetBytes(req.ResponseFormat.JSONSchema.Schema, "properties.n.type").String())
}
