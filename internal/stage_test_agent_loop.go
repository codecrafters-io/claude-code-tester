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

func testAgentLoop(stageHarness *test_case_harness.TestCaseHarness) error {
	proxy_server.StartProxyServer(stageHarness)
	settings_manager.InitializeBypassPermissionSettings(stageHarness)
	stageHarness.Executable.TimeoutInMilliseconds = 30 * 1000

	workspaceManager := workspace_manager.NewWorkspaceManager()
	workspaceManager.BootstrapExecutableWorkspace(stageHarness)

	mainFileName := random.RandomElementFromArray([]string{"main.py", "init.py", "start.py"})
	extraFileNameWithoutExtension := random.RandomElementFromArray([]string{"chemical", "substance", "expiry", "duration"})
	extraFileName := extraFileNameWithoutExtension + ".py"
	readmeFileName := "README.md"

	mainFileContent := fmt.Sprintf(`from %s import chemical_expiry_period

	def main():
		print(f"Chemical expiry period: {chemical_expiry_period} months")
	
	if __name__ == "__main__":
		main()
`, extraFileNameWithoutExtension)

	chemicalExpiryPeriod := random.RandomInt(6, 36)

	readmeContent := fmt.Sprintf(`This is a simple python project.
- The starting point of this project is app/%s.
- The file app/%s contains chemical properties.`, mainFileName, extraFileName)

	workspaceManager.MustCreateFilesWithLogger([]workspace_manager.WorkspaceFile{
		{
			RelativePath: readmeFileName,
			Content:      readmeContent,
			FileMode:     0644,
		},
		{

			RelativePath: fmt.Sprintf("app/%s", extraFileName),
			Content:      fmt.Sprintf("chemical_expiry_period = %d  # months", chemicalExpiryPeriod),
			FileMode:     0644,
		},
		{
			RelativePath: fmt.Sprintf("app/%s", mainFileName),
			Content:      mainFileContent,
			FileMode:     0644,
		},
	}, stageHarness.Logger)

	prompt := utils.GetPromptWithGuardRailPrompt(
		[]string{
			"Use README.md to determine the chemical expiry period in months.",
			"Find the chemical expiry period in months from README.md.",
			"Determine in how many months the chemical expires by reading README.md.",
		},
		"Number only.",
	)

	promptTestCase := test_cases.NonInteractiveTestCase{
		InputPrompt:      prompt,
		ExpectedExitCode: 0,
		StdoutAssertion: string_assertion.ExactMatchAssertion{
			ExpectedValue: fmt.Sprintf("%d", chemicalExpiryPeriod),
		},
	}

	return promptTestCase.Run(stageHarness)
}
