package utils

import (
	"fmt"
	"math"
	"os"

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

func GetBypassPermissionsSettings() []byte {
	return []byte(`{
    "permissions": {
        "defaultMode": "bypassPermissions"
    }
}`)
}
