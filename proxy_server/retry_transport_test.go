package proxy_server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type trackedResponseBody struct {
	io.Reader
	closed bool
	reads  int
}

func (b *trackedResponseBody) Read(p []byte) (int, error) {
	b.reads++
	return b.Reader.Read(p)
}

func (b *trackedResponseBody) Close() error {
	b.closed = true
	return nil
}

func proxyWithUpstream(t *testing.T, upstream http.RoundTripper) (*proxyServer, *RequestRecorder) {
	t.Helper()
	t.Setenv("CODECRAFTERS_SECRET_OPENROUTER_API_KEY", "test-key")
	original := http.DefaultTransport
	http.DefaultTransport = upstream
	defer func() { http.DefaultTransport = original }()
	return newProxyServer()
}

// A flushed whitespace-only HTTP response is retried before the learner receives JSON.
func TestProxyRetriesWhitespaceOverHTTP(t *testing.T) {
	// Arrange
	var attempts atomic.Int32
	requestBody := `{"model":"anthropic/claude-haiku-4.5","messages":[]}`
	responseBody := `{"choices":[{"message":{"content":"hello"}}]}`
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, err := io.ReadAll(request.Body)
		assert.NoError(t, err)
		assert.Equal(t, requestBody, string(body))
		writer.Header().Set("Content-Type", "application/json")
		if attempts.Add(1) == 1 {
			io.WriteString(writer, " \t\n")
			writer.(http.Flusher).Flush()
			return
		}
		io.WriteString(writer, responseBody)
	}))
	defer upstream.Close()
	upstreamURL, err := url.Parse(upstream.URL)
	require.NoError(t, err)
	base := http.DefaultTransport
	proxy, recorder := proxyWithUpstream(t, roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		request = request.Clone(request.Context())
		request.URL.Scheme = upstreamURL.Scheme
		request.URL.Host = upstreamURL.Host
		return base.RoundTrip(request)
	}))
	server := httptest.NewServer(proxy.server.Handler)
	defer server.Close()

	// Act
	response, err := http.Post(server.URL+"/api/v1/chat/completions", "application/json", strings.NewReader(requestBody))
	require.NoError(t, err)
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, responseBody, string(body))
	assert.Equal(t, int32(2), attempts.Load())
	assert.Equal(t, []string{requestBody}, recorder.RequestBodies())
}

// The real proxy retries empty upstream responses without recording extra learner requests.
func TestProxyRetriesEmptyResponses(t *testing.T) {
	for _, endpoint := range []string{"chat/completions", "responses", "messages"} {
		t.Run(endpoint, func(t *testing.T) {
			// Arrange
			requestBody := `{"model":"anthropic/claude-haiku-4.5","messages":[]}`
			responseBody := `{"id":"completion","choices":[]}`
			attempts := 0
			bodies := []*trackedResponseBody{}
			proxy, recorder := proxyWithUpstream(t, roundTripperFunc(func(request *http.Request) (*http.Response, error) {
				body, err := io.ReadAll(request.Body)
				request.Body.Close()
				require.NoError(t, err)
				assert.Equal(t, requestBody, string(body))
				assert.Equal(t, "/api/v1/"+endpoint, request.URL.Path)
				assert.Equal(t, "openrouter.ai", request.URL.Host)
				assert.Equal(t, "Bearer test-key", request.Header.Get("Authorization"))
				contents := []string{"", " \t\r\n", responseBody}[attempts]
				attempts++
				response := &trackedResponseBody{Reader: strings.NewReader(contents)}
				bodies = append(bodies, response)
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: response}, nil
			}))
			response := httptest.NewRecorder()

			// Act
			proxy.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/"+endpoint, strings.NewReader(requestBody)))

			// Assert
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Equal(t, responseBody, response.Body.String())
			assert.Equal(t, 3, attempts)
			assert.Equal(t, []string{requestBody}, recorder.RequestBodies())
			for _, body := range bodies {
				assert.True(t, body.closed)
			}
		})
	}
}

