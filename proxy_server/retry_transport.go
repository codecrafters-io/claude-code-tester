package proxy_server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenRouter can close a non-streaming 200 response after only keep-alive
// whitespace. Retry at the upstream boundary so learners do not have to, and
// the recorder still sees just one request from their program.
type emptyResponseRetryTransport struct {
	base http.RoundTripper
}

func (t *emptyResponseRetryTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Method != http.MethodPost || request.Body == nil {
		return t.base.RoundTrip(request)
	}

	requestBody, err := io.ReadAll(request.Body)
	request.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read upstream request body: %w", err)
	}
	request = request.Clone(request.Context())
	request.Body = io.NopCloser(bytes.NewReader(requestBody))

	var options struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(requestBody, &options); err != nil || options.Stream {
		return t.base.RoundTrip(request)
	}
	request.Body.Close()

	const maxAttempts = 3
	for attempt := 0; ; attempt++ {
		if err := request.Context().Err(); err != nil {
			return nil, err
		}
		upstreamRequest := request.Clone(request.Context())
		upstreamRequest.Body = io.NopCloser(bytes.NewReader(requestBody))
		response, err := t.base.RoundTrip(upstreamRequest)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusOK || strings.HasPrefix(strings.ToLower(response.Header.Get("Content-Type")), "text/event-stream") {
			return response, nil
		}

		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read upstream response body: %w", err)
		}
		if len(bytes.Trim(body, " \t\r\n")) != 0 {
			response.Body = io.NopCloser(bytes.NewReader(body))
			return response, nil
		}
		if attempt == maxAttempts-1 {
			return nil, fmt.Errorf("Codecrafters Internal Error - OpenRouter returned an empty 200 OK response after %d attempts", maxAttempts)
		}

		timer := time.NewTimer((250 * time.Millisecond) << attempt)
		select {
		case <-request.Context().Done():
			timer.Stop()
			return nil, request.Context().Err()
		case <-timer.C:
		}
	}
}
