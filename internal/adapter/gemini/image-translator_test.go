package gemini

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildGenerateContentRequestGolden(t *testing.T) {
	body, err := BuildGenerateContentRequest(
		[]byte(`{"prompt":" Draw a red panda ","n":2,"size":"1536x1024"}`),
		"gemini-3.1-flash-image",
		[]InlineImage{{Data: []byte("jpeg"), MIMEType: "image/jpeg"}},
	)
	if err != nil {
		t.Fatalf("BuildGenerateContentRequest() error = %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	encoded, _ := json.Marshal(got)
	want := `{"contents":[{"parts":[{"text":"Draw a red panda"},{"inline_data":{"data":"anBlZw==","mime_type":"image/jpeg"}}]}],"generationConfig":{"candidateCount":2,"responseFormat":{"image":{"aspectRatio":"3:2","imageSize":"2K"}},"responseModalities":["IMAGE"]}}`
	if string(encoded) != want {
		t.Fatalf("request = %s\nwant    = %s", encoded, want)
	}
}

func TestBuildGenerateContentRequestSizeMapping(t *testing.T) {
	tests := []struct {
		size      string
		wantRatio string
		wantScale string
		wantErr   string
	}{
		{size: "", wantRatio: "1:1", wantScale: "1K"},
		{size: "1024x1792", wantRatio: "9:16", wantScale: "2K"},
		{size: "1792x1024", wantRatio: "16:9", wantScale: "2K"},
		{size: "2048x2048", wantRatio: "1:1", wantScale: "2K"},
		{size: "4096x4096", wantRatio: "1:1", wantScale: "4K"},
	}
	for _, test := range tests {
		t.Run(test.size, func(t *testing.T) {
			body, err := BuildGenerateContentRequest([]byte(`{"prompt":"x","size":"`+test.size+`"}`), "gemini-3.1-flash-image", nil)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var request generateContentRequest
			if err := json.Unmarshal(body, &request); err != nil {
				t.Fatal(err)
			}
			image := request.GenerationConfig.ResponseFormat.Image
			if image.AspectRatio != test.wantRatio || image.ImageSize != test.wantScale {
				t.Fatalf("image format = %#v", image)
			}
		})
	}
}

func TestBuildGenerateContentRequestFlashLiteForcesOneKAndSupportedRatios(t *testing.T) {
	body, err := BuildGenerateContentRequest(
		[]byte(`{"prompt":"x","size":"1792x1024"}`),
		"gemini-3.1-flash-lite-image",
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	var request generateContentRequest
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatal(err)
	}
	image := request.GenerationConfig.ResponseFormat.Image
	if image.AspectRatio != "16:9" || image.ImageSize != "1K" {
		t.Fatalf("image format = %#v", image)
	}
	if _, err := BuildGenerateContentRequest(
		[]byte(`{"prompt":"x","size":"1024x4096"}`),
		"gemini-3.1-flash-lite-image",
		nil,
	); err == nil || !strings.Contains(err.Error(), "unsupported Gemini image aspect ratio") {
		t.Fatalf("error = %v", err)
	}
}

func TestParseEditInput(t *testing.T) {
	input, err := ParseEditInput([]byte(`{
		"prompt":"combine these",
		"image":"data:image/png;base64,YQ==",
		"images":[{"image_url":"https://example.test/a.jpg"},{"url":"https://example.test/b.webp"}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Sources) != 3 {
		t.Fatalf("sources = %#v", input.Sources)
	}
}

func TestParseEditInputRejectsMaskAndMissingReference(t *testing.T) {
	for _, input := range []string{
		`{"prompt":"x"}`,
		`{"prompt":"x","image":"https://example.test/a.png","mask":"data:image/png;base64,YQ=="}`,
	} {
		if _, err := ParseEditInput([]byte(input)); err == nil {
			t.Fatalf("ParseEditInput(%s) expected error", input)
		}
	}
}

func TestParseGenerateContentResponseSkipsThoughtImages(t *testing.T) {
	results, err := ParseGenerateContentResponse([]byte(`{
		"candidates":[{"content":{"parts":[
			{"thought":true,"inlineData":{"mimeType":"image/png","data":"dGhvdWdodA=="}},
			{"text":"done"},
			{"inlineData":{"mimeType":"image/png","data":"ZmluYWw="}}
		]}}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].B64JSON != "ZmluYWw=" {
		t.Fatalf("results = %#v", results)
	}
}

func TestParseGenerateContentResponseSupportsSnakeCase(t *testing.T) {
	results, err := ParseGenerateContentResponse([]byte(`{
		"candidates":[{"content":{"parts":[
			{"inline_data":{"mime_type":"image/jpeg","data":"aW1hZ2U="}}
		]}}]
	}`))
	if err != nil || len(results) != 1 {
		t.Fatalf("results = %#v, err = %v", results, err)
	}
}

func TestParseGenerateContentResponseNoFinalImage(t *testing.T) {
	_, err := ParseGenerateContentResponse([]byte(`{
		"candidates":[{"content":{"parts":[
			{"thought":true,"inlineData":{"mimeType":"image/png","data":"dGhvdWdodA=="}},
			{"text":"blocked"}
		]}}]
	}`))
	if err == nil || err.Error() != "Gemini response did not contain a final image" {
		t.Fatalf("error = %v", err)
	}
}
