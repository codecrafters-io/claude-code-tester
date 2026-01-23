package string_assertion

import (
	"fmt"

	"github.com/codecrafters-io/tester-utils/logger"
)

type ExactMatchAssertion struct {
	ExpectedValue string
}

func (a ExactMatchAssertion) Run(actualValue string, logger *logger.Logger) error {
	if actualValue != a.ExpectedValue {
		return fmt.Errorf("Expected %q, got %q", a.ExpectedValue, actualValue)
	}

	logger.Successf("✔ Value is %q", a.ExpectedValue)
	return nil
}
