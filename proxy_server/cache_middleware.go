package proxy_server

import (
	"bytes"
	"io"
	"net/http"

	"github.com/codecrafters-io/tester-utils/tester_cache"
)

type cacheMiddleware struct {
	cache *tester_cache.TesterCache
}

func newCacheMiddleware(testerCache *tester_cache.TesterCache) *cacheMiddleware {
	return &cacheMiddleware{
		cache: testerCache,
	}
}

func (c *cacheMiddleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBytes, err := serializeRequest(r)
		if err != nil {
			http.Error(w, "Failed to read request", http.StatusInternalServerError)
			return
		}

		key := getSha256HashString(requestBytes)

		cachedResponse, found := c.cache.Get(key)

		if found {
			w.Write(cachedResponse)
			return
		}

		recorder := newResponseRecorder(w)
		next.ServeHTTP(recorder, r)

		c.cache.Set(key, recorder.responseBuffer.Bytes())
	})
}

// serializeRequest converts the request to byte slice
func serializeRequest(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)

	if err != nil {
		return nil, err
	}

	r.Body.Close()

	r.Body = io.NopCloser(bytes.NewBuffer(body))

	var buf bytes.Buffer

	buf.WriteString(r.Method)
	buf.WriteString(r.URL.String())

	headerKeys := make([]string, 0, len(r.Header))

	for key := range r.Header {
		headerKeys = append(headerKeys, key)
	}

	for key, values := range r.Header {
		buf.WriteString(key)
		for _, value := range values {
			buf.WriteString(value)
		}
	}

	buf.Write(body)

	return buf.Bytes(), nil
}

type responseRecorder struct {
	http.ResponseWriter
	responseBuffer bytes.Buffer
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
	}
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	r.responseBuffer.Write(data)
	return r.ResponseWriter.Write(data)
}
