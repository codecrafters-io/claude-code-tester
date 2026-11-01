package proxy_server

import (
	"bytes"
	"io"
	"net/http"
	"slices"
	"sync"
)

// RequestRecorder keeps a copy of every request body that reaches the proxy.
type RequestRecorder struct {
	mutex         sync.Mutex
	requestBodies []string
}

// WrapHandler copies each request body before passing the request on. The body
// is a single-use reader, so it has to be replaced with one over the same bytes
// or there's nothing left for the proxy to forward.
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
