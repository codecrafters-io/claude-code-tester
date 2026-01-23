package filesystem_assertion

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/codecrafters-io/tester-utils/logger"
)

type FileContentsAssertion struct {
	ExpectedContents string
}

func (a FileContentsAssertion) Run(absolutePath string, logger *logger.Logger, shortFilePathConverter func(string) string) error {
	_, err := os.Stat(absolutePath)
	shortFilePath := shortFilePathConverter(absolutePath)

	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Expected file %s does not exist", shortFilePath)
		}

		return fmt.Errorf("Failed to check file %s: %v", shortFilePath, err)
	}

	contents, err := os.ReadFile(absolutePath)

	if err != nil {
		return fmt.Errorf("Failed to read file %s: %v", shortFilePath, err)
	}

	// Trim space from the right (File could have an extra \n: That's fine)
	fileContentsTrimmed := strings.TrimRightFunc(string(contents), unicode.IsSpace)

	if fileContentsTrimmed != a.ExpectedContents {
		logger.Plainf("Expected contents:")
		logger.WithAdditionalSecondaryPrefix(shortFilePath, func() {
			logger.Plainf("%s", a.ExpectedContents)
		})

		logger.Errorf("Actual contents:")
		logger.WithAdditionalSecondaryPrefix(shortFilePath, func() {
			logger.Errorf("%s", fileContentsTrimmed)
		})

		return errors.New("Expected file contents differ from actual contents")
	}

	logger.WithAdditionalSecondaryPrefix(shortFilePath, func() {
		logger.Successf("%s", fileContentsTrimmed)
	})

	return nil
}
