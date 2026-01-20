package internal

import (
	"fmt"
	"path/filepath"

	"github.com/codecrafters-io/claude-code-tester/internal/assertions/filesystem_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/assertions/string_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/utils"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/random"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testWriteTool(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	workspace_manager.BootstrapExecutableWorkspace(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 30 * 1000
	workspaceDirPath := stageHarness.Executable.WorkingDir

	appDirPath := filepath.Join(workspaceDirPath, "app")
	utils.MustCreateDirWithLogging(appDirPath, stageHarness.Logger)

	mainFileName := random.RandomElementFromArray([]string{"main.py", "start.py", "init.py"})
	mainFilePath := filepath.Join(appDirPath, mainFileName)

	readmePath := filepath.Join(workspaceDirPath, "README.md")
	readmeContent := fmt.Sprintf(`This is a very simple python project.
This should print "Hello world"
This contains only one file: app/%s.`, mainFileName)

	utils.MustCreateFileWithContentsWithLogger(readmePath, readmeContent, stageHarness.Logger)

	expectedFileContents := `print("Hello world")`

	promptTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt: utils.GetPromptWithGuardRailPrompt(
			[]string{
				"Read README.md and create that file.",
				"Read the README.md, figure out which file to create, and create that file.",
				"Read README.md, determine the file that needs to be created, and create it.",
				"Read the README.md file, identify the file to create, and create that file.",
				"Read README.md, find out which file should be created, and create it.",
			},
			// We put the `Done` here because there is no way to guarantee fixed output for the fixtures
			// We cannot 'normalize' the output either. There is no specific pattern if we leave the output up to the LLM
			"Use single print statement inside the file. Always respond with `Done`",
		),
		StdoutAssertion: string_assertion.ExactMatchAssertion{
			ExpectedValue: "Done",
		},
		ExpectedExitCode: 0,
	}

	if err := promptTestCase.Run(stageHarness); err != nil {
		return err
	}

	fileAssertion := filesystem_assertion.FileContentsAssertion{
		ExpectedContents: expectedFileContents,
	}

	if err := fileAssertion.Run(mainFilePath, stageHarness.Logger); err != nil {
		return err
	}

	return nil
}
