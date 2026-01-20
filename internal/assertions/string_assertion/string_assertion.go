package string_assertion

import "github.com/codecrafters-io/tester-utils/logger"

type StringAssertion interface {
	Run(value string, logger *logger.Logger) error
}
