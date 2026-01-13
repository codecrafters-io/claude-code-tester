package settings_manager

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func InitializeSettings(stageHarness *test_case_harness.TestCaseHarness, newSettings []byte) {
	oldSettings := getExistingSettings()
	writeSettings(newSettings)

	stageHarness.RegisterTeardownFunc(func() {
		restoreOriginalSettings(oldSettings)
	})
}

func getSettingsFilePath() string {
	homedirPath, err := os.UserHomeDir()
	if err != nil {
		panic("Codecrafters Internal Error - Error getting user's home directory: " + err.Error())
	}

	return path.Join(homedirPath, ".claude", "settings.json")
}

func getExistingSettings() (existingSettingsContent []byte) {
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

func writeSettings(settingsContents []byte) {
	settingsFilePath := getSettingsFilePath()
	dir := filepath.Dir(settingsFilePath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(fmt.Sprintf("Codecrafters Internal Error - Failed initializing directory %s: %s", dir, err.Error()))
	}

	if err := os.WriteFile(settingsFilePath, settingsContents, 0644); err != nil {
		panic(fmt.Sprintf("Codecrafters Internal Error - Failed writing to file %s: %s", settingsFilePath, err.Error()))
	}
}

func restoreOriginalSettings(originalSettings []byte) {
	if originalSettings != nil {
		writeSettings(originalSettings)
		return
	}

	os.Remove(getSettingsFilePath())
}
