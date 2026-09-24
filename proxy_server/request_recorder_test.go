package proxy_server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Recording consumes the body, so the two halves of this test are inseparable:
// capturing the bytes is worthless if the proxy can no longer forward them.
func TestRequestRecorderRecordsBodiesAndLeavesThemReadable(t *testing.T) {
	recorder := &RequestRecorder{}

	forwardedBodies := []string{}

	handler := recorder.WrapHandler(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		forwardedBody, err := io.ReadAll(request.Body)
		assert.NoError(t, err)

		forwardedBodies = append(forwardedBodies, string(forwardedBody))
	}))

	for _, requestBody := range []string{"first", "second"} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(requestBody))
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}

	assert.Equal(t, []string{"first", "second"}, recorder.RequestBodies())
	assert.Equal(t, []string{"first", "second"}, forwardedBodies)
}

// Stages read the recording after the program has exited, so a caller mutating
// the returned slice must not be able to corrupt a later read.
func TestRequestRecorderReturnsACopy(t *testing.T) {
	recorder := &RequestRecorder{}
	recorder.appendRequestBody("first")

	recorder.RequestBodies()[0] = "mutated"

	assert.Equal(t, []string{"first"}, recorder.RequestBodies())
}

func TestRequestRecorderStartsEmpty(t *testing.T) {
	assert.Empty(t, (&RequestRecorder{}).RequestBodies())
}
