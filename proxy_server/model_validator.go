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
	Model string `json:"model"`
}

// modelValidator returns an error only if
// Request.Body["model"] is present and is not a Haiku model
// This is done so that every other errors (invalid body / wrong schema), etc
// are handled by OpenRouter instead of the proxy
func modelValidator(r *http.Request) (ok bool, errorMessage string) {
	if r.Body == nil {
		return true, ""
	}

	requestBodyBytes, err := io.ReadAll(r.Body)

	if err != nil {
		return false, "Failed to read request body"
	}

	// Restore body
	r.Body = io.NopCloser(bytes.NewBuffer(requestBodyBytes))

	var requestBody requestBody

	// Ignore unmarshal error -> Will be handled by OpenRouter
	if err := json.Unmarshal(requestBodyBytes, &requestBody); err != nil {
		return true, ""
	}

	if !isHaikuModel(requestBody.Model) {
		return false, fmt.Sprintf("%s is not a valid model ID", requestBody.Model)
	}

	return true, ""
}

// isHaikuModel checks if the model is a valid Haiku model.
// This handles both newer naming (anthropic/claude-haiku-4.5) and
// older naming patterns (anthropic/claude-3-haiku, anthropic/claude-3.5-haiku).
func isHaikuModel(model string) bool {
	return strings.HasPrefix(model, "anthropic/claude-") && strings.Contains(model, "haiku")
}
