package proxy_server

import (
	"bytes"
	"encoding/gob"
	"io"
	"net/http"
	"sort"

	"github.com/codecrafters-io/tester-utils/tester_cache"
)

type cachedResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

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

		cachedResponseBytes, found := c.cache.Get(key)
		if found {
			cached, err := decodeCachedResponse(cachedResponseBytes)
			if err == nil {
				writeCachedResponse(w, cached)
				return
			}
		}

		recorder := newResponseRecorder(w)
		next.ServeHTTP(recorder, r)

		cached := &cachedResponse{
			StatusCode: recorder.statusCode,
			Headers:    recorder.headers,
			Body:       recorder.body.Bytes(),
		}

		if encodedResp, err := encodeCachedResponse(cached); err == nil {
			c.cache.Set(key, encodedResp)
		}
	})
}

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

	// Sort headers by key for consistent ordering
	headerKeys := make([]string, 0, len(r.Header))
	for key := range r.Header {
		headerKeys = append(headerKeys, key)
	}
	sort.Strings(headerKeys)

	for _, key := range headerKeys {
		buf.WriteString(key)
		for _, value := range r.Header[key] {
			buf.WriteString(value)
		}
	}

	buf.Write(body)

	return buf.Bytes(), nil
}

func encodeCachedResponse(resp *cachedResponse) ([]byte, error) {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(resp); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeCachedResponse(data []byte) (*cachedResponse, error) {
	var resp cachedResponse
	decoder := gob.NewDecoder(bytes.NewReader(data))

	if err := decoder.Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func writeCachedResponse(w http.ResponseWriter, cached *cachedResponse) {
	for key, values := range cached.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(cached.StatusCode)
	w.Write(cached.Body)
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode  int
	headers     http.Header
	body        bytes.Buffer
	wroteHeader bool
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		headers:        make(http.Header),
	}
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	if !r.wroteHeader {
		r.statusCode = statusCode

		for key, values := range r.ResponseWriter.Header() {
			r.headers[key] = append([]string{}, values...)
		}

		r.wroteHeader = true
	}

	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}

	r.body.Write(data)
	return r.ResponseWriter.Write(data)
}
