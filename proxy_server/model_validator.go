package proxy_server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type requestBody struct {
	Model *string `json:"model"`
}

// modelValidator validates that the model in the request body is "anthropic/claude-haiku-4.5"
// Returns error ONLY when model field exists and is NOT "anthropic/claude-haiku-4.5"
func modelValidator(r *http.Request) (ok bool, errorMessage string) {
	if r.Body == nil {
		return true, ""
	}

	requestBodyBytes, err := io.ReadAll(r.Body)

	if err != nil {
		return false, "Failed to read request body"
	}

	r.Body = io.NopCloser(bytes.NewBuffer(requestBodyBytes))

	var requestBody requestBody

	if err := json.Unmarshal(requestBodyBytes, &requestBody); err != nil {
		return true, ""
	}

	if requestBody.Model == nil {
		return true, ""
	}

	if !strings.HasPrefix(*requestBody.Model, "anthropic/claude-haiku") {
		return false, fmt.Sprintf("%s is not a valid model ID", *requestBody.Model)
	}

	return true, ""
}
