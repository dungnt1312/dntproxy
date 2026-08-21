package commandcode

import (
	"encoding/json"
	"strings"
)

func extractSystem(msgs []chatMessage) (string, []chatMessage) {
	var system strings.Builder
	rest := make([]chatMessage, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "system" {
			if system.Len() > 0 {
				system.WriteByte('\n')
			}
			system.WriteString(contentToString(m.Content))
			continue
		}
		rest = append(rest, m)
	}
	return system.String(), rest
}

func convertMessages(msgs []chatMessage) []ccMessage {
	out := make([]ccMessage, 0, len(msgs))
	toolNames := map[string]string{}
	for _, m := range msgs {
		for _, tc := range m.ToolCalls {
			if tc.ID != "" && tc.Function.Name != "" {
				toolNames[tc.ID] = tc.Function.Name
			}
		}
		switch {
		case m.Role == "tool":
			out = append(out, convertToolResult(m, toolNames))
		case m.Role == "assistant" && len(m.ToolCalls) > 0:
			out = append(out, convertAssistantTools(m, toolNames))
		default:
			out = append(out, ccMessage{Role: m.Role, Content: parseContent(m.Content)})
		}
	}
	return out
}

func convertToolResult(m chatMessage, toolNames map[string]string) ccMessage {
	name := m.Name
	if name == "" {
		name = toolNames[m.ToolCallID]
	}
	if name == "" {
		name = "unknown"
	}
	value := contentToString(m.Content)
	outType := "text"
	if strings.HasPrefix(value, "Error:") {
		outType = "error-text"
	}
	return ccMessage{
		Role: "tool",
		Content: []ccContentPart{{
			Type:       "tool-result",
			ToolCallID: strPtr(m.ToolCallID),
			ToolName:   strPtr(name),
			Output:     &ccToolOutput{Type: outType, Value: value},
		}},
	}
}

func convertAssistantTools(m chatMessage, toolNames map[string]string) ccMessage {
	parts := parseContent(m.Content)
	seen := map[string]bool{}
	for _, p := range parts {
		if p.Type == "tool-call" && p.ToolCallID != nil {
			seen[*p.ToolCallID] = true
		}
	}
	for _, tc := range m.ToolCalls {
		if seen[tc.ID] {
			continue
		}
		parts = append(parts, ccContentPart{
			Type:       "tool-call",
			ToolCallID: strPtr(tc.ID),
			ToolName:   strPtr(tc.Function.Name),
			Input:      parseToolInput(tc.Function.Arguments),
		})
		if tc.ID != "" {
			toolNames[tc.ID] = tc.Function.Name
		}
	}
	return ccMessage{Role: "assistant", Content: parts}
}

func convertTools(openAITools []any) []any {
	if len(openAITools) == 0 {
		return []any{}
	}
	out := make([]any, 0, len(openAITools))
	for _, tool := range openAITools {
		toolMap, ok := tool.(map[string]any)
		if !ok {
			continue
		}
		if typ, _ := toolMap["type"].(string); typ != "function" {
			out = append(out, toolMap)
			continue
		}
		fn, ok := toolMap["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := fn["name"].(string)
		if name == "" {
			continue
		}
		schema, _ := fn["parameters"].(map[string]any)
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		ccTool := map[string]any{"name": name, "input_schema": schema}
		if desc, ok := fn["description"].(string); ok && desc != "" {
			ccTool["description"] = desc
		}
		out = append(out, ccTool)
	}
	return out
}

func parseToolInput(arguments string) any {
	if arguments == "" {
		return map[string]any{}
	}
	var input any
	if err := json.Unmarshal([]byte(arguments), &input); err != nil {
		return map[string]any{"arguments": arguments}
	}
	return input
}

func parseContent(content any) []ccContentPart {
	switch v := content.(type) {
	case nil:
		return nil
	case string:
		if v == "" {
			return nil
		}
		return []ccContentPart{{Type: "text", Text: strPtr(v)}}
	case []any:
		var parts []ccContentPart
		for _, item := range v {
			part, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if text := contentPartText(part); text != "" {
				parts = append(parts, ccContentPart{Type: "text", Text: strPtr(text)})
			}
		}
		return parts
	default:
		if s := contentToString(v); s != "" {
			return []ccContentPart{{Type: "text", Text: strPtr(s)}}
		}
		return nil
	}
}

func contentToString(content any) string {
	switch v := content.(type) {
	case nil:
		return ""
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				b.WriteString(contentPartText(m))
			}
		}
		return b.String()
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(data)
	}
}

func contentPartText(part map[string]any) string {
	for _, key := range []string{"text", "content", "output_text", "input_text"} {
		if text, ok := part[key].(string); ok && text != "" {
			return text
		}
	}
	if img, ok := part["image_url"].(map[string]any); ok {
		if url, ok := img["url"].(string); ok {
			return "[Image URL: " + url + "]"
		}
	}
	return ""
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
