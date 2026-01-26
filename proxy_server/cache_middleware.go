package proxy_server

import (
	"bytes"
	"encoding/gob"
	"io"
	"net/http"
	"sort"

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
			var cached cachedResponseData
			dec := gob.NewDecoder(bytes.NewReader(cachedResponse))
			if err := dec.Decode(&cached); err == nil {
				for key, values := range cached.Headers {
					for _, value := range values {
						w.Header().Add(key, value)
					}
				}
				w.WriteHeader(cached.StatusCode)
				w.Write(cached.Body)
				return
			}
		}

		recorder := newResponseRecorder(w)
		next.ServeHTTP(recorder, r)

		cached := cachedResponseData{
			StatusCode: recorder.statusCode,
			Headers:    recorder.headers,
			Body:       recorder.responseBuffer.Bytes(),
		}
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		if err := enc.Encode(cached); err == nil {
			c.cache.Set(key, buf.Bytes())
		}
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

	sort.Strings(headerKeys)

	for _, key := range headerKeys {
		buf.WriteString(key)
		values := r.Header[key]
		for _, value := range values {
			buf.WriteString(value)
		}
	}

	buf.Write(body)

	return buf.Bytes(), nil
}

type cachedResponseData struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

type responseRecorder struct {
	http.ResponseWriter
	responseBuffer bytes.Buffer
	statusCode     int
	headers        http.Header
	wroteHeader    bool
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		headers:        make(http.Header),
	}
}

func (r *responseRecorder) Header() http.Header {
	return r.headers
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.statusCode = statusCode
	for key, values := range r.headers {
		for _, value := range values {
			r.ResponseWriter.Header().Add(key, value)
		}
	}
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	r.responseBuffer.Write(data)
	return r.ResponseWriter.Write(data)
}
