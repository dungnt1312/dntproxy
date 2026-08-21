package commandcode

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dungnt/dntproxy/internal/adapter/shared"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

// Executor translates OpenAI chat requests to Command Code /alpha/generate.
type Executor struct{}

func NewExecutor() *Executor {
	return &Executor{}
}

func (e *Executor) Execute(ctx context.Context, model string, body []byte, credentials *domain.Credentials, reqlog port.RequestLogger) (io.ReadCloser, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	apiKey := apiKeyFromCredentials(credentials)
	if apiKey == "" {
		return nil, http.StatusUnauthorized, fmt.Errorf("commandcode api key required")
	}

	ccBody, err := buildRequest(model, body)
	if err != nil {
		return nil, http.StatusBadRequest, err
	}
	baseURL := ""
	if credentials != nil {
		baseURL = credentials.BaseURL
	}
	req, loggedBody, err := newUpstreamRequest(ctx, baseURL, apiKey, ccBody)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	reqlog.SetBodies(shared.PrepareLoggedBody(loggedBody), "")

	start := time.Now()
	resp, err := shared.StreamingHTTPClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		reqlog.Upstream(req.URL.String(), http.MethodPost, http.StatusBadGateway, duration, err)
		return nil, http.StatusBadGateway, fmt.Errorf("commandcode request failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, errRead := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		resp.Body.Close()
		respBody := "Unknown error"
		if errRead == nil {
			respBody = string(bodyBytes)
		}
		reqlog.SetBodies("", shared.PrepareLoggedBody(bodyBytes))
		errUpstream := fmt.Errorf("%s", respBody)
		reqlog.Upstream(req.URL.String(), http.MethodPost, resp.StatusCode, duration, errUpstream)
		return nil, resp.StatusCode, fmt.Errorf("%s", formatUpstreamError(resp.StatusCode, respBody))
	}

	br := bufio.NewReaderSize(resp.Body, 64*1024)
	state := newStreamState(ccBody.Params.Model, time.Now().Unix())
	prelude, done, failStatus, failErr := readUntilFirstSSE(br, state)
	duration = time.Since(start)
	if failErr != nil && failErr != io.EOF {
		resp.Body.Close()
		if failStatus == 0 {
			failStatus = http.StatusBadGateway
		}
		reqlog.Upstream(req.URL.String(), http.MethodPost, failStatus, duration, failErr)
		return nil, failStatus, failErr
	}
	if failErr == io.EOF && prelude == "" {
		reason := "stop"
		prelude = writeChunk(state, map[string]any{}, &reason) + "data: [DONE]\n\n"
		done = true
	}
	reqlog.Upstream(req.URL.String(), http.MethodPost, http.StatusOK, duration, nil)

	pr, pw := io.Pipe()
	go func() {
		defer resp.Body.Close()
		var pumpErr error
		if prelude != "" {
			_, pumpErr = io.WriteString(pw, prelude)
		}
		if pumpErr == nil && !done {
			pumpErr = pumpNDJSONReader(br, pw, state)
		}
		if state.promptTokens > 0 || state.completionToks > 0 {
			reqlog.SetUsage(state.promptTokens, state.completionToks, "commandcode_usage")
		}
		if pumpErr != nil {
			_ = pw.CloseWithError(pumpErr)
			return
		}
		_ = pw.Close()
	}()
	return pr, http.StatusOK, nil
}
