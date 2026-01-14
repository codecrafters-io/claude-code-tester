package settings_manager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Permissions struct {
	DefaultMode string `json:"defaultMode"`
}
type Settings struct {
	Permissions Permissions `json:"permissions"`
}

func (s *Settings) write() {
	settingsFilePath := getSettingsFilePath()
	dir := filepath.Dir(settingsFilePath)

	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(fmt.Sprintf("Codecrafters Internal Error - Failed initializing directory %s: %s", dir, err.Error()))
	}

	settingsContents, err := json.MarshalIndent(s, "", "    ")

	if err != nil {
		panic(fmt.Sprintf("Codecrafters Internal Error - Failed serializing settings: %s", err.Error()))
	}

	if err := os.WriteFile(settingsFilePath, settingsContents, 0644); err != nil {
		panic(fmt.Sprintf("Codecrafters Internal Error - Failed writing to file %s: %s", settingsFilePath, err.Error()))
	}
}
