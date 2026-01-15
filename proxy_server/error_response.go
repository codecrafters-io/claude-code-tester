package proxy_server

import (
	"encoding/json"
	"net/http"
)

// The error response format was derived using a python script
// By testing against a wrong model and unauthorized endpoint
// userId was present, which have been removed here for security reasons

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
