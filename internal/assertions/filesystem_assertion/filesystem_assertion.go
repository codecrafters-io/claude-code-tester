package filesystem_assertion

import "github.com/codecrafters-io/tester-utils/logger"

type FileSystemAssertion interface {
	Run(filePath string, logger *logger.Logger) error
}
