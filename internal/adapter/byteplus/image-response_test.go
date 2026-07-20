package byteplus

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestParseImageResponseURL(t *testing.T) {
	results, err := ParseImageResponse(readFixture(t, "response-url.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].URL != "https://example.com/generated.png" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestParseImageResponseBase64(t *testing.T) {
	results, err := ParseImageResponse(readFixture(t, "response-b64.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].B64JSON != "aW1hZ2U=" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestParseImageResponseError(t *testing.T) {
	_, err := ParseImageResponse(readFixture(t, "response-error.json"))
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "InvalidParameter" {
		t.Fatalf("error = %#v", err)
	}
	if HTTPStatus(err) != http.StatusBadRequest {
		t.Fatalf("HTTPStatus = %d", HTTPStatus(err))
	}
}

func TestParseImageResponseRejectsAmbiguousOrEmptyData(t *testing.T) {
	tests := []string{
		`{"data":[]}`,
		`{"data":[{}]}`,
		`{"data":[{"url":"u","b64_json":"b"}]}`,
		`{"data":[{"error":{"code":500,"message":"failed"}}]}`,
	}
	for _, body := range tests {
		if _, err := ParseImageResponse([]byte(body)); err == nil {
			t.Fatalf("expected error for %s", body)
		}
	}
}

func TestAPIErrorMessage(t *testing.T) {
	err := (&APIError{Code: "InvalidParameter", Message: "bad size"}).Error()
	if !strings.Contains(err, "InvalidParameter") || !strings.Contains(err, "bad size") {
		t.Fatal(err)
	}
}

func TestAPIErrorRedactsSignedURLsAndDataURIs(t *testing.T) {
	err := newAPIError(&errorBody{
		Code:    []byte(`"InvalidParameter"`),
		Message: `bad https://example.com/input.png?token=secret and data:image/png;base64,c2VjcmV0`,
	})
	message := err.Error()
	if strings.Contains(message, "token=secret") || strings.Contains(message, "c2VjcmV0") {
		t.Fatalf("sensitive image source leaked: %s", message)
	}
	if !strings.Contains(message, "[redacted-url]") {
		t.Fatalf("redaction marker missing: %s", message)
	}
}
