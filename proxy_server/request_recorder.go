package proxy_server

import (
	"bytes"
	"io"
	"net/http"
	"slices"
	"sync"
)

// RequestRecorder keeps a copy of every request body that reaches the proxy.
//
// Most stages can separate a correct submission from a broken one by looking at
// what the user's program printed. A few can't: running a skill in a forked
// context produces the same answer as not forking at all, and the only place
// the difference exists is the shape of the requests that went out.
//
// Recording is passive. It never inspects, blocks, or alters a request — a
// validator that rejects one makes the proxy answer 400, which surfaces to the
// user as an upstream API error instead of a tester failure message.
type RequestRecorder struct {
	mutex         sync.Mutex
	requestBodies []string
}

// WrapHandler returns a handler that copies each request body before passing
// the request on.
//
// The body is a single-use reader, so consuming it here would leave nothing for
// the proxy to forward. It's replaced with an equivalent reader over the same
// bytes, the way modelValidator does before unmarshalling.
func (r *RequestRecorder) WrapHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Body != nil {
			if requestBodyBytes, err := io.ReadAll(request.Body); err == nil {
				request.Body = io.NopCloser(bytes.NewBuffer(requestBodyBytes))
				r.appendRequestBody(string(requestBodyBytes))
			}
		}

		next.ServeHTTP(writer, request)
	})
}

// RequestBodies returns the bodies recorded so far, oldest first.
func (r *RequestRecorder) RequestBodies() []string {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return slices.Clone(r.requestBodies)
}

func (r *RequestRecorder) appendRequestBody(requestBody string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.requestBodies = append(r.requestBodies, requestBody)
}
