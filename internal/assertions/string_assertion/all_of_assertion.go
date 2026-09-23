package string_assertion

import (
	"github.com/codecrafters-io/tester-utils/logger"
)

// AllOfAssertion runs several assertions against one value, failing on the
// first that does.
//
// A test case makes a single assertion against a single program run, so this is
// how a stage checks more than one property without paying for a second run.
type AllOfAssertion struct {
	Assertions []StringAssertion
}

func (a AllOfAssertion) Run(actualValue string, logger *logger.Logger) error {
	for _, assertion := range a.Assertions {
		if err := assertion.Run(actualValue, logger); err != nil {
			return err
		}
	}

	return nil
}