// Exhausting the retry limit returns a gateway error instead of an empty success.
func TestProxyRejectsPersistentlyEmptyResponses(t *testing.T) {
	// Arrange
	attempts := 0
	proxy, _ := proxyWithUpstream(t, roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		request.Body.Close()
		attempts++
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(" \n"))}, nil
	}))
	response := httptest.NewRecorder()

	// Act
	proxy.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(`{"model":"anthropic/claude-haiku-4.5"}`)))

	// Assert
	assert.Equal(t, 3, attempts)
	assert.Equal(t, http.StatusBadGateway, response.Code)
}

// Non-empty responses and HTTP errors keep their original status and body without retries.
func TestRetryTransportPassesThroughOtherResponses(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		status int
		body   string
	}{
		{"valid JSON", http.StatusOK, " \n{\"choices\":[]}\n"},
		{"malformed JSON", http.StatusOK, "{broken"},
		{"non-JSON whitespace", http.StatusOK, "\u00a0"},
		{"bad request", http.StatusBadRequest, ""},
		{"rate limit", http.StatusTooManyRequests, `{"error":"rate limited"}`},
		{"server error", http.StatusInternalServerError, ""},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// Arrange
			attempts := 0
			transport := &emptyResponseRetryTransport{base: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
				request.Body.Close()
				attempts++
				return &http.Response{StatusCode: scenario.status, Header: http.Header{"X-Upstream": {"preserved"}}, Body: io.NopCloser(strings.NewReader(scenario.body))}, nil
			})}
			request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(`{"stream":false}`))

			// Act
			response, err := transport.RoundTrip(request)
			require.NoError(t, err)
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, 1, attempts)
			assert.Equal(t, scenario.status, response.StatusCode)
			assert.Equal(t, scenario.body, string(body))
			assert.Equal(t, "preserved", response.Header.Get("X-Upstream"))
		})
	}
}

// Streaming bodies are returned without reading or buffering them.
func TestRetryTransportDoesNotReadStreams(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		requestBody string
		contentType string
	}{
		{"stream requested", `{"stream":true}`, "application/json"},
		{"SSE returned", `{}`, "text/event-stream; charset=utf-8"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// Arrange
			body := &trackedResponseBody{Reader: strings.NewReader("data: {}\n\n")}
			attempts := 0
			transport := &emptyResponseRetryTransport{base: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
				request.Body.Close()
				attempts++
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {scenario.contentType}}, Body: body}, nil
			})}

			// Act
			response, err := transport.RoundTrip(httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(scenario.requestBody)))

			// Assert
			require.NoError(t, err)
			defer response.Body.Close()
			assert.Equal(t, 1, attempts)
			assert.Same(t, body, response.Body)
			assert.Zero(t, body.reads)
			assert.False(t, body.closed)
		})
	}
}

// Cancellation during backoff prevents another upstream attempt.
func TestRetryTransportHonorsCancellation(t *testing.T) {
	// Arrange
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	attempts := 0
	transport := &emptyResponseRetryTransport{base: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		request.Body.Close()
		attempts++
		cancel()
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(`{}`)).WithContext(ctx)

	// Act
	response, err := transport.RoundTrip(request)

	// Assert
	assert.Nil(t, response)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, attempts)
}

// Network errors retain their original error and are not retried by this transport.
func TestRetryTransportPreservesTransportErrors(t *testing.T) {
	// Arrange
	expected := errors.New("connection failed")
	attempts := 0
	transport := &emptyResponseRetryTransport{base: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		request.Body.Close()
		attempts++
		return nil, expected
	})}

	// Act
	response, err := transport.RoundTrip(httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(`{}`)))

	// Assert
	assert.Nil(t, response)
	assert.ErrorIs(t, err, expected)
	assert.Equal(t, 1, attempts)
}

// Validation failures never reach the upstream retry transport.
func TestProxyRejectsInvalidModelBeforeRetrying(t *testing.T) {
	// Arrange
	proxy, recorder := proxyWithUpstream(t, roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		t.Fatal("invalid model reached upstream")
		return nil, nil
	}))
	response := httptest.NewRecorder()
	requestBody := `{"model":"unsupported"}`

	// Act
	proxy.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(requestBody)))

	// Assert
	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Equal(t, []string{requestBody}, recorder.RequestBodies())
}
