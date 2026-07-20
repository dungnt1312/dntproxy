package openai

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

type codexCancellationBody struct {
	ctx    context.Context
	closed chan struct{}
}

func (body *codexCancellationBody) Read([]byte) (int, error) {
	<-body.ctx.Done()
	return 0, body.ctx.Err()
}

func (body *codexCancellationBody) Close() error {
	select {
	case <-body.closed:
	default:
		close(body.closed)
	}
	return nil
}

func TestImageProviderCapabilitiesStrictAndCompatible(t *testing.T) {
	if got := NewImageProvider().Capabilities("gpt-4o"); got.Generate || got.Edit {
		t.Fatalf("native chat model advertised image capabilities: %#v", got)
	}
	if got := NewImageProvider().Capabilities("gpt-image-2"); !got.Generate || !got.Edit {
		t.Fatalf("native image model capabilities: %#v", got)
	}
	if got := NewCompatibleImageProvider().Capabilities("flux-custom"); !got.Generate || !got.Edit {
		t.Fatalf("compatible unknown image model capabilities: %#v", got)
	}
}

func TestParseCodexImageStreamPartialFinalAndDone(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"type":"response.image_generation_call.partial_image","partial_image_b64":"YWJj","output_index":0}`,
		`data: {"type":"response.image_generation_call.partial_image","partial_image_b64":"ZGVm","output_index":0}`,
		`data: {"type":"response.output_text.delta","delta":"revised"}`,
		`data: {"type":"response.completed","response":{"created_at":123}}`,
		"",
	}, "\n")
	var chunks []ImageStreamChunk
	for chunk := range ParseCodexImageStream(strings.NewReader(stream)) {
		chunks = append(chunks, chunk)
	}
	if len(chunks) != 4 || !chunks[0].IsPartial || !chunks[1].IsPartial {
		t.Fatalf("chunks = %#v", chunks)
	}
	if chunks[2].B64JSON != "YWJjZGVm" || chunks[2].RevisedPrompt != "revised" || chunks[2].CreatedAt != 123 {
		t.Fatalf("final chunk = %#v", chunks[2])
	}
	if !chunks[3].IsDone {
		t.Fatalf("done chunk = %#v", chunks[3])
	}
}

func TestImageProviderCodexStreamHonorsCancellation(t *testing.T) {
	originalClient := codexHTTPClient
	t.Cleanup(func() { codexHTTPClient = originalClient })

	ctx, cancel := context.WithCancel(context.Background())
	body := &codexCancellationBody{ctx: ctx, closed: make(chan struct{})}
	codexHTTPClient = &http.Client{
		Transport: codexRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       body,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			}, nil
		}),
	}

	events, status, err := NewImageProvider().StreamGenerate(ctx, port.ImageRequest{
		Model: "gpt-image-2",
		Body:  []byte(`{"prompt":"draw","stream":true}`),
		Credentials: &domain.Credentials{
			AccessToken: "oauth-token",
			ProviderSpecificData: map[string]interface{}{
				"authMethod": "oauth",
			},
		},
	})
	if err != nil || status != http.StatusOK {
		t.Fatalf("status=%d err=%v", status, err)
	}
	cancel()

	select {
	case _, ok := <-events:
		if ok {
			for range events {
			}
		}
	case <-time.After(time.Second):
		t.Fatal("Codex image stream did not close after cancellation")
	}
	select {
	case <-body.closed:
	case <-time.After(time.Second):
		t.Fatal("Codex response body was not closed after cancellation")
	}
}

func TestTranslateMultipartEditToCodex(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("prompt", "replace sky"); err != nil {
		t.Fatal(err)
	}
	file, err := writer.CreateFormFile("image", "input.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte("png-bytes")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequest(http.MethodPost, "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	translated, err := TranslateMultipartEditToCodex(request.MultipartForm, "gpt-image-2")
	if err != nil {
		t.Fatal(err)
	}
	text := string(translated)
	if !strings.Contains(text, "replace sky") || !strings.Contains(text, "cG5nLWJ5dGVz") {
		t.Fatalf("translated body = %s", text)
	}
}

type codexRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn codexRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

var _ io.ReadCloser = (*codexCancellationBody)(nil)
