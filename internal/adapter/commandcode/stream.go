package commandcode

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
)

type streamState struct {
	model          string
	id             string
	created        int64
	sentRole       bool
	toolCallIndex  int
	toolCallByID   map[string]int
	promptTokens   int
	completionToks int
}

func newStreamState(model string, created int64) *streamState {
	return &streamState{
		model:        model,
		id:           "chatcmpl-" + uuid.NewString(),
		created:      created,
		toolCallByID: map[string]int{},
	}
}

func translateNDJSON(line string, state *streamState) (string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", false
	}
	var event ccStreamEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		return "", false
	}
	switch event.Type {
	case "text-delta":
		return writeChunk(state, map[string]any{"role": maybeRole(state), "content": event.Text}, nil), false
	case "tool-use":
		return writeToolStart(state, event.ToolCallID, event.ToolName), false
	case "tool-delta":
		return writeToolArgs(state, state.toolCallIndex-1, event.Text), false
	case "tool-input-start":
		return writeToolStart(state, firstNonEmpty(event.ID, event.ToolCallID), event.ToolName), false
	case "tool-input-delta":
		idx := toolIndex(state, firstNonEmpty(event.ID, event.ToolCallID), true)
		return writeToolArgs(state, idx, event.Delta), false
	case "tool-call":
		if _, streamed := state.toolCallByID[event.ToolCallID]; streamed {
			return "", false
		}
		args := ""
		if event.Input != nil {
			if data, err := json.Marshal(event.Input); err == nil {
				args = string(data)
			}
		}
		idx := toolIndex(state, event.ToolCallID, false)
		return writeChunk(state, map[string]any{
			"role": maybeRole(state),
			"tool_calls": []map[string]any{{
				"index": idx,
				"id":    event.ToolCallID,
				"type":  "function",
				"function": map[string]any{
					"name":      event.ToolName,
					"arguments": args,
				},
			}},
		}, nil), false
	case "finish", "finish-step":
		captureUsage(state, event)
		if event.Type == "finish-step" {
			return "", false
		}
		reason := normalizeFinishReason(event.FinishReason)
		return writeChunk(state, map[string]any{}, &reason) + "data: [DONE]\n\n", true
	case "error":
		msg := "upstream error"
		if event.Error != nil && event.Error.Message != "" {
			msg = event.Error.Message
		}
		payload, _ := json.Marshal(map[string]any{"error": map[string]any{"message": msg, "type": "api_error"}})
		return "data: " + string(payload) + "\n\ndata: [DONE]\n\n", true
	default:
		captureUsage(state, event)
		return "", false
	}
}

func writeToolStart(state *streamState, id, name string) string {
	idx := toolIndex(state, id, false)
	return writeChunk(state, map[string]any{
		"role": maybeRole(state),
		"tool_calls": []map[string]any{{
			"index": idx,
			"id":    id,
			"type":  "function",
			"function": map[string]any{
				"name": name,
			},
		}},
	}, nil)
}

func writeToolArgs(state *streamState, idx int, args string) string {
	if idx < 0 {
		idx = 0
	}
	return writeChunk(state, map[string]any{
		"tool_calls": []map[string]any{{
			"index":    idx,
			"function": map[string]any{"arguments": args},
		}},
	}, nil)
}

func writeChunk(state *streamState, delta map[string]any, finish *string) string {
	if role, _ := delta["role"].(string); role == "" {
		delete(delta, "role")
	}
	choice := map[string]any{"index": 0, "delta": delta}
	if finish != nil {
		choice["finish_reason"] = *finish
	}
	payload, _ := json.Marshal(map[string]any{
		"id":      state.id,
		"object":  "chat.completion.chunk",
		"created": state.created,
		"model":   state.model,
		"choices": []any{choice},
	})
	return "data: " + string(payload) + "\n\n"
}

func toolIndex(state *streamState, id string, allocateIfMissing bool) int {
	if id != "" {
		if idx, ok := state.toolCallByID[id]; ok {
			return idx
		}
	}
	if !allocateIfMissing && id == "" {
		return state.toolCallIndex - 1
	}
	idx := state.toolCallIndex
	if id != "" {
		state.toolCallByID[id] = idx
	}
	state.toolCallIndex++
	return idx
}

func maybeRole(state *streamState) string {
	if state.sentRole {
		return ""
	}
	state.sentRole = true
	return "assistant"
}

func captureUsage(state *streamState, event ccStreamEvent) {
	u := event.TotalUsage
	if u == nil {
		u = event.Usage
	}
	if u == nil {
		return
	}
	state.promptTokens = u.InputTokens
	state.completionToks = u.OutputTokens
}

func normalizeFinishReason(reason string) string {
	switch reason {
	case "tool_calls", "tool-calls":
		return "tool_calls"
	case "length", "max_tokens":
		return "length"
	case "content_filter", "content-filter":
		return "content_filter"
	default:
		return "stop"
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

const maxNDJSONLine = 50 * 1024 * 1024

func readNDJSONLine(br *bufio.Reader) (string, error) {
	var b strings.Builder
	for {
		part, err := br.ReadString('\n')
		if part != "" {
			if b.Len()+len(part) > maxNDJSONLine {
				return "", fmt.Errorf("ndjson line exceeds 50MB")
			}
			b.WriteString(part)
		}
		if err == nil {
			return strings.TrimSpace(b.String()), nil
		}
		if err == io.EOF {
			return strings.TrimSpace(b.String()), io.EOF
		}
		return "", err
	}
}

// readUntilFirstSSE consumes NDJSON until the first OpenAI SSE chunk, a
// terminal finish, or a pre-content error event. Thinking/unknown events are
// skipped so a later `type=error` can still fail the request for account
// fallback instead of being forwarded as HTTP 200 + SSE error.
func readUntilFirstSSE(br *bufio.Reader, state *streamState) (prelude string, done bool, status int, err error) {
	for {
		line, readErr := readNDJSONLine(br)
		if line != "" {
			if event, ok := parseErrorEvent(line); ok {
				st, e := commandCodeStreamFailure(event)
				return "", true, st, e
			}
			chunk, finished := translateNDJSON(line, state)
			if chunk != "" {
				return chunk, finished, 0, nil
			}
			if finished {
				return "", true, 0, nil
			}
		}
		if readErr == io.EOF {
			return prelude, done, 0, io.EOF
		}
		if readErr != nil {
			return "", false, 0, readErr
		}
	}
}

func pumpNDJSON(r io.Reader, w io.Writer, state *streamState) error {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReaderSize(r, 64*1024)
	}
	return pumpNDJSONReader(br, w, state)
}

func pumpNDJSONReader(br *bufio.Reader, w io.Writer, state *streamState) error {
	finished := false
	for {
		line, err := readNDJSONLine(br)
		if line != "" {
			chunk, done := translateNDJSON(line, state)
			if chunk != "" {
				if _, werr := io.WriteString(w, chunk); werr != nil {
					return werr
				}
			}
			if done {
				finished = true
				break
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	if !finished {
		reason := "stop"
		if _, err := io.WriteString(w, writeChunk(state, map[string]any{}, &reason)+"data: [DONE]\n\n"); err != nil {
			return err
		}
	}
	return nil
}

func formatUpstreamError(status int, body string) string {
	return fmt.Sprintf("commandcode returned %d: %s", status, body)
}
