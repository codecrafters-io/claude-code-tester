package internal

import (
	"github.com/codecrafters-io/claude-code-tester/internal/assertions/filesystem_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/utils"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testGlobTool(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 45 * 1000
	stageLogger := stageHarness.Logger

	workspaceManager := workspace_manager.NewWorkspaceManager()
	workspaceManager.BootstrapExecutableWorkspace(stageHarness)

	mainContent := `def add(a, b):
    return a + b

def area_of_square(n):
    return n * n`

	arithmeticFileRelativePath := "app/test_arithmetic.py"
	geometryFileRelativePath := "app/test_geometry.py"

	arithmeticFileBuggyContent := `from main import add
assert add(1, 1) == 3`

	geometryFileBuggyContent := `from main import area_of_square
assert area_of_square(5) == 55`

	workspaceManager.MustCreateFilesWithLogger([]workspace_manager.WorkspaceFile{
		{
			RelativePath: "app/main.py",
			Content:      mainContent,
			FileMode:     0644,
		},
		{
			RelativePath: arithmeticFileRelativePath,
			Content:      arithmeticFileBuggyContent,
			FileMode:     0644,
		},
		{
			RelativePath: geometryFileRelativePath,
			Content:      geometryFileBuggyContent,
			FileMode:     0644,
		},
	}, stageLogger)

	promptTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt: utils.GetPromptWithGuardRailPrompt(
			[]string{
				"Fix all bugs in files in `app` that start with `test`.",
				"Find files in `app` starting with `test` and fix their bugs.",
				"Identify files in `app` that start with `test` and fix bugs.",
				"Locate files in `app` starting with `test` and fix all bugs.",
				"Fix bugs in files under `app` that start with `test`.",
			},
			"Respond with `Fixed all bugs`",
		),
		ExpectedExitCode: 0,
	}

	if err := promptTestCase.Run(stageHarness); err != nil {
		return err
	}

	stageLogger.Infof("Checking workspace contents")

	arithmeticFileFixedContent := `from main import add
assert add(1, 1) == 2`

	arithmeticFileAssertion := filesystem_assertion.FileContentsAssertion{ExpectedContents: arithmeticFileFixedContent}
	arithmeticFileAbsPath := workspaceManager.ConvertToAbsPath(arithmeticFileRelativePath)

	if err := arithmeticFileAssertion.Run(arithmeticFileAbsPath, stageLogger, workspaceManager.GetRelPathConverter()); err != nil {
		return err
	}

	geometryFileFixedContent := `from main import area_of_square
assert area_of_square(5) == 25`

	geometryFileAbsPath := workspaceManager.ConvertToAbsPath(geometryFileRelativePath)
	geometryFileAssertion := filesystem_assertion.FileContentsAssertion{ExpectedContents: geometryFileFixedContent}

	return geometryFileAssertion.Run(geometryFileAbsPath, stageLogger, workspaceManager.GetRelPathConverter())
}
