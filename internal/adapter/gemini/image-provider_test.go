package gemini

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

type geminiRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn geminiRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type geminiZeroReader struct{}

func (geminiZeroReader) Read(buffer []byte) (int, error) {
	for index := range buffer {
		buffer[index] = 0
	}
	return len(buffer), nil
}

func TestImageProviderGenerateUsesNativeEndpoint(t *testing.T) {
	var gotPath, gotKey, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("x-goog-api-key")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}]}}]}`))
	}))
	defer server.Close()

	provider := NewImageProviderWithClient(nil, server.Client())
	results, status, err := provider.Generate(context.Background(), port.ImageRequest{
		Model: "gemini-3.1-flash-image",
		Body:  []byte(`{"prompt":"draw"}`),
		Credentials: &domain.Credentials{
			APIKey:  "secret-key",
			BaseURL: server.URL + "/v1beta/openai",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusOK || len(results) != 1 {
		t.Fatalf("status = %d, results = %#v", status, results)
	}
	if gotPath != "/v1/models/gemini-3.1-flash-image:generateContent" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotKey != "secret-key" {
		t.Fatalf("x-goog-api-key = %q", gotKey)
	}
	if !strings.Contains(gotBody, `"responseModalities":["IMAGE"]`) {
		t.Fatalf("body = %s", gotBody)
	}
}

func TestImageProviderEditLoadsReferencesInline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"inline_data":{"mime_type":"image/png","data":"cG5n"}`) {
			t.Errorf("body = %s", body)
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"ZWRpdA=="}}]}}]}`))
	}))
	defer server.Close()

	loader := func(_ context.Context, source string) ([]byte, string, error) {
		if strings.Contains(source, "secret-token") {
			return []byte("png"), "image/png", nil
		}
		t.Fatalf("unexpected source passed to loader")
		return nil, "", nil
	}
	provider := NewImageProviderWithClient(loader, server.Client())
	results, _, err := provider.Edit(context.Background(), port.ImageRequest{
		Model: "gemini-3.1-flash-image",
		Body:  []byte(`{"prompt":"edit","image":"https://example.test/a.png?secret-token=abc"}`),
		Credentials: &domain.Credentials{
			APIKey:  "key",
			BaseURL: server.URL,
		},
	})
	if err != nil || len(results) != 1 {
		t.Fatalf("results = %#v, err = %v", results, err)
	}
}

func TestImageProviderSanitizesUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad input https://signed.test/image.png?token=secret","status":"INVALID_ARGUMENT"}}`))
	}))
	defer server.Close()

	provider := NewImageProviderWithClient(nil, server.Client())
	_, status, err := provider.Generate(context.Background(), port.ImageRequest{
		Model:       "gemini-3.1-flash-image",
		Body:        []byte(`{"prompt":"draw"}`),
		Credentials: &domain.Credentials{APIKey: "key", BaseURL: server.URL},
	})
	if status != http.StatusBadRequest || err == nil {
		t.Fatalf("status = %d, err = %v", status, err)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "signed.test") {
		t.Fatalf("error leaked signed URL: %v", err)
	}
	if !strings.Contains(err.Error(), "[redacted-url]") {
		t.Fatalf("error was not redacted: %v", err)
	}
}

func TestImageProviderGenerateHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	provider := NewImageProviderWithClient(nil, &http.Client{
		Transport: geminiRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}),
	})
	_, status, err := provider.Generate(ctx, port.ImageRequest{
		Model:       "gemini-3.1-flash-image",
		Body:        []byte(`{"prompt":"draw"}`),
		Credentials: &domain.Credentials{APIKey: "key"},
	})
	if status != http.StatusBadGateway || err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("status=%d err=%v", status, err)
	}
}

