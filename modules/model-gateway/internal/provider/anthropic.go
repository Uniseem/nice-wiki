package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"nice-wiki/libs/platform"
	"nice-wiki/modules/model-gateway/internal/gen"
	"strings"
)

type MapAnthropicRequestInput struct {
	Messages            []gen.GatewayMessage                 `json:"messages"`
	ModelID             string                               `json:"modelId"`
	MaximumOutputTokens int                                  `json:"maximumOutputTokens"`
	Images              map[string][]byte                    `json:"images,omitempty"`
	Stream              bool                                 `json:"stream"`
	Tools               *[]gen.GatewayTool                   `json:"tools,omitempty"`
	ToolChoice          *gen.CreateInvocationInputToolChoice `json:"toolChoice,omitempty"`
}
type MapAnthropicRequestResult struct {
	Body   string `json:"body"`
	System string `json:"system"`
}

// ImageKey 以已核AssetRef完整身份作内存索引，不接受任意外部URL。
func ImageKey(asset gen.AssetRef) string {
	raw, _ := platform.CanonicalJSON(asset)
	return platform.HashBytes(raw)
}
func ProviderContent(message gen.GatewayMessage, images map[string][]byte, protocol string) ([]map[string]any, error) {
	parts := []map[string]any{{"type": "text", "text": message.Content}}
	if protocol == "gemini" {
		parts = []map[string]any{{"text": message.Content}}
	}
	if message.Parts == nil {
		return parts, nil
	}
	for _, part := range *message.Parts {
		if part.Kind == "text" {
			if protocol == "gemini" {
				parts = append(parts, map[string]any{"text": *part.Text})
			} else {
				parts = append(parts, map[string]any{"type": "text", "text": *part.Text})
			}
			continue
		}
		asset, err := MediaPartAsset(part)
		if err != nil {
			return nil, err
		}
		data, exists := images[ImageKey(asset)]
		if !exists || len(data) != asset.ByteSize || platform.HashBytes(data) != asset.Sha256 {
			return nil, platform.NewError(platform.CodeIntegrityError, false, "媒体资源未受控读取或完整hash不符")
		}
		encoded := base64.StdEncoding.EncodeToString(data)
		if part.Kind == "file" {
			switch protocol {
			case "anthropic":
				parts = append(parts, map[string]any{"type": "document", "source": map[string]any{"type": "base64", "media_type": "application/pdf", "data": encoded}})
			case "gemini":
				parts = append(parts, map[string]any{"inlineData": map[string]any{"mimeType": "application/pdf", "data": encoded}})
			default:
				parts = append(parts, map[string]any{"type": "file", "file": map[string]any{"filename": "document.pdf", "file_data": "data:application/pdf;base64," + encoded}})
			}
		} else {
			switch protocol {
			case "anthropic":
				parts = append(parts, map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": asset.MediaType, "data": encoded}})
			case "gemini":
				parts = append(parts, map[string]any{"inlineData": map[string]any{"mimeType": asset.MediaType, "data": encoded}})
			default:
				parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + asset.MediaType + ";base64," + encoded}})
			}
		}
	}
	return parts, nil
}
func MapAnthropicRequest(ctx context.Context, op OperationContext, input MapAnthropicRequestInput) (MapAnthropicRequestResult, error) {
	systems := make([]string, 0)
	messages := make([]map[string]any, 0)
	for _, message := range input.Messages {
		if message.Role == "system" {
			system := message.Content
			if message.Parts != nil {
				for _, part := range *message.Parts {
					if part.Kind != "text" || part.Text == nil {
						return MapAnthropicRequestResult{}, platform.NewError(platform.CodeUnsupportedSchema, false, "Messages系统段只允许文本")
					}
					system += "\n" + *part.Text
				}
			}
			systems = append(systems, system)
			continue
		}
		content, err := ProviderContent(message, input.Images, "anthropic")
		if err != nil {
			return MapAnthropicRequestResult{}, err
		}
		role := string(message.Role)
		if message.Content == "" && message.Parts == nil {
			content = []map[string]any{}
		}
		if message.ToolCalls != nil {
			for _, call := range *message.ToolCalls {
				arguments, err := decodeToolArguments(call.Arguments)
				if err != nil {
					return MapAnthropicRequestResult{}, err
				}
				content = append(content, map[string]any{"type": "tool_use", "id": call.Id, "name": call.Name, "input": arguments})
			}
		}
		if message.Role == "tool" {
			role = "user"
			content = []map[string]any{{"type": "tool_result", "tool_use_id": *message.ToolCallId, "content": message.Content}}
		}
		messages = appendProviderMessage(messages, role, content)
	}
	system := strings.Join(systems, "\n\n")
	body := map[string]any{"model": input.ModelID, "messages": messages, "max_tokens": input.MaximumOutputTokens, "stream": input.Stream}
	if system != "" {
		body["system"] = system
	}
	addProviderTools(body, input.Tools, input.ToolChoice, "anthropic")
	raw, err := json.Marshal(body)
	if err != nil {
		return MapAnthropicRequestResult{}, platform.NewError(platform.CodeIntegrityError, false, "Messages请求无法编码")
	}
	return MapAnthropicRequestResult{Body: string(raw), System: system}, nil
}
