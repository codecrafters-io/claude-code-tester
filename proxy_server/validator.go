package proxy_server

import (
	"fmt"
	"net/http"
	"net/http/httputil"
)

// ValidationFunc is a function type that validates a request
// It should return (true, "") if the validation was successful
// and (false, errorMessage) if unsuccessful
type ValidationFunc func(*http.Request) (ok bool, errorMessage string)

type validationError struct {
	StatusCode   int
	errorMessage string
}

type validationMiddleware struct {
	endPointAndValidators map[string][]ValidationFunc
}

// WrapProxy wraps the reverse proxy with validator middleware
func (v *validationMiddleware) WrapProxy(proxy *httputil.ReverseProxy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := v.validateRequest(r); err != nil {
			sendErrorResponse(w, *err)
			return
		}

		proxy.ServeHTTP(w, r)
	})
}

// setEndPointsWithValidators sets allowed endpoints and their validators in one call.
// Keys are endpoint paths; values are the validator to use, or nil to use emptyValidator.
func (v *validationMiddleware) setEndPointsWithValidators(config map[string][]ValidationFunc) {
	v.endPointAndValidators = config
}

func (v *validationMiddleware) validateRequest(r *http.Request) *validationError {
	endpoint := r.URL.Path

	validators, ok := v.endPointAndValidators[endpoint]

	if !ok {
		return &validationError{
			StatusCode:   404,
			errorMessage: fmt.Sprintf("Endpoint not found: %s", endpoint),
		}
	}

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
