package string_assertion

import (
	"fmt"
	"strings"

	"github.com/codecrafters-io/tester-utils/logger"
)

// DoesNotContainAssertion fails if UnexpectedValue appears anywhere in the output.
//
// The comparison is case-insensitive, since the model decides the casing of its
// own prose and we only care about whether the value leaked at all.
type DoesNotContainAssertion struct {
	UnexpectedValue string
}

func (a DoesNotContainAssertion) Run(actualValue string, logger *logger.Logger) error {
	if strings.Contains(strings.ToLower(actualValue), strings.ToLower(a.UnexpectedValue)) {
		return fmt.Errorf("Expected output to not contain %q, got %q", a.UnexpectedValue, actualValue)
	}

	logger.Successf("✔ Value does not contain %q", a.UnexpectedValue)
	return nil
}
