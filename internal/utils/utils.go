package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/codecrafters-io/tester-utils/logger"
	"github.com/codecrafters-io/tester-utils/random"
)

// CreateTempDirWithPrefix creates a random working directory using codecrafters-io/tester-utils/random
// Returns the path to the directory
func CreateTempDirWithPrefix(prefix string) string {
	wordPrefix := random.RandomWord()
	integerSuffix := random.RandomInt(1, 1000)

	dirPath := fmt.Sprintf("/tmp/%s-%s-%d", prefix, wordPrefix, integerSuffix)

	// Remove directory if it already exists -> Could have been a leftover from an interrupted run
	err := os.RemoveAll(dirPath)
	if err != nil {
		panic("Codecrafters Internal Error - Failed to remove existing directory " + dirPath)
	}

	// Create the directory
	err = os.Mkdir(dirPath, 0755)

	if err != nil {
		panic("Codecrafters Internal Error - Failed to create temporary workspace directory: " + err.Error())
	}

	return dirPath
}

// GetPromptWithGuardRailPrompt returns a prompt randomly chosen from promptChoices and appends
// the provided guardRailPrompt and returns the result
func GetPromptWithGuardRailPrompt(promptChoices []string, guardRailPrompt string) string {
	prompt := random.RandomElementFromArray(promptChoices)

	if guardRailPrompt != "" {
		guardRailPrompt = " " + guardRailPrompt
	}

	return fmt.Sprintf("%s%s", prompt, guardRailPrompt)
}

// MustCreateDirWithLogging creates a directory with given path and logs the creation
func MustCreateDirWithLogging(path string, logger *logger.Logger) {
	err := os.MkdirAll(path, 0755)
	if err != nil {
		panic(fmt.Sprintf("Codecrafters Internal Error - Failed to create directory %s: %v", path, err))
	}

	dirName := filepath.Base(path)
	parentDir := filepath.Dir(path)

	logger.Infof("Created %q inside %q", dirName, parentDir)
}

// MustCreateFileWithContentsWithLogger creates a file with given path and contents, and logs the file contents using the logger
func MustCreateFileWithContentsWithLogger(path string, contents string, logger *logger.Logger) {
	file, err := os.Create(path)

	if err != nil {
		panic(fmt.Sprintf("Codecrafters Internal Error - Failed to create file %s: %v", path, err))
	}

	defer file.Close()
	_, err = file.WriteString(contents)

	if err != nil {
		panic(fmt.Sprintf("Codecrafters Internal Error - Failed to write to file %s: %v", path, err))
	}

	fileName := filepath.Base(path)
	fileDir := filepath.Dir(path)

	logger.Infof("Created %q inside %q", fileName, fileDir)

	logger.WithAdditionalSecondaryPrefix(fileName, func() {
		logger.Plainf("%s", contents)
	})
}
