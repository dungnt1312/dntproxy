package byteplus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildImageRequestGolden(t *testing.T) {
	body, err := BuildImageRequest([]byte(`{
		"model":"ignored",
		"prompt":"  A lighthouse in a storm  ",
		"n":1,
		"size":"1024x1024",
		"quality":"standard",
		"response_format":"url",
		"watermark":false
	}`), "byteplus/seedream-4-0-250828")
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, body, readFixture(t, "generation-request.golden.json"))
}

func TestBuildImageEditRequestGolden(t *testing.T) {
	body, err := BuildImageEditRequest([]byte(`{
		"prompt":"Replace the sky",
		"image":"https://example.com/one.png",
		"images":["data:image/jpeg;base64,aGVsbG8="],
		"size":"2K",
		"response_format":"b64_json"
	}`), "seedream-4-5-250428")
	if err != nil {
		t.Fatal(err)
	}
	assertJSONEqual(t, body, readFixture(t, "edit-request.golden.json"))
}

func TestBuildImageEditRequestAcceptsOfficialImageArray(t *testing.T) {
	body, err := BuildImageEditRequest([]byte(`{
		"prompt":"Blend references",
		"image":["https://example.com/one.png","https://example.com/two.png"]
	}`), "seedream-4-5-250428")
	if err != nil {
		t.Fatal(err)
	}
	var native ImageRequest
	if err := json.Unmarshal(body, &native); err != nil {
		t.Fatal(err)
	}
	images, ok := native.Image.([]any)
	if !ok || len(images) != 2 {
		t.Fatalf("image = %#v", native.Image)
	}
}

func TestBuildImageEditRequestAcceptsOpenAIImageURLObjects(t *testing.T) {
	body, err := BuildImageEditRequest([]byte(`{
		"prompt":"Blend references",
		"images":[
			{"image_url":"https://example.com/one.png"},
			{"url":"https://example.com/two.png"}
		]
	}`), "seedream-4-5-251128")
	if err != nil {
		t.Fatal(err)
	}
	var native ImageRequest
	if err := json.Unmarshal(body, &native); err != nil {
		t.Fatal(err)
	}
	images, ok := native.Image.([]any)
	if !ok || len(images) != 2 {
		t.Fatalf("image = %#v", native.Image)
	}
}

func TestBuildImageRequestValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"prompt", `{"prompt":" "}`, "prompt is required"},
		{"n", `{"prompt":"ok","n":2}`, "n must be 1"},
		{"format", `{"prompt":"ok","response_format":"binary"}`, "response_format"},
		{"style", `{"prompt":"ok","style":"vivid"}`, "style is not supported"},
		{"stream", `{"prompt":"ok","stream":true}`, "streaming"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := BuildImageRequest([]byte(test.body), "seedream")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestBuildImageEditRequestValidation(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"missing", `{"prompt":"ok"}`, "at least one"},
		{"mask", `{"prompt":"ok","image":"https://example.com/a.png","mask":"x"}`, "mask editing"},
		{"scheme", `{"prompt":"ok","image":"file:///tmp/a.png"}`, "http(s)"},
		{"base64", `{"prompt":"ok","image":"data:image/png;base64,!"}`, "invalid base64"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := BuildImageEditRequest([]byte(test.body), "seedream")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func assertJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.Marshal(gotValue)
	wantJSON, _ := json.Marshal(wantValue)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("JSON mismatch\ngot:  %s\nwant: %s", gotJSON, wantJSON)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
