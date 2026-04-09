package kiro

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// EventFrame represents a parsed AWS EventStream frame.
type EventFrame struct {
	Headers map[string]string
	Payload json.RawMessage
}

// ParseEventFrames extracts complete EventStream frames from a buffer.
// Returns parsed frames and the remaining unprocessed bytes.
func ParseEventFrames(buf []byte) ([]EventFrame, []byte) {
	var frames []EventFrame
	iterations := 0
	maxIterations := 1000

	for len(buf) >= 16 && iterations < maxIterations {
		iterations++

		// First 4 bytes: total message length (big-endian)
		totalLength := int(binary.BigEndian.Uint32(buf[0:4]))
		if totalLength < 16 || totalLength > len(buf) {
			break
		}

		frameData := buf[:totalLength]
		buf = buf[totalLength:]

		frame := parseOneFrame(frameData)
		if frame != nil {
			frames = append(frames, *frame)
		}
	}

	return frames, buf
}

func parseOneFrame(data []byte) *EventFrame {
	if len(data) < 16 {
		return nil
	}

	// Bytes 4-8: headers length (big-endian)
	headersLength := int(binary.BigEndian.Uint32(data[4:8]))
	// Bytes 8-12: prelude CRC (skip)

	// Parse headers starting at offset 12
	headers := make(map[string]string)
	offset := 12
	headerEnd := 12 + headersLength

	for offset < headerEnd && offset < len(data) {
		// Name length (1 byte)
		if offset >= len(data) {
			break
		}
		nameLen := int(data[offset])
		offset++

		if offset+nameLen > len(data) {
			break
		}
		name := string(data[offset : offset+nameLen])
		offset += nameLen

		// Header type (1 byte)
		if offset >= len(data) {
			break
		}
		headerType := data[offset]
		offset++

		if headerType == 7 { // String type
			if offset+2 > len(data) {
				break
			}
			valueLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
			offset += 2

			if offset+valueLen > len(data) {
				break
			}
			value := string(data[offset : offset+valueLen])
			offset += valueLen
			headers[name] = value
		} else {
			// Unknown header type — stop parsing headers
			break
		}
	}

	// Parse payload: from end of headers to 4 bytes before end (message CRC)
	payloadStart := 12 + headersLength
	payloadEnd := len(data) - 4 // Exclude message CRC

	var payload json.RawMessage
	if payloadEnd > payloadStart {
		payloadBytes := data[payloadStart:payloadEnd]
		// Skip empty/whitespace payloads
		trimmed := trimBytes(payloadBytes)
		if len(trimmed) > 0 {
			payload = json.RawMessage(trimmed)
		}
	}

	return &EventFrame{
		Headers: headers,
		Payload: payload,
	}
}

func trimBytes(b []byte) []byte {
	start := 0
	end := len(b)
	for start < end && (b[start] == ' ' || b[start] == '\t' || b[start] == '\n' || b[start] == '\r') {
		start++
	}
	for end > start && (b[end-1] == ' ' || b[end-1] == '\t' || b[end-1] == '\n' || b[end-1] == '\r') {
		end--
	}
	return b[start:end]
}

// GetEventType returns the :event-type header value.
func (f *EventFrame) GetEventType() string {
	return f.Headers[":event-type"]
}

// ParsePayloadAs unmarshals the payload into the given target.
func (f *EventFrame) ParsePayloadAs(target interface{}) error {
	if f.Payload == nil {
		return fmt.Errorf("no payload")
	}
	return json.Unmarshal(f.Payload, target)
}
