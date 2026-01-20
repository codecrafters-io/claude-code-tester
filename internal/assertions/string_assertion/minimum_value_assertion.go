package string_assertion

import (
	"fmt"
	"strconv"

	"github.com/codecrafters-io/tester-utils/logger"
)

type MinimumValueAssertion struct {
	ExpectedMinimumValue int
}

func (a MinimumValueAssertion) Run(actualValue string, logger *logger.Logger) error {
	integerValue, err := strconv.Atoi(actualValue)

	if err != nil {
		return fmt.Errorf("Expected integer value, got %q", actualValue)
	}

	if integerValue < a.ExpectedMinimumValue {
		return fmt.Errorf("Expected value to be at least %d, got %d", a.ExpectedMinimumValue, integerValue)
	}

	logger.Successf("✔ Value is at least %d", a.ExpectedMinimumValue)
	return nil
}
