package minimax

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestBuildImageRequest(t *testing.T) {
	promptOptimizer := true
	seed := int64(0)
	tests := []struct {
		name    string
		input   string
		model   string
		want    ImageRequest
		wantErr string
	}{
		{
			name:  "defaults to base64 square image",
			input: `{"prompt":" hello "}`,
			model: ImageModel,
			want: ImageRequest{
				Model:          ImageModel,
				Prompt:         "hello",
				AspectRatio:    "1:1",
				ResponseFormat: "base64",
				N:              1,
			},
		},
		{
			name:  "maps OpenAI fields and preserves extensions",
			input: `{"prompt":"portrait","n":2,"size":"1792x1024","response_format":"url","seed":0,"prompt_optimizer":true}`,
			model: "minimax/image-01",
			want: ImageRequest{
				Model:           ImageModel,
				Prompt:          "portrait",
				Width:           1792,
				Height:          1024,
				ResponseFormat:  "url",
				Seed:            &seed,
				N:               2,
				PromptOptimizer: &promptOptimizer,
			},
		},
		{
			name:  "explicit aspect ratio takes precedence",
			input: `{"prompt":"wide","size":"1024x1024","width":768,"height":768,"aspect_ratio":"21:9"}`,
			model: ImageModel,
			want: ImageRequest{
				Model:          ImageModel,
				Prompt:         "wide",
				AspectRatio:    "21:9",
				ResponseFormat: "base64",
				N:              1,
			},
		},
		{name: "rejects unsupported model", input: `{"prompt":"x"}`, model: "image-01-live", wantErr: "unsupported MiniMax image model"},
		{name: "rejects zero count", input: `{"prompt":"x","n":0}`, model: ImageModel, wantErr: "n must be between"},
		{name: "rejects negative count", input: `{"prompt":"x","n":-1}`, model: ImageModel, wantErr: "n must be between"},
		{name: "rejects oversized batch", input: `{"prompt":"x","n":10}`, model: ImageModel, wantErr: "n must be between"},
		{name: "rejects bad response format", input: `{"prompt":"x","response_format":"binary"}`, model: ImageModel, wantErr: "response_format"},
		{name: "rejects invalid dimensions", input: `{"prompt":"x","size":"513x1024"}`, model: ImageModel, wantErr: "divisible by 8"},
		{name: "rejects incomplete native dimensions", input: `{"prompt":"x","width":1024}`, model: ImageModel, wantErr: "between 512 and 2048"},
		{name: "rejects overlong prompt", input: `{"prompt":"` + strings.Repeat("界", MaxImagePromptChars+1) + `"}`, model: ImageModel, wantErr: "exceeds 1500"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := BuildImageRequest([]byte(tt.input), tt.model)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("BuildImageRequest() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildImageRequest() error = %v", err)
			}
			var got ImageRequest
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tt.want)
			if string(gotJSON) != string(wantJSON) {
				t.Fatalf("request = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestBuildImageEditRequest(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantReference string
		wantErr       string
	}{
		{
			name:          "maps playground image object",
			input:         `{"prompt":"put the same person in a library","images":[{"image_url":"https://example.com/person.jpg"}],"size":"1024x1024","response_format":"url"}`,
			wantReference: "https://example.com/person.jpg",
		},
		{
			name:          "accepts single image data URL",
			input:         `{"prompt":"change the background","image":"data:image/png;base64,YWJj"}`,
			wantReference: "data:image/png;base64,YWJj",
		},
		{
			name:    "requires a reference",
			input:   `{"prompt":"change the background"}`,
			wantErr: "exactly one reference image is required",
		},
		{
			name:    "rejects multiple references",
			input:   `{"prompt":"change the background","images":[{"image_url":"https://example.com/a.jpg"},{"image_url":"https://example.com/b.jpg"}]}`,
			wantErr: "exactly one reference image",
		},
		{
			name:    "rejects mask",
			input:   `{"prompt":"change the background","image":"https://example.com/a.jpg","mask":"data:image/png;base64,YWJj"}`,
			wantErr: "mask editing is not supported",
		},
		{
			name:    "rejects local file",
			input:   `{"prompt":"change the background","image":"C:\\temp\\a.jpg"}`,
			wantErr: "HTTP(S) URL or image data URL",
		},
		{
			name:    "rejects invalid data URL base64",
			input:   `{"prompt":"change the background","image":"data:image/png;base64,not-valid!"}`,
			wantErr: "invalid base64",
		},
		{
			name:    "rejects unsupported data URL media type",
			input:   `{"prompt":"change the background","image":"data:image/webp;base64,YWJj"}`,
			wantErr: "PNG or JPEG",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := BuildImageEditRequest([]byte(tt.input), "minimax/image-01")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("BuildImageEditRequest() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildImageEditRequest() error = %v", err)
			}
			var request ImageRequest
			if err := json.Unmarshal(body, &request); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			if len(request.SubjectReference) != 1 {
				t.Fatalf("SubjectReference = %#v", request.SubjectReference)
			}
			if request.SubjectReference[0].Type != "character" ||
				request.SubjectReference[0].ImageFile != tt.wantReference {
				t.Fatalf("SubjectReference = %#v", request.SubjectReference)
			}
		})
	}
}

func TestParseImageResponse(t *testing.T) {
	results, err := ParseImageResponse([]byte(`{
		"data":{"image_urls":["https://example.test/a.jpg"],"image_base64":["YmFzZTY0"]},
		"base_resp":{"status_code":0,"status_msg":"success"}
	}`))
	if err != nil {
		t.Fatalf("ParseImageResponse() error = %v", err)
	}
	if len(results) != 2 || results[0].URL == "" || results[1].B64JSON == "" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestParseImageResponseBusinessError(t *testing.T) {
	_, err := ParseImageResponse([]byte(`{"base_resp":{"status_code":1002,"status_msg":"rate limit"}}`))
	if err == nil {
		t.Fatal("expected business error")
	}
	if got := HTTPStatus(err); got != http.StatusTooManyRequests {
		t.Fatalf("HTTPStatus() = %d, want %d", got, http.StatusTooManyRequests)
	}
}

func TestParseImageResponseRequiresOutput(t *testing.T) {
	_, err := ParseImageResponse([]byte(`{"base_resp":{"status_code":0,"status_msg":"success"}}`))
	if err == nil || !strings.Contains(err.Error(), "did not contain image output") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseImageResponseRequiresEnvelope(t *testing.T) {
	_, err := ParseImageResponse([]byte(`{"data":{"image_urls":["https://example.test/a.jpg"]}}`))
	if err == nil || !strings.Contains(err.Error(), "missing base_resp") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseImageResponseRejectsPartialBatch(t *testing.T) {
	_, err := ParseImageResponse([]byte(`{
		"data":{"image_urls":["https://example.test/a.jpg"]},
		"metadata":{"failed_count":"1","success_count":"1"},
		"base_resp":{"status_code":0,"status_msg":"success"}
	}`))
	if err == nil || !strings.Contains(err.Error(), "partially failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHTTPStatusUnknownBusinessCodeUsesHTTPFallback(t *testing.T) {
	err := &APIError{Code: 9999, Message: "new error"}
	if got := HTTPStatus(err); got != 0 {
		t.Fatalf("HTTPStatus() = %d, want 0", got)
	}
}

func TestResolveImageBaseURL(t *testing.T) {
	if got := ResolveImageBaseURL(nil); got != "https://api.minimax.io" {
		t.Fatalf("default base URL = %q", got)
	}
}
