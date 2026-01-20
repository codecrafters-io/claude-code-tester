package internal

import (
	"path/filepath"

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
	workspace_manager.BootstrapExecutableWorkspace(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 45 * 1000
	workspaceDirPath := stageHarness.Executable.WorkingDir

	appDirPath := filepath.Join(workspaceDirPath, "app")
	utils.MustCreateDirWithLogging(appDirPath, stageHarness.Logger)

	mainFilePath := filepath.Join(appDirPath, "main.py")
	mainContent := `def add(a, b):
    return a + b

def area_of_square(n):
    return n * n`
	utils.MustCreateFileWithContentsWithLogger(mainFilePath, mainContent, stageHarness.Logger)

	type testFile struct {
		name         string
		buggyContent string
		fixedContent string
	}

	testFiles := []testFile{
		{
			name: "test_arithmetic.py",
			buggyContent: `from main import add
assert add(1, 1) == 3`,
			fixedContent: `from main import add
assert add(1, 1) == 2`,
		},
		{
			name: "test_geometry.py",
			buggyContent: `from main import area_of_square
assert area_of_square(5) == 55`,
			fixedContent: `from main import area_of_square
assert area_of_square(5) == 25`,
		},
	}

	for _, tf := range testFiles {
		testFilePath := filepath.Join(appDirPath, tf.name)
		utils.MustCreateFileWithContentsWithLogger(testFilePath, tf.buggyContent, stageHarness.Logger)
	}

	promptTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt: utils.GetPromptWithGuardRailPrompt(
			[]string{
				"Find files in `app` directory that start with 'test' and fix the bugs in them.",
				"Find all files in `app` directory that start with 'test' and fix any bugs you find.",
				"Use glob to find files in `app` directory that start with 'test' and fix the bugs in those files.",
				"Find all files in `app` directory that start with 'test' using glob patterns and fix the bugs in them.",
				"Identify files in `app` directory that start with 'test' and fix any bugs you discover.",
			},
			"",
		),
		ExpectedExitCode: 0,
	}

	// Run the test case to execute the prompt
	if err := promptTestCase.Run(stageHarness); err != nil {
		return err
	}

	// Assert each test file has been fixed
	for _, tf := range testFiles {
		testFilePath := filepath.Join(appDirPath, tf.name)
		fileAssertion := filesystem_assertion.FileContentsAssertion{
			ExpectedContents: tf.fixedContent,
		}
		if err := fileAssertion.Run(testFilePath, stageHarness.Logger); err != nil {
			return err
		}
	}

	return nil
}
