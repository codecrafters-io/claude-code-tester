package proxy_server

import (
	"encoding/json"
	"net/http"
)

// OpenRouter compatible Error response format
type ErrorResponse struct {
	Error struct {
		ErrorMessage string `json:"message"`
	} `json:"error"`
}

// sendErrorResponse sends a JSON error response with the given status code and error data
func sendErrorResponse(w http.ResponseWriter, validationError validationError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(validationError.StatusCode)

	response := ErrorResponse{}
	response.Error.ErrorMessage = validationError.errorMessage

	json.NewEncoder(w).Encode(response)
}

// {'error': {'message': 'modelName is not a valid model ID', 'code': 400}}
