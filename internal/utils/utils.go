package utils

import (
	"fmt"
	"os"

	"github.com/codecrafters-io/tester-utils/random"
)

// CreateTempDirWithPrefix creates a random working directory using codecrafters-io/tester-utils/random
// Returns the path to the directory
func CreateTempDirWithPrefix(prefix string) string {
	wordPrefix := random.RandomWord()
	integerSuffix := random.RandomInt(1, 1000)

	dirPath := fmt.Sprintf("/tmp/%s-%s-%d", prefix, wordPrefix, integerSuffix)

	// Remove any existing directory at this path first. This handles the case where
	// a previous test run was interrupted before cleanup could happen, and with
	// deterministic random seeds the same path would be generated again.
	os.RemoveAll(dirPath)

	err := os.Mkdir(dirPath, 0755)

	if err != nil {
		panic("Codecrafters Internal Error - Failed to create temporary workspace directory: " + err.Error())
	}

	return dirPath
}

// GetPromptWithGuardRailPrompt returns a prompt randomly chosen from promptChoices and appends
// the provided guardRailPrompt and returns the result
func GetPromptWithGuardRailPrompt(promptChoices []string, guardRailPrompt string) string {
	prompt := random.RandomElementFromArray(promptChoices)
	return fmt.Sprintf("%s %s", prompt, guardRailPrompt)
}
