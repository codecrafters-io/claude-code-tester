package settings_manager

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func InitializeSettings(stageHarness *test_case_harness.TestCaseHarness, newSettings Settings) {
	oldSettings := getExistingSettingsAsBytes()

	newSettings.write()

	stageHarness.RegisterTeardownFunc(func() {
		restoreOriginalSettingsFromBytes(oldSettings)
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

func restoreOriginalSettingsFromBytes(originalSettings []byte) {
	settingsFilePath := getSettingsFilePath()

	if originalSettings == nil {
		os.Remove(settingsFilePath)
		return
	}

	os.WriteFile(settingsFilePath, originalSettings, 0644)
}
