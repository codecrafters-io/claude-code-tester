package filesystem_assertion

import "github.com/codecrafters-io/tester-utils/logger"

type FileSystemAssertion interface {
	// Run takes absFilePath, logger, and shortFilePathGetter
	// absPath is the absolute path of the file
	// logger is the logger object using which the success/failure logs are logged
	// shortFilePathGetter is a function that converts absolute path to a short file path suitable for logging
	Run(absFilePath string, logger *logger.Logger, shortFilePathConverter func(string) string) error
}
