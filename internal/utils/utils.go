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
