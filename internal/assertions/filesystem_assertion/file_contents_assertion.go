package filesystem_assertion

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

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

	// Trim space from the right (File could have an extra \n: That's fine)
	fileContentsTrimmed := strings.TrimRightFunc(string(contents), unicode.IsSpace)
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

		return errors.New("Expected file contents differ from actual contents")
	}

	logger.Successf("✔ File %s exists with contents:", path)

	logger.WithAdditionalSecondaryPrefix(filepath.Base(path), func() {
		logger.Successf("%s", fileContentsTrimmed)
	})

	return nil
}
