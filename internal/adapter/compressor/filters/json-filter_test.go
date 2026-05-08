package filters

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONFilter_ReducesSize(t *testing.T) {
	// Build a large JSON with arrays
	type item struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	var items []item
	for i := 0; i < 50; i++ {
		items = append(items, item{ID: i, Name: "item-name-long-string", Value: "some-long-value-here"})
	}
	payload := map[string]interface{}{
		"results": items,
		"total":   50,
		"page":    1,
	}
	b, _ := json.Marshal(payload)
	in := string(b)

	out, ok := JSONFilter(in)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if len(out) >= len(in) {
		t.Errorf("expected size reduction, got orig=%d out=%d", len(in), len(out))
	}
	// Output must be valid JSON
	if !json.Valid([]byte(out)) {
		t.Fatal("output is not valid JSON")
	}
}

func TestJSONFilter_InvalidJSON_FailOpen(t *testing.T) {
	in := `{"broken": [1, 2,`
	out, ok := JSONFilter(in)
	if ok {
		t.Fatal("expected ok=false on invalid JSON")
	}
	if out != in {
		t.Fatal("should return original on parse failure")
	}
}

func TestJSONFilter_DepthLimit(t *testing.T) {
	in := `{"a":{"b":{"c":{"d":{"e":"deep"}}}}}`
	out, ok := JSONFilter(in)
	if !ok {
		t.Fatal("expected ok=true")
	}
	// Depth > 3 should be truncated to "..."
	if strings.Contains(out, `"deep"`) {
		t.Error("deep values should be truncated")
	}
	if !strings.Contains(out, `"..."`) {
		t.Error("truncation marker '...' expected")
	}
}

func TestJSONFilter_EmptyArray(t *testing.T) {
	in := `{"items":[]}`
	out, ok := JSONFilter(in)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !json.Valid([]byte(out)) {
		t.Fatal("output must be valid JSON")
	}
}
