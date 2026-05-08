package filters

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSONFilter emits a structural skeleton of JSON, truncating deep nesting
// and collapsing repeated array elements.
func JSONFilter(s string) (string, bool) {
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return s, false
	}
	skeleton := buildSkeleton(v, 0)
	out, err := json.MarshalIndent(skeleton, "", "  ")
	if err != nil {
		return s, false
	}
	return string(out), true
}

func buildSkeleton(v interface{}, depth int) interface{} {
	if depth >= 3 {
		return "..."
	}
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(val))
		for k, child := range val {
			result[k] = buildSkeleton(child, depth+1)
		}
		return result
	case []interface{}:
		if len(val) == 0 {
			return []interface{}{}
		}
		if len(val) == 1 {
			return []interface{}{buildSkeleton(val[0], depth+1)}
		}
		return []interface{}{
			buildSkeleton(val[0], depth+1),
			fmt.Sprintf("... ×%d", len(val)-1),
		}
	case string:
		if len(val) > 50 {
			return val[:50] + "..."
		}
		return val
	case float64:
		// Compact number representation
		if val == float64(int64(val)) {
			return int64(val)
		}
		return val
	default:
		return v
	}
}

// buildSkeletonString renders a skeleton as compact JSON string for embedding.
func buildSkeletonString(v interface{}) string {
	out, err := json.Marshal(buildSkeleton(v, 0))
	if err != nil {
		return "{}"
	}
	return strings.TrimSpace(string(out))
}
