package settings_manager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

// InitializeBypassPermissionSettings is used to initialize settings with 'bypassPermissions'
// This settings should be used when permissions are not the concern for the stages
// Eg. base stages
func InitializeBypassPermissionSettings(stageHarness *test_case_harness.TestCaseHarness) {
	InitializeSettings(stageHarness, Settings{
		Permissions: Permissions{
			DefaultMode: "bypassPermissions",
		},
	})
}

// InitializeSettings initializes ~/.claude/settings.json with given newSettings
func InitializeSettings(stageHarness *test_case_harness.TestCaseHarness, newSettings Settings) {
	oldSettings := getExistingSettingsAsBytes()

	newSettings.write()

	stageHarness.RegisterTeardownFunc(func() {
		if err := restoreOriginalSettingsFromBytes(oldSettings); err != nil {
			panic(fmt.Sprintf("Failed to restore original settings to %s: %s", getSettingsFilePath(), err))
		}
	})
}

func getSettingsFilePath() string {
	homedirPath, err := os.UserHomeDir()

	if err != nil {
		panic("Codecrafters Internal Error - Error getting user's home directory: " + err.Error())
	}

	return filepath.Join(homedirPath, ".claude", "settings.json")
}

func getExistingSettingsAsBytes() (existingSettingsContent []byte) {
	settingsFilePath := getSettingsFilePath()
	data, err := os.ReadFile(settingsFilePath)

	if err == nil {
		return data
	}

	if !os.IsNotExist(err) {
		panic(fmt.Sprintf("Codecrafters Internal Error - Failed reading file %s: %s", settingsFilePath, err.Error()))
	}

	return nil
}

func restoreOriginalSettingsFromBytes(originalSettings []byte) error {
	settingsFilePath := getSettingsFilePath()

	if originalSettings == nil {
		return os.Remove(settingsFilePath)
	}

	return os.WriteFile(settingsFilePath, originalSettings, 0644)
}
