package string_assertion

import (
	"testing"

	"github.com/codecrafters-io/tester-utils/logger"
	"github.com/stretchr/testify/assert"
)

func TestMinimumValueAssertionReadsTheLastNonEmptyLine(t *testing.T) {
	testCases := map[string]struct {
		actualValue string
		shouldPass  bool
	}{
		"a bare number": {
			actualValue: "3",
			shouldPass:  true,
		},
		"a number the model narrated its way to": {
			actualValue: "I have access to three skills:\n\n1. harbor\n2. kestrel\n3. onyx\n\n3",
			shouldPass:  true,
		},
		"trailing blank lines": {
			actualValue: "3\n\n",
			shouldPass:  true,
		},
		"a number followed by more prose": {
			actualValue: "3\n\nLet me know if you'd like details.",
			shouldPass:  false,
		},
		"no number at all": {
			actualValue: "I couldn't find any skills.",
			shouldPass:  false,
		},
		"a number below the minimum": {
			actualValue: "1",
			shouldPass:  false,
		},
		"no output": {
			actualValue: "",
			shouldPass:  false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			assertion := MinimumValueAssertion{ExpectedMinimumValue: 3}
			err := assertion.Run(testCase.actualValue, logger.GetQuietLogger(""))

			if testCase.shouldPass {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
