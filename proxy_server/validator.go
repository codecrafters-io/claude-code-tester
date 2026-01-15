package proxy_server

import (
	"net/http"
	"net/http/httputil"
)

// ValidationFunc is a function type that validates a request
// Returns: ok (bool), errorType (string), errorMessage (string)
type ValidationFunc func(*http.Request) (ok bool, errorMessage string)

type validationError struct {
	StatusCode   int
	errorMessage string
}

type validationMiddleware struct {
	allowedEndPoints   map[string]bool
	endpointValidators map[string][]ValidationFunc
}

// WrapProxy wraps the reverse proxy with validator middleware
func (v *validationMiddleware) WrapProxy(proxy *httputil.ReverseProxy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if validationError := v.validateRequest(r); validationError != nil {
			sendErrorResponse(w, *validationError)
			return
		}

		proxy.ServeHTTP(w, r)
	})
}

func (v *validationMiddleware) setAllowedEndPoints(endpoints []string) {
	v.allowedEndPoints = make(map[string]bool)
	for _, endpoint := range endpoints {
		v.allowedEndPoints[endpoint] = true
	}
}

func (v *validationMiddleware) validateRequest(r *http.Request) *validationError {
	endpoint := r.URL.Path

	if !v.allowedEndPoints[endpoint] {
		return &validationError{
			StatusCode:   403,
			errorMessage: "Unauthorized endpoint access",
		}
	}

	validators := v.endpointValidators[endpoint]

	for _, validatorFunc := range validators {
		ok, errorMessage := validatorFunc(r)
		if !ok {
			return &validationError{
				StatusCode:   400,
				errorMessage: errorMessage,
			}
		}
	}

	return nil
}

// registerEndPointValidator registers a validation function for a specific endpoint
func (v *validationMiddleware) registerEndPointValidator(endpoint string, validator ValidationFunc) {
	if v.endpointValidators == nil {
		v.endpointValidators = make(map[string][]ValidationFunc)
	}
	v.endpointValidators[endpoint] = append(v.endpointValidators[endpoint], validator)
}
