package domain

// ImageCapabilities describes the image operations a provider/model accepts.
// Runtime provider metadata is authoritative; model-registry capabilities are
// only used for coarse discovery.
type ImageCapabilities struct {
	Generate           bool     `json:"generate"`
	Edit               bool     `json:"edit"`
	Multipart          bool     `json:"multipart,omitempty"`
	Mask               bool     `json:"mask,omitempty"`
	MultiReference     bool     `json:"multi_reference,omitempty"`
	Streaming          bool     `json:"streaming,omitempty"`
	MaxReferences      int      `json:"max_references,omitempty"`
	MaxInputBytes      int64    `json:"max_input_bytes,omitempty"`
	MaxTotalInputBytes int64    `json:"max_total_input_bytes,omitempty"`
	InputFormats       []string `json:"input_formats,omitempty"`
	ResponseFormats    []string `json:"response_formats,omitempty"`
}

// SupportsResponseFormat reports whether format is accepted. An empty list
// means the provider uses the OpenAI-compatible defaults.
func (c ImageCapabilities) SupportsResponseFormat(format string) bool {
	if len(c.ResponseFormats) == 0 {
		return format == "" || format == "url" || format == "b64_json"
	}
	for _, supported := range c.ResponseFormats {
		if supported == format {
			return true
		}
	}
	return false
}

// ImageStreamEvent is a provider-neutral event emitted during image streaming.
type ImageStreamEvent struct {
	Result  *ImageResult
	Partial bool
	Done    bool
	Created int64
}