func TestImageProviderGenerateHonorsHTTPClientTimeout(t *testing.T) {
	const timeout = 20 * time.Millisecond
	provider := NewImageProviderWithClient(nil, &http.Client{
		Timeout: timeout,
		Transport: geminiRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}),
	})

	started := time.Now()
	_, status, err := provider.Generate(context.Background(), port.ImageRequest{
		Model:       "gemini-3.1-flash-image",
		Body:        []byte(`{"prompt":"draw"}`),
		Credentials: &domain.Credentials{APIKey: "key"},
	})
	elapsed := time.Since(started)
	if status != http.StatusBadGateway || err == nil || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if elapsed < timeout/2 || elapsed > time.Second {
		t.Fatalf("request elapsed %v, expected configured timeout near %v", elapsed, timeout)
	}
}

func TestImageProviderGenerateRejectsMalformedResponse(t *testing.T) {
	provider := NewImageProviderWithClient(nil, &http.Client{
		Transport: geminiRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"candidates":[`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	_, status, err := provider.Generate(context.Background(), port.ImageRequest{
		Model:       "gemini-3.1-flash-image",
		Body:        []byte(`{"prompt":"draw"}`),
		Credentials: &domain.Credentials{APIKey: "key"},
	})
	if status != http.StatusBadGateway || err == nil || !strings.Contains(err.Error(), "parse Gemini image response") {
		t.Fatalf("status=%d err=%v", status, err)
	}
}

func TestImageProviderGenerateRejectsOversizedResponse(t *testing.T) {
	provider := NewImageProviderWithClient(nil, &http.Client{
		Transport: geminiRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(io.LimitReader(
					geminiZeroReader{},
					maxImageResponseBody+1,
				)),
				Header: make(http.Header),
			}, nil
		}),
	})
	_, status, err := provider.Generate(context.Background(), port.ImageRequest{
		Model:       "gemini-3.1-flash-image",
		Body:        []byte(`{"prompt":"draw"}`),
		Credentials: &domain.Credentials{APIKey: "key"},
	})
	if status != http.StatusBadGateway || err == nil || !strings.Contains(err.Error(), "exceeds size limit") {
		t.Fatalf("status=%d err=%v", status, err)
	}
}

func TestImageProviderEditEnforcesAggregateInputLimit(t *testing.T) {
	provider := NewImageProvider(func(_ context.Context, _ string) ([]byte, string, error) {
		return make([]byte, 2<<20), "image/png", nil
	})
	_, status, err := provider.Edit(context.Background(), port.ImageRequest{
		Model: "gemini-3.1-flash-image",
		Body: []byte(`{
			"prompt":"combine",
			"images":["https://example.test/1","https://example.test/2","https://example.test/3","https://example.test/4"]
		}`),
		Credentials: &domain.Credentials{APIKey: "key"},
	})
	if status != http.StatusBadRequest || err == nil || !strings.Contains(err.Error(), "aggregate input size limit") {
		t.Fatalf("status = %d, err = %v", status, err)
	}
}

func TestPrepareLoggedGeminiBodyRedactsInlineImageData(t *testing.T) {
	shared.SetLogBodiesEnabled(true)
	t.Cleanup(func() { shared.SetLogBodiesEnabled(false) })
	logged := prepareLoggedGeminiBody([]byte(`{
		"contents":[{"parts":[{"inline_data":{"mime_type":"image/png","data":"secret-input"}}]}],
		"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"secret-output"}}]}}]
	}`))
	if strings.Contains(logged, "secret-input") || strings.Contains(logged, "secret-output") {
		t.Fatalf("logged body leaked image bytes: %s", logged)
	}
	if !strings.Contains(logged, "***REDACTED***") {
		t.Fatalf("logged body did not contain redaction marker: %s", logged)
	}
}

func TestCapabilitiesVaryForGemini25(t *testing.T) {
	provider := NewImageProvider(nil)
	if got := provider.Capabilities("gemini-2.5-flash-image").MaxReferences; got != 3 {
		t.Fatalf("max references = %d", got)
	}
	if got := provider.Capabilities("gemini-3.1-flash-image").MaxReferences; got != 14 {
		t.Fatalf("max references = %d", got)
	}
}
