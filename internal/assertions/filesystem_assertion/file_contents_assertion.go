package filesystem_assertion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/codecrafters-io/tester-utils/logger"
)

type FileContentsAssertion struct {
	ExpectedContents string
}

func (a FileContentsAssertion) Run(path string, logger *logger.Logger) error {
	_, err := os.Stat(path)

	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("Expected file %s does not exist", path)
		}

		return fmt.Errorf("Failed to check file %s: %v", path, err)
	}

	contents, err := os.ReadFile(path)

	if err != nil {
		return fmt.Errorf("Failed to read file %s: %v", path, err)
	}

	fileContentsTrimmed := strings.TrimSpace(string(contents))
	fileName := filepath.Base(path)

	if fileContentsTrimmed != a.ExpectedContents {
		logger.Plainf("Expected contents:")
		logger.WithAdditionalSecondaryPrefix(fileName, func() {
			logger.Plainf("%s", a.ExpectedContents)
		})

		logger.Errorf("Actual contents:")
		logger.WithAdditionalSecondaryPrefix(fileName, func() {
			logger.Errorf("%s", fileContentsTrimmed)
		})

		return fmt.Errorf("Expected file contents to be %q, got %q", a.ExpectedContents, string(contents))
	}

	logger.Successf("✔ File %s exists with contents:", path)

	logger.WithAdditionalSecondaryPrefix(filepath.Base(path), func() {
		logger.Successf("%s", fileContentsTrimmed)
	})

	return nil
}
