package request_assertion

import (
	"fmt"
	"strings"

	"github.com/codecrafters-io/tester-utils/logger"
)

// SomeRequestAssertion passes when at least one request contains every value in
// ExpectedValues and none of the values in UnexpectedValues.
//
// Both halves are needed together to say anything useful. "A request carried
// this" is satisfied by a program that sends everything in one conversation;
// "and not that" is what makes it evidence of a conversation boundary.
//
// It asserts on "some request" rather than on a count or an index because the
// number of requests a correct submission sends isn't fixed — a tool call adds
// a round trip, and the user's program decides how many it needs.
//
// The comparison is case-insensitive, matching the string assertions.
type SomeRequestAssertion struct {
	ExpectedValues   []string
	UnexpectedValues []string
}

func (a SomeRequestAssertion) Run(requestBodies []string, logger *logger.Logger) error {
	for _, requestBody := range requestBodies {
		if a.matches(requestBody) {
			logger.Successf("✔ Found %s", a.description())
			return nil
		}
	}

	return fmt.Errorf("Expected to find %s, but none of the %d request(s) sent to the LLM matched", a.description(), len(requestBodies))
}

func (a SomeRequestAssertion) matches(requestBody string) bool {
	loweredRequestBody := strings.ToLower(requestBody)

	for _, expectedValue := range a.ExpectedValues {
		if !strings.Contains(loweredRequestBody, strings.ToLower(expectedValue)) {
			return false
		}
	}

	for _, unexpectedValue := range a.UnexpectedValues {
		if strings.Contains(loweredRequestBody, strings.ToLower(unexpectedValue)) {
			return false
		}
	}

	return true
}

// description renders the assertion as the phrase both its messages need.
func (a SomeRequestAssertion) description() string {
	description := fmt.Sprintf("a request containing %s", quotedList(a.ExpectedValues))

	if len(a.UnexpectedValues) > 0 {
		description += fmt.Sprintf(" but not %s", quotedList(a.UnexpectedValues))
	}

	return description
}

func quotedList(values []string) string {
	quotedValues := make([]string, len(values))

	for i, value := range values {
		quotedValues[i] = fmt.Sprintf("%q", value)
	}

	return strings.Join(quotedValues, " and ")
}
