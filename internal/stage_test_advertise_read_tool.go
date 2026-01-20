package internal

import (
	"github.com/codecrafters-io/claude-code-tester/internal/assertions/string_assertion"
	"github.com/codecrafters-io/claude-code-tester/internal/settings_manager"
	"github.com/codecrafters-io/claude-code-tester/internal/test_cases"
	"github.com/codecrafters-io/claude-code-tester/internal/utils"
	"github.com/codecrafters-io/claude-code-tester/internal/workspace_manager"
	"github.com/codecrafters-io/claude-code-tester/proxy_server"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testAdvertiseReadTool(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	workspace_manager.BootstrapExecutableWorkspace(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 30 * 1000

	prompt := utils.GetPromptWithGuardRailPrompt(
		[]string{
			"What is the length of the tools array available to you in this request?",
			"How many tools are available to you in this request?",
			"Count the number of tools available to you for this request.",
			"Give the number of tools accessible in this request.",
		},
		"Number only.",
	)

	promptTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt:      prompt,
		ExpectedExitCode: 0,
		StdoutAssertion: string_assertion.MinimumValueAssertion{
			ExpectedMinimumValue: 1,
		},
	}

	return promptTestCase.Run(stageHarness)
}
