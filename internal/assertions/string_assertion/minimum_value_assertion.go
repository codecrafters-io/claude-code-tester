package string_assertion

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/codecrafters-io/tester-utils/logger"
)

type MinimumValueAssertion struct {
	ExpectedMinimumValue int
}

func (a MinimumValueAssertion) Run(actualValue string, logger *logger.Logger) error {
	integerValue, err := strconv.Atoi(lastNonEmptyLine(actualValue))

	if err != nil {
		return fmt.Errorf("Expected integer value, got %q", actualValue)
	}

	if integerValue < a.ExpectedMinimumValue {
		return fmt.Errorf("Expected value to be at least %d, got %d", a.ExpectedMinimumValue, integerValue)
	}

	logger.Successf("✔ Value is at least %d", a.ExpectedMinimumValue)
	return nil
}

// lastNonEmptyLine is what the value is read from, because a model asked for a
// bare number often narrates its way to one and puts the number last. Insisting
// on the whole output being the number fails submissions over the model's
// choice of words rather than over anything the submission did.
func lastNonEmptyLine(value string) string {
	lines := strings.Split(value, "\n")

	for index := len(lines) - 1; index >= 0; index-- {
		if trimmed := strings.TrimSpace(lines[index]); trimmed != "" {
			return trimmed
		}
	}

	return ""
}
