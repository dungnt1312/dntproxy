package byteplus

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
)

type bytePlusRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn bytePlusRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type bytePlusZeroReader struct{}

func (bytePlusZeroReader) Read(buffer []byte) (int, error) {
	for index := range buffer {
		buffer[index] = 0
	}
	return len(buffer), nil
}

func TestResolveImageEndpoint(t *testing.T) {
	tests := map[string]string{
		"":                            DefaultBaseURL + "/images/generations",
		"https://ark.example/api/v3/": "https://ark.example/api/v3/images/generations",
		"https://ark.example/api/v3/images/generations":    "https://ark.example/api/v3/images/generations",
		"https://ark.example/prefix?region=ap-southeast-1": "https://ark.example/prefix/images/generations?region=ap-southeast-1",
	}
	for input, want := range tests {
		got, err := ResolveImageEndpoint(input)
		if err != nil || got != want {
			t.Fatalf("ResolveImageEndpoint(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
}

func TestImageClientExecute(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/images/generations" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"seedream"`) {
			t.Errorf("body = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://example.com/result.png"}]}`))
	}))
	defer server.Close()

	client := &ImageClient{HTTPClient: server.Client()}
	results, status, err := client.Execute(context.Background(), []byte(`{"model":"seedream"}`), &domain.Credentials{
		BaseURL: server.URL + "/api/v3",
		APIKey:  "secret",
	}, nil)
	if err != nil || status != http.StatusOK || len(results) != 1 {
		t.Fatalf("results=%#v status=%d err=%v", results, status, err)
	}
}

func TestImageClientExecuteErrorEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"InvalidParameter","message":"bad size"}}`))
	}))
	defer server.Close()

	client := &ImageClient{HTTPClient: server.Client()}
	_, status, err := client.Execute(context.Background(), []byte(`{}`), &domain.Credentials{
		BaseURL: server.URL,
		APIKey:  "secret",
	}, nil)
	if err == nil || status != http.StatusBadRequest {
		t.Fatalf("status=%d err=%v", status, err)
	}
}

func TestImageClientExecuteHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := &ImageClient{HTTPClient: &http.Client{
		Transport: bytePlusRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}),
	}}
	_, status, err := client.Execute(ctx, []byte(`{}`), &domain.Credentials{
		BaseURL: "https://ark.example/api/v3",
		APIKey:  "secret",
	}, nil)
	if status != http.StatusBadGateway || err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("status=%d err=%v", status, err)
	}
}

func TestImageClientExecuteHonorsHTTPClientTimeout(t *testing.T) {
	const timeout = 20 * time.Millisecond
	client := &ImageClient{HTTPClient: &http.Client{
		Timeout: timeout,
		Transport: bytePlusRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}),
	}}

	started := time.Now()
	_, status, err := client.Execute(context.Background(), []byte(`{}`), &domain.Credentials{
		BaseURL: "https://ark.example/api/v3",
		APIKey:  "secret",
	}, nil)
	elapsed := time.Since(started)
	if status != http.StatusBadGateway || err == nil || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if elapsed < timeout/2 || elapsed > time.Second {
		t.Fatalf("request elapsed %v, expected configured timeout near %v", elapsed, timeout)
	}
}

func TestImageClientExecuteRejectsMalformedResponse(t *testing.T) {
	client := &ImageClient{HTTPClient: &http.Client{
		Transport: bytePlusRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"data":[`)),
				Header:     make(http.Header),
			}, nil
		}),
	}}
	_, status, err := client.Execute(context.Background(), []byte(`{}`), &domain.Credentials{
		BaseURL: "https://ark.example/api/v3",
		APIKey:  "secret",
	}, nil)
	if status != http.StatusBadGateway || err == nil || !strings.Contains(err.Error(), "parse BytePlus image response") {
		t.Fatalf("status=%d err=%v", status, err)
	}
}

func TestImageClientExecuteRejectsOversizedResponse(t *testing.T) {
	client := &ImageClient{HTTPClient: &http.Client{
		Transport: bytePlusRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(io.LimitReader(
					bytePlusZeroReader{},
					maxImageResponseSize+1,
				)),
				Header: make(http.Header),
			}, nil
		}),
	}}
	_, status, err := client.Execute(context.Background(), []byte(`{}`), &domain.Credentials{
		BaseURL: "https://ark.example/api/v3",
		APIKey:  "secret",
	}, nil)
	if status != http.StatusBadGateway || err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("status=%d err=%v", status, err)
	}
}

func TestImageProviderCapabilities(t *testing.T) {
	provider := NewImageProvider()
	if capabilities := provider.Capabilities("seedream-5-0-pro-260628"); !capabilities.Edit || capabilities.MaxReferences != 10 {
		t.Fatalf("pro capabilities = %#v", capabilities)
	}
	if capabilities := provider.Capabilities("seedream-3-0-t2i"); capabilities.Edit || capabilities.MaxReferences != 0 {
		t.Fatalf("t2i capabilities = %#v", capabilities)
	}
}
