package string_assertion

import (
	"fmt"
	"strings"

	"github.com/codecrafters-io/tester-utils/logger"
)

// ContainsAllAssertion fails unless every expected value appears in the output.
//
// The comparison is case-insensitive, since the model decides the casing of its
// own prose and we only care about whether each value reached the response.
type ContainsAllAssertion struct {
	ExpectedValues []string
}

func (a ContainsAllAssertion) Run(actualValue string, logger *logger.Logger) error {
	loweredActualValue := strings.ToLower(actualValue)

	for _, expectedValue := range a.ExpectedValues {
		if !strings.Contains(loweredActualValue, strings.ToLower(expectedValue)) {
			return fmt.Errorf("Expected output to contain %q, got %q", expectedValue, actualValue)
		}

		logger.Successf("✔ Value contains %q", expectedValue)
	}

	return nil
}
