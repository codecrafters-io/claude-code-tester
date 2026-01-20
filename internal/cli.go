package internal

import (
	"os"

	testerutils "github.com/codecrafters-io/tester-utils"
)

func RunCLI(env map[string]string) int {
	// Set environment variables for users' program
	os.Setenv("OPENROUTER_BASE_URL", "http://localhost:10000/api/v1/")
	os.Setenv("OPENROUTER_API_KEY", "dummy-openrouter-key")

	return testerutils.RunCLI(env, testerDefinition)
}
