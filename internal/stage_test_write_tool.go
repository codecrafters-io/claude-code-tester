package internal

import (
	"fmt"

	"github.com/codecrafters-io/claude-code-tester/internal/assertions/filesystem_assertion"
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
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 30 * 1000
	stageLogger := stageHarness.Logger

	workspaceManager := workspace_manager.NewWorkspaceManager()
	workspaceManager.BootstrapExecutableWorkspace(stageHarness)

	mainFileName := random.RandomElementFromArray([]string{"main.py", "start.py", "init.py"})
	readmeFileName := "README.md"

	readmeContent := fmt.Sprintf(`This is a very simple python project.
This should print "Hello world"
This contains only one file: app/%s.`, mainFileName)

	workspaceManager.MustCreateDir("app")
	workspaceManager.MustCreateFilesWithLogger([]workspace_manager.WorkspaceFile{
		{
			RelativePath: readmeFileName,
			Content:      readmeContent,
			FileMode:     0644,
		},
	}, stageHarness.Logger)

	promptTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt: utils.GetPromptWithGuardRailPrompt(
			[]string{
				"Read README.md and create the required file.",
				"From README.md, create the indicated file.",
				"Check README.md and create the file it specifies.",
				"Use README.md to create the file needed.",
			},
			"File should have 1 line. Reply with `Created the file`",
		),
		ExpectedExitCode: 0,
	}

	if err := promptTestCase.Run(stageHarness); err != nil {
		return err
	}

	stageLogger.Infof("Checking workspace contents")

	mainFileAssertion := filesystem_assertion.FileContentsAssertion{
		ExpectedContents: `print("Hello world")`,
	}

	mainFileAbsPath := workspaceManager.ConvertToAbsPath(fmt.Sprintf("app/%s", mainFileName))
	return mainFileAssertion.Run(mainFileAbsPath, stageHarness.Logger, workspaceManager.GetRelPathConverter())
}
