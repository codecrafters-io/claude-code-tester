package filesystem_assertion

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/codecrafters-io/tester-utils/logger"
)

type FileDoesNotExistAssertion struct {
}

func (a FileDoesNotExistAssertion) Run(filePath string, logger *logger.Logger) error {
	_, err := os.Stat(filePath)

	if err == nil {
		return fmt.Errorf("Expected file %s to not exist, but it exists", filePath)
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("Failed to check existence of %s: %v", filePath, err)
	}

	logger.Successf("✔ File %s does not exist", filePath)
	return nil
}
