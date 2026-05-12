package compressor

import "encoding/json"

// walkAndCompress traverses the messages array in body and compresses eligible
// tool result content. Returns original body on any parse error.
func walkAndCompress(body []byte, c *Compressor) ([]byte, Stats) {
	opts := c.currentOpts()
	stats := Stats{
		OriginalBytes: len(body),
		Detections:    make(map[string]int),
		LogSavings:    opts.LogSavings,
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body, Stats{}
	}

	msgsRaw, ok := raw["messages"]
	if !ok {
		return body, Stats{}
	}

	var msgs []json.RawMessage
	if err := json.Unmarshal(msgsRaw, &msgs); err != nil {
		return body, Stats{}
	}

	changed := false
	for i, msgRaw := range msgs {
		var msg map[string]json.RawMessage
		if err := json.Unmarshal(msgRaw, &msg); err != nil {
			continue
		}
		var role string
		if err := json.Unmarshal(msg["role"], &role); err != nil {
			continue
		}

		var newMsg map[string]json.RawMessage
		switch role {
		case "tool":
			newMsg, stats = compressShapeA(msg, stats, c, opts)
		case "user":
			newMsg, stats = compressShapeB(msg, stats, c, opts)
		}

		if newMsg != nil {
			b, err := json.Marshal(newMsg)
			if err == nil {
				msgs[i] = b
				changed = true
			}
		}
	}

	if !changed {
		stats.CompressedBytes = stats.OriginalBytes
		return body, stats
	}

	newMsgs, err := json.Marshal(msgs)
	if err != nil {
		return body, Stats{}
	}
	raw["messages"] = newMsgs

	out, err := json.Marshal(raw)
	if err != nil {
		return body, Stats{}
	}
	stats.CompressedBytes = len(out)
	stats.TokensSaved = (stats.OriginalBytes - stats.CompressedBytes) / 4
	return out, stats
}

// compressShapeA handles OpenAI tool role: {"role":"tool","content":"string"}.
func compressShapeA(msg map[string]json.RawMessage, stats Stats, c *Compressor, opts Options) (map[string]json.RawMessage, Stats) {
	contentRaw, ok := msg["content"]
	if !ok {
		return nil, stats
	}
	var content string
	if err := json.Unmarshal(contentRaw, &content); err != nil {
		stats.Skipped++
		return nil, stats
	}
	compressed, stats := compressContent(content, stats, c, opts)
	if compressed == content {
		return nil, stats
	}
	newMsg := copyRawMap(msg)
	b, err := json.Marshal(compressed)
	if err != nil {
		return nil, stats
	}
	newMsg["content"] = b
	return newMsg, stats
}

// compressShapeB handles Anthropic user role with tool_result content blocks.
func compressShapeB(msg map[string]json.RawMessage, stats Stats, c *Compressor, opts Options) (map[string]json.RawMessage, Stats) {
	contentRaw, ok := msg["content"]
	if !ok {
		return nil, stats
	}
	var contentArr []json.RawMessage
	if err := json.Unmarshal(contentRaw, &contentArr); err != nil {
		return nil, stats
	}

	changed := false
	for i, blockRaw := range contentArr {
		var block map[string]json.RawMessage
		if err := json.Unmarshal(blockRaw, &block); err != nil {
			continue
		}
		var btype string
		json.Unmarshal(block["type"], &btype) //nolint:errcheck
		if btype != "tool_result" {
			continue
		}
		var isError bool
		json.Unmarshal(block["is_error"], &isError) //nolint:errcheck
		if isError {
			stats.Skipped++
			continue
		}

		blockContentRaw, ok := block["content"]
		if !ok {
			continue
		}

		// Shape B1: content is a string
		var contentStr string
		if err := json.Unmarshal(blockContentRaw, &contentStr); err == nil {
			compressed, newStats := compressContent(contentStr, stats, c, opts)
			stats = newStats
			if compressed != contentStr {
				newBlock := copyRawMap(block)
				b, err := json.Marshal(compressed)
				if err == nil {
					newBlock["content"] = b
					nb, err := json.Marshal(newBlock)
					if err == nil {
						contentArr[i] = nb
						changed = true
					}
				}
			}
			continue
		}

		// Shape B2: content is array of text blocks
		var textBlocks []json.RawMessage
		if err := json.Unmarshal(blockContentRaw, &textBlocks); err != nil {
			continue
		}
		blockChanged := false
		for j, tbRaw := range textBlocks {
			var tb map[string]json.RawMessage
			if err := json.Unmarshal(tbRaw, &tb); err != nil {
				continue
			}
			var tbType string
			json.Unmarshal(tb["type"], &tbType) //nolint:errcheck
			if tbType != "text" {
				continue
			}
			var text string
			if err := json.Unmarshal(tb["text"], &text); err != nil {
				continue
			}
			compressed, newStats := compressContent(text, stats, c, opts)
			stats = newStats
			if compressed != text {
				newTb := copyRawMap(tb)
				b, err := json.Marshal(compressed)
				if err == nil {
					newTb["text"] = b
					nb, err := json.Marshal(newTb)
					if err == nil {
						textBlocks[j] = nb
						blockChanged = true
					}
				}
			}
		}
		if blockChanged {
			newBlock := copyRawMap(block)
			tbBytes, err := json.Marshal(textBlocks)
			if err == nil {
				newBlock["content"] = tbBytes
				nb, err := json.Marshal(newBlock)
				if err == nil {
					contentArr[i] = nb
					changed = true
				}
			}
		}
	}

	if !changed {
		return nil, stats
	}
	newContentBytes, err := json.Marshal(contentArr)
	if err != nil {
		return nil, stats
	}
	newMsg := copyRawMap(msg)
	newMsg["content"] = newContentBytes
	return newMsg, stats
}

// compressContent applies detection + filter to a single string value.
func compressContent(content string, stats Stats, c *Compressor, opts Options) (string, Stats) {
	if len(content) < opts.MinContentLength {
		stats.Skipped++
		return content, stats
	}
	if HasBase64Line(content) {
		stats.Skipped++
		return content, stats
	}
	ct := Detect(content)
	if ct == ContentCodeFile || ct == ContentGeneric {
		stats.Skipped++
		return content, stats
	}
	compressed, ok := c.applyFilter(content, ct)
	if !ok || len(compressed) > int(float64(len(content))*0.85) {
		stats.Skipped++
		return content, stats
	}
	if stats.Detections == nil {
		stats.Detections = make(map[string]int)
	}
	stats.Detections[ct.String()]++
	return compressed, stats
}

func copyRawMap(m map[string]json.RawMessage) map[string]json.RawMessage {
	n := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		n[k] = v
	}
	return n
}
