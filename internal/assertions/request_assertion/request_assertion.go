package request_assertion

import "github.com/codecrafters-io/tester-utils/logger"

// RequestAssertion checks a property of the requests the user's program sent to
// the LLM, oldest first.
//
// This is a separate interface from StringAssertion because the subject is a
// list: the interesting properties are about which request carried what, and
// concatenating them would erase the boundaries that carry the meaning.
type RequestAssertion interface {
	Run(requestBodies []string, logger *logger.Logger) error
}
