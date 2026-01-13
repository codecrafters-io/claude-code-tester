package workspace_manager

import (
	"os"
	"path/filepath"

	"github.com/codecrafters-io/claude-code-tester/internal/utils"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func BootstrapExecutableWorkspace(stageHarness *test_case_harness.TestCaseHarness) {
	workspaceDirPath := utils.CreateTemporaryDirectory()
	stageHarness.Executable.WorkingDir = workspaceDirPath
	stageHarness.Logger.Infof("Setting the workspace directory to %q", workspaceDirPath)

	// Convert path to absolute: This is so that the absolute path is resolved early
	// since we set the executable's working directory to be a random one
	absolutePath, err := filepath.Abs(stageHarness.Executable.Path)

	if err != nil {
		panic("Codecrafters Internal Error - Failed to convert executable path to absolute: " + err.Error())
	}

	stageHarness.Executable.Path = absolutePath

	// Remove workspace directory after tests
	stageHarness.RegisterTeardownFunc(func() {
		os.RemoveAll(workspaceDirPath)
	})
}
