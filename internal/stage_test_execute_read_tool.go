package internal

import (
	"fmt"
	"path/filepath"

	"github.com/codecrafters-io/claude-code-tester/internal/assertions/string_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/utils"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/random"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testExecuteReadTool(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	workspace_manager.BootstrapExecutableWorkspace(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 30 * 1000
	workspaceDirPath := stageHarness.Executable.WorkingDir

	fileName := fmt.Sprintf("%s.py", random.RandomWord())
	filePath := filepath.Join(workspaceDirPath, fileName)
	fileContents := random.RandomElementFromArray([]string{
		"print('Hello, World!')",
		"print('Hello, program!)",
		"print('Hello there!')",
	})

	utils.MustCreateFileWithContentsWithLogger(
		filePath,
		fileContents,
		stageHarness.Logger,
	)

	prompt := utils.GetPromptWithGuardRailPrompt(
		[]string{
			fmt.Sprintf("What is the content of the file `%s`", fileName),
			fmt.Sprintf("Read the file `%s` and return its contents.", fileName),
			fmt.Sprintf("Show me what is inside `%s`.", fileName),
			fmt.Sprintf("What does the file `%s` contain?", fileName),
		},
		"Print File contents only. Nothing more. No backticks either.",
	)

	promptTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt:      prompt,
		ExpectedExitCode: 0,
		StdoutAssertion: string_assertion.ExactMatchAssertion{
			ExpectedValue: fileContents,
		},
	}

	return promptTestCase.Run(stageHarness)
}
