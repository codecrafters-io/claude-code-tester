package proxy_server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
)

type validator struct {
	allowedModelPrefix string
}

type requestBody struct {
	Model string `json:"model"`
}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (v *validator) wrapProxy(proxy *httputil.ReverseProxy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		bodyBytes, err := v.readRequestBody(r)

		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		if v.isModelInvalid(bodyBytes) {
			v.writeModelNotFoundError(w)
			return
		}

		proxy.ServeHTTP(w, r)
	})
}

func (v *validator) readRequestBody(r *http.Request) ([]byte, error) {
	bodyBytes, err := io.ReadAll(r.Body)

	if err != nil {
		return nil, err
	}

	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return bodyBytes, nil
}

func (v *validator) isModelInvalid(requestBodyInbytes []byte) bool {
	var reqBody requestBody

	if err := json.Unmarshal(requestBodyInbytes, &reqBody); err != nil {
		return false
	}

	if reqBody.Model == "" {
		return false
	}

	return !strings.HasPrefix(reqBody.Model, v.allowedModelPrefix)
}

func (v *validator) writeModelNotFoundError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)

	var errorResp errorResponse
	errorResp.Error.Message = "Model not found"
	errorResp.Error.Type = "invalid_request_error"

	json.NewEncoder(w).Encode(errorResp)
}
