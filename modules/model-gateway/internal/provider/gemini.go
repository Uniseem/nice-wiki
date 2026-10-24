package provider

import (
	"context"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/gen"
	"strings"
)

type MapGeminiRequestInput struct {
	Messages            []gen.GatewayMessage                 `json:"messages"`
	ModelID             string                               `json:"modelId"`
	MaximumOutputTokens int                                  `json:"maximumOutputTokens"`
	Capability          string                               `json:"capability"`
	Texts               []string                             `json:"texts"`
	Dimensions          int                                  `json:"dimensions"`
	Images              map[string][]byte                    `json:"images,omitempty"`
	Stream              bool                                 `json:"stream"`
	Tools               *[]gen.GatewayTool                   `json:"tools,omitempty"`
	ToolChoice          *gen.CreateInvocationInputToolChoice `json:"toolChoice,omitempty"`
}
type MapGeminiRequestResult struct {
	Body   string `json:"body"`
	Method string `json:"method"`
}

func MapGeminiRequest(ctx context.Context, op OperationContext, input MapGeminiRequestInput) (MapGeminiRequestResult, error) {
	method := "generateContent"
	var body any
	if input.Capability == "embedding" {
		requests := make([]map[string]any, 0, len(input.Texts))
		for _, text := range input.Texts {
			requests = append(requests, map[string]any{"model": "models/" + strings.TrimPrefix(input.ModelID, "models/"), "content": map[string]any{"parts": []map[string]any{{"text": text}}}, "outputDimensionality": input.Dimensions})
		}
		if len(requests) == 1 {
			method = "embedContent"
			body = requests[0]
		} else {
			method = "batchEmbedContents"
			body = map[string]any{"requests": requests}
		}
	} else {
		contents := make([]map[string]any, 0)
		systemParts := make([]map[string]any, 0)
		callNames := map[string]string{}
		for _, message := range input.Messages {
			if message.Role == "system" {
				systemParts = append(systemParts, map[string]any{"text": message.Content})
				if message.Parts != nil {
					for _, part := range *message.Parts {
						if part.Kind != "text" || part.Text == nil {
							return MapGeminiRequestResult{}, platform.NewError(platform.CodeUnsupportedSchema, false, "Gemini系统段只允许文本")
						}
						systemParts = append(systemParts, map[string]any{"text": *part.Text})
					}
				}
				continue
			}
			parts, err := ProviderContent(message, input.Images, "gemini")
			if err != nil {
				return MapGeminiRequestResult{}, err
			}
			role := string(message.Role)
			if message.Content == "" && message.Parts == nil {
				parts = []map[string]any{}
			}
			if role == "assistant" {
				role = "model"
			}
			if message.ToolCalls != nil {
				for _, call := range *message.ToolCalls {
					arguments, err := decodeToolArguments(call.Arguments)
					if err != nil {
						return MapGeminiRequestResult{}, err
					}
					function := map[string]any{"name": call.Name, "args": arguments}
					if !strings.HasPrefix(call.Id, "gemini.call.") {
						function["id"] = call.Id
					}
					part := map[string]any{"functionCall": function}
					if call.Signature != nil {
						part["thoughtSignature"] = *call.Signature
					}
					parts = append(parts, part)
					callNames[call.Id] = call.Name
				}
			}
			if message.Role == "tool" {
				role = "user"
				response := map[string]any{}
				if json.Unmarshal([]byte(message.Content), &response) != nil || response == nil {
					response = map[string]any{"content": message.Content}
				}
				function := map[string]any{"name": callNames[*message.ToolCallId], "response": response}
				if !strings.HasPrefix(*message.ToolCallId, "gemini.call.") {
					function["id"] = *message.ToolCallId
				}
				parts = []map[string]any{{"functionResponse": function}}
			}
			if len(contents) > 0 && contents[len(contents)-1]["role"] == role {
				last := contents[len(contents)-1]
				last["parts"] = append(last["parts"].([]map[string]any), parts...)
				continue
			}
			contents = append(contents, map[string]any{"role": role, "parts": parts})
		}
		payload := map[string]any{"contents": contents, "generationConfig": map[string]any{"maxOutputTokens": input.MaximumOutputTokens}}
		if len(systemParts) > 0 {
			payload["systemInstruction"] = map[string]any{"parts": systemParts}
		}
		addProviderTools(payload, input.Tools, input.ToolChoice, "gemini")
		body = payload
		if input.Stream {
			method = "streamGenerateContent"
		}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return MapGeminiRequestResult{}, platform.NewError(platform.CodeIntegrityError, false, "Gemini请求无法编码")
	}
	return MapGeminiRequestResult{Body: string(raw), Method: method}, nil
}
