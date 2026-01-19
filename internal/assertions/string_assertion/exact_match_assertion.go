package string_assertion

import "fmt"

type ExactMatchAssertion struct {
	ExpectedValue string
}

func (a ExactMatchAssertion) Run(actualValue string) error {
	if actualValue != a.ExpectedValue {
		return fmt.Errorf("Expected %q, got %q", a.ExpectedValue, actualValue)
	}

	return nil
}
