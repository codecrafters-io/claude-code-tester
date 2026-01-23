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

func (a FileDoesNotExistAssertion) Run(absolutePath string, logger *logger.Logger, shortFilePathConverter func(string) string) error {
	_, err := os.Stat(absolutePath)
	shortFilePath := shortFilePathConverter(absolutePath)

	if err == nil {
		return fmt.Errorf("Expected file %s to not exist, but it exists", shortFilePath)
	}

	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("Failed to check existence of %s: %v", shortFilePath, err)
	}

	logger.Successf("✔ File %s does not exist", shortFilePath)
	return nil
}
