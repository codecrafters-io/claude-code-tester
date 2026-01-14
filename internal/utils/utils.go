package utils

import (
	"fmt"
	"math"
	"os"

	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/tester-utils/random"
)

// CreateTemporaryDirectory creates a random working directory using codecrafters-io/tester-utils/random
// Returns the path to the directory
func CreateTemporaryDirectory() string {
	wordPrefix := random.RandomWord()
	integerSuffix := random.RandomInt(1, math.MaxInt)

	dirPath := fmt.Sprintf("/tmp/%s-%d", wordPrefix, integerSuffix)

	err := os.Mkdir(dirPath, 0755)

	if err != nil {
		panic("Codecrafters Internal Error - Failed to create temporary workspace directory: " + err.Error())
	}

	return dirPath
}

// GetBypassPermissionsSettings returns the settings for claude code where
// the permissions are bypassed (useful for stages where permissions are not concerned)
func GetBypassPermissionsSettings() settings_manager.Settings {
	return settings_manager.Settings{
		Permissions: settings_manager.Permissions{
			DefaultMode: "bypassPermissions",
		},
	}
}

// GetPromptWithGuardRailPrompt returns a prompt randomly chosen from promptChoices and appends
// the provided guardRailPrompt and returns the result
func GetPromptWithGuardRailPrompt(promptChoices []string, guardRailPrompt string) string {
	prompt := random.RandomElementFromArray(promptChoices)
	return fmt.Sprintf("%s %s", prompt, guardRailPrompt)
}
