package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestImageExecutionErrorType(t *testing.T) {
	if got := imageExecutionErrorType(stdhttp.StatusBadRequest); got != "invalid_request_error" {
		t.Fatalf("400 error type = %q", got)
	}
	if got := imageExecutionErrorType(stdhttp.StatusTooManyRequests); got != "api_error" {
		t.Fatalf("429 error type = %q", got)
	}
}

func TestSelectImageCredentialsRejectsDisallowedPinnedConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("apiKeyAllowedConnectionIDs", []string{"allowed-connection"})
	c.Set("apiKeyAllowedModels", []string{"minimax/image-01"})

	_, err := selectImageCredentials(
		c,
		nil,
		"minimax",
		"image-01",
		"minimax/image-01@forbidden-connection",
		"forbidden-connection",
		"",
	)
	if err == nil || !strings.Contains(err.Error(), "connection not allowed") {
		t.Fatalf("error = %v", err)
	}
}

func TestSelectImageCredentialsRejectsDisallowedPinnedModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("apiKeyAllowedConnectionIDs", []string{"allowed-connection"})
	c.Set("apiKeyAllowedModels", []string{"minimax/other-model"})

	_, err := selectImageCredentials(
		c,
		nil,
		"minimax",
		"image-01",
		"minimax/image-01@allowed-connection",
		"allowed-connection",
		"",
	)
	if err == nil || !strings.Contains(err.Error(), "model not allowed") {
		t.Fatalf("error = %v", err)
	}
}
