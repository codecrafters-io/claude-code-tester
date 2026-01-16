package proxy_server

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse is the error response format of OpenRouter
// in case of wrong model and unauthorized endpoint access (Tested using a client)
// userId is also present in case of 'wrong model error', but has been removed here for security reasons
type ErrorResponse struct {
	Error struct {
		ErrorMessage string `json:"message"`
		ErrorCode    int    `json:"code"`
	} `json:"error"`
}

// sendErrorResponse sends a JSON error response with the given status code and error data
func sendErrorResponse(w http.ResponseWriter, validationError validationError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(validationError.StatusCode)

	response := ErrorResponse{}
	response.Error.ErrorMessage = validationError.errorMessage
	response.Error.ErrorCode = validationError.StatusCode

	json.NewEncoder(w).Encode(response)
}
