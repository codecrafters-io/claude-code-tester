package internal

import (
	"fmt"

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
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 30 * 1000

	workspaceManager := workspace_manager.NewWorkspaceManager()
	workspaceManager.BootstrapExecutableWorkspace(stageHarness)

	fileName := fmt.Sprintf("%s.py", random.RandomWord())
	fileContents := random.RandomElementFromArray([]string{
		"print('Hello, World!')",
		"print('Hello, program!')",
		"print('Hello there!')",
	})

	workspaceManager.MustCreateFilesWithLogger(
		[]workspace_manager.WorkspaceFile{
			{
				RelativePath: fileName,
				Content:      fileContents,
				FileMode:     0644,
			},
		},
		stageHarness.Logger,
	)

	prompt := utils.GetPromptWithGuardRailPrompt(
		[]string{
			fmt.Sprintf("What is the content of `%s`?", fileName),
			fmt.Sprintf("Read `%s` and return its contents.", fileName),
			fmt.Sprintf("Show me what is inside `%s`.", fileName),
			fmt.Sprintf("What does `%s` contain?", fileName),
		},
		"Respond with only file contents, no surrounding text/backticks.",
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
